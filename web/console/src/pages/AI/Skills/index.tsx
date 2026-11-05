import React from 'react';
import {
  Button,
  Empty,
  Input,
  Loading,
  Select,
  Space,
  Table,
  Tag,
} from 'components/Fluent';
import EditorDrawer, { EditorDrawerActions } from 'components/EditorDrawer';
import { useNavigate } from 'components/Router';
import { ResourceHeader, ResourceToolbar } from 'components/ResourceLayout';
import { useTranslation } from 'react-i18next';
import {
  MarketplaceReview,
  MarketplaceSkill,
  RegistrySource,
  GitImportResult,
  GitSkillDiscovery,
  SkillVisibility,
  createRegistrySource,
  decideMarketplaceReview,
  discoverMarketplaceSkillsFromGit,
  describeMarketplaceReviews,
  describeMarketplaceSkills,
  describeRegistrySources,
  importMarketplaceSkillsFromGit,
  syncRegistrySource,
  uploadMarketplaceSkillRelease,
} from 'services/skill_marketplace';
import { openErrNotification, openInfoNotification } from 'utils/notifition';
import style from './index.module.less';

type DrawerMode = 'upload' | 'git' | 'reviews' | 'sources' | null;

const visibilityOptions = [
  { label: '全部可见性', value: '' },
  { label: '公共', value: 'public' },
  { label: '私有', value: 'private' },
];

const statusOptions = [
  { label: '全部状态', value: '' },
  { label: '已发布', value: 'published' },
  { label: '待审核', value: 'pending_review' },
  { label: '已撤回', value: 'yanked' },
  { label: '已弃用', value: 'deprecated' },
];

const sourceOptions = [
  { label: '全部来源', value: '' },
  { label: 'Pole 本地', value: 'pole' },
  { label: 'Git', value: 'git' },
  { label: 'HTTP Registry', value: 'http_index' },
];

const statusTheme = (status?: string) => {
  if (status === 'published') return 'success';
  if (status === 'pending_review') return 'warning';
  if (status === 'rejected' || status === 'yanked') return 'danger';
  return 'default';
};

const statusLabel = (status?: string) => ({
  published: '已发布', pending_review: '待审核', rejected: '已拒绝', yanked: '已撤回', deprecated: '已弃用', draft: '草稿',
}[status || ''] || status || '未知');

const sourceLabel = (source?: string) => ({ pole: 'Pole 本地', git: 'Git', http_index: 'HTTP Registry' }[source || ''] || source || '未标注');

const initialUpload = { publisher: '', name: '', version: '', visibility: 'private' as SkillVisibility };
const initialGit = { repository_url: '', reference: '', root_path: '', version: '', publisher: '', visibility: 'private' as SkillVisibility };
const initialSource = { name: '', url: '', type: 'http_index', trust_level: 'untrusted', enabled: true };
const formatBytes = (value: number) => value < 1024 ? `${value} B` : value < 1024 * 1024 ? `${(value / 1024).toFixed(1)} KiB` : `${(value / 1024 / 1024).toFixed(1)} MiB`;
const SkillMarketplacePage: React.FC = () => {
  const navigate = useNavigate();
  const { t } = useTranslation();
  const marketplaceLabel = t('menu.ai.skills');
  const notifyError = (message: string) => openErrNotification(marketplaceLabel, message);
  const notifyInfo = (message: string) => openInfoNotification(marketplaceLabel, message);
  const [items, setItems] = React.useState<MarketplaceSkill[]>([]);
  const [total, setTotal] = React.useState(0);
  const [loading, setLoading] = React.useState(true);
  const [error, setError] = React.useState('');
  const [query, setQuery] = React.useState('');
  const [visibility, setVisibility] = React.useState('');
  const [status, setStatus] = React.useState('');
  const [source, setSource] = React.useState('');
  const [drawer, setDrawer] = React.useState<DrawerMode>(null);
  const [upload, setUpload] = React.useState(initialUpload);
  const [bundle, setBundle] = React.useState<File | null>(null);
  const [signature, setSignature] = React.useState<File | null>(null);
  const [gitImport, setGitImport] = React.useState(initialGit);
  const [gitDiscovery, setGitDiscovery] = React.useState<GitSkillDiscovery | null>(null);
  const [gitResult, setGitResult] = React.useState<GitImportResult | null>(null);
  const [reviews, setReviews] = React.useState<MarketplaceReview[]>([]);
  const [sources, setSources] = React.useState<RegistrySource[]>([]);
  const [sourceDraft, setSourceDraft] = React.useState(initialSource);
  const [submitting, setSubmitting] = React.useState(false);

  const refresh = React.useCallback(async () => {
    setLoading(true);
    setError('');
    try {
      const result = await describeMarketplaceSkills({ offset: 0, limit: 100, query, visibility, status, source });
      setItems(result.items);
      setTotal(result.total);
    } catch (reason) {
      setItems([]);
      setTotal(0);
      setError(reason instanceof Error ? reason.message : `${marketplaceLabel}尚未配置或目录查询失败。`);
    } finally {
      setLoading(false);
    }
  }, [marketplaceLabel, query, source, status, visibility]);

  React.useEffect(() => { void refresh(); }, [refresh]);

  const closeDrawer = () => {
    setDrawer(null);
    setSubmitting(false);
  };

  const updateGitImport = (patch: Partial<typeof initialGit>) => {
    setGitImport((current) => ({ ...current, ...patch }));
    setGitDiscovery(null);
    setGitResult(null);
  };

  const openReviews = async () => {
    setDrawer('reviews');
    try { setReviews(await describeMarketplaceReviews()); } catch { setReviews([]); }
  };
  const openSources = async () => {
    setDrawer('sources');
    try { setSources(await describeRegistrySources()); } catch { setSources([]); }
  };
  const submitUpload = async () => {
    if (!bundle || !upload.publisher || !upload.name || !upload.version) {
      notifyError('请填写发布身份、Skill 名称、SemVer，并选择 Bundle。');
      return;
    }
    if (upload.visibility === 'public' && !signature) {
      notifyError('公共 Release 必须提供 Ed25519 detached signature。');
      return;
    }
    setSubmitting(true);
    try {
      await uploadMarketplaceSkillRelease({ ...upload, bundle, signature: signature || undefined });
      notifyInfo('Bundle 已提交；公共发布将进入审核队列。');
      setUpload(initialUpload); setBundle(null); setSignature(null); closeDrawer(); void refresh();
    } catch (reason) { notifyError(reason instanceof Error ? reason.message : 'Bundle 上传失败'); setSubmitting(false); }
  };
  const discoverGitImport = async () => {
    if (!gitImport.repository_url || !gitImport.reference || !gitImport.publisher) {
      notifyError('请填写仓库地址、Tag/Release 和 Publisher。');
      return;
    }
    setSubmitting(true);
    try {
      const discovery = await discoverMarketplaceSkillsFromGit(gitImport);
      setGitDiscovery(discovery);
      setGitResult(null);
      notifyInfo(`已从 ${discovery.tag} 发现 ${discovery.items.length} 个 Skill，请确认后导入。`);
    } catch (reason) { notifyError(reason instanceof Error ? reason.message : 'Git Skill 发现失败'); }
    finally { setSubmitting(false); }
  };
  const submitGitImport = async () => {
    if (!gitDiscovery || gitResult) {
      await discoverGitImport();
      return;
    }
    setSubmitting(true);
    try {
      const result = await importMarketplaceSkillsFromGit({ ...gitImport, commit_sha: gitDiscovery.commit_sha });
      setGitResult(result);
      notifyInfo(`Git 导入完成：${result.succeeded} 成功，${result.skipped} 跳过，${result.failed} 失败。`);
      void refresh();
    } catch (reason) { notifyError(reason instanceof Error ? reason.message : 'Git 导入失败'); }
    finally { setSubmitting(false); }
  };
  const decideReview = async (review: MarketplaceReview, decision: 'approved' | 'rejected') => {
    try {
      await decideMarketplaceReview(review.id, decision);
      setReviews((current) => current.filter((item) => item.id !== review.id));
      notifyInfo(decision === 'approved' ? '审核已通过。' : '审核已拒绝。');
      void refresh();
    } catch (reason) { notifyError(reason instanceof Error ? reason.message : '审核操作失败'); }
  };
  const submitSource = async () => {
    if (!sourceDraft.name || !sourceDraft.url) { notifyError('请填写 Registry 名称和地址。'); return; }
    setSubmitting(true);
    try {
      await createRegistrySource(sourceDraft);
      setSourceDraft(initialSource);
      setSources(await describeRegistrySources());
      notifyInfo('Registry Source 已保存，后续将按周期同步。');
    } catch (reason) { notifyError(reason instanceof Error ? reason.message : 'Registry Source 保存失败'); }
    finally { setSubmitting(false); }
  };
  const triggerSync = async (registry: RegistrySource) => {
    try { await syncRegistrySource(registry.id); notifyInfo(`已请求同步 ${registry.name}`); setSources(await describeRegistrySources()); }
    catch (reason) { notifyError(reason instanceof Error ? reason.message : '同步请求失败'); }
  };

  return (
    <div className={style.page}>
      <ResourceHeader
        placement="app-header"
        eyebrow={`AI 工具 / ${marketplaceLabel}`}
        title={marketplaceLabel}
        description="发现、发布、审核并镜像遵循 Agent Skills 规范的不可变 Bundle。"
        actions={<Space><Button variant="outline" onClick={openSources}>Registry Source</Button><Button variant="outline" onClick={openReviews}>审核工作台</Button><Button variant="outline" onClick={() => setDrawer('git')}>Git 导入</Button><Button theme="primary" onClick={() => setDrawer('upload')}>上传 Bundle</Button></Space>}
      />

      <section className={style.intro} aria-label={`${marketplaceLabel}说明`}>
        <div><strong>可移植 Skill Bundle</strong><span>发布版本由 SemVer 与 SHA-256 固定；仅显示命令，不在 Console 执行或安装 Skill。</span></div>
        <code>pole-ai skill install publisher/name@version</code>
      </section>

      <ResourceToolbar
        title="Skill 目录"
        count={`${total} 个 Skill`}
        description="公共条目可供匿名读取；私有条目由资源授权控制。"
        filters={<div className={style.filters}>
          <Input aria-label="搜索 Skill" value={query} clearable placeholder="名称、发布者或标签" onChange={setQuery} onEnter={() => void refresh()} />
          <Select aria-label="来源筛选" options={sourceOptions} value={source} onChange={setSource} />
          <Select aria-label="可见性筛选" options={visibilityOptions} value={visibility} onChange={setVisibility} />
          <Select aria-label="状态筛选" options={statusOptions} value={status} onChange={setStatus} />
          <Button variant="outline" onClick={() => void refresh()}>刷新</Button>
        </div>}
      />

      {error && <div className={style.error} role="status"><strong>目录暂不可用</strong><span>{error}</span><Button size="small" variant="outline" onClick={() => void refresh()}>重试</Button></div>}
      <Loading loading={loading} text="加载 Skill 目录…">
        {items.length ? <Table
          ariaLabel={`${marketplaceLabel}目录`}
          className={style.table}
          data={items}
          rowKey={(row: MarketplaceSkill) => `${row.publisher}/${row.name}`}
          pagination={false}
          columns={[
            { colKey: 'skill', title: 'Skill', minWidth: 260, cell: ({ row }: any) => <Button variant="text" type="button" className={style.skillLink} onClick={() => navigate(`/ai/skills/${encodeURIComponent(row.publisher)}/${encodeURIComponent(row.name)}`)}><strong>{row.display_name || row.name}</strong><span>{row.publisher}/{row.name}</span><small>{row.description || '暂无描述'}</small></Button> },
            { colKey: 'version', title: '最新版本', width: 140, cell: ({ row }: any) => <code>{row.latest_version || row.latest_release?.version || '-'}</code> },
            { colKey: 'visibility', title: '可见性', width: 120, cell: ({ row }: any) => <Tag theme={row.visibility === 'public' ? 'success' : 'default'} variant="outline">{row.visibility === 'public' ? '公共' : '私有'}</Tag> },
            { colKey: 'source', title: '来源', width: 160, cell: ({ row }: any) => <div className={style.cellStack}><span>{sourceLabel(row.source)}</span><small title={row.source_url}>{row.source_url || '-'}</small></div> },
            { colKey: 'status', title: '状态', width: 140, cell: ({ row }: any) => <Tag theme={statusTheme(row.status || row.latest_release?.status)} variant="outline">{statusLabel(row.status || row.latest_release?.status)}</Tag> },
            { colKey: 'updated', title: '更新时间', width: 180, cell: ({ row }: any) => <span>{row.updated_at || row.latest_release?.published_at || '-'}</span> },
          ]}
        /> : !loading && <Empty title={error ? `等待 ${marketplaceLabel}服务可用` : '未找到匹配的 Skill'} description={error ? `完成控制面 ${marketplaceLabel}配置后，目录会自动显示。` : '可调整筛选条件，或上传符合 Agent Skills 规范的 Bundle。'} action={!error && <Button theme="primary" onClick={() => setDrawer('upload')}>上传 Bundle</Button>} />}
      </Loading>

      <EditorDrawer visible={drawer === 'upload'} width="wide" title="上传 Skill Bundle" description="发布版本不可修改；公共 Release 必须含有效 Ed25519 detached signature 并通过审核。" onClose={closeDrawer} footer={<EditorDrawerActions onCancel={closeDrawer} onSubmit={submitUpload} submitText="提交 Release" submitting={submitting} />}>
        <div className={style.drawerContent}>
          <div className={style.drawerForm}>
            <label><span>Publisher</span><Input value={upload.publisher} placeholder="例如 pole" onChange={(value) => setUpload({ ...upload, publisher: value })} /></label>
            <label><span>Skill 名称</span><Input value={upload.name} placeholder="例如 incident-response" onChange={(value) => setUpload({ ...upload, name: value })} /></label>
            <label><span>SemVer</span><Input value={upload.version} placeholder="例如 1.2.0" onChange={(value) => setUpload({ ...upload, version: value })} /></label>
            <label><span>可见性</span><Select label="可见性" options={visibilityOptions.slice(1)} value={upload.visibility} onChange={(value: SkillVisibility) => setUpload({ ...upload, visibility: value })} /></label>
            <label className={style.fileField}><span>压缩 Bundle</span><Input type="file" accept=".zip,.tgz,.tar.gz" onChange={(_value, context) => setBundle(context?.e.currentTarget.files?.[0] || null)} /><small>{bundle ? bundle.name : '最大 16 MiB；服务端会校验路径、解包大小和摘要。'}</small></label>
            <label className={style.fileField}><span>Ed25519 Detached Signature JSON envelope（公共必填）</span><Input type="file" accept="application/json,.json" onChange={(_value, context) => setSignature(context?.e.currentTarget.files?.[0] || null)} /><small>{signature ? signature.name : 'JSON 内容必须包含 algorithm=Ed25519、signedAt（RFC3339）和 signature（base64）；私有 Release 可省略。'}</small></label>
          </div>
        </div>
      </EditorDrawer>

      <EditorDrawer visible={drawer === 'git'} width="wide" title="从 Git 导入 Release" description="只接受明确的 Tag 或 Release；先解析到不可变 commit 并预览 Skill，再逐项冻结 Bundle 摘要。" onClose={closeDrawer} footer={<EditorDrawerActions onCancel={closeDrawer} onSubmit={submitGitImport} submitText={gitResult ? '重新扫描' : gitDiscovery ? `导入全部 ${gitDiscovery.items.length} 个 Skill` : '扫描 Skill'} submitting={submitting} />}>
        <div className={style.drawerContent}>
          <div className={style.drawerForm}>
            <label><span>仓库地址</span><Input value={gitImport.repository_url} placeholder="https://github.com/org/repo" onChange={(value) => updateGitImport({ repository_url: value })} /></label>
            <label><span>Tag / Release</span><Input value={gitImport.reference} placeholder="v1.2.0" onChange={(value) => updateGitImport({ reference: value })} /></label>
            <label><span>Skill 根目录</span><Input value={gitImport.root_path} placeholder="留空表示仓库根目录；多 Skill 如 skills" onChange={(value) => updateGitImport({ root_path: value })} /></label>
            <label><span>SemVer 覆盖（可选）</span><Input value={gitImport.version} placeholder="Tag 非 SemVer 时必填，如 1.2.0" onChange={(value) => updateGitImport({ version: value })} /></label>
            <label><span>Publisher</span><Input value={gitImport.publisher} placeholder="所有发现的 Skill 归属此 Publisher" onChange={(value) => updateGitImport({ publisher: value })} /></label>
            <label><span>可见性</span><Select label="可见性" options={[{ label: '私有', value: 'private' }]} value={gitImport.visibility} onChange={(value: SkillVisibility) => updateGitImport({ visibility: value })} /></label>
          </div>
          {gitDiscovery && <section className={style.gitPreview} aria-label="Git Skill 发现预览">
            <header><div><strong>{gitResult ? '导入结果' : `已发现 ${gitDiscovery.items.length} 个 Skill`}</strong><span>{gitDiscovery.tag} · {gitDiscovery.version} · commit <code>{gitDiscovery.commit_sha.slice(0, 12)}</code></span></div>{gitResult && <Tag theme={gitResult.failed ? 'warning' : 'success'} variant="outline">{gitResult.succeeded} 成功 / {gitResult.skipped} 跳过 / {gitResult.failed} 失败</Tag>}</header>
            <div className={style.gitSkillList}>{(gitResult?.items || gitDiscovery.items).map((item) => <article className={style.gitSkillRow} key={`${item.path}/${item.name}`}>
              <div><strong>{item.name || '无效 Skill'}</strong><span>{item.path || '仓库根目录'} · {formatBytes(item.size || 0)} · {item.entries || 0} 个文件</span><small title={item.digest}>{item.digest ? `SHA-256 ${item.digest.slice(0, 16)}…` : '未生成摘要'}{item.error ? ` · ${item.error}` : ''}</small></div>
              {'status' in item ? <Tag theme={item.status === 'succeeded' ? 'success' : item.status === 'failed' ? 'danger' : 'warning'} variant="outline">{item.status === 'succeeded' ? '成功' : item.status === 'failed' ? '失败' : '跳过'}</Tag> : item.error && <Tag theme="danger" variant="outline">无法导入</Tag>}
            </article>)}</div>
          </section>}
        </div>
      </EditorDrawer>

      <EditorDrawer visible={drawer === 'reviews'} width="workspace" title="公共 Release 审核工作台" description="审核决定只切换 Release 状态；已发布 Bundle、版本与摘要始终不可编辑。" onClose={closeDrawer} footer={false}>
        <div className={style.drawerContent}>
          <div className={style.workspaceList}>{reviews.length ? reviews.map((review) => <article className={style.reviewRow} key={review.id}><div><strong>{review.publisher}/{review.name}@{review.version}</strong><span>签名：{review.signature_status || '待验证'} · 扫描：{review.scan_status || '待验证'} · 请求于 {review.requested_at || '-'}</span></div><Space><Button size="small" variant="outline" onClick={() => void decideReview(review, 'rejected')}>拒绝</Button><Button size="small" theme="primary" onClick={() => void decideReview(review, 'approved')}>通过</Button></Space></article>) : <Empty title="没有待审核的公共 Release" description="受信 Publisher 可在通过自动扫描后直接发布。" />}</div>
        </div>
      </EditorDrawer>

      <EditorDrawer visible={drawer === 'sources'} width="workspace" title="Registry Source 管理" description="定期同步目录；镜像的精确 Release 与摘要不会被上游同版本替换静默覆盖。" onClose={closeDrawer} footer={false}>
        <div className={style.drawerContent}>
          <div className={style.sourcesWorkspace}>
            <section className={style.sourceCreate}><h3>新增来源</h3><label><span>名称</span><Input value={sourceDraft.name} onChange={(value) => setSourceDraft({ ...sourceDraft, name: value })} /></label><label><span>地址</span><Input value={sourceDraft.url} placeholder="https://registry.example.com" onChange={(value) => setSourceDraft({ ...sourceDraft, url: value })} /></label><label><span>类型</span><Select label="类型" options={sourceOptions.slice(1)} value={sourceDraft.type} onChange={(value: string) => setSourceDraft({ ...sourceDraft, type: value })} /></label><label><span>信任级别</span><Select label="信任级别" options={[{ label: '不受信（隔离）', value: 'untrusted' }, { label: '受信', value: 'trusted' }]} value={sourceDraft.trust_level} onChange={(value: string) => setSourceDraft({ ...sourceDraft, trust_level: value })} /></label><Button theme="primary" loading={submitting} onClick={() => void submitSource()}>保存 Source</Button></section>
            <section className={style.sourceList}><h3>已配置来源</h3>{sources.length ? sources.map((registry) => <article className={style.sourceRow} key={registry.id}><div><strong>{registry.name}</strong><span>{registry.type} · {registry.url}</span><small>信任：{registry.trust_level} · 上次同步：{registry.last_synced_at || '从未'}{registry.error_message ? ` · ${registry.error_message}` : ''}</small></div><Button size="small" variant="outline" onClick={() => void triggerSync(registry)}>立即同步</Button></article>) : <Empty title="尚未配置外部 Registry" description="可添加 Pole 原生、Git 或 HTTP Registry Source。" />}</section>
          </div>
        </div>
      </EditorDrawer>
    </div>
  );
};

export default SkillMarketplacePage;

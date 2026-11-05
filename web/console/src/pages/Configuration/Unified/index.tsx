import React from 'react';
import { Button, Dialog } from 'components/Fluent';
import { useNavigate, useSearchParams } from 'components/Router';
import { ResourceHeader } from 'components/ResourceLayout';
import { describeBusinessNamespaces } from 'services/namespace';
import { ConfigDefinition, ReleaseKind, parseConfigDefinition } from 'services/unified_config';
import FileList from './FileList';
import ContentEditor from './ContentEditor';
import ValuesEditor from './ValuesEditor';
import PreviewDialog from './PreviewDialog';
import PublishDrawer from './PublishDrawer';
import ReleaseHistory from './ReleaseHistory';
import useConfigurationSession, { sharedDefinition } from './useConfigurationSession';
import style from './workspace.module.less';

const rawDrafts = new Map<string, string>();

export default function UnifiedConfiguration() {
  const navigate = useNavigate(); const [params] = useSearchParams();
  const environment = params.get('environment') || ''; const group = params.get('group') || ''; const name = params.get('name') || '';
  const key = group && name ? { group, name } : undefined;
  const session = useConfigurationSession(key, environment);
  const [environments, setEnvironments] = React.useState<string[]>([]); const [tab, setTab] = React.useState('配置内容'); const [preview, setPreview] = React.useState(false); const [confirmSave, setConfirmSave] = React.useState(false); const [publish, setPublish] = React.useState(false); const [kind, setKind] = React.useState<ReleaseKind>('RELEASE_KIND_NORMAL'); const [raw, setRaw] = React.useState('');
  const rawKey = JSON.stringify([localStorage.getItem('login-user-id'), group, name, environment]);
  const rawDirty = !!session.definition && raw !== JSON.stringify(session.definition, null, 2);
  const dirty = session.dirty || rawDirty;
  React.useEffect(() => { void describeBusinessNamespaces().then(items => setEnvironments(items.map(x => x.name))).catch(e => session.setError(`加载环境失败：${String(e)}`)); }, []);
  React.useEffect(() => {
    if (!rawDirty) return;
    const handler = (event: BeforeUnloadEvent) => { event.preventDefault(); event.returnValue = ''; };
    window.addEventListener('beforeunload', handler);
    return () => window.removeEventListener('beforeunload', handler);
  }, [rawDirty]);
  React.useEffect(() => {
    if (!dirty && !session.busy) return;
    const intercept = (event: MouseEvent) => {
      const target = event.target as Element | null;
      const anchor = target?.closest('a[href]') as HTMLAnchorElement | null;
      const navItem = target?.closest('.fui-NavItem, .fui-NavSubItem');
      if (navItem && !anchor) { if (session.busy || !window.confirm('更改会保留在当前会话，是否离开？')) { event.preventDefault(); event.stopImmediatePropagation(); } return; }
      if (!anchor || anchor.origin !== window.location.origin || anchor.target === '_blank' || event.metaKey || event.ctrlKey || event.shiftKey || event.altKey) return;
      if (anchor.pathname === window.location.pathname && anchor.search === window.location.search) return;
      if (session.busy || !window.confirm('更改会保留在当前会话，是否离开？')) { event.preventDefault(); event.stopImmediatePropagation(); }
    };
    document.addEventListener('click', intercept, true);
    return () => document.removeEventListener('click', intercept, true);
  }, [dirty, session.busy]);
  React.useEffect(() => { if (session.definition) setRaw(rawDrafts.get(rawKey) ?? JSON.stringify(session.definition, null, 2)); }, [session.definition, rawKey]);
  const go = (url: string) => { if (dirty && !window.confirm('更改会保留在当前会话，是否离开？')) return; navigate(url); };
  const update = (definition: ConfigDefinition) => session.setDefinition(definition);
  const configKey = { environment, group, name };
  return <main className={style.page}><ResourceHeader eyebrow="配置中心" title="配置文件" density="compact" placement="app-header" />{session.restored && <p role="status">已恢复当前会话中未保存的草稿；保存时会检查原始修订。</p>}{session.error && <p role="alert">{session.error}</p>}{session.results.length > 0 && <section className={style.saveResults} aria-label="保存结果"><strong>环境保存结果</strong>{session.results.map(result => <p key={result.environment}>{result.environment}：{result.ok ? '已保存' : `失败：${result.message}`}</p>)}</section>}{!key ? <FileList environments={environments} onOpen={(env, g, n) => go(`/configuration/files?${new URLSearchParams({ environment: env, group: g, name: n })}`)} /> : <><div className={style.toolbar}><Button variant="outline" disabled={session.busy} onClick={() => go('/configuration/files')}>返回列表</Button><h2>{group} / {name}</h2><label>编辑环境 <select disabled={session.busy} value={environment} onChange={e => { navigate(`/configuration/files?${new URLSearchParams({ environment: e.target.value, group, name })}`); }}>{(session.template?.environments || environments).map(e => <option key={e}>{e}</option>)}</select></label>{dirty && <span>未保存</span>}<Button disabled={session.busy || !session.definition || (!session.dirty || rawDirty)} onClick={() => { const changesShared = session.template && session.definition && JSON.stringify(sharedDefinition(session.template.definition)) !== JSON.stringify(sharedDefinition(session.definition)); if (changesShared) setConfirmSave(true); else void session.save(); }}>保存草稿</Button><Button variant="outline" disabled={session.busy || !session.definition} onClick={() => setPreview(true)}>取值预览</Button><Button disabled={session.busy || dirty || !session.state?.draft} onClick={() => { setKind('RELEASE_KIND_NORMAL'); setPublish(true); }}>发布配置</Button></div>{session.busy && <p role="status">正在加载或保存配置…</p>}{session.definition && session.state && <><div ref={element => { element?.toggleAttribute('inert', session.busy); }} aria-busy={session.busy}><nav className={style.tabs} aria-label="配置详情">{['配置内容', '变量与取值', '发布记录', '接入方式'].map(t => <button key={t} aria-current={tab === t ? 'page' : undefined} onClick={() => setTab(t)}>{t}</button>)}</nav>{tab === '配置内容' && <ContentEditor definition={session.definition} name={name} modelPath={`inmemory://config/${encodeURIComponent(group)}/${encodeURIComponent(name)}/${encodeURIComponent(environment)}`} onChange={update} />}{tab === '变量与取值' && <ValuesEditor definition={session.definition} onChange={update} />}{tab === '发布记录' && <><div><Button disabled={dirty || session.busy} onClick={() => { setKind('RELEASE_KIND_NORMAL'); setPublish(true); }}>正式发布</Button><Button disabled={dirty || session.busy} onClick={() => { setKind('RELEASE_KIND_GRAY'); setPublish(true); }}>新增灰度版本</Button></div><ReleaseHistory configKey={configKey} state={session.state} draft={session.definition} disabled={dirty || session.busy} onReload={session.refresh} /></>}{tab === '接入方式' && <section><h3>客户端读取</h3><p>通过配置 Discover 接口按环境、分组、文件名及客户端标签读取配置；服务端依据请求标签选择发布版本和取值。</p><p>当前没有客户端读取或订阅记录的管理接口。</p></section>}<details><summary>高级：完整 Definition JSON</summary><textarea className={style.raw} value={raw} onChange={e => { setRaw(e.target.value); if (localStorage.getItem('login-user-id')) rawDrafts.set(rawKey, e.target.value); }} /><Button variant="outline" onClick={() => { try { const parsed = parseConfigDefinition(raw); rawDrafts.delete(rawKey); update(parsed); } catch (e) { session.setError(String(e)); } }}>应用 JSON</Button></details></div></>}{key && <><Dialog visible={confirmSave} header="确认保存共享更改" confirmBtn="确认保存" onClose={() => setConfirmSave(false)} onConfirm={() => { setConfirmSave(false); void session.save(); }}><p>共享正文或变量声明将应用到以下环境的草稿：{session.template?.environments.join('、')}。</p><p>各环境取值将保留。</p></Dialog>{session.definition && <PreviewDialog open={preview} onClose={() => setPreview(false)} configKey={configKey} definition={session.definition} />}<PublishDrawer open={publish} onClose={() => setPublish(false)} fileKey={key} environments={session.template?.environments || []} initialEnvironment={environment} initialKind={kind} onPublished={session.refresh} /></>}</>}</main>;
}

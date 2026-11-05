import React from 'react';
import { Button, Dialog } from 'components/Fluent';
import { ConfigDefinition, ConfigKey, ConfigState, getConfigState, copyConfigReleaseToDraft, rollbackConfig, stopConfigGray, stopAllConfigGray, offlineConfig, promoteConfigGray } from 'services/unified_config';
import DefinitionView from './DefinitionView';
import { audienceConditions } from './audience';
import style from './workspace.module.less';

const matchLabels: Record<string, string> = { MATCH_TYPE_EXACT: '等于', MATCH_TYPE_REGEX: '正则匹配', MATCH_TYPE_NOT_EQUALS: '不等于', MATCH_TYPE_IN: '属于', MATCH_TYPE_NOT_IN: '不属于', MATCH_TYPE_RANGE: '范围' };
const formatAudience = (audience: unknown) => {
  try { return audienceConditions(audience).map(c => `${c.key} ${matchLabels[c.type] || c.type} ${c.value}`).join(' 且 ') || '无条件'; }
  catch { return `条件数据无法解析：${JSON.stringify(audience)}`; }
};
const formatTime = (value?: string) => { if (!value) return '—'; const date = new Date(value); return Number.isNaN(date.getTime()) ? value : date.toLocaleString('zh-CN', { hour12: false }); };

// Compare protobuf defaults explicitly: protojson omits empty strings, arrays and false booleans.
export function sameSharedDefinition(a: ConfigDefinition, b: ConfigDefinition): boolean {
  const shared = (d: ConfigDefinition) => ({
    content: d.content || '', engine: d.engine, format: d.format || '',
    parameters: (d.parameters || []).map(p => ({ name: p.name, type: p.type, required: !!p.required })),
    compatibility: { apollo: !!d.compatibility?.apollo, nacos: !!d.compatibility?.nacos, consul: !!d.compatibility?.consul },
  });
  return JSON.stringify(shared(a)) === JSON.stringify(shared(b));
}

export async function activeNormalReleaseId(key: ConfigKey, current: ConfigState): Promise<string> {
  const ids = current.activeReleaseIds || [];
  const releases = await Promise.all(ids.map(async id => {
    const snapshot = await getConfigState(key, '', id);
    if (snapshot.releaseSetRevision !== current.releaseSetRevision || !snapshot.selectedRelease ||
        snapshot.selectedRelease.id !== id || !snapshot.activeReleaseIds?.includes(id)) {
      throw new Error('发布集合已变化，请重新加载');
    }
    return snapshot.selectedRelease;
  }));
  const normals = releases.filter(release => release.kind === 'RELEASE_KIND_NORMAL');
  if (normals.length > 1) throw new Error('存在多个生效正式版本，请重新加载');
  return normals[0]?.id || '';
}

export async function copyRelease(key: ConfigKey, id: string, draft: ConfigDefinition, state: ConfigState) {
    const snapshot = await getConfigState(key, '', id);
    if (!snapshot.selectedRelease?.definition) throw new Error('无法读取版本内容');
    if (!sameSharedDefinition(snapshot.selectedRelease.definition, draft)) {
      throw new Error('该版本的共享正文或变量声明与当前模板不同。请先查看版本内容并在配置内容中更新共享模板，再复制环境取值。');
    }
    return copyConfigReleaseToDraft(key, id, state.draft?.revision || '0', state.releaseSetRevision);
}

export default function ReleaseHistory({ configKey, state, draft, onReload, disabled }: { configKey: ConfigKey; state: ConfigState; draft: ConfigDefinition; onReload: () => void; disabled: boolean }) {
  const [compare, setCompare] = React.useState(false); const [selected, setSelected] = React.useState<ConfigState['selectedRelease']>(); const [error, setError] = React.useState(''); const [pages, setPages] = React.useState(['']); const [page, setPage] = React.useState(0); const [data, setData] = React.useState(state);
  React.useEffect(() => { setData(state); setPages(['']); setPage(0); setSelected(undefined); }, [state, configKey.environment]);
  const [pending, setPending] = React.useState<{ label: string; fn: () => Promise<ConfigState> }>();
  const [running, setRunning] = React.useState(false);
  const run = (label: string, fn: () => Promise<ConfigState>) => { if (!running) setPending({ label, fn }); };
  const confirmAction = async () => {
    if (!pending || running) return;
    setRunning(true);
    try { await pending.fn(); setError(''); setPending(undefined); setSelected(undefined); onReload(); }
    catch (e) { setPending(undefined); setError(`${pending.label}失败：${e instanceof Error ? e.message : String(e)}`); }
    finally { setRunning(false); }
  };
  return <section><h2>发布记录</h2>{error && <p role="alert">{error}</p>}<Button variant="outline" disabled={disabled} onClick={() => void run('停止全部灰度', () => stopAllConfigGray(configKey, data.releaseSetRevision))}>停止全部灰度</Button><Button variant="outline" disabled={disabled} onClick={() => void run('下线', () => offlineConfig(configKey, data.releaseSetRevision))}>下线</Button><table className={style.compactTable}><thead><tr><th>版本</th><th>类型</th><th>状态</th><th>条件与优先级</th><th>时间</th><th>操作</th></tr></thead><tbody>{(data.releases || []).map(r => <tr key={r.id}><td title={`版本 ID：${r.id}`}>{r.sequence}</td><td>{r.kind === 'RELEASE_KIND_GRAY' ? '灰度' : '正式'}</td><td>{r.active ? '生效中' : '已停止'}</td><td>{r.kind === 'RELEASE_KIND_GRAY' ? <>{formatAudience(r.audience)}<br />优先级：{r.priority || 0}</> : '—'}</td><td><time dateTime={r.createdAt} title={r.createdAt}>{formatTime(r.createdAt)}</time></td><td><Button variant="outline" onClick={() => { void getConfigState(configKey, pages[page], r.id).then(x => setSelected(x.selectedRelease)).catch(e => setError(String(e))); }}>查看</Button><Button variant="outline" disabled={disabled} onClick={() => void run('复制到草稿', () => copyRelease(configKey, r.id, draft, data))}>复制到草稿</Button>{r.kind === 'RELEASE_KIND_NORMAL' && <Button variant="outline" disabled={disabled} onClick={() => void run('回滚', async () => rollbackConfig(configKey, r.id, await activeNormalReleaseId(configKey, data), data.releaseSetRevision))}>回滚</Button>}{r.kind === 'RELEASE_KIND_GRAY' && r.active && <Button variant="outline" disabled={disabled} onClick={() => void getConfigState(configKey, pages[page], r.id).then(x => { setSelected(x.selectedRelease); setCompare(false); }).catch(e => setError(String(e)))}>审阅并转正式</Button>}{r.kind === 'RELEASE_KIND_GRAY' && r.active && <Button variant="outline" disabled={disabled} onClick={() => void run('停止灰度', () => stopConfigGray(configKey, data.releaseSetRevision, r.id))}>停止灰度</Button>}</td></tr>)}</tbody></table><Button variant="outline" disabled={!page} onClick={() => { void getConfigState(configKey, pages[page - 1]).then(x => { setData(x); setPage(page - 1); }); }}>上一页</Button><Button variant="outline" disabled={!data.nextPageToken} onClick={() => { const token = data.nextPageToken || ''; void getConfigState(configKey, token).then(x => { setData(x); setPages([...pages.slice(0, page + 1), token]); setPage(page + 1); }); }}>下一页</Button><Dialog visible={!!selected && !pending} header="版本详情" onClose={() => setSelected(undefined)} onConfirm={() => setSelected(undefined)}>{selected && <>{selected.kind === 'RELEASE_KIND_GRAY' && data.activeReleaseIds?.includes(selected.id) && <Button variant="outline" disabled={disabled} onClick={() => void run(`将灰度版本 ${selected.sequence} 转正式（保留草稿及其他灰度）`, () => promoteConfigGray(configKey, selected.id, data.releaseSetRevision))}>转为正式</Button>}<div><Button variant="outline" onClick={() => setCompare(!compare)}>{compare ? '查看版本' : '与当前草稿对比'}</Button></div>{compare ? <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(260px, 1fr))', gap: 16 }}><section><h3>历史版本</h3><DefinitionView definition={selected.definition} name={configKey.name} /></section><section><h3>当前草稿</h3><DefinitionView definition={draft} name={configKey.name} /></section></div> : <DefinitionView definition={selected.definition} name={configKey.name} />}</>}</Dialog><Dialog visible={!!pending} header={pending ? `确认${pending.label}` : '确认操作'} onClose={() => { if (!running) setPending(undefined); }} footer={<><Button variant="outline" disabled={running} onClick={() => setPending(undefined)}>取消</Button><Button disabled={running} onClick={() => { void confirmAction(); }}>{running ? '处理中…' : '确认'}</Button></>}>确认{pending?.label}？</Dialog></section>;
}

import React from 'react';
import { Button, Checkbox, Dialog, DialogActions, DialogBody, DialogContent, DialogSurface, DialogTitle, Field, Input, Select, Textarea } from '@fluentui/react-components';
import style from './workspace.module.less';
import { ConfigFileSummary, listConfigFiles, saveConfigFileTemplate, saveConfigDraft } from 'services/unified_config';
import { fileFormat } from './ContentEditor';

export default function FileList({ environments, onOpen }: { environments: string[]; onOpen: (environment: string, group: string, name: string) => void }) {
  const [environment, setEnvironment] = React.useState(''); const [group, setGroup] = React.useState(''); const [query, setQuery] = React.useState('');
  const [items, setItems] = React.useState<ConfigFileSummary[]>([]); const [tokens, setTokens] = React.useState(['']); const [page, setPage] = React.useState(0); const [next, setNext] = React.useState(''); const [error, setError] = React.useState(''); const [loading, setLoading] = React.useState(false);
  const [create, setCreate] = React.useState(false); const [createGroup, setCreateGroup] = React.useState('DEFAULT_GROUP'); const [name, setName] = React.useState(''); const [description, setDescription] = React.useState(''); const [selected, setSelected] = React.useState<string[]>([]); const [busy, setBusy] = React.useState(false); const [createError, setCreateError] = React.useState(''); const [created, setCreated] = React.useState<{ group: string; name: string; environment: string }>();
  React.useEffect(() => { if (!environment && environments.length) setEnvironment(environments[0]); }, [environment, environments]);
  React.useEffect(() => { if (!environment) return; let active = true; setError(''); setLoading(true); listConfigFiles(environment, group, query, tokens[page]).then(data => { if (active) { setItems(data.items || []); setNext(data.nextPageToken || ''); } }).catch(e => { if (active) setError(e instanceof Error ? e.message : String(e)); }).finally(() => { if (active) setLoading(false); }); return () => { active = false; }; }, [environment, group, query, page, tokens]);
  const openCreate = () => { setCreateGroup('DEFAULT_GROUP'); setName(''); setDescription(''); setSelected(environment ? [environment] : []); setCreated(undefined); setCreateError(''); setCreate(true); };
  const createFile = async () => {
    if (!name.trim() || !createGroup.trim() || !selected.length) { setCreateError('请填写分组、文件名并选择至少一个环境'); return; }
    setBusy(true); setCreateError('');
    try {
      const key = { group: createGroup.trim(), name: name.trim() }; const definition = { content: '', engine: 'literal' as const, format: fileFormat(name), parameters: [], conditionGroups: [], parameterBindings: [] };
      await saveConfigFileTemplate({ key, definition, environments: selected, revision: '0', description }, '0');
      setCreated({ ...key, environment: selected[0] });
      const failures: string[] = [];
      for (const env of selected) { try { await saveConfigDraft({ ...key, environment: env }, definition, '0'); } catch (e) { failures.push(`${env}: ${e instanceof Error ? e.message : String(e)}`); } }
      if (failures.length) { setCreateError(`共享模板已创建，以下环境草稿创建失败：${failures.join('；')}。可进入详情继续处理。`); return; }
      onOpen(selected[0], key.group, key.name);
    } catch (e) { setCreateError(`创建失败：${e instanceof Error ? e.message : String(e)}`); }
    finally { setBusy(false); }
  };
  return <section><h2>配置文件</h2><div className={style.listToolbar}><Field label="环境"><Select value={environment} onChange={e => { setEnvironment(e.target.value); setTokens(['']); setPage(0); }}>{environments.map(e => <option key={e}>{e}</option>)}</Select></Field><Field label="分组"><Input value={group} onChange={(_, data) => { setGroup(data.value); setTokens(['']); setPage(0); }} /></Field><Field label="搜索"><Input value={query} onChange={(_, data) => { setQuery(data.value); setTokens(['']); setPage(0); }} /></Field><Button appearance="primary" onClick={openCreate}>新建配置</Button></div>
    {error && <p role="alert">{error}</p>}{loading ? <p role="status">正在加载配置文件…</p> : <><table><thead><tr><th>文件名</th><th>分组</th><th>描述</th><th>状态</th></tr></thead><tbody>{items.map(item => <tr key={`${item.key.group}/${item.key.name}`}><td><button onClick={() => onOpen(environment, item.key.group, item.key.name)}>{item.key.name}</button></td><td>{item.key.group}</td><td>{item.description}</td><td>{item.offline ? '已下线' : item.hasActiveRelease ? '已发布' : item.hasDraft ? '草稿' : '未创建'}</td></tr>)}</tbody></table>{!items.length && !error && <p className={style.empty}>当前环境没有符合条件的配置文件。</p>}<Button appearance="outline" disabled={!page} onClick={() => setPage(page - 1)}>上一页</Button><Button appearance="outline" disabled={!next} onClick={() => { setTokens([...tokens.slice(0, page + 1), next]); setPage(page + 1); }}>下一页</Button></>}
    <Dialog open={create} onOpenChange={(_, data) => { if (!busy) setCreate(data.open); }}><DialogSurface className={style.dialog}><DialogBody><DialogTitle>新建配置文件</DialogTitle><DialogContent className={style.createForm}><Field label="分组" required><Input disabled={busy || !!created} value={createGroup} onChange={(_, data) => setCreateGroup(data.value)} /></Field><Field label="文件名" required><Input disabled={busy || !!created} value={name} onChange={(_, data) => setName(data.value)} /></Field><Field label="描述"><Textarea disabled={busy || !!created} value={description} onChange={(_, data) => setDescription(data.value)} /></Field><fieldset disabled={busy || !!created}><legend>绑定环境</legend>{environments.map(env => <Checkbox key={env} label={env} checked={selected.includes(env)} onChange={(_, data) => setSelected(data.checked === true ? [...selected, env] : selected.filter(x => x !== env))} />)}</fieldset>{createError && <p role="alert" className={style.error}>{createError}</p>}</DialogContent><DialogActions><Button disabled={busy} onClick={() => setCreate(false)}>关闭</Button>{created ? <Button appearance="primary" disabled={busy} onClick={() => onOpen(created.environment, created.group, created.name)}>进入详情</Button> : <Button appearance="primary" disabled={busy} onClick={() => void createFile()}>创建</Button>}</DialogActions></DialogBody></DialogSurface></Dialog>
  </section>;
}

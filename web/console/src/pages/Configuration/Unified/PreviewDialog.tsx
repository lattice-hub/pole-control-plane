import React from 'react';
import { Button, Dialog, DialogActions, DialogBody, DialogContent, DialogSurface, DialogTitle, Field, Tab, TabList, Textarea } from '@fluentui/react-components';
import { ConfigDefinition, ConfigKey, previewConfig } from 'services/unified_config';
import { renderConfigSnapshot } from 'services/unified_config_renderer';
import style from './workspace.module.less';

export default function PreviewDialog({ open, onClose, configKey, definition }: { open: boolean; onClose: () => void; configKey: ConfigKey; definition: ConfigDefinition }) {
  const [labels, setLabels] = React.useState('{}'); const [selected, setSelected] = React.useState<string[]>([]);
  const [result, setResult] = React.useState<{ text: string; values: Record<string, string>; missing: string[] }>(); const [error, setError] = React.useState(''); const [tab, setTab] = React.useState('result'); const generation = React.useRef(0);
  React.useEffect(() => { generation.current++; setResult(undefined); }, [definition, labels, selected, configKey.environment]);
  React.useEffect(() => { setSelected([]); setLabels('{}'); setResult(undefined); setError(''); }, [configKey.group, configKey.name, configKey.environment]);
  const preview = async () => { const current = ++generation.current; setError(''); setResult(undefined); try {
    const parsed: unknown = JSON.parse(labels); if (!parsed || Array.isArray(parsed) || typeof parsed !== 'object' || Object.values(parsed).some(v => typeof v !== 'string')) throw new Error('标签必须是字符串到字符串的 JSON 对象');
    const merged: Record<string, string> = Object.assign(Object.create(null), parsed);
    for (const group of definition.conditionGroups || []) if (selected.includes(group.id)) {
      const pairs = [...Object.entries(group.audience.labels || {}), ...(group.audience.conditions || []).map(condition => { if (condition.type !== 'MATCH_TYPE_EXACT') throw new Error(`条件组 ${group.name} 包含非精确条件，请手动填写标签`); return [condition.key, condition.value] as [string, string]; })];
      for (const [key, value] of pairs) { if (Object.hasOwn(merged, key) && merged[key] !== value) throw new Error(`标签 ${key} 的值冲突，请调整所选条件组或自定义标签`); merged[key] = value; }
    }
    const snapshot = await previewConfig(configKey, definition, merged); if (current !== generation.current) return;
    const values = Object.fromEntries(Object.entries(snapshot.values || {}).map(([key, value]) => [key, value.value ?? '']));
    const missing = (definition.parameters || []).filter(p => !Object.hasOwn(values, p.name)).map(p => p.name);
    try { const normalized = { ...snapshot, values: Object.fromEntries(Object.entries(snapshot.values || {}).map(([key, value]) => [key, { ...value, value: value.value ?? '' }])) }; const rendered = renderConfigSnapshot(normalized); setResult({ text: rendered.text ?? `二进制正文：${rendered.bytes.length} 字节`, values, missing }); }
    catch (e) { setResult({ text: `渲染失败：${e instanceof Error ? e.message : String(e)}`, values, missing }); }
  } catch (e) { if (current === generation.current) setError(e instanceof Error ? e.message : String(e)); } };
  return <Dialog open={open} onOpenChange={(_, data) => { if (!data.open) onClose(); }}><DialogSurface className={style.dialog}><DialogBody><DialogTitle>取值预览</DialogTitle><DialogContent><p>使用当前未保存内容预览；标签只用于本次请求。</p><Field label="选择条件组"><div className={style.previewGroups}>{(definition.conditionGroups || []).map(group => <label key={group.id}><input type="checkbox" checked={selected.includes(group.id)} onChange={e => setSelected(e.target.checked ? [...selected, group.id] : selected.filter(id => id !== group.id))} />{group.name}</label>)}{!definition.conditionGroups?.length && <span>暂无条件组</span>}</div></Field><p className={style.muted}>精确条件可生成标签；其他匹配方式请手动输入满足条件的标签。</p><Field label="自定义客户端标签 JSON"><Textarea value={labels} onChange={(_, data) => setLabels(data.value)} /></Field><Button appearance="primary" onClick={() => void preview()}>渲染预览</Button>{error && <p className={style.error} role="alert">{error}</p>}{result && <><TabList selectedValue={tab} onTabSelect={(_, data) => setTab(String(data.value))}><Tab value="result">渲染结果</Tab><Tab value="values">有效取值</Tab></TabList>{tab === 'result' ? <pre className={style.preview}>{result.text || '输出为空（0 字节）'}</pre> : <><p className={style.muted}>结果由服务端选值；接口未返回取值来源。</p><table className={style.compactTable}><thead><tr><th>变量</th><th>值</th></tr></thead><tbody>{Object.entries(result.values).map(([key, value]) => <tr key={key}><td>{key}</td><td>{value === '' ? '空字符串' : value}</td></tr>)}</tbody></table><p>缺项：{result.missing.join('、') || '无'}</p></>}</>}</DialogContent><DialogActions><Button onClick={onClose}>关闭</Button></DialogActions></DialogBody></DialogSurface></Dialog>;
}

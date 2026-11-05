import React from 'react';
import CodeEditor from 'components/CodeEditor';
import { ConfigDefinition } from 'services/unified_config';
import { textFromBase64 } from 'services/unified_config_renderer';
import { fileFormat } from './ContentEditor';
import style from './workspace.module.less';

export default function DefinitionView({ definition, name }: { definition: ConfigDefinition; name: string }) {
  const instance = React.useId();
  const [tab, setTab] = React.useState<'content' | 'values'>('content');
  let content: string;
  try { content = textFromBase64(definition.content || ''); } catch { content = `二进制正文（base64）：${definition.content || ''}`; }
  return <div><div role="tablist" aria-label="版本详情"><button role="tab" aria-selected={tab === 'content'} onClick={() => setTab('content')}>配置内容</button><button role="tab" aria-selected={tab === 'values'} onClick={() => setTab('values')}>变量与取值</button></div>
    {tab === 'content' ? <div style={{ height: 360 }}><CodeEditor readonly validate={definition.engine !== 'pole-mustache-v1'} modelPath={`inmemory://config-version/${encodeURIComponent(name)}/${encodeURIComponent(instance)}`} ariaLabel="版本配置正文" height="100%" value={content} language={fileFormat(name)} /></div> : <><h4>变量声明</h4><table className={style.compactTable}><thead><tr><th>名称</th><th>类型</th><th>必填</th></tr></thead><tbody>{(definition.parameters || []).map(p => <tr key={p.name}><td>{p.name}</td><td>{p.type}</td><td>{p.required ? '是' : '否'}</td></tr>)}</tbody></table><h4>条件组</h4>{(definition.conditionGroups || []).map(group => <p key={group.id}><strong>{group.name}</strong>：{(group.audience.conditions || []).map(c => `${c.key} ${c.type.replace('MATCH_TYPE_', '')} ${c.value}`).join(' 且 ') || JSON.stringify(group.audience.labels || {})}</p>)}<h4>变量取值</h4><table className={style.compactTable}><thead><tr><th>变量</th><th>条件组与取值（按顺序）</th><th>兜底</th></tr></thead><tbody>{(definition.parameterBindings || []).map(setting => <tr key={setting.name}><td>{setting.name}</td><td>{(setting.bindings || []).map(b => `${definition.conditionGroups?.find(g => g.id === b.groupId)?.name || b.groupId}: ${b.value.value ?? ''}`).join('；')}</td><td>{setting.fallback === undefined ? '未设置' : (setting.fallback.value ?? '') === '' ? '空字符串' : setting.fallback.value}</td></tr>)}</tbody></table>{(definition.valueRules?.length || Object.keys(definition.defaultValues || {}).length) ? <><h4>旧版取值规则</h4><pre>{JSON.stringify({ defaultValues: definition.defaultValues, valueRules: definition.valueRules }, null, 2)}</pre></> : null}</>}
  </div>;
}

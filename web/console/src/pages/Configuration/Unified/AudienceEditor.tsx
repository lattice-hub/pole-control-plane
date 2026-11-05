import React, { useEffect, useId, useRef, useState } from 'react';
import { Button, Field, Input, Select } from '@fluentui/react-components';
import { testConfigAudience } from 'services/unified_config';
import type { Audience, ConfigKey, MatchCondition, MatchType, NamedConditionGroup } from 'services/unified_config';
import { audienceConditions, validateAudience } from './audience';
import style from './audience.module.less';

const types: [MatchType, string][] = [['MATCH_TYPE_EXACT', '精确匹配'], ['MATCH_TYPE_REGEX', '正则匹配'], ['MATCH_TYPE_NOT_EQUALS', '不等于'], ['MATCH_TYPE_IN', '属于集合'], ['MATCH_TYPE_NOT_IN', '不属于集合'], ['MATCH_TYPE_RANGE', '整数范围']];
const keys = [['CLIENT_IP', '客户端 IP'], ['CLIENT_ID', '客户端 ID'], ['CLIENT_LANGUAGE', '客户端语言']];
const errorText = (error: unknown) => error instanceof Error ? error.message : String(error);
export default function AudienceEditor({ audience, onChange, disabled, label, testKey, groups = [] }: { audience: Audience; onChange: (value: Audience) => void; disabled: boolean; label: string; testKey?: ConfigKey; groups?: NamedConditionGroup[] }) {
  const id = useId();
  const [group, setGroup] = useState('');
  const [error, setError] = useState('');
  const [labels, setLabels] = useState('{}');
  const [result, setResult] = useState<string | null>(null);
  const [testError, setTestError] = useState('');
  const [busy, setBusy] = useState(false);
  const generation = useRef(0);
  const invalidate = () => { generation.current += 1; setResult(null); setTestError(''); setBusy(false); };
  useEffect(() => { invalidate(); }, [audience, labels, testKey?.environment, testKey?.group, testKey?.name]);
  let conditions: MatchCondition[];
  try { conditions = audienceConditions(audience); } catch (e) { return <div className={style.editor}><p className={style.error} role="alert">{errorText(e)}。请在 Definition JSON 中修正。</p><pre className={style.raw}>{JSON.stringify(audience, null, 2)}</pre></div>; }
  const write = (next: MatchCondition[]) => { invalidate(); setError(''); onChange({ conditions: next }); };
  const update = (index: number, patch: Partial<MatchCondition>) => write(conditions.map((item, i) => i === index ? { ...item, ...patch } : item));
  const runTest = async () => {
    if (!testKey) return;
    const request = ++generation.current;
    setResult(null); setTestError(''); setBusy(true);
    try {
      const parsed: unknown = JSON.parse(labels);
      if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed) || Object.values(parsed).some(item => typeof item !== 'string')) throw new Error('客户端标签必须是值均为字符串的 JSON 对象。');
      validateAudience(audience, true);
      const matched = await testConfigAudience(testKey, audience, parsed as Record<string, string>);
      if (request === generation.current) setResult(matched ? '命中' : '未命中');
    } catch (e) { if (request === generation.current) setTestError(errorText(e)); }
    finally { if (request === generation.current) setBusy(false); }
  };
  return <fieldset className={style.editor} disabled={disabled}><legend>{label}</legend><p>所有条件同时满足（AND）才命中；客户端必须显式提供对应键，缺少键时负向条件也不命中。</p>
    {conditions.map((condition, index) => {
      const known = keys.some(([key]) => key === condition.key);
      const range = condition.type === 'MATCH_TYPE_RANGE';
      const list = condition.type === 'MATCH_TYPE_IN' || condition.type === 'MATCH_TYPE_NOT_IN';
      const parts = condition.value.split(range ? '~' : ',');
      return <div className={style.condition} key={index}>
        <Field label="客户端标签"><Select aria-label={`条件 ${index + 1} 键类型`} disabled={disabled} value={known ? condition.key : '__custom'} onChange={(_, d) => update(index, { key: d.value === '__custom' ? '' : d.value })}>{keys.map(([key, text]) => <option value={key} key={key}>{text}</option>)}<option value="__custom">自定义键</option></Select></Field>
        <Field label="标签键"><Input aria-label={`条件 ${index + 1} 标签键`} readOnly={known} disabled={disabled} value={condition.key} placeholder="输入标签键" onChange={(_, d) => update(index, { key: d.value })} /></Field>
        <Field label="匹配方式"><Select aria-label={`条件 ${index + 1} 匹配方式`} disabled={disabled} value={condition.type} onChange={(_, d) => update(index, { type: d.value as MatchType, value: d.value === 'MATCH_TYPE_RANGE' ? '~' : '' })}>{types.map(([type, text]) => <option value={type} key={type}>{text}</option>)}</Select></Field>
        <div className={style.value}>{range ? <><span className={style.fieldLabel}>整数范围</span><div className={style.range}><Input aria-label={`条件 ${index + 1} 范围起点`} value={parts[0] ?? ''} disabled={disabled} placeholder="起点" onChange={(_, d) => update(index, { value: `${d.value}~${parts[1] ?? ''}` })} /><span>至</span><Input aria-label={`条件 ${index + 1} 范围终点`} value={parts[1] ?? ''} disabled={disabled} placeholder="终点" onChange={(_, d) => update(index, { value: `${parts[0] ?? ''}~${d.value}` })} /></div><small>包含两端，输入有符号 int64 整数。</small></> : list ? <><span className={style.fieldLabel}>集合成员</span><div className={style.members}>{parts.map((member, memberIndex) => <div className={style.member} key={memberIndex}><Input aria-label={`条件 ${index + 1} 集合成员 ${memberIndex + 1}`} disabled={disabled} value={member} onChange={(_, d) => { if (d.value.includes(',')) { setError('单个成员不能包含逗号；逗号用于分隔成员。'); return; } const next = [...parts]; next[memberIndex] = d.value; update(index, { value: next.join(',') }); }} /><Button disabled={disabled || parts.length === 1} onClick={() => update(index, { value: parts.filter((_, i) => i !== memberIndex).join(',') })} aria-label={`删除条件 ${index + 1} 集合成员 ${memberIndex + 1}`}>删除</Button></div>)}<Button disabled={disabled} onClick={() => update(index, { value: [...parts, ''].join(',') })}>添加成员</Button></div><small>成员以逗号分隔，空串及空格会保留；单个成员不能包含逗号。</small></> : <Field label={condition.type === 'MATCH_TYPE_REGEX' ? '正则表达式' : '匹配值'}><Input aria-label={`条件 ${index + 1} 匹配值`} value={condition.value} disabled={disabled} onChange={(_, d) => update(index, { value: d.value })} />{condition.type === 'MATCH_TYPE_REGEX' && <small>使用 Go RE2 搜索语义，由服务端校验。</small>}</Field>}</div>
        <Button className={style.remove} disabled={disabled} onClick={() => write(conditions.filter((_, i) => i !== index))} aria-label={`删除条件 ${index + 1}`}>删除</Button>
      </div>;
    })}
    <Button disabled={disabled} onClick={() => write([...conditions, { key: '', type: 'MATCH_TYPE_EXACT', value: '' }])}>添加条件</Button>
    {!!groups.length && <details className={style.extra}><summary>从已有条件组带入（可选）</summary><p>复制条件，不保留组关联；冲突不会覆盖。</p><div className={style.group}><Select aria-label="选择条件组" disabled={disabled} value={group} onChange={(_, d) => setGroup(d.value)}><option value="">选择条件组</option>{groups.map(item => <option key={item.id} value={item.id}>{item.name}</option>)}</Select><Button disabled={disabled || !group} onClick={() => { try { const selected = groups.find(item => item.id === group); if (!selected) throw new Error('所选条件组不存在。'); const incoming = audienceConditions(selected.audience); if (!incoming.length) throw new Error('条件组为空。'); const next = [...conditions]; for (const item of incoming) { const existing = next.filter(row => row.key === item.key); if (existing.some(row => row.type !== item.type || row.value !== item.value)) throw new Error(`标签 ${item.key} 与已有条件冲突，请先调整。`); if (!existing.length) next.push({ ...item }); } write(next); setGroup(''); } catch (e) { setError(errorText(e)); } }}>带入条件</Button></div></details>}
    {error && <p className={style.error} role="alert">{error}</p>}
    {testKey && <details className={style.extra}><summary>测试匹配</summary><p>输入客户端标签 JSON 对象，键和值均为字符串。</p><label htmlFor={`${id}-labels`}>客户端标签</label><textarea id={`${id}-labels`} disabled={disabled} value={labels} onChange={e => { invalidate(); setLabels(e.target.value); }} rows={4} spellCheck={false} /><Button disabled={disabled || busy} onClick={runTest}>{busy ? '测试中…' : '测试匹配'}</Button>{testError && <p className={style.error} role="alert">{testError}</p>}{result && <p role="status">测试结果：{result}</p>}</details>}
  </fieldset>;
}

import React from 'react';
import { OverlayDrawer, DrawerHeader, DrawerHeaderTitle, DrawerBody, DrawerFooter, Button, Checkbox, RadioGroup, Radio, Field, Input, TabList, Tab, Spinner } from '@fluentui/react-components';
import { Dismiss24Regular } from '@fluentui/react-icons';
import { Audience, ConfigFileKey, ConfigState, ReleaseKind, getConfigState, publishConfig, testConfigAudience } from 'services/unified_config';
import AudienceEditor from './AudienceEditor';
import { audienceConditions, validateAudience } from './audience';
import DefinitionView from './DefinitionView';
import style from './publish.module.less';

type Step = 'environment' | 'type' | 'review' | 'result';
type Candidate = { environment: string; state: ConfigState };
type Result = { environment: string; ok: boolean; message: string };
const steps: { id: Step; name: string }[] = [{ id: 'environment', name: '选择环境' }, { id: 'type', name: '发布类型' }, { id: 'review', name: '确认内容' }, { id: 'result', name: '发布结果' }];
const message = (error: unknown) => (error instanceof Error ? error.message : String(error)).replace('invalid audience regular expression', '正则表达式无效').replace('invalid audience range', '整数范围无效');
export default function PublishDrawer({ open, onClose, fileKey, environments, initialEnvironment, initialKind = 'RELEASE_KIND_NORMAL', onPublished }: {
  open: boolean; onClose: () => void; fileKey: ConfigFileKey; environments: string[]; initialEnvironment: string; initialKind?: ReleaseKind; onPublished: () => void;
}) {
  const [step, setStep] = React.useState<Step>('environment');
  const [targets, setTargets] = React.useState<string[]>([]);
  const [kind, setKind] = React.useState<ReleaseKind>(initialKind);
  const [audience, setAudience] = React.useState<Audience>({ conditions: [{ key: 'CLIENT_IP', type: 'MATCH_TYPE_EXACT', value: '' }] });
  const [priority, setPriority] = React.useState('10');
  const [candidates, setCandidates] = React.useState<Candidate[]>([]);
  const [selected, setSelected] = React.useState(initialEnvironment);
  const [results, setResults] = React.useState<Result[]>([]);
  const [error, setError] = React.useState('');
  const [busy, setBusy] = React.useState(false);
  const [frozen, setFrozen] = React.useState(false);
  const generation = React.useRef(0);
  const inFlight = React.useRef(false);
  const body = React.useRef<HTMLDivElement>(null);
  const callbacks = React.useRef({ onClose, onPublished }); callbacks.current = { onClose, onPublished };
  React.useEffect(() => {
    const token = ++generation.current;
    if (open) {
      setStep('environment'); setTargets(environments.includes(initialEnvironment) ? [initialEnvironment] : []);
      setKind(initialKind); setAudience({ conditions: [{ key: 'CLIENT_IP', type: 'MATCH_TYPE_EXACT', value: '' }] });
      setPriority('10'); setCandidates([]); setResults([]); setError(''); setFrozen(false); setBusy(false); setSelected(initialEnvironment);
    }
    return () => { if (generation.current === token) generation.current++; };
  }, [open, fileKey.group, fileKey.name, initialEnvironment, initialKind]);
  React.useEffect(() => { if (open) { if (body.current) body.current.scrollTop = 0; body.current?.querySelector<HTMLElement>('h2')?.focus({ preventScroll: true }); } }, [open, step]);
  const close = () => { if (!inFlight.current) callbacks.current.onClose(); };
  const loadCandidates = async () => {
    if (!targets.length || targets.some(env => !environments.includes(env))) throw new Error('请至少选择一个有效的目标环境');
    const loaded = await Promise.all(targets.map(async environment => {
      const state = await getConfigState({ ...fileKey, environment });
      if (!state.draft) throw new Error(`${environment} 尚无已保存草稿，请先保存各环境草稿`);
      return { environment, state };
    }));
    return loaded;
  };
  const next = async () => {
    if (inFlight.current) return;
    inFlight.current = true; const token = generation.current; setBusy(true); setError('');
    try {
      if (step === 'environment') {
        const loaded = await loadCandidates();
        if (generation.current !== token) return;
        setCandidates(loaded); setSelected(loaded[0].environment); setStep('type');
      } else if (step === 'type') {
        if (kind === 'RELEASE_KIND_GRAY') {
          validateAudience(audience, true);
          if (!/^\d+$/.test(priority) || !Number.isSafeInteger(Number(priority)) || Number(priority) > 2147483647) throw new Error('优先级须为 0 到 2147483647 的整数');
          // 用与正式选版共用的后端匹配器校验 RE2 等条件，不能以 JS 正则替代。
          await testConfigAudience({ ...fileKey, environment: targets[0] }, audience, {});
        }
        const loaded = await loadCandidates();
        if (generation.current !== token) return;
        setCandidates(loaded); setSelected(loaded[0].environment); setStep('review');
      }
    } catch (reason) { if (generation.current === token) setError(message(reason)); }
    finally { inFlight.current = false; if (generation.current === token) setBusy(false); }
  };
  const publish = async (retry = false) => {
    if (inFlight.current || (step !== 'review' && step !== 'result')) return;
    inFlight.current = true; setBusy(true); setError(''); setFrozen(true);
    const token = generation.current;
    const chosen = retry ? candidates.filter(c => results.some(r => r.environment === c.environment && !r.ok)) : candidates;
    const nextResults = [...results];
    let changed = false;
    for (const candidate of chosen) {
      const key = { ...fileKey, environment: candidate.environment };
      let result: Result;
      try {
        await publishConfig(key, candidate.state.draft!.revision, candidate.state.releaseSetRevision || '0', kind,
          kind === 'RELEASE_KIND_GRAY' ? audience : {}, kind === 'RELEASE_KIND_GRAY' ? Number(priority) : 0);
        changed = true; result = { environment: key.environment, ok: true, message: '发布成功' };
      } catch (reason) { result = { environment: key.environment, ok: false, message: message(reason) }; }
      const index = nextResults.findIndex(item => item.environment === result.environment);
      if (index < 0) nextResults.push(result); else nextResults[index] = result;
      if (generation.current === token) { setResults([...nextResults]); setStep('result'); }
    }
    inFlight.current = false;
    if (generation.current === token) {
      setBusy(false);
      if (changed) callbacks.current.onPublished();
    }
  };
  const index = steps.findIndex(item => item.id === step);
  const candidate = candidates.find(item => item.environment === selected) || candidates[0];
  const groups = candidates.find(item => item.environment === initialEnvironment)?.state.draft?.definition.conditionGroups || candidates[0]?.state.draft?.definition.conditionGroups || [];
  return <OverlayDrawer open={open} onOpenChange={(_, data) => { if (!data.open) close(); }} position="end" size="large" className={style.drawer}>
    <DrawerHeader><DrawerHeaderTitle action={<Button appearance="subtle" icon={<Dismiss24Regular />} aria-label="关闭发布抽屉" disabled={busy} onClick={close} />}>发布配置</DrawerHeaderTitle><p className={style.context}>{fileKey.name} · {fileKey.group}</p></DrawerHeader>
    <DrawerBody ref={body} className={style.body}>
      <p className={style.intro}>选择发布范围，核对各环境已保存的正文和变量取值。</p>
      <nav aria-label="发布步骤" className={style.steps}><ol>{steps.map((item, i) => <li key={item.id}><button aria-current={item.id === step ? 'step' : undefined} disabled={busy || (frozen ? item.id !== 'review' && item.id !== 'result' : i > index || item.id === 'result')} onClick={() => { setError(''); setStep(item.id); }}><span>{i + 1}</span>{item.name}</button></li>)}</ol></nav>
      {step !== 'environment' && <p className={style.context}>目标环境：{targets.join('、')}{step !== 'type' && ` · ${kind === 'RELEASE_KIND_GRAY' ? '灰度发布' : '正式发布'}`}</p>}
      {step === 'environment' && <section><h2 tabIndex={-1}>选择发布环境</h2><p>各环境独立发布；未选择的环境保持当前版本。</p><div className={style.environments}>{environments.map(env => <Checkbox key={env} label={env} checked={targets.includes(env)} disabled={busy} onChange={(_, data) => setTargets(data.checked ? [...targets, env] : targets.filter(value => value !== env))} />)}</div>{!environments.length && <p role="alert">请先保存共享正文并绑定业务环境。</p>}</section>}
      {step === 'type' && <section><h2 tabIndex={-1}>选择发布类型</h2><RadioGroup value={kind} onChange={(_, data) => { setKind(data.value as ReleaseKind); setError(''); }} disabled={busy}>
        <Radio value="RELEASE_KIND_NORMAL" label={<span><strong>正式发布</strong><small>替换所选环境的正式版，保留现有灰度。</small></span>} />
        <Radio value="RELEASE_KIND_GRAY" label={<span><strong>灰度发布</strong><small>设置下方客户端标签，只向匹配的请求提供该版本。</small></span>} />
      </RadioGroup>{kind === 'RELEASE_KIND_GRAY' && <div className={style.gray}><AudienceEditor label="客户端标签条件" audience={audience} onChange={nextAudience => { setAudience(nextAudience); setError(''); }} disabled={busy} groups={groups} testKey={{ ...fileKey, environment: targets[0] }} /><Field label="灰度优先级" hint="数值越小越优先；同级取最新版本。"><Input type="number" min={0} max={2147483647} step={1} aria-label="灰度优先级" value={priority} disabled={busy} onChange={(_, data) => { setPriority(data.value); setError(''); }} /></Field></div>}</section>}
      {step === 'review' && <section><h2 tabIndex={-1}>确认发布内容</h2><p>{kind === 'RELEASE_KIND_NORMAL' ? '替换正式版本，保留现有灰度。' : `客户端标签全部满足：${audienceConditions(audience).map(c => `${c.key} ${c.type.replace('MATCH_TYPE_', '')} ${JSON.stringify(c.value)}`).join(' 且 ')} · 优先级 ${priority}`}</p><p className={style.context}>{frozen ? '候选已冻结；重试只处理失败环境。' : '只发布下方已保存草稿。草稿或发布集合变化时会拒绝发布，请重新加载审阅。'}</p><TabList aria-label="候选环境" selectedValue={candidate?.environment} onTabSelect={(_, data) => setSelected(String(data.value))}>{candidates.map(c => <Tab key={c.environment} value={c.environment}>{c.environment}</Tab>)}</TabList>{candidate && <div role="tabpanel" aria-label={`${candidate.environment} 环境候选`}><p className={style.context}>草稿修订 {candidate.state.draft?.revision} · 发布集合修订 {candidate.state.releaseSetRevision || '0'}</p><DefinitionView definition={candidate.state.draft!.definition} name={fileKey.name} /></div>}</section>}
      {step === 'result' && <section><h2 tabIndex={-1}>{busy ? '正在发布' : results.every(r => r.ok) ? '发布完成' : '部分环境未发布'}</h2><p>每个环境独立反馈；不会重复发布已经成功的环境。</p><ul className={style.results}>{results.map(result => <li key={result.environment}><strong>{result.environment}</strong><span>{result.message}</span></li>)}</ul><Button onClick={() => setStep('review')} disabled={busy}>查看冻结的发布内容</Button></section>}
    </DrawerBody>
    <DrawerFooter className={style.footer}>{error && <p role="alert" className={style.error}>{error}</p>}<div className={style.footerRow}><span>{targets.length} 个环境 · {frozen ? '候选已冻结' : '仅发布已保存草稿'}</span><div><Button disabled={busy} onClick={close}>关闭</Button>{!frozen && step !== 'environment' && <Button disabled={busy} onClick={() => { setError(''); setStep(steps[index - 1].id); }}>上一步</Button>}{!frozen && step !== 'review' && <Button appearance="primary" disabled={busy} onClick={() => void next()}>{busy ? <Spinner size="tiny" /> : '下一步'}</Button>}{!frozen && step === 'review' && <Button appearance="primary" disabled={busy} onClick={() => void publish()}>确认发布</Button>}{frozen && step === 'review' && <Button onClick={() => setStep('result')}>返回发布结果</Button>}{step === 'result' && results.some(r => !r.ok) && <Button appearance="primary" disabled={busy} onClick={() => void publish(true)}>仅重试失败环境</Button>}</div></div></DrawerFooter>
  </OverlayDrawer>;
}

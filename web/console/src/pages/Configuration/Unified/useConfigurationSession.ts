import React from 'react';
import { hasPendingEnvironmentDraft } from './sessionDirty';
import { ConfigDefinition, ConfigFileKey, ConfigFileTemplate, ConfigState, getConfigFileTemplate, getConfigState, saveConfigDraft, saveConfigFileTemplate, isConfigNotFound } from 'services/unified_config';

export function sharedDefinition(d: ConfigDefinition): ConfigDefinition {
  return { content: d.content, engine: d.engine, format: d.format, parameters: d.parameters, compatibility: d.compatibility };
}
export function composeDefinition(shared: ConfigDefinition, environment: ConfigDefinition): ConfigDefinition {
  return { ...environment, ...sharedDefinition(shared) };
}
type Entry = { definition: ConfigDefinition; saved: string; revision: string; needsDraft: boolean };
type SessionSnapshot = { entries: Record<string, Entry>; shared: ConfigDefinition; template: ConfigFileTemplate; needsTemplate: boolean; incomplete: boolean };
const draftSessions = new Map<string, SessionSnapshot>();
export type SaveResult = { environment: string; ok: boolean; message: string };
export default function useConfigurationSession(key: ConfigFileKey | undefined, environment: string) {
  const [template, setTemplate] = React.useState<ConfigFileTemplate>();
  const [definition, setDefinitionState] = React.useState<ConfigDefinition>();
  const [state, setState] = React.useState<ConfigState>();
  const [error, setError] = React.useState(''); const [results, setResults] = React.useState<SaveResult[]>([]);
  const [restored, setRestored] = React.useState(false);
  const [busy, setBusy] = React.useState(false); const [needsTemplate, setNeedsTemplate] = React.useState(false); const [incomplete, setIncomplete] = React.useState(false);
  const generation = React.useRef(0); const cache = React.useRef<Record<string, Entry>>({});
  const shared = React.useRef<ConfigDefinition>(); const capturedTemplate = React.useRef<ConfigFileTemplate>(); const file = React.useRef('');
  const userId = localStorage.getItem('login-user-id');
  const identity = key ? JSON.stringify([userId || '', key.group, key.name]) : '';
  const scope = userId ? identity : '';
  const persist = (missing = needsTemplate, partial = incomplete) => { if (scope && file.current === identity && shared.current && capturedTemplate.current) draftSessions.set(scope, { entries: structuredClone(cache.current), shared: structuredClone(shared.current), template: structuredClone(capturedTemplate.current), needsTemplate: missing, incomplete: partial }); };
  const setDefinition = (value: ConfigDefinition) => { if (!key || file.current !== identity || !cache.current[environment]) return; shared.current = sharedDefinition(value); cache.current[environment].definition = value; persist(); setDefinitionState(value); };
  const dirty = !!definition && (needsTemplate || incomplete || hasPendingEnvironmentDraft(Object.values(cache.current), value => composeDefinition(shared.current || definition, value as ConfigDefinition)));
  const load = React.useCallback(async () => {
    if (!key || !environment) return;
    const current = ++generation.current; setBusy(true); setError('');
    try {
      const s = await getConfigState({ ...key, environment });
      let remote: ConfigFileTemplate; let missing = false;
      try { remote = await getConfigFileTemplate(key, environment); }
      catch (e) { if (!isConfigNotFound(e) || !s.draft) throw e; missing = true; remote = { key, definition: sharedDefinition(s.draft.definition), environments: [environment], revision: '0', description: '' }; }
      if (generation.current !== current) return;
      const recovery = !capturedTemplate.current ? draftSessions.get(scope) : undefined;
      if (recovery) { cache.current = structuredClone(recovery.entries); shared.current = structuredClone(recovery.shared); capturedTemplate.current = structuredClone(recovery.template); setIncomplete(recovery.incomplete); setRestored(true); }
      const t = capturedTemplate.current || remote;
      if (generation.current !== current) return;
      capturedTemplate.current = t; setNeedsTemplate(recovery?.needsTemplate ?? (missing || t.revision === '0'));
      const original = composeDefinition(t.definition, s.draft?.definition || t.definition);
      const actualDraft = s.draft?.definition || t.definition;
      const cached = cache.current[environment]; const d = composeDefinition(shared.current || t.definition, cached?.definition || original);
      if (!cached) cache.current[environment] = { definition: d, saved: JSON.stringify(actualDraft), revision: s.draft?.revision || '0', needsDraft: !s.draft };
      shared.current ||= sharedDefinition(t.definition);
      setTemplate(t); setState(s); setDefinitionState(d);
      if (recovery || missing || draftSessions.has(scope)) persist(recovery?.needsTemplate ?? missing, recovery?.incomplete ?? incomplete);
    } catch (e) { if (generation.current === current) setError(e instanceof Error ? e.message : String(e)); }
    finally { if (generation.current === current) setBusy(false); }
  }, [key?.group, key?.name, environment, scope, identity]);
  React.useEffect(() => { if (file.current !== identity) { file.current = identity; cache.current = {}; shared.current = undefined; capturedTemplate.current = undefined; setIncomplete(false); setRestored(false); setResults([]); } setTemplate(undefined); setState(undefined); setDefinitionState(undefined); void load(); return () => { generation.current++; }; }, [load]);
  React.useEffect(() => { const handler = (e: BeforeUnloadEvent) => { if (dirty) { e.preventDefault(); e.returnValue = ''; } }; window.addEventListener('beforeunload', handler); return () => window.removeEventListener('beforeunload', handler); }, [dirty]);
  const save = async () => {
    if (!template || !definition || !key || busy) return;
    const current = generation.current; setBusy(true); setError(''); setResults([]);
    try {
      // Capture every environment before changing the shared source. Edited environments retain their original CAS.
      const captured = await Promise.all(template.environments.map(async env => {
        try { return { env, state: await getConfigState({ ...key, environment: env }) }; }
        catch (e) { return { env, error: e instanceof Error ? e.message : String(e) }; }
      }));
      if (generation.current !== current) return;
      let updated = template;
      if (needsTemplate || JSON.stringify(sharedDefinition(template.definition)) !== JSON.stringify(sharedDefinition(definition))) {
        updated = await saveConfigFileTemplate({ ...template, definition: sharedDefinition(definition) }, template.revision);
        if (generation.current !== current) return;
        capturedTemplate.current = updated; setTemplate(updated); setNeedsTemplate(false);
      }
      const next: SaveResult[] = [];
      for (const item of captured) {
        const env = item.env;
        try {
          if (!item.state) throw new Error(item.error || '读取环境草稿失败');
          const latest = item.state; let cached = cache.current[env];
          if (!cached && !latest.draft) { cached = { definition: sharedDefinition(definition), saved: JSON.stringify(sharedDefinition(definition)), revision: '0', needsDraft: true }; cache.current[env] = cached; }
          if (cached && (latest.draft?.revision || '0') !== cached.revision) throw new Error('草稿已被其他操作修改，请重新加载后合并');
          const settings = cached?.definition || latest.draft?.definition || sharedDefinition(definition);
          const composed = composeDefinition(definition, settings);
          const result = await saveConfigDraft({ ...key, environment: env }, composed, latest.draft?.revision || '0');
          cache.current[env] = { definition: composed, saved: JSON.stringify(composed), revision: result.draft?.revision || '0', needsDraft: !result.draft };
          if (env === environment) setState(result);
          next.push({ environment: env, ok: true, message: '已保存' });
        } catch (e) { next.push({ environment: env, ok: false, message: e instanceof Error ? e.message : String(e) }); }
      }
      if (generation.current !== current) return;
      const partial = next.some(result => !result.ok); setResults(next); setIncomplete(partial);
      if (partial) persist(false, true); else { draftSessions.delete(scope); setRestored(false); }
    } catch (e) { if (generation.current === current) setError(`保存失败，请检查冲突并重新加载：${e instanceof Error ? e.message : String(e)}`); }
    finally { if (generation.current === current) setBusy(false); }
  };
  const refresh = () => { if (dirty) return; cache.current = {}; shared.current = undefined; capturedTemplate.current = undefined; void load(); };
  return { template, definition, setDefinition, state, dirty, busy, error, setError, results, restored, load, refresh, save };
}

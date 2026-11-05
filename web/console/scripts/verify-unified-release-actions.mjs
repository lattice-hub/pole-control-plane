import assert from 'node:assert/strict';
import fs from 'node:fs';
import ts from 'typescript';

function load(path, require) {
  const source = fs.readFileSync(path, 'utf8');
  const compiled = ts.transpileModule(source, { compilerOptions: {
    module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2022, jsx: ts.JsxEmit.React,
  } }).outputText;
  const module = { exports: {} };
  new Function('require', 'module', 'exports', compiled)(require, module, module.exports);
  return module.exports;
}

const calls = [];
const snapshots = new Map();
const service = load('src/services/unified_config.ts', name => {
  assert.equal(name, 'utils/request');
  return { apiRequest: async request => {
    calls.push(request);
    if (request.action.endsWith('/state')) {
      const snapshot = snapshots.get(request.data.releaseId);
      assert.ok(snapshot, `missing snapshot for ${request.data.releaseId}`);
      return { '@type': 'type.googleapis.com/pole.config.v1.ConfigState', ...snapshot };
    }
    assert.equal(request.action, '/config/v1/copy-release-to-draft');
    return { '@type': 'type.googleapis.com/pole.config.v1.ConfigState' };
  } };
});
const actions = load('src/pages/Configuration/Unified/ReleaseHistory.tsx', name =>
  name === 'services/unified_config' ? service : {});
const key = { environment: 'dev', group: 'app', name: 'settings' };
const definition = { engine: 'pole-mustache-v1', parameters: [{ name: 'port', type: 'VALUE_TYPE_INTEGER' }] };
assert.ok(actions.sameSharedDefinition(definition, { ...definition, content: '', format: '',
  compatibility: { apollo: false }, parameters: [{ ...definition.parameters[0], required: false }] }));
assert.ok(!actions.sameSharedDefinition(definition, { ...definition, content: 'eA==' }));
assert.ok(!actions.sameSharedDefinition(definition, { ...definition, parameters: [{ name: 'port', type: 'VALUE_TYPE_STRING' }] }));

const state = { releaseSetRevision: '5', draft: { revision: '2' }, activeReleaseIds: ['gray', 'normal'], releases: [{ id: 'gray' }] };
for (const [id, kind] of [['gray', 'RELEASE_KIND_GRAY'], ['normal', 'RELEASE_KIND_NORMAL']]) {
  snapshots.set(id, { releaseSetRevision: '5', activeReleaseIds: state.activeReleaseIds, selectedRelease: { id, kind } });
}
assert.equal(await actions.activeNormalReleaseId(key, state), 'normal');
assert.equal(await actions.activeNormalReleaseId(key, { ...state, activeReleaseIds: [] }), '');
snapshots.get('normal').releaseSetRevision = '6';
await assert.rejects(actions.activeNormalReleaseId(key, state), /发布集合已变化/);
snapshots.get('normal').releaseSetRevision = '5';

snapshots.set('old', { selectedRelease: { id: 'old', definition: { ...definition, content: 'eA==' } } });
const before = calls.length;
await assert.rejects(actions.copyRelease(key, 'old', definition, state), /共享正文或变量声明与当前模板不同/);
assert.equal(calls.slice(before).filter(call => call.action.endsWith('/copy-release-to-draft')).length, 0);
snapshots.get('old').selectedRelease.definition = definition;
await actions.copyRelease(key, 'old', definition, state);
assert.deepEqual(calls.at(-1), { action: '/config/v1/copy-release-to-draft', data: {
  key, releaseId: 'old', expectedDraftRevision: '2', expectedReleaseSetRevision: '5',
} });
console.log('unified release action checks passed');

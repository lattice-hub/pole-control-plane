import assert from 'node:assert/strict';
import crypto from 'node:crypto';
import fs from 'node:fs';
import path from 'node:path';
import { createRequire } from 'node:module';
import { execFileSync } from 'node:child_process';
import ts from 'typescript';

const project = process.cwd();
const nodeRequire = createRequire(import.meta.url);
const load = (relative, mockedRequire = nodeRequire) => {
  const source = fs.readFileSync(path.join(project, relative), 'utf8');
  const compiled = ts.transpileModule(source, { compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2022, esModuleInterop: true } }).outputText;
  const module = { exports: {} };
  new Function('require', 'module', 'exports', compiled)(mockedRequire, module, module.exports);
  return module.exports;
};

const renderer = load('src/services/unified_config_renderer.ts');
// Resolve the exact Go dependency, including a developer's explicit local replacement.
const specification = execFileSync('go', ['list', '-m', '-f', '{{.Dir}}', 'github.com/pole-io/specification'],
  { cwd: path.resolve(project, '../..'), encoding: 'utf8' }).trim();
const vectors = JSON.parse(fs.readFileSync(path.join(specification, 'CONFIG_TEMPLATE_TEST_VECTORS.json'), 'utf8')).vectors;
for (const vector of vectors) {
  const definition = vector.request.definition;
  const render = () => renderer.renderConfigSnapshot({ content: definition.content, engine: definition.engine, format: definition.format,
    parameters: definition.parameters, values: renderer.selectConfigValues(definition, vector.request.labels || {}) });
  if (!vector.valid) {
    assert.throws(render, undefined, vector.name);
    continue;
  }
  const output = render();
  assert.equal(Buffer.from(output.bytes).toString('base64'), vector.expectedContent, vector.name);
  assert.equal(crypto.createHash('sha256').update(output.bytes).digest('hex'), vector.expectedSha256, vector.name);
}
assert.throws(() => renderer.selectConfigValues({ valueRules: [{ audience: { conditions: [{ key: 'client', type: 'MATCH_TYPE_REGEX', value: '^cn' }] } }] }, { client: 'cn' }), /服务端 Preview/);
assert.throws(() => renderer.renderConfigSnapshot({ content: renderer.textToBase64('{{{{name}}}}'), engine: 'pole-mustache-v1', format: 'text',
  parameters: [{ name: 'name', type: 'VALUE_TYPE_STRING' }], values: { name: { type: 'VALUE_TYPE_STRING', value: 'x' } } }));
assert.throws(() => renderer.renderConfigSnapshot({ content: renderer.textToBase64('{"a":1,"a":2}'), engine: 'pole-mustache-v1', format: 'json' }));
const stringParameter = [{ name: 'x', type: 'VALUE_TYPE_STRING' }];
const stringValue = { x: { type: 'VALUE_TYPE_STRING', value: 'x'.repeat(260_000) } };
assert.throws(() => renderer.renderConfigSnapshot({ content: renderer.textToBase64('{{{x}}}{{{x}}}{{{x}}}'),
  engine: 'pole-mustache-v1', format: 'text', parameters: stringParameter, values: stringValue }), /超过 512 KiB/);
assert.throws(() => renderer.renderConfigSnapshot({ content: renderer.textToBase64('x'.repeat(510_000)),
  engine: 'pole-mustache-v1', format: 'text', parameters: stringParameter,
  values: { x: { type: 'VALUE_TYPE_STRING', value: 'y'.repeat(20_000) } } }), /模板输入超过 512 KiB/);
assert.throws(() => renderer.renderConfigSnapshot({ content: '', engine: 'literal', format: 'text',
  parameters: stringParameter }), /literal 不允许参数/);
assert.throws(() => renderer.renderConfigSnapshot({ content: '', engine: 'pole-mustache-v1', format: 'text',
  parameters: Array.from({ length: 129 }, (_, index) => ({ name: `p${index}`, type: 'VALUE_TYPE_STRING' })) }), /128/);
assert.throws(() => renderer.renderConfigSnapshot({ content: '', engine: 'pole-mustache-v1', format: 'text',
  parameters: [{ name: 'unused', type: 'VALUE_TYPE_UNSPECIFIED' }] }), /类型未知/);
for (const content of ['bareKey', 'escaped\\=key=value', 'long=first\\\n  second']) {
  renderer.renderConfigSnapshot({ content: renderer.textToBase64(content), engine: 'pole-mustache-v1', format: 'properties' });
}
assert.throws(() => renderer.renderConfigSnapshot({ content: renderer.textToBase64('key=\\uZZZZ'),
  engine: 'pole-mustache-v1', format: 'properties' }), /Unicode 转义无效/);
assert.throws(() => renderer.renderConfigSnapshot({ content: renderer.textToBase64('key=value\\'),
  engine: 'pole-mustache-v1', format: 'properties' }), /转义未结束/);
renderer.renderConfigSnapshot({ content: renderer.textToBase64('key=${ENV}'), engine: 'pole-mustache-v1', format: 'properties' });
const bomTemplate = '\uFEFFline={{{x}}}\r\n';
const bomValue = '\uFEFF值';
const bomContent = renderer.textToBase64(bomTemplate);
assert.equal(renderer.textFromBase64(bomContent), bomTemplate);
const bomOutput = renderer.renderConfigSnapshot({ content: bomContent, engine: 'pole-mustache-v1', format: 'text',
  parameters: stringParameter, values: { x: { type: 'VALUE_TYPE_STRING', value: bomValue } } });
assert.equal(Buffer.from(bomOutput.bytes).toString('base64'), renderer.textToBase64('\uFEFFline=\uFEFF值\r\n'));

const calls = [];
const service = load('src/services/unified_config.ts', (name) => {
  if (name === 'utils/request') return { apiRequest: async (request) => {
    calls.push(request);
    return { '@type': `type.googleapis.com/pole.config.v1.${request.action.endsWith('/preview') ? 'RenderSnapshot' : request.action.endsWith('/files') ? 'ConfigFileList' : request.action.includes('file-template') ? 'ConfigFileTemplate' : 'ConfigState'}` };
  } };
  return nodeRequire(name);
});
const key = { environment: 'dev', group: 'app', name: 'settings' };
const definition = { content: renderer.textToBase64('hello'), engine: 'literal', format: 'text' };
assert.deepEqual(service.parseConfigDefinition('{"engine":"literal","compatibility":{"apollo":true}}'),
  { engine: 'literal', content: '', format: '', compatibility: { apollo: true } });
await service.getConfigState(key, 'opaque-token', 'release-id');
await service.saveConfigDraft(key, definition, '9007199254740993');
await service.previewConfig(key, definition, { region: 'cn' });
await service.publishConfig(key, '9007199254740993', '9007199254740994', 'RELEASE_KIND_GRAY', { labels: { region: 'cn' } }, 0);
await service.stopConfigGray(key, '9007199254740994', 'release-id');
await service.stopAllConfigGray(key, '9007199254740994');
await service.copyConfigReleaseToDraft(key, 'release-id', '9007199254740993', '9007199254740994');
await service.rollbackConfig(key, 'source-id', 'release-id', '9007199254740994');
await service.offlineConfig(key, '9007199254740994');
assert.deepEqual(calls.map((call) => call.action), ['state', 'draft', 'preview', 'publish', 'stop', 'stop-all-gray',
  'copy-release-to-draft', 'rollback', 'offline'].map((action) => `/config/v1/${action}`));
for (const call of calls) assert.deepEqual(call.data.key, key);
assert.equal(calls[0].data.pageToken, 'opaque-token');
assert.equal(calls[1].data.expectedDraftRevision, '9007199254740993');
assert.equal(calls[3].data.expectedReleaseSetRevision, '9007199254740994');
assert.equal(calls[3].data.priority, 0);
await import('./verify-unified-audience-boundaries.mjs');
await service.listConfigFiles('dev', 'app', 'settings', 'next');
await service.getConfigFileTemplate({ group: 'app', name: 'settings' }, 'dev');
await service.saveConfigFileTemplate({ key: { group: 'app', name: 'settings' }, definition, environments: ['dev'], revision: '2' }, '2');
await service.promoteConfigGray(key, 'source-gray', '9007199254740994');
assert.deepEqual(calls.slice(9).map(call => call.action), ['/config/v1/files', '/config/v1/file-template', '/config/v1/file-template-draft', '/config/v1/promote-gray']);
assert.equal(calls[11].data.expectedRevision, '2');
const emptyScalar = renderer.renderConfigSnapshot({ content: renderer.textToBase64('a={{{x}}}'), engine: 'pole-mustache-v1', format: 'text', parameters: [{name:'x', type:'VALUE_TYPE_STRING'}], values: { x: { type:'VALUE_TYPE_STRING' } } });
assert.equal(emptyScalar.text, 'a=');
console.log(`unified configuration contract passed: ${vectors.length} shared vectors, 13 HTTP actions`);

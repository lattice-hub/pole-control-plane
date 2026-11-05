import assert from 'node:assert/strict';
import fs from 'node:fs';
import ts from 'typescript';

const source = fs.readFileSync('src/services/unified_config.ts', 'utf8');
const compiled = ts.transpileModule(source, { compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2022 } }).outputText;
const calls = [];
const template = { key: { group: 'app', name: 'settings' }, definition: { engine: 'pole-mustache-v1', content: '', format: 'text', parameters: [{ name: 'port', type: 'VALUE_TYPE_INTEGER' }, { name: 'logLevel', type: 'VALUE_TYPE_STRING' }] }, environments: ['dev'], revision: '2' };
const module = { exports: {} };
new Function('require', 'module', 'exports', compiled)((name) => {
  assert.equal(name, 'utils/request');
  return { apiRequest: async request => { calls.push(request); return { '@type': 'type.googleapis.com/pole.config.v1.ConfigFileTemplate', ...template }; } };
}, module, module.exports);
const response = await module.exports.getConfigFileTemplate(template.key, 'dev');
assert.equal('@type' in response, false);
await module.exports.saveConfigFileTemplate({ ...response, definition: template.definition }, response.revision);
assert.deepEqual(calls[1].data, { template, expectedRevision: '2' });
console.log('unified template response-to-save roundtrip passed');

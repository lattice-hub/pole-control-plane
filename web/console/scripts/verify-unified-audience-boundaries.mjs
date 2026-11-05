import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import ts from 'typescript';

const source = await readFile(new URL('../src/pages/Configuration/Unified/audience.ts', import.meta.url), 'utf8');
const js = ts.transpileModule(source, { compilerOptions: { module: ts.ModuleKind.ESNext, target: ts.ScriptTarget.ES2020 } }).outputText;
const { audienceConditions, validateAudience } = await import(`data:text/javascript;base64,${Buffer.from(js).toString('base64')}`);
for (const invalid of [null, [], { conditions: {} }, { conditions: [null] }, { conditions: [{ key: 'x', type: 1, value: '' }] }, { conditions: [{ key: 'x', type: 'MATCH_TYPE_EXACT', value: 1 }] }, { conditions: [{ key: 'x', type: 'MATCH_TYPE_UNSPECIFIED', value: '' }] }, { labels: { x: 1 } }, { labels: { x: '' }, conditions: [{ key: 'y', type: 'MATCH_TYPE_EXACT', value: '' }] }, { conditions: [{ key: 'x', type: 'MATCH_TYPE_EXACT', value: null }] }]) {
  assert.throws(() => audienceConditions(invalid));
}
assert.deepEqual(audienceConditions({ labels: { x: '' } }), [{ key: 'x', type: 'MATCH_TYPE_EXACT', value: '' }]);
assert.deepEqual(audienceConditions({ labels: {}, conditions: [{ key: 'x', type: 'MATCH_TYPE_EXACT' }] }), [{ key: 'x', type: 'MATCH_TYPE_EXACT', value: '' }]);
assert.deepEqual(audienceConditions({ labels: { x: '' }, conditions: [] }), [{ key: 'x', type: 'MATCH_TYPE_EXACT', value: '' }]);
assert.throws(() => audienceConditions({ labels: [], conditions: [] }));
assert.throws(() => audienceConditions({ labels: {}, conditions: {} }));
const range = value => ({ conditions: [{ key: 'x', type: 'MATCH_TYPE_RANGE', value }] });
for (const valid of ['+1~+2', '-9223372036854775808~9223372036854775807', '0~0']) validateAudience(range(valid));
for (const invalid of ['9223372036854775808~9223372036854775808', '-9223372036854775809~0', '2~1', ' 1~2', '1~2~3']) assert.throws(() => validateAudience(range(invalid)));
assert.throws(() => validateAudience({ conditions: [{ key: 'x', type: 'MATCH_TYPE_EXACT', value: '' }, { key: 'x', type: 'MATCH_TYPE_EXACT', value: '' }] }));
console.log('unified audience boundary checks passed');

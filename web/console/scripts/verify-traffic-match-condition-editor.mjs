import fs from 'node:fs';
import path from 'node:path';
import assert from 'node:assert/strict';

const root = process.cwd();
const read = (file) => fs.readFileSync(path.join(root, file), 'utf8');

const sharedSource = read('src/pages/Governance/shared/TrafficMatchConditionEditor.tsx');
const sharedStyleSource = read('src/pages/Governance/shared/TrafficMatchConditionEditor.module.less');
assert.match(sharedSource, /export interface TrafficMatchConditionRow/);
assert.match(sharedSource, /TagInput/);
assert.match(sharedSource, /RadioGroup/);
assert.match(sharedSource, /role="table"/);
assert.match(sharedSource, /role="row"/);
assert.match(sharedSource, /MatchTypeOption/);
assert.doesNotMatch(sharedSource, /relationOptions|segmentButton/);
assert.match(sharedSource, /参数类型/);
assert.match(sharedSource, /参数键/);
assert.match(sharedSource, /匹配类型/);
assert.match(sharedSource, /值来源/);
assert.match(sharedSource, /匹配值/);
assert.match(sharedSource, /DeleteIcon/);
assert.match(sharedSource, /暂无匹配条件/);
assert.match(sharedStyleSource, /\.headActions\s*\{[\s\S]*?flex-wrap:\s*wrap;/);
assert.match(sharedStyleSource, /\.headActions\s*\{[\s\S]*?justify-content:\s*flex-end;/);
assert.match(sharedStyleSource, /\.headActions\s*\{[\s\S]*?min-width:\s*0;/);
assert.match(sharedStyleSource, /\.conditionRow\s*\{[\s\S]*?display:\s*grid;/);
assert.match(sharedStyleSource, /\.tableHeader\s*\{[\s\S]*?white-space:\s*nowrap;/);
assert.match(sharedStyleSource, /container-type:\s*inline-size/);
assert.match(sharedStyleSource, /@container\s*\(max-width:\s*920px\)/);
assert.match(sharedStyleSource, /\.fieldLabel\s*\{[\s\S]*?display:\s*none;/);
assert.doesNotMatch(sharedStyleSource, /overflow:\s*hidden/);

const consumers = [
  {
    file: 'src/pages/Governance/Router/CustomRouteEditor.tsx',
    minUses: 1,
    forbidden: [/trafficTableColumns/, /renderMatchTable/],
  },
  {
    file: 'src/pages/Governance/RateLimit/RateLimitEditor.tsx',
    minUses: 1,
    forbidden: [/conditionToolbar/, /className=\{styles\.conditionGrid\}/],
  },
  {
    file: 'src/pages/Governance/Security/TrafficGovernanceEditor.tsx',
    minUses: 3,
    forbidden: [/renderMatchValueEditor/, /matchModeOptions/, /matchParamDefaultKey/],
  },
  {
    file: 'src/pages/Governance/Router/LaneGroupEdtor.tsx',
    minUses: 1,
    forbidden: [/renderLaneMatchModeSwitch/, /className=\{styles\.laneMatchRows\}/],
  },
];

for (const item of consumers) {
  const source = read(item.file);
  assert.match(source, /TrafficMatchConditionEditor/);
  const uses = source.match(/<TrafficMatchConditionEditor/g)?.length || 0;
  assert.ok(uses >= item.minUses, `${item.file} should use TrafficMatchConditionEditor at least ${item.minUses} time(s)`);
  for (const pattern of item.forbidden) {
    assert.doesNotMatch(source, pattern, `${item.file} should not contain ${pattern}`);
  }
  for (const pattern of item.required || []) {
    assert.match(source, pattern, `${item.file} should contain ${pattern}`);
  }
}

console.log('Traffic match condition editor verification passed.');

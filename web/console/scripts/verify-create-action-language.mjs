import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';

const root = process.cwd();
const read = (relativePath) => fs.readFileSync(path.join(root, relativePath), 'utf8');
const findCreateButtonIndex = (source, label) => {
  let iconIndex = source.indexOf('icon={<AddIcon />}');
  while (iconIndex >= 0) {
    if (source.slice(iconIndex, iconIndex + 500).includes(label)) return iconIndex;
    iconIndex = source.indexOf('icon={<AddIcon />}', iconIndex + 1);
  }
  return -1;
};

const governanceLists = [
  ['流量治理', 'src/pages/Governance/Security/index.tsx', /新建\{TrafficGovernanceKindLabel\[kind\]\}规则/],
  ['限流规则', 'src/pages/Governance/RateLimit/RateLimitTable.tsx', /新建限流规则/],
  ['无损规则', 'src/pages/Governance/LossLess/LossLessTable.tsx', /新建无损规则/],
  ['自定义路由', 'src/pages/Governance/Router/CustomRoute.tsx', /新建自定义路由/],
  ['泳道组', 'src/pages/Governance/Router/LaneGroupTable.tsx', /新建泳道组/],
  ['主动探测规则', 'src/pages/Governance/CircuitBreaker/FaultDetectTable.tsx', /新建主动探测规则/],
  ['熔断规则', 'src/pages/Governance/CircuitBreaker/CircuitBreakerTable.tsx', /新建熔断规则/],
  ['就近路由', 'src/pages/Governance/Router/NearbyRoute.tsx', /新建就近路由/],
];

for (const [name, file, labelPattern] of governanceLists) {
  const source = read(file);
  assert.match(source, /import \{[^}]*AddIcon[^}]*\} from 'components\/Fluent\/icons'/, `${name}列表必须使用统一加号图标。`);
  assert.match(source, /shape="square"[\s\S]*variant="outline"[\s\S]*icon=\{<RefreshIcon \/>}/, `${name}列表刷新必须使用方形次级按钮。`);
  assert.match(source, new RegExp(`theme="primary"[\\s\\S]*icon=\\{<AddIcon \\/>\\}[\\s\\S]*${labelPattern.source}`), `${name}列表新建必须使用主按钮和资源化文案。`);

  const searchIndex = source.indexOf('<Search');
  const refreshIndex = source.indexOf('icon={<RefreshIcon />}');
  const createLabelMatch = source.match(labelPattern);
  const createIndex = createLabelMatch ? source.indexOf(createLabelMatch[0]) : -1;
  const toolbarIndex = source.lastIndexOf('<ResourceToolbar', createIndex);
  assert.ok(searchIndex >= 0 && refreshIndex > searchIndex, `${name}列表刷新必须位于查询之后。`);
  assert.ok(createLabelMatch && createIndex > refreshIndex, `${name}列表主新增必须位于查询和刷新之后。`);
  assert.ok(toolbarIndex >= 0 && source.slice(toolbarIndex, createIndex).includes('filters={('), `${name}列表主新增必须属于清单 ResourceToolbar。`);
  assert.doesNotMatch(source, />\s*新建\s*<\/Button>/, `${name}列表不能使用无资源语义的“新建”。`);
}

const laneRules = read('src/pages/Governance/Router/LaneRuleTable.tsx');
assert.match(laneRules, /<ResourceToolbar[\s\S]*title="泳道规则清单"[\s\S]*filters=\{\([\s\S]*theme="primary"[\s\S]*icon=\{<AddIcon \/>}[\s\S]*新建泳道规则/, '泳道规则子列表主新增必须属于清单 ResourceToolbar。');

const instanceList = read('src/pages/Discovery/Services/Instance/InstanceTable.tsx');
assert.match(instanceList, /<ResourceToolbar[\s\S]*title="实例清单"[\s\S]*filters=\{\([\s\S]*新建实例/, '实例主新增必须属于实例清单 ResourceToolbar。');
assert.ok(
  instanceList.indexOf('icon={<RefreshIcon />}') < instanceList.indexOf('新建实例'),
  '实例列表主新增必须位于查询和刷新之后。',
);

const aliasList = read('src/pages/Discovery/Services/alias.tsx');
assert.match(aliasList, /<ResourceToolbar[\s\S]*title="别名清单"[\s\S]*filters=\{\([\s\S]*新建别名/, '别名主新增必须属于别名清单 ResourceToolbar。');
assert.ok(
  aliasList.indexOf('onClick={resetFilter}>重置') < aliasList.indexOf('新建别名'),
  '别名列表主新增必须位于筛选操作之后。',
);

const topLevelToolbarPages = [
  ['命名空间', 'src/pages/Namespace/index.tsx', '新建业务环境'],
  ['A2A Agent', 'src/pages/AI/A2A/index.tsx', '新建 A2A 环境实例'],
  ['MCP Server', 'src/pages/AI/Mcp/index.tsx', '新建 MCP 环境实例'],
  ['治理工作台', 'src/pages/Governance/Workbench/index.tsx', '新建规则'],
];

for (const [name, file, label] of topLevelToolbarPages) {
  const source = read(file);
  const createIndex = findCreateButtonIndex(source, label);
  const toolbarIndex = source.lastIndexOf('<ResourceToolbar', createIndex);
  const headerIndex = source.lastIndexOf('<ResourceHeader', createIndex);
  assert.ok(createIndex >= 0 && toolbarIndex > headerIndex, `${name}主新增必须从 ResourceHeader 下移到清单 ResourceToolbar。`);
  assert.ok(source.slice(toolbarIndex, createIndex).includes('filters={'), `${name}主新增必须位于清单工具栏 filters 区域。`);
}

const representativePrimaryLists = [
  ['命名空间', 'src/pages/Namespace/index.tsx', '新建业务环境'],
  ['逻辑服务', 'src/pages/Discovery/Services/services.tsx', '新建逻辑服务'],
  ['A2A Agent', 'src/pages/AI/A2A/index.tsx', '新建 A2A 环境实例'],
  ['MCP Server', 'src/pages/AI/Mcp/index.tsx', '新建 MCP 环境实例'],
  ['用户', 'src/pages/Auth/Principal/UserTable.tsx', '新建用户'],
  ['角色', 'src/pages/Auth/Principal/RoleTable.tsx', '新建角色'],
  ['用户组', 'src/pages/Auth/Principal/GroupTable.tsx', '新建用户组'],
  ['策略', 'src/pages/Auth/Policy/PolicyTable.tsx', '新建策略'],
];

for (const [name, file, label] of representativePrimaryLists) {
  assert.match(
    read(file),
    new RegExp(`theme="primary"[\\s\\S]*icon=\\{<AddIcon \\/>\\}[\\s\\S]*${label}`),
    `${name}列表必须保持 primary + AddIcon + 资源名的主新增语言。`,
  );
}

const rateLimitEditor = read('src/pages/Governance/RateLimit/RateLimitEditor.tsx');
assert.match(rateLimitEditor, /className=\{styles\.inlineAdd\}[\s\S]*variant="text"[\s\S]*新增接口/, '编辑器内部局部新增必须保持次级文本按钮。');

const laneGroupEditor = read('src/pages/Governance/Router/LaneGroupEdtor.tsx');
assert.match(laneGroupEditor, /variant="outline"[\s\S]*icon=\{<AddIcon \/>}[\s\S]*新增入口/, '编辑器内部结构新增必须保持次级描边按钮。');

console.log('列表主新增按钮设计语言检查通过。');

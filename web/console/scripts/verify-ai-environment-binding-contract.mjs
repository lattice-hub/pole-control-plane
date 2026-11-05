import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';

const read = (path) => readFile(new URL(`../${path}`, import.meta.url), 'utf8');
const [
  definitionService,
  mcpService,
  a2aService,
  mcpDetail,
  a2aDetail,
  bindingComponent,
  logicalDirectory,
  mcpPage,
  a2aPage,
] = await Promise.all([
  read('src/services/ai_definition.ts'),
  read('src/services/mcp.ts'),
  read('src/services/a2a.ts'),
  read('src/pages/AI/Mcp/Detail.tsx'),
  read('src/pages/AI/A2A/Detail.tsx'),
  read('src/components/AIEnvironmentBinding/index.tsx'),
  read('src/components/AILogicalResourceDirectory/index.tsx'),
  read('src/pages/AI/Mcp/index.tsx'),
  read('src/pages/AI/A2A/index.tsx'),
]);

assert.match(definitionService, /environment-binding/, '共享客户端应支持解析当前环境绑定');
assert.match(definitionService, /environments/, '共享客户端应支持列出逻辑定义的环境实例');
assert.match(definitionService, /environment-bindings/, '共享客户端应支持显式绑定环境实例');
assert.match(definitionService, /definition_ids: \[definitionId\]/, '共享客户端应按后端批量删除契约提交逻辑定义 ID');
assert.match(
  definitionService,
  /return getApiRequest<AIEnvironmentBinding<Kind>>\(\{[\s\S]*environment-binding/,
  '单条绑定响应已经由请求层解包，不应再次读取 data 字段',
);

for (const [label, source] of [['MCP', mcpService], ['A2A', a2aService]]) {
  assert.match(source, /describeAIEnvironmentBinding/, `${label} 应复用共享绑定客户端`);
  assert.match(source, /bindAIResourceDefinition/, `${label} 应复用共享显式绑定客户端`);
}

for (const [label, source] of [['MCP', mcpDetail], ['A2A', a2aDetail]]) {
  assert.match(source, /EnvironmentResourceSwitcher/, `${label} 详情应提供环境切换`);
  assert.match(source, /未关联跨环境逻辑定义/, `${label} 详情应显式展示旧记录的未关联状态`);
  assert.match(source, /namespace !== 'pole-system'/, `${label} 不应把系统投影纳入业务聚合`);
}

assert.match(bindingComponent, /不会按名称自动合并/, '关联交互必须声明禁止按同名自动合并');
assert.match(bindingComponent, /选择已有逻辑定义/, '关联交互应允许显式选择已有定义');

assert.match(logicalDirectory, /逻辑资源/, '列表页应提供全局逻辑资源目录');
assert.match(logicalDirectory, /绑定环境实例/, '逻辑资源目录应提供显式环境绑定入口');
assert.match(logicalDirectory, /ConfirmOperationButton[\s\S]*删除逻辑资源/, '逻辑资源目录应提供带确认提示的删除入口');
assert.match(logicalDirectory, /bindings\.length > 0[\s\S]*请先解绑全部环境实例/, '仍有环境绑定的逻辑定义必须阻止删除');
assert.match(logicalDirectory, /occupiedEnvironments/, '绑定交互应阻止同一逻辑资源在同一环境重复部署');
assert.match(logicalDirectory, /namespace !== 'pole-system'/, '逻辑目录不应聚合系统空间实例');

for (const [label, source] of [['MCP', mcpPage], ['A2A', a2aPage]]) {
  assert.match(source, /AIResourceViewSwitch/, `${label} 列表页应提供逻辑资源与环境实例切换`);
  assert.match(source, /deleteDefinition=\{delete(?:MCPServer|A2AAgent)Definition\}/, `${label} 逻辑目录应接入对应删除客户端`);
  assert.match(source, /requestedNamespace \? 'environment' : 'logical'/, `${label} 从环境入口进入时应默认显示环境实例`);
  assert.match(source, /}, \[requestedNamespace\]\);/, `${label} 同一路由切换环境时应重新同步查询上下文`);
  assert.match(source, /title: '环境空间'/, `${label} 环境实例表应显式展示环境空间`);
  assert.match(source, /environment=\{editorState\.mode === 'create' \? requestedNamespace : ''\}/, `${label} 创建环境实例时应锁定入口环境`);
}

console.log('AI 环境逻辑定义与显式绑定契约检查通过。');

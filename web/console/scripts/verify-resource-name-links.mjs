import fs from 'node:fs';
import path from 'node:path';

const root = process.cwd();

const read = (relativePath) => fs.readFileSync(path.join(root, relativePath), 'utf8');

const assert = (condition, message) => {
  if (!condition) {
    throw new Error(message);
  }
};

const betweenColumns = (source, startKey, endKey) => {
  const start = source.indexOf(`colKey: '${startKey}'`);
  const end = source.indexOf(`colKey: '${endKey}'`, start + 1);
  assert(start >= 0 && end > start, `无法定位列区间 ${startKey} -> ${endKey}`);
  return source.slice(start, end);
};

const resourceLink = read('src/components/ResourceNameLink/index.tsx');
assert(resourceLink.includes('event.stopPropagation()'), '资源名称链接必须阻止表格行点击重复触发');
assert(resourceLink.includes('aria-label='), '资源名称链接必须提供可访问名称');
assert(resourceLink.includes('title={title}'), '资源名称链接必须为省略名称提供完整悬浮提示');

const namespaceName = betweenColumns(read('src/pages/Namespace/index.tsx'), 'name', 'commnet');
assert(namespaceName.includes('ResourceNameLink'), '命名空间名称必须是查看链接');
assert(!namespaceName.includes('metadata') && !namespaceName.includes('editable'), '命名空间名称列不能混入标签或权限状态');

const serviceName = betweenColumns(read('src/pages/Discovery/Services/services.tsx'), 'name', 'environment_count');
assert(serviceName.includes('ResourceNameLink'), '服务名称必须是查看链接');
assert(!serviceName.includes('comment') && !serviceName.includes('serviceMeta'), '服务名称列不能混入描述');

const governanceName = betweenColumns(read('src/pages/Governance/Workbench/index.tsx'), 'name', 'typeLabel');
assert(governanceName.includes('ResourceNameLink'), '治理规则名称必须是查看链接');
assert(!governanceName.includes('condition') && !governanceName.includes('description'), '治理规则名称列不能混入条件或描述');

const a2aName = betweenColumns(read('src/pages/AI/A2A/index.tsx'), 'name', 'namespace');
assert(a2aName.includes('ResourceNameLink'), 'A2A Agent 名称必须是查看链接');
assert(!a2aName.includes('<Tag') && !a2aName.includes('namespace'), 'A2A Agent 名称列不能混入协议或命名空间');

const mcpName = betweenColumns(read('src/pages/AI/Mcp/index.tsx'), 'name', 'namespace');
assert(mcpName.includes('ResourceNameLink'), 'MCP Server 名称必须是查看链接');
assert(!mcpName.includes('<Tag') && !mcpName.includes('namespace'), 'MCP Server 名称列不能混入协议、命名空间或后端摘要');

const governanceReleaseName = betweenColumns(read('src/pages/Governance/RuleRelease/ReleaseTable.tsx'), 'release_name', 'version');
assert(governanceReleaseName.includes('ResourceNameLink'), '治理发布版本名称必须是可用的查看链接');
assert(!governanceReleaseName.includes('系统版本'), '治理发布版本名称列不能重复展示系统版本');

const aliasName = betweenColumns(read('src/pages/Discovery/Services/alias.tsx'), 'alias', 'namespace');
assert(aliasName.includes('ResourceNameLink'), '服务别名名称必须是查看链接');

const policyResources = read('src/pages/Auth/Policy/PolicyDetailView.tsx');
assert(policyResources.includes('name={row.name}') && policyResources.includes('setSelectedResource'), '策略授权资源名称必须能够打开查看详情');

console.log('资源列表名称列静态约束验证通过');

import fs from 'node:fs';
import path from 'node:path';

const root = path.resolve(import.meta.dirname, '..');
const read = (relativePath) => fs.readFileSync(path.join(root, relativePath), 'utf8');
const assertIncludes = (content, needle, message) => {
  if (!content.includes(needle)) throw new Error(message);
};

const workbench = read('src/pages/Governance/Workbench/index.tsx');
const createPage = read('src/pages/Governance/RuleCreatePage.tsx');

assertIncludes(workbench, "params.set('ruleNamespace'", '创建规则时必须使用独立的 ruleNamespace 传递规则归属环境');
assertIncludes(workbench, 'namespace = selectedNamespace', '规则列表请求必须按归属环境查询');
assertIncludes(workbench, 'rule.namespace === selectedNamespace', '工作台必须按规则顶层 namespace 过滤');
assertIncludes(
  workbench,
  "React.useState(serviceContext?.namespace || requestedNamespace || '')",
  '独立治理工作台不能自动选择 default 环境',
);
assertIncludes(
  workbench,
  "const nextNamespace = serviceContext?.namespace || ''",
  '重置筛选时独立治理工作台必须恢复为全部环境',
);
assertIncludes(workbench, 'disabled={!selectedNamespace}', '未选择归属环境时不能创建治理规则');
assertIncludes(createPage, "searchParams.get('ruleNamespace')", '创建页必须读取独立的规则归属环境');
assertIncludes(createPage, 'RuleNamespaceProvider', '创建页必须向所有规则编辑器提供统一的规则归属环境');

console.log('governance rule namespace contract verified');

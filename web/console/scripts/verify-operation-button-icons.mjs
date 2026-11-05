#!/usr/bin/env node

import fs from 'node:fs';
import path from 'node:path';

const root = process.cwd();
const read = (file) => fs.readFileSync(path.join(root, file), 'utf8');

const assert = (condition, message) => {
  if (!condition) {
    throw new Error(message);
  }
};

const componentFile = 'src/components/OperationButton/index.tsx';
assert(fs.existsSync(path.join(root, componentFile)), '缺少统一操作按钮组件 OperationButton');
const styleFile = 'src/components/OperationButton/index.module.less';
assert(fs.existsSync(path.join(root, styleFile)), '缺少统一操作按钮样式');

const component = read(componentFile);
const styles = read(styleFile);
[
  'export type OperationAction',
  'export const OperationButton',
  'export const ConfirmOperationButton',
  'export const OperationButtonGroup',
  'viewEdit',
  'authorize',
  'delete',
  'publish',
  'rollback',
  'copy',
  'tools',
  'token',
  'refresh',
  'Tooltip',
  'Popconfirm',
  'aria-label',
  'title',
].forEach((token) => assert(component.includes(token), `OperationButton 缺少 ${token}`));

assert(component.includes("size = 'small'"), '统一操作按钮必须默认使用 small 尺寸');
assert(styles.includes('flex: 0 0 28px'), '统一操作按钮项必须固定为 28px');
assert(styles.includes('width: max-content'), '统一操作按钮组必须保持内容宽度');
assert(styles.includes('gap: 4px'), '统一操作按钮组间距必须为 4px');

[
  'src/pages/Namespace/index.tsx',
  'src/pages/Discovery/Services/services.tsx',
].forEach((file) => {
  const source = read(file);
  assert(source.includes('<OperationButtonGroup className={style.actionCell}>'), `${file} 的操作列未使用紧凑按钮组`);
});

[
  'Edit1Icon',
  'LockOnIcon',
  'DeleteIcon',
  'SendIcon',
  'RollbackIcon',
  'CopyIcon',
  'ListIcon',
  'UserVisibleIcon',
  'RefreshIcon',
].forEach((icon) => assert(component.includes(icon), `OperationButton 未统一声明 ${icon}`));

assert(component.includes("viewEdit: '查看'"), '查看/编辑合并入口必须展示为查看');
assert(component.includes('viewEdit: <BrowseIcon />'), '查看/编辑合并入口必须使用查看图标');

[
  'src/pages/Namespace/index.tsx',
  'src/pages/Auth/Principal/UserTable.tsx',
  'src/pages/Auth/Principal/GroupTable.tsx',
  'src/pages/Auth/Principal/RoleTable.tsx',
  'src/pages/Auth/Policy/PolicyTable.tsx',
  'src/pages/Discovery/Services/services.tsx',
  'src/pages/Discovery/Services/alias.tsx',
  'src/pages/Governance/Workbench/index.tsx',
  'src/pages/AI/A2A/index.tsx',
  'src/pages/AI/Mcp/index.tsx',
].forEach((file) => {
  const source = read(file);
  assert(source.includes("components/OperationButton"), `${file} 未使用统一操作按钮组件`);
  assert(source.includes('OperationButton'), `${file} 未使用 OperationButton`);
  if (source.includes('<Tooltip')) {
    assert(/import\s+\{[^}]*\bTooltip\b[^}]*\}\s+from\s+['"]components\/Fluent['"]/.test(source), `${file} 使用了 Tooltip 但未从 Fluent 适配入口导入`);
  }
  if (source.includes('Popconfirm')) {
    assert(source.includes('ConfirmOperationButton'), `${file} 的确认操作未使用 ConfirmOperationButton`);
  }
});

const mcpPage = read('src/pages/AI/Mcp/index.tsx');
assert(mcpPage.includes('header="MCP 服务详情"'), 'MCP 查看入口必须打开服务详情抽屉');
assert(mcpPage.includes('OperationButton action="view" onClick={() => operateServer(\'detail\', row)}'), 'MCP 操作列主入口必须是查看');
assert(!mcpPage.includes('OperationButton action="edit" onClick={() => operateServer(\'edit\', row)}'), 'MCP 操作列不能再直接暴露编辑按钮');
assert(!mcpPage.includes('OperationButton action="tools"'), 'MCP 操作列不能再单独暴露查看工具按钮');
assert(mcpPage.includes('编辑 Server'), 'MCP 详情抽屉内部必须保留可见编辑入口');

[
  'CreditcardIcon',
  'Delete1Icon',
  'FileIcon',
].forEach((icon) => {
  const offenders = [
    'src/pages/Namespace/index.tsx',
    'src/pages/Auth/Principal/UserTable.tsx',
    'src/pages/Auth/Principal/GroupTable.tsx',
    'src/pages/Auth/Principal/RoleTable.tsx',
    'src/pages/Auth/Policy/PolicyTable.tsx',
    'src/pages/Discovery/Services/services.tsx',
    'src/pages/Discovery/Services/alias.tsx',
    'src/pages/Governance/Workbench/index.tsx',
    'src/pages/AI/A2A/index.tsx',
    'src/pages/AI/Mcp/index.tsx',
  ].filter((file) => read(file).includes(icon));
  assert(offenders.length === 0, `${icon} 仍被用于主要操作列：${offenders.join(', ')}`);
});

console.log('Operation button icon rules verified');

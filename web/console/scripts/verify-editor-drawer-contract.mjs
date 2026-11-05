import fs from 'node:fs';
import path from 'node:path';

const root = process.cwd();
const expectedWidths = new Map([
  ['src/pages/AI/A2A/index.tsx', 'workspace'],
  ['src/pages/AI/Mcp/index.tsx', 'workspace'],
  ['src/pages/Auth/Policy/PolicyEditor.tsx', 'wide'],
  ['src/pages/Auth/Principal/GroupEditor.tsx', 'standard'],
  ['src/pages/Auth/Principal/RoleEditor.tsx', 'standard'],
  ['src/pages/Auth/Principal/UserEditor.tsx', 'standard'],
  ['src/pages/Discovery/Services/AliasEditor.tsx', 'compact'],
  ['src/pages/Discovery/Services/Instance/InstanceEditor.tsx', 'wide'],
  ['src/pages/Discovery/Services/ServiceEditor.tsx', 'standard'],
  ['src/pages/Discovery/Services/services.tsx', 'compact'],
  ['src/pages/Discovery/Services/LogicalServiceDetail.tsx', 'compact'],
  ['src/pages/Governance/Router/LaneRuleEditor.tsx', 'wide'],
  ['src/pages/Governance/RuleRelease/PublishForm.tsx', 'wide'],
  ['src/pages/Namespace/NamespaceEditor.tsx', 'standard'],
  ['src/components/Authorize/index.tsx', 'standard'],
  ['src/pages/SystemConfiguration/components/ManagedDomainDraftPanel.tsx', 'standard'],
  ['src/pages/SystemConfiguration/components/AgentGatewayDraftPanel.tsx', 'standard'],
]);

const failures = [];
for (const [relativePath, width] of expectedWidths) {
  const source = fs.readFileSync(path.join(root, relativePath), 'utf8');
  const hasSize = source.includes(`size="${width}"`) || source.includes(`size='${width}'`);
  const hasEditorWidth = source.includes(`width="${width}"`) || source.includes(`width='${width}'`);
  if (!hasSize && !hasEditorWidth) {
    failures.push(`${relativePath}: 应使用 ${width} 抽屉宽度档位`);
  }
  if (source.includes('showOverlay={false}')) {
    failures.push(`${relativePath}: 编辑抽屉不得关闭遮罩`);
  }
  if (/closeOnOverlayClick(?!\s*=\s*\{false\})/.test(source)) {
    failures.push(`${relativePath}: 点击遮罩不得丢弃编辑内容`);
  }
}

const editorDrawer = fs.readFileSync(path.join(root, 'src/components/EditorDrawer/index.tsx'), 'utf8');
for (const width of ['compact', 'standard', 'wide', 'workspace']) {
  if (!editorDrawer.includes(`'${width}'`)) failures.push(`EditorDrawer: 缺少 ${width} 档位`);
}
for (const contract of ['closeOnOverlayClick = false', 'EditorDrawerActions', 'submitting']) {
  if (!editorDrawer.includes(contract)) failures.push(`EditorDrawer: 缺少 ${contract} 契约`);
}

if (failures.length) {
  console.error(`编辑抽屉契约验证失败：\n- ${failures.join('\n- ')}`);
  process.exit(1);
}

console.log(`编辑抽屉契约验证通过（${expectedWidths.size} 个创建/编辑入口）`);

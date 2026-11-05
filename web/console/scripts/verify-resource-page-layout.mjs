import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';

const root = process.cwd();
const read = (relativePath) => fs.readFileSync(path.join(root, relativePath), 'utf8');

const resourceLayout = read('src/components/ResourceLayout/index.tsx');
const resourceLayoutStyle = read('src/components/ResourceLayout/index.module.less');

assert.match(resourceLayout, /export const ResourceHeader/, '必须提供统一资源页页头组件 ResourceHeader。');
assert.match(resourceLayout, /export const ResourceToolbar/, '必须提供统一资源页列表工具栏组件 ResourceToolbar。');
assert.doesNotMatch(resourceLayout, /filtersClassName/, 'ResourceToolbar 不应再为单个资源页暴露搜索区特例布局入口。');
assert.match(resourceLayoutStyle, /\.resourceHeader[\s\S]*justify-content: space-between/, '统一页头必须固定左标题右操作结构。');
assert.match(resourceLayoutStyle, /\.resourceToolbar[\s\S]*justify-content: space-between/, '统一工具栏必须固定左列表标题右查询筛选结构。');
assert.match(resourceLayoutStyle, /\.headerActions[\s\S]*white-space: nowrap/, '页头操作区不能因文字换行导致按钮错位。');
assert.match(resourceLayoutStyle, /\.toolbarFilters[\s\S]*justify-content: flex-end/, '查询筛选区在宽屏下必须右对齐。');

const pages = [
  {
    name: '命名空间',
    file: 'src/pages/Namespace/index.tsx',
    title: '命名空间管理',
    create: '新建命名空间',
    toolbar: '命名空间列表',
  },
  {
    name: '注册发现服务',
    file: 'src/pages/Discovery/Services/index.tsx',
    companion: 'src/pages/Discovery/Services/services.tsx',
    title: '注册发现',
    create: '新建服务',
    toolbar: '服务清单',
  },
  {
    name: 'MCP 服务',
    file: 'src/pages/AI/Mcp/index.tsx',
    title: 'MCP 服务',
    create: '新建 MCP Server',
    toolbar: '服务列表',
  },
  {
    name: 'A2A Agent',
    file: 'src/pages/AI/A2A/index.tsx',
    title: 'A2A Agent',
    create: '新建 A2A Agent',
    toolbar: 'Agent 列表',
  },
  {
    name: '治理工作台',
    file: 'src/pages/Governance/Workbench/index.tsx',
    title: '规则治理工作台',
    create: '新建规则',
    toolbar: '规则清单',
  },
];

for (const page of pages) {
  const source = read(page.file);
  const companion = page.companion ? read(page.companion) : '';
  const combined = `${source}\n${companion}`;

  assert.match(combined, /components\/ResourceLayout/, `${page.name} 必须引用统一 ResourceLayout。`);
  assert.match(combined, /<ResourceHeader[\s\S]*title=/, `${page.name} 必须使用 ResourceHeader。`);
  assert.match(combined, new RegExp(page.title), `${page.name} 页头必须展示标题：${page.title}`);
  assert.match(combined, new RegExp(page.create.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')), `${page.name} 页头必须提供主创建按钮：${page.create}`);
  assert.match(combined, /RefreshIcon/, `${page.name} 页头必须提供统一刷新图标按钮。`);
  assert.match(combined, /shape="square"[\s\S]*variant="outline"|variant="outline"[\s\S]*shape="square"/, `${page.name} 刷新按钮必须使用方形 outline 样式。`);
  assert.match(combined, /<ResourceToolbar[\s\S]*title=/, `${page.name} 列表区必须使用 ResourceToolbar。`);
  assert.match(combined, new RegExp(page.toolbar), `${page.name} 工具栏必须展示列表标题：${page.toolbar}`);
  assert.match(combined, /当前显示/, `${page.name} 工具栏必须展示当前数量。`);
  assert.match(combined, /查询<\/Button>/, `${page.name} 筛选区必须提供查询按钮。`);
  assert.match(combined, /重置[\s\S]{0,80}<\/Button>/, `${page.name} 筛选区必须提供重置入口。`);
}

const governance = read('src/pages/Governance/Workbench/index.tsx');
const governanceStyle = read('src/pages/Governance/Workbench/index.module.less');
assert.doesNotMatch(governance, /description="点击规则行查看详情/, '治理工作台列表工具栏不能保留额外说明行，必须和其它资源页保持单行结构。');
assert.doesNotMatch(governance, /filtersClassName=\{style\.panelToolbarFilters\}/, '治理工作台不能再使用专用搜索区布局分支。');
assert.match(governance, /const statusOptions = \[[\s\S]*已启用[\s\S]*已停用/, '治理工作台必须提供状态筛选，形成关键字、类型、状态三段查询区。');
assert.match(governance, /<Input[\s\S]*label="关键字"[\s\S]*placeholder="规则名、服务、条件"[\s\S]*<Select[\s\S]*label="规则类型"[\s\S]*placeholder="全部规则类型"[\s\S]*<Select[\s\S]*label="状态"[\s\S]*placeholder="全部"[\s\S]*<Button variant="outline" onClick=\{\(\) => refreshData\(search\)\}>查询<\/Button>[\s\S]*重置/, '治理工作台查询区顺序必须和配置分组一致：关键字、筛选项、状态、查询、重置。');
assert.match(governance, /const statusMatched = !statusFilter \|\| rule\.status === statusFilter/, '治理工作台状态筛选必须参与本地过滤。');
const governanceToolbarIndex = governance.indexOf('<ResourceToolbar');
const governanceListPanelIndex = governance.indexOf('<div className={style.listPanel}>');
const governanceTableIndex = governance.indexOf('<Table', governanceListPanelIndex);
assert.ok(
  governanceToolbarIndex > -1
    && governanceListPanelIndex > governanceToolbarIndex
    && governanceTableIndex > governanceListPanelIndex,
  '治理工作台查询工具栏必须在表格白色面板外，和配置分组等资源页保持同一页面结构。',
);
assert.match(governanceStyle, /\.panelToolbar\s*\{[^}]*align-items: center/, '治理工作台工具栏必须回到共享资源页的单行居中结构。');
assert.doesNotMatch(governanceStyle, /\.panelToolbar\s*\{[^}]*(padding|border-bottom):/, '治理工作台工具栏不能有面板内 padding 或底部分割线，必须在灰色页面背景上独立展示。');
assert.doesNotMatch(governanceStyle, /\.panelToolbarFilters/, '治理工作台不能保留专用搜索区网格样式。');
assert.match(governanceStyle, /\.searchInput\s*\{[\s\S]*width: 340px/, '治理工作台搜索输入应使用固定宽度，和其它资源页查询区保持同一语言。');
assert.match(governanceStyle, /\.typeSelect\s*\{[\s\S]*width: 260px/, '治理工作台规则类型筛选宽度应与其它资源页筛选控件接近。');
assert.match(governanceStyle, /\.statusSelect\s*\{[\s\S]*width: 180px/, '治理工作台状态筛选必须使用稳定宽度。');

console.log('resource page layout checks passed');

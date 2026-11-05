import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';

const root = process.cwd();
const read = (relativePath) => fs.readFileSync(path.join(root, relativePath), 'utf8');

const resourceToolbarStyle = read('src/components/ResourceLayout/index.module.less');
const queryComposer = read('src/components/QueryComposer/index.tsx');
const queryComposerStyle = read('src/components/QueryComposer/index.module.less');

assert.match(
  resourceToolbarStyle,
  /\.resourceToolbar[\s\S]*align-items: flex-start[\s\S]*\.toolbarIntro[\s\S]*padding-top: 5px/,
  '清单标题必须与复合查询主行顶部对齐，不能随条件摘要整体垂直居中。',
);
assert.match(
  resourceToolbarStyle,
  /\.toolbarFilters[\s\S]*align-items: flex-start[\s\S]*> :global\(\[data-query-composer\]\)[\s\S]*width: 100%[\s\S]*min-width: 0[\s\S]*flex: 1 1 auto/,
  '清单工具栏中的 QueryComposer 必须占满剩余空间并保持顶部对齐。',
);
assert.match(
  resourceToolbarStyle,
  /\.toolbarFilters[\s\S]*:global\(\.fui-Button\)[\s\S]*height: 32px[\s\S]*flex: 0 0 auto[\s\S]*white-space: nowrap/,
  '清单工具栏按钮必须保持 32px 单行高度，禁止被压缩成多行。',
);
assert.match(
  resourceToolbarStyle,
  /@media \(max-width: 720px\)[\s\S]*\[data-query-composer\][\s\S]*flex-basis: 100%/,
  '窄屏空间不足时 QueryComposer 应整行换行，而不是压缩主新增按钮。',
);
assert.match(
  queryComposer,
  /mode === 'explicit'[\s\S]*<Button variant="outline" loading=\{loading\} onClick=\{submit\}>[\s\S]*查询/,
  '查询必须使用次级描边按钮，将唯一主色留给新建操作。',
);
assert.doesNotMatch(
  queryComposer,
  /mode === 'explicit'[\s\S]{0,180}<Button theme="primary"/,
  '显式查询不能继续与主新增形成双主按钮。',
);
assert.match(
  queryComposerStyle,
  /\.keywordWrap[\s\S]*min-width: 180px[\s\S]*max-width: 420px[\s\S]*flex: 1 1 280px/,
  '搜索框必须在 180px 到 420px 之间弹性收缩。',
);
assert.match(
  queryComposerStyle,
  /\.composerWithActions[\s\S]*max-width: none/,
  '带刷新和新建操作的 QueryComposer 不得再受容器最大宽度限制。',
);
assert.match(
  queryComposerStyle,
  /\.activeConditions[\s\S]*justify-content: flex-start[\s\S]*\.tags[\s\S]*justify-content: flex-start/,
  '已选条件必须在查询主行下方左对齐，不能漂移到操作区下方。',
);
assert.match(
  queryComposerStyle,
  /\.primaryRow > :global\(\.fui-Button\)[\s\S]*flex: 0 0 auto[\s\S]*white-space: nowrap/,
  'QueryComposer 内部按钮也必须保持单行。',
);
assert.match(
  queryComposer,
  /data-query-actions=\{actions \? 'true' : undefined\}[\s\S]*\{actions\}/,
  'QueryComposer 必须明确标记并渲染同行操作区。',
);

for (const file of [
  'src/pages/Namespace/index.tsx',
  'src/pages/Discovery/Services/services.tsx',
  'src/pages/Governance/Workbench/index.tsx',
  'src/pages/AI/Mcp/index.tsx',
  'src/pages/AI/A2A/index.tsx',
]) {
  const source = read(file);
  assert.match(source, /<QueryComposer[\s\S]*actions=\{\(/, `${file} 必须把刷新和主新增并入 QueryComposer.actions。`);
}

console.log('清单工具栏视觉与响应式契约检查通过。');

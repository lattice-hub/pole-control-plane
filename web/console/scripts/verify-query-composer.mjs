#!/usr/bin/env node

import assert from 'node:assert/strict';
import fs from 'node:fs';

const read = (file) => fs.readFileSync(file, 'utf8');

const composer = read('src/components/QueryComposer/index.tsx');
const composerStyle = read('src/components/QueryComposer/index.module.less');

assert.match(composer, /<Combobox[\s\S]*freeform[\s\S]*inlinePopup/, '主搜索必须使用支持自由输入和自动补全的 Fluent Combobox。');
assert.match(composer, /nativeEvent\.isComposing/, '主搜索回车提交必须避开中文输入法组合态。');
assert.match(composer, /onKeyDownCapture=\{[\s\S]*nativeEvent\.isComposing[\s\S]*stopPropagation/, '中文输入法组合态 Enter 必须在 Fluent 内部处理前于捕获阶段拦截。');
assert.match(composer, /<Popover[\s\S]*trapFocus[\s\S]*<PopoverSurface/, '高级筛选必须使用可锁定焦点的 Fluent Popover。');
assert.match(composer, /draftValues[\s\S]*setDraftValues\(values\)/, '高级筛选必须使用草稿状态，取消或 Esc 不得写回半成品条件。');
assert.match(composer, /const snapshot = \{ keyword: draftKeyword, values: draftValues \}[\s\S]*onSubmit\?\.\(snapshot\)/, '显式查询必须把同一份草稿快照交给页面，避免异步状态导致条件与分页错位。');
assert.match(composer, /if \(mode === 'instant'\) onValuesChange\?\.\(draftValues\)/, '即时过滤的高级条件必须在点击完成时一次性应用。');
assert.doesNotMatch(composer, /setDraftValues\(\(current\) => \{[\s\S]*onValuesChange/, '状态更新函数内部不得执行父组件回调副作用。');
assert.match(composer, /<Select[\s\S]*clearable=\{field\.type !== 'multiselect'\}[\s\S]*inlinePopup/, '高级筛选内的下拉必须内联弹层，并避免多选清空产生空字符串选项。');
assert.match(composer, /<TagGroup[\s\S]*dismissible[\s\S]*onDismiss=/, '已选条件必须通过 TagGroup 处理可靠删除。');
assert.match(composer, /data-query-time-range/, '时间范围必须保留为独立插槽。');
assert.match(composer, /type: 'text'[\s\S]*type: 'select' \| 'multiselect'[\s\S]*type: 'boolean'/, '查询字段必须显式覆盖文本、单选、多选与三态布尔类型。');
assert.match(composerStyle, /@media \(max-width: 720px\)[\s\S]*grid-template-columns: minmax\(0, 1fr\)/, '高级筛选在窄屏必须退化为单列。');
assert.match(composerStyle, /@media \(max-width: 720px\)[\s\S]*\.advancedSurface[\s\S]*max-height: 48vh/, '窄屏高级筛选必须限制高度并保留内部滚动。');

const migratedPages = [
  'src/pages/Namespace/index.tsx',
  'src/pages/Discovery/Services/services.tsx',
  'src/pages/Governance/Workbench/index.tsx',
  'src/pages/AI/Mcp/index.tsx',
  'src/pages/AI/A2A/index.tsx',
  'src/pages/SystemConfiguration/index.tsx',
  'src/pages/Metrics/SystemMonitor/index.tsx',
  'src/pages/Metrics/ServiceMonitor/index.tsx',
  'src/pages/Metrics/ServerEvent/index.tsx',
  'src/pages/Metrics/ServerOperation/index.tsx',
];

for (const file of migratedPages) {
  assert.match(read(file), /<QueryComposer/, `${file} 必须接入统一复合查询组件。`);
}

for (const file of [
  'src/pages/SystemConfiguration/index.tsx',
  'src/pages/Metrics/SystemMonitor/index.tsx',
  'src/pages/Metrics/ServiceMonitor/index.tsx',
]) {
  assert.doesNotMatch(read(file), /onSubmit=\{\(\) => undefined\}/, `${file} 的即时过滤模式不应传递空提交函数。`);
}

for (const file of [
  'src/pages/Metrics/ServerEvent/index.tsx',
  'src/pages/Metrics/ServerOperation/index.tsx',
]) {
  const source = read(file);
  assert.match(source, /timeRange=\{\([\s\S]*<DateRangePicker/, `${file} 必须把时间范围作为独立插槽。`);
  assert.doesNotMatch(source, /key:\s*'(?:startTime|endTime)'/, `${file} 不得把时间范围放入高级条件标签。`);
  assert.match(source, /setPaginationVersion\(\(version\) => version \+ 1\)/, `${file} 查询和重置后必须回到本地分页第一页。`);
  assert.match(source, /<Table[\s\S]*key=\{paginationVersion\}/, `${file} 本地分页必须随查询版本复位。`);
}

assert.match(
  read('src/pages/Governance/Workbench/index.tsx'),
  /setPaginationVersion\(\(version\) => version \+ 1\)[\s\S]*<Table[\s\S]*key=\{paginationVersion\}/,
  '治理工作台查询和重置后必须回到本地分页第一页。',
);

assert.doesNotMatch(
  read('src/pages/AI/A2A/index.tsx'),
  /className=\{style\.filterControls\}[\s\S]*placeholder="Streaming"/,
  'A2A 不得恢复多条件平铺工具栏。',
);
assert.doesNotMatch(
  read('src/pages/AI/Mcp/index.tsx'),
  /className=\{style\.protocolTabs\}[\s\S]*名称前缀/,
  'MCP 不得恢复协议快捷按钮与文本条件混排。',
);

console.log('复合查询组件专项契约检查通过');

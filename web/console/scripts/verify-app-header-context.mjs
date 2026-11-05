import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';

const root = process.cwd();
const read = (relativePath) => fs.readFileSync(path.join(root, relativePath), 'utf8');

const resourceLayout = read('src/components/ResourceLayout/index.tsx');
const resourceLayoutStyle = read('src/components/ResourceLayout/index.module.less');

assert.match(
  resourceLayout,
  /appHeaderTarget[\s\S]*createPortal\(header, appHeaderTarget\)[\s\S]*style\.integratedFallback[\s\S]*className=\{style\.integratedActions\}/,
  '集成页头必须只把标题上下文 portal 到全局 Header，并将页面操作保留在正文。',
);
assert.match(
  resourceLayoutStyle,
  /\.resourceHeaderIntegrated[\s\S]*grid-template-columns: auto minmax\(0, 1fr\)[\s\S]*\.headerContent h1[\s\S]*font-size: 14px[\s\S]*line-height: 20px[\s\S]*\.eyebrow[\s\S]*font-size: 14px[\s\S]*line-height: 20px/,
  '顶部路径和资源名必须使用两列布局及统一的 14px / 20px。',
);
assert.match(resourceLayout, /!integrated && description && <p/, '集成页头不得渲染长说明，普通正文页头仍须支持 description。');
assert.doesNotMatch(resourceLayoutStyle, /\.resourceHeaderIntegrated[\s\S]*\.headerContent p/, '集成页头不能保留说明文字样式。');
assert.match(resourceLayout, /const Title = integrated \? 'h1' : 'h2'/, '一级集成页头必须保留 h1 文档标题语义。');
assert.match(resourceLayoutStyle, /\.eyebrow[\s\S]*overflow: hidden[\s\S]*max-width: clamp\([\s\S]*text-overflow: ellipsis/, '顶部路径必须支持受控截断。');
assert.match(resourceLayoutStyle, /\.integratedFallback[\s\S]*margin-bottom: 12px/, '全局 Header 不可用时必须在正文回退展示标题。');
assert.match(
  resourceLayoutStyle,
  /\.integratedActions[\s\S]*justify-content: flex-end[\s\S]*white-space: nowrap/,
  '集成页头的页面操作必须在正文保持右对齐和稳定按钮布局。',
);
assert.match(
  resourceLayoutStyle,
  /@media \(max-width: 980px\)[\s\S]*\.integratedActions[\s\S]*justify-content: flex-start[\s\S]*flex-wrap: wrap/,
  '窄屏下正文操作必须允许换行并左对齐。',
);

const primaryPages = [
  ['命名空间', 'src/pages/Namespace/index.tsx'],
  ['A2A Agent', 'src/pages/AI/A2A/index.tsx'],
  ['MCP 服务', 'src/pages/AI/Mcp/index.tsx'],
  ['统一配置', 'src/pages/Configuration/Unified/index.tsx'],
  ['治理工作台', 'src/pages/Governance/Workbench/index.tsx'],
  ['身份主体', 'src/pages/Auth/Principal/index.tsx'],
  ['访问策略', 'src/pages/Auth/Policy/index.tsx'],
  ['系统配置', 'src/pages/SystemConfiguration/index.tsx'],
  ['逻辑服务', 'src/pages/Discovery/Services/index.tsx'],
  ['逻辑服务详情', 'src/pages/Discovery/Services/LogicalServiceDetail.tsx'],
  ['环境服务详情', 'src/pages/Discovery/Services/Instance/index.tsx'],
  ['系统监控', 'src/pages/Metrics/SystemMonitor/index.tsx'],
  ['服务监控', 'src/pages/Metrics/ServiceMonitor/index.tsx'],
  ['事件指标', 'src/pages/Metrics/ServerEvent/index.tsx'],
  ['操作审计', 'src/pages/Metrics/ServerOperation/index.tsx'],
];

for (const [name, file] of primaryPages) {
  const source = read(file);
  assert.match(
    source,
    /<ResourceHeader[\s\S]*?density="compact"[\s\S]*?placement="app-header"/,
    `${name} 必须使用紧凑的全局顶部资源页头。`,
  );
}

const monitorPages = [
  ['src/pages/Metrics/SystemMonitor/index.tsx', /style\.dashboardHeader|<h1>系统监控<\/h1>/],
  ['src/pages/Metrics/ServiceMonitor/index.tsx', /style\.dashboardHeader|<h1>服务监控<\/h1>/],
  ['src/pages/Metrics/ServerEvent/index.tsx', /style\.headerPanel|<h1>事件指标<\/h1>/],
  ['src/pages/Metrics/ServerOperation/index.tsx', /style\.headerPanel|<h1>操作审计<\/h1>/],
];

for (const [file, legacyHeaderPattern] of monitorPages) {
  assert.doesNotMatch(read(file), legacyHeaderPattern, `${file} 不能继续渲染正文私有大页头。`);
}

console.log('一级板块顶部资源页头统一检查通过。');

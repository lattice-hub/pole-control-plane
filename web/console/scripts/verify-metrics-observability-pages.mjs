import fs from 'node:fs';
import path from 'node:path';
import process from 'node:process';

const root = process.cwd();

const checks = [
  {
    file: 'src/router/modules/metrics.ts',
    expectations: [
      ['routes system monitor before event pages', /path:\s*'system'[\s\S]*pages\/Metrics\/SystemMonitor/],
      ['routes service monitor before event pages', /path:\s*'service'[\s\S]*pages\/Metrics\/ServiceMonitor/],
      ['routes hidden service monitor detail page', /path:\s*'service\/detail'[\s\S]*pages\/Metrics\/ServiceMonitor\/Detail[\s\S]*hidden:\s*true/],
      ['keeps event metrics route', /path:\s*'event'[\s\S]*pages\/Metrics\/ServerEvent/],
      ['keeps operation audit route', /path:\s*'operation'[\s\S]*pages\/Metrics\/ServerOperation/],
    ],
    forbidden: [
      ['must not keep stale control placeholder', /path:\s*'control'/],
      ['must not keep stale microservice placeholder', /path:\s*'microservice'/],
    ],
  },
  {
    file: 'src/locales/zh-CN.json',
    expectations: [
      ['contains system monitor menu label', /"menu\.metrics\.system":\s*"系统监控"/],
      ['contains service monitor menu label', /"menu\.metrics\.service":\s*"服务监控"/],
      ['contains operation audit menu label', /"menu\.metrics\.operation":\s*"操作审计"/],
    ],
    forbidden: [
      ['must not call operation audit operation metrics', /"menu\.metrics\.operation":\s*"操作指标"/],
    ],
  },
  {
    file: 'src/locales/en-US.json',
    expectations: [
      ['contains system monitor English label', /"menu\.metrics\.system":\s*"System Monitor"/],
      ['contains service monitor English label', /"menu\.metrics\.service":\s*"Service Monitor"/],
      ['contains operation audit English label', /"menu\.metrics\.operation":\s*"Operation Audit"/],
    ],
    forbidden: [
      ['must not call operation audit operation metrics', /"menu\.metrics\.operation":\s*"Operation Metrics"/],
    ],
  },
  {
    file: 'src/pages/Metrics/SystemMonitor/index.tsx',
    expectations: [
      ['loads platform overview from observability API', /describePlatformOverview/],
      ['renders system monitor title in shared app header', /<ResourceHeader[\s\S]*title="系统监控"[\s\S]*placement="app-header"|<ResourceHeader[\s\S]*placement="app-header"[\s\S]*title="系统监控"/],
      ['uses grafana-like dashboard wording', /Grafana-like 平台组件看板/],
      ['renders monitor app header path', /eyebrow="监控指标 \/ 系统监控"/],
      ['renders dashboard time range pill', /Last 1 hour/],
      ['renders local timezone pill', /formatTimezoneOffset/],
      ['renders dashboard variables region', /aria-label="Dashboard variables"/],
      ['uses the unified query composer', /<QueryComposer/],
      ['uses component and pod as the main search', /keyword=\{componentKeyword\}[\s\S]*keywordPlaceholder="搜索组件、Pod 或角色"/],
      ['uses the shared reset action', /onReset=\{resetFilters\}/],
      ['contains category advanced filter', /key:\s*'category'[\s\S]*label:\s*'类别'/],
      ['contains api advanced filter', /key:\s*'api'[\s\S]*label:\s*'接口'/],
      ['renders explicit observability query failure state', /观测数据查询失败/],
      ['renders component resource board', /组件资源看板/],
      ['renders go runtime board', /Go runtime 看板/],
      ['renders go runtime board as full-width panel', /title="Go runtime 看板"[\s\S]*className=\{style\.panelFull\}/],
      ['renders panel query metadata', /query="pole_control_plane_request_duration_seconds"[\s\S]*query="process_runtime_go_\*"/],
      ['renders main time series hover tooltip', /timeSeriesTooltip/],
      ['main time series has timeline and y axis', /className=\{style\.chartFrame\}[\s\S]*className=\{style\.yAxis\}[\s\S]*className=\{style\.axisLabels\}/],
      ['main time series reacts to mouse movement', /接口延迟趋势，包含时间线和纵坐标，悬浮查看采样值[\s\S]*onMouseMove=\{updateHover\}/],
      ['main time series uses local query time labels', /buildTimeLabels\(timeRange\)[\s\S]*<TimeSeriesPanel rows=\{filteredRows\} timeLabels=\{timeLabels\}/],
      ['heatmap has timeline and y dimension', /className=\{style\.heatmapXAxis\}[\s\S]*className=\{style\.heatmapYAxis\}/],
      ['renders runtime trend hover tooltip', /tinyTrendTooltip/],
      ['runtime stat uses sparkline with time range', /stat sparkline[\s\S]*className=\{style\.tinyTimeRange\}/],
      ['runtime sparkline keeps readable canvas ratio', /const width = 280[\s\S]*const height = 64/],
      ['runtime sparkline uses grafana-like area fill', /className=\{style\.tinyTrendArea\}[\s\S]*className=\{style\.tinyTrendLine\}/],
      ['runtime trend reacts to mouse movement', /onMouseMove=\{updateHover\}/],
      ['runtime trend receives labels and metric unit for tooltip', /<TinyTrend values=\{row\.series\} label=\{row\.title\} timeLabels=\{timeLabels\} unit=\{row\.unit\}/],
      ['consumes platform resource metrics field', /overview\.resources/],
      ['consumes platform runtime metrics field', /overview\.runtime/],
      ['renders interface detail table', /接口明细/],
      ['renders accurate request count and qps columns', /title:\s*'请求量'[\s\S]*title:\s*'QPS'/],
      ['opens trace drilldown from interface detail', /title:\s*'链路'[\s\S]*查看 Trace[\s\S]*<TraceDrawer/],
      ['uses OTel resource labels copy', /OTel resource labels/],
      ['paginates interface detail rows', /const INTERFACE_DETAIL_PAGE_SIZE = 10[\s\S]*title="接口明细"[\s\S]*pagination=\{\{[\s\S]*pageSize: INTERFACE_DETAIL_PAGE_SIZE[\s\S]*total: filteredRows\.length[\s\S]*showJumper: true/],
      ['resets interface detail pagination with filters', /const interfacePaginationKey = \[category, api, componentKeyword\]\.join\('\|'\)[\s\S]*<Table[\s\S]*key=\{interfacePaginationKey\}/],
    ],
    forbidden: [
      ['must not use fixed UTC-looking time labels', /const TIME_LABELS/],
      ['must not disable interface detail pagination', /title="接口明细"[\s\S]*pagination=\{false\}/],
      ['must not keep system monitor mock component rows', /SYSTEM_ROWS/],
      ['must not keep system monitor mock runtime rows', /MOCK_RUNTIME_ROWS/],
      ['must not render mock preview state', /Mock 预览/],
    ],
  },
  {
    file: 'src/pages/Metrics/SystemMonitor/TraceDrawer.tsx',
    expectations: [
      ['renders trace as an in-content workspace', /<main ref=\{workspaceRef\} className=\{style\.workspace\} aria-label="接口 Trace 工作区">/],
      ['keeps the global app header visible', /placement="app-header"[\s\S]*aria-label="Trace 页面导航"[\s\S]*返回系统监控/],
      ['resets inherited dashboard scroll on entry', /useLayoutEffect[\s\S]*scrollIntoView\(\{ block: 'start' \}\)/],
      ['supports browser trace navigation', /window\.history\.pushState[\s\S]*window\.addEventListener\('popstate'/],
      ['keeps list and detail back actions distinct', /返回系统监控[\s\S]*返回 Trace 列表/],
      ['queries traces by interface time window', /describePlatformTraces\([\s\S]*start_time:[\s\S]*end_time:[\s\S]*api:/],
      ['loads selected trace details', /describeTrace\(trace\.trace_id\)/],
      ['renders trace list', /同接口请求链路/],
      ['renders span waterfall', /Span 瀑布/],
      ['orders spans as an explicit parent child tree', /buildSpanTreeRows[\s\S]*role="tree"[\s\S]*role="treeitem"/],
      ['supports expanding and collapsing the span tree', /全部展开[\s\S]*全部折叠[\s\S]*aria-expanded/],
      ['renders a global timeline overview', /全局时间概览[\s\S]*overviewTrack/],
      ['renders selected span details', /选中 Span[\s\S]*开始偏移[\s\S]*Parent Span/],
      ['uses microsecond timestamps when available', /start_time_us[\s\S]*spanStartUS/],
      ['renders real empty trace state', /当前时间窗暂无 Trace/],
    ],
    forbidden: [
      ['must not use a full-screen overlay drawer', /<Drawer|OverlayDrawer|size="full"/],
      ['must not render mock traces', /MOCK_TRACE|Mock Trace/],
    ],
  },
  {
    file: 'src/pages/Metrics/SystemMonitor/index.module.less',
    expectations: [
      ['uses a balanced wide-screen dashboard grid', /\.dashboardGrid\s*\{[\s\S]*grid-template-columns:\s*minmax\(0, 1\.35fr\) minmax\(360px, 0\.9fr\)/],
      ['balances five stat cards as three plus two on compact desktops', /@media \(max-width:\s*1280px\)[\s\S]*\.statGrid\s*\{[\s\S]*grid-template-columns:\s*repeat\(6,[\s\S]*:nth-child\(n \+ 4\)[\s\S]*grid-column:\s*span 3;/],
      ['keeps the two-column dashboard until compact desktop widths', /@media \(max-width:\s*1080px\)[\s\S]*\.dashboardGrid\s*\{[\s\S]*grid-template-columns:\s*1fr;/],
      ['caps heatmap height with an internal scroller', /\.heatmapBody\s*\{[\s\S]*overflow-y:\s*auto;[\s\S]*max-height:\s*172px;/],
      ['keeps heatmap cells compact', /\.heatmapRow span\s*\{[\s\S]*height:\s*22px;/],
    ],
    forbidden: [],
  },
  {
    file: 'src/pages/Metrics/ServiceMonitor/index.tsx',
    expectations: [
      ['contains service signal mock rows', /SERVICE_SIGNALS/],
      ['renders service monitor title in shared app header', /<ResourceHeader[\s\S]*title="服务监控"[\s\S]*placement="app-header"|<ResourceHeader[\s\S]*placement="app-header"[\s\S]*title="服务监控"/],
      ['uses service level overview wording', /服务级整体概览/],
      ['uses the unified query composer', /<QueryComposer/],
      ['contains service main search', /keyword=\{service\}[\s\S]*keywordPlaceholder="搜索命名空间 \/ 服务"/],
      ['contains governance advanced filter', /key:\s*'kind'[\s\S]*label:\s*'治理能力'/],
      ['renders service overview table', /服务级总览/],
      ['places filters between numeric overview and service table', /style\.statGrid[\s\S]*style\.variableBar[\s\S]*服务级总览/],
      ['opens independent drilldown detail page', /navigate\(`\/metrics\/service\/detail\?/],
      ['renders drilldown action', /title:\s*'下钻'/],
    ],
    forbidden: [
      ['must not put trend panel before service overview table', /服务流量趋势|TrafficSeries/],
      ['must not put governance distribution panel before service overview table', /治理分布|GovernanceDistribution|distributionList/],
      ['must not keep dashboard grid on service overview', /dashboardGrid/],
      ['must not keep api filter on service overview', /placeholder="接口"|setApi|matchApi/],
      ['must not keep instance filter on service overview', /placeholder="实例"|setInstance|matchInstance/],
    ],
  },
  {
    file: 'src/pages/Metrics/ServiceMonitor/data.ts',
    expectations: [
      ['covers rate limit signal', /ratelimit/],
      ['covers route signal', /route/],
      ['covers circuit breaker signal', /circuitbreaker/],
      ['covers fault detect signal', /faultdetect/],
      ['covers auth signal', /auth/],
      ['covers mock signal', /mock/],
      ['covers mirror signal', /mirror/],
      ['keeps language labels for runtime metrics', /language:\s*'java'[\s\S]*language:\s*'go'[\s\S]*language:\s*'rust'/],
    ],
    forbidden: [],
  },
  {
    file: 'src/pages/Metrics/ServiceMonitor/Detail.tsx',
    expectations: [
      ['renders service monitor breadcrumb', /BreadcrumbItem onClick=\{\(\) => navigate\('\/metrics\/service'\)\}>服务监控/],
      ['renders api search', /placeholder="输入接口名称"/],
      ['renders explicit server call perspective', /服务端调用视角[\s\S]*按当前服务接收的请求聚合/],
      ['renders interface overview tab', /label="接口概览"/],
      ['renders instance detail tab', /label="实例详情"/],
      ['renders traffic tab', /label="调用流量"/],
      ['renders route tab', /label="路由"/],
      ['renders rate limit tab', /label="限流"/],
      ['renders circuit breaker tab', /label="熔断"/],
      ['renders fault detect tab', /label="探测"/],
      ['renders auth tab', /label="鉴权"/],
      ['renders mock tab', /label="Mock"/],
      ['renders mirror tab', /label="镜像"/],
      ['renders event timeline tab', /label="事件时间线"/],
      ['renders instance call overview instead of runtime cards', /实例调用概览[\s\S]*service\.instance\.id 聚合接口请求/],
      ['mentions service labels binding', /service\.name[\s\S]*service\.namespace[\s\S]*service\.instance\.id/],
    ],
    forbidden: [
      ['must not split service monitor by web/rpc service type', /WEB服务|RPC服务|服务协议|protocol/],
      ['must not expose client perspective without direction data', />客户端<\/Button>/],
      ['must not render language runtime metric cards in instance detail', /运行时指标|JVM Heap|GC Pause|Goroutine|Tokio Tasks|runtimeMetrics|RuntimeMetric|runtimeGrid|runtimeItem/],
    ],
  },
  {
    file: 'src/pages/Metrics/ServerEvent/index.tsx',
    expectations: [
      ['reports event query errors and supports retry', /setFetchError\(true\)[\s\S]*事件查询失败[\s\S]*fetchData\(lastQuery\.current\)/],
      ['labels unavailable event trend', /当前暂不提供趋势统计[\s\S]*暂无趋势数据/],
      ['uses observability event API first', /describeObservabilityEvents/],
      ['keeps remote event API call', /describeEventLog/],
      ['falls back to legacy event metrics API', /describeObservabilityEvents\(requestParams\)[\s\S]*describeEventLog\(requestParams\)/],
      ['maps event resource filter to resource', /resource:\s*nextSearchState\.searchResource/],
      ['maps event type filter to event_type', /event_type:\s*nextSearchState\.searchEvent/],
      ['renders event detail drawer', /<Drawer[\s\S]*selectedEvent/],
      ['renders event summary', /buildEventSummary/],
    ],
    forbidden: [
      ['must not substitute mock event data', /MOCK_EVENTS|EVENT_TREND|filterMockEvents/],
      ['must not swap resource with event filter', /resource:\s*nextSearchState\.searchEvent/],
      ['must not swap event type with resource filter', /event_type:\s*nextSearchState\.searchResource/],
    ],
  },
  {
    file: 'src/pages/Metrics/ServerOperation/index.tsx',
    expectations: [
      ['reports operation query errors and supports retry', /setFetchError\(true\)[\s\S]*操作查询失败[\s\S]*fetchData\(lastQuery\.current\)/],
      ['labels unavailable operation trend', /当前暂不提供趋势统计[\s\S]*暂无趋势数据/],
      ['uses observability operation API first', /describeObservabilityOperations/],
      ['keeps remote operation API call', /describeOperationLog/],
      ['falls back to legacy operation metrics API', /describeObservabilityOperations\(requestParams\)[\s\S]*describeOperationLog\(requestParams\)/],
      ['passes resource type filter', /resource_type:\s*nextSearchState\.searchResourceType/],
      ['passes operation type filter', /operation_type:\s*nextSearchState\.searchOperation/],
      ['passes operator filter', /operator:\s*nextSearchState\.searchOperator/],
      ['normalizes operation detail compatibility field', /normalizeOperationRows[\s\S]*operation_detail/],
      ['uses operation detail field from API contract', /colKey:\s*'detail'/],
      ['renders operation detail drawer', /<Drawer[\s\S]*selectedOperation/],
    ],
    forbidden: [
      ['must not substitute mock operation data', /MOCK_OPERATIONS|OPERATION_TREND|filterMockOperations/],
      ['must not render old operation_detail column', /colKey:\s*'operation_detail'/],
    ],
  },
];

let failed = false;

for (const check of checks) {
  const target = path.join(root, check.file);
  const content = fs.readFileSync(target, 'utf8');

  for (const [label, pattern] of check.expectations) {
    if (!pattern.test(content)) {
      console.error(`[metrics-observability] ${check.file}: missing ${label}`);
      failed = true;
    }
  }

  for (const [label, pattern] of check.forbidden) {
    if (pattern.test(content)) {
      console.error(`[metrics-observability] ${check.file}: forbidden ${label}`);
      failed = true;
    }
  }
}

if (failed) {
  process.exit(1);
}

console.log('[metrics-observability] system, service, event and operation pages passed static checks');

import React from 'react';
import { Breadcrumb, Button, Empty, Input, Table, TableColumnData, TabPanel, Tabs, Tag } from 'components/Fluent';
import { ArrowRightIcon, SearchIcon, ServiceIcon } from 'components/Fluent/icons';
import { useNavigate, useSearchParams } from 'components/Router';

import {
  GOVERNANCE_LABEL,
  GovernanceKind,
  SERVICE_SIGNALS,
  ServiceSignal,
  TIME_LABELS,
  average,
  buildPath,
  formatNumber,
  serviceKey,
} from './data';
import style from './index.module.less';

const { BreadcrumbItem } = Breadcrumb;

type ApiOverview = {
  api: string;
  requests: number;
  blocked: number;
  p95: number;
  p99: number;
  successRate: number;
  errorRate: number;
  rows: ServiceSignal[];
};

function toneClass(status: ServiceSignal['status']) {
  if (status === 'critical') return style.toneDanger;
  if (status === 'warning') return style.toneWarning;
  return style.toneSuccess;
}

function governanceToneClass(kind: GovernanceKind) {
  if (kind === 'circuitbreaker' || kind === 'ratelimit') return style.toneWarning;
  if (kind === 'auth') return style.toneSuccess;
  return style.toneInfo;
}

function Panel(props: { title: string; subtitle?: string; children: React.ReactNode; className?: string }) {
  const { title, subtitle, children, className } = props;
  return (
    <section className={`${style.panel} ${className || ''}`}>
      <div className={style.panelHeader}>
        <div>
          <h2>{title}</h2>
          {subtitle && <span>{subtitle}</span>}
        </div>
      </div>
      {children}
    </section>
  );
}

function StatPanel(props: { title: string; value: React.ReactNode; hint: string; tone?: string }) {
  const { title, value, hint, tone } = props;
  return (
    <Panel title={title} className={`${style.statPanel} ${tone || ''}`}>
      <strong className={style.statValue}>{value}</strong>
      <span className={style.statHint}>{hint}</span>
    </Panel>
  );
}

function sumSeries(rows: ServiceSignal[]) {
  if (!rows.length) return [];
  return rows.map((row) => row.series).reduce((acc, current) => acc.map((value, index) => Math.round(value + current[index])));
}

function TrafficSeries(props: { rows: ServiceSignal[]; label: string }) {
  const { rows, label } = props;
  const series = sumSeries(rows);
  const width = 300;
  const height = 120;
  return (
    <div className={style.timeSeries}>
      {series.length ? (
        <svg viewBox={`0 0 ${width} ${height}`} role="img" aria-label={label}>
          <path className={style.gridLine} d="M 0 24 L 300 24 M 0 60 L 300 60 M 0 96 L 300 96" />
          <path className={style.seriesArea} d={`${buildPath(series, width, height)} L ${width} ${height} L 0 ${height} Z`} />
          <path className={style.seriesLine} d={buildPath(series, width, height)} />
        </svg>
      ) : <Empty description="暂无趋势数据" />}
      <div className={style.axisLabels}>
        <span>{TIME_LABELS[0]}</span>
        <span>{TIME_LABELS[Math.floor(TIME_LABELS.length / 2)]}</span>
        <span>{TIME_LABELS[TIME_LABELS.length - 1]}</span>
      </div>
    </div>
  );
}

function apiOverview(rows: ServiceSignal[]): ApiOverview[] {
  const groups = rows.reduce<Record<string, ServiceSignal[]>>((acc, row) => {
    acc[row.api] = acc[row.api] || [];
    acc[row.api].push(row);
    return acc;
  }, {});
  return Object.entries(groups).map(([api, group]) => ({
    api,
    requests: group.reduce((sum, row) => sum + row.requests, 0),
    blocked: group.reduce((sum, row) => sum + row.blocked, 0),
    p95: Math.round(average(group, 'p95')),
    p99: Math.round(average(group, 'p99')),
    successRate: Number(average(group, 'successRate').toFixed(2)),
    errorRate: Number(average(group, 'errorRate').toFixed(2)),
    rows: group,
  })).sort((a, b) => b.requests - a.requests);
}

function RulePanel(props: { kind: GovernanceKind; rows: ServiceSignal[] }) {
  const { kind, rows } = props;
  const targets = rows.filter((row) => row.kind === kind);
  return (
    <div className={style.ruleList}>
      {targets.map((row) => (
        <div className={style.ruleItem} key={row.id}>
          <div>
            <Tag className={governanceToneClass(row.kind)}>{GOVERNANCE_LABEL[row.kind]}</Tag>
            <strong>{row.label}</strong>
          </div>
          <span>{row.api}</span>
          <dl>
            <div><dt>请求量</dt><dd>{formatNumber(row.requests)}</dd></div>
            <div><dt>命中率</dt><dd>{row.hitRate}%</dd></div>
            <div><dt>拦截/降级</dt><dd>{formatNumber(row.blocked)}</dd></div>
            <div><dt>实例</dt><dd>{row.instance}</dd></div>
          </dl>
        </div>
      ))}
      {!targets.length && <Empty description={`当前接口暂无${GOVERNANCE_LABEL[kind]}信号`} />}
    </div>
  );
}

export default function ServiceMonitorDetail() {
  const pageRef = React.useRef<HTMLDivElement>(null);
  const navigate = useNavigate();
  const [searchParams] = useSearchParams();
  const namespaceParam = searchParams.get('namespace') ?? SERVICE_SIGNALS[0].namespace;
  const serviceParam = searchParams.get('service') ?? SERVICE_SIGNALS[0].service;
  const serviceRows = React.useMemo(() => SERVICE_SIGNALS.filter((row) => row.namespace === namespaceParam && row.service === serviceParam), [namespaceParam, serviceParam]);
  const fallbackRows = serviceRows.length ? serviceRows : SERVICE_SIGNALS.filter((row) => serviceKey(row) === serviceKey(SERVICE_SIGNALS[0]));
  const serviceName = serviceKey(fallbackRows[0]);
  const language = fallbackRows[0].language;
  const apis = React.useMemo(() => apiOverview(fallbackRows), [fallbackRows]);
  const [apiSearch, setApiSearch] = React.useState('');
  const [selectedApi, setSelectedApi] = React.useState(searchParams.get('api') || apis[0]?.api || '');

  React.useLayoutEffect(() => {
    pageRef.current?.scrollIntoView({ block: 'start' });
  }, []);

  React.useEffect(() => {
    setSelectedApi((current) => (apis.some((item) => item.api === current) ? current : apis[0]?.api || ''));
  }, [apis]);

  const selectedApiData = apis.find((item) => item.api === selectedApi) || apis[0];
  const selectedRows = selectedApiData?.rows || fallbackRows;
  const totalRequests = fallbackRows.reduce((sum, item) => sum + item.requests, 0);
  const totalBlocked = fallbackRows.reduce((sum, item) => sum + item.blocked, 0);
  const warningCount = fallbackRows.filter((item) => item.status !== 'normal').length;
  const filteredApis = apis.filter((item) => item.api.toLowerCase().includes(apiSearch.toLowerCase()));
  const instanceRows = Array.from(new Map(selectedRows.map((row) => [row.instance, row])).values());

  const instanceColumns = React.useMemo<TableColumnData<ServiceSignal>[]>(() => [
    { colKey: 'instance', title: '实例', width: 170, ellipsis: true },
    { colKey: 'resource', title: 'CPU / MEM', width: 120, align: 'right', cell: ({ row }) => `${row.cpu}% / ${row.memory}%` },
    { colKey: 'latency', title: 'P95 / P99', width: 130, align: 'right', cell: ({ row }) => `${row.p95}ms / ${row.p99}ms` },
    { colKey: 'successRate', title: '成功率', width: 90, align: 'right', cell: ({ row }) => `${row.successRate}%` },
    { colKey: 'status', title: '状态', width: 90, cell: ({ row }) => <Tag className={toneClass(row.status)}>{row.status === 'normal' ? '正常' : '关注'}</Tag> },
  ], []);

  const signalColumns = React.useMemo<TableColumnData<ServiceSignal>[]>(() => [
    { colKey: 'kind', title: '治理能力', width: 110, cell: ({ row }) => <Tag className={governanceToneClass(row.kind)}>{GOVERNANCE_LABEL[row.kind]}</Tag> },
    { colKey: 'label', title: '规则 / 信号', width: 180, ellipsis: true },
    { colKey: 'api', title: '接口', width: 220, ellipsis: true },
    { colKey: 'instance', title: '实例', width: 160, ellipsis: true },
    { colKey: 'requests', title: '请求量', width: 110, align: 'right', cell: ({ row }) => formatNumber(row.requests) },
    { colKey: 'hitRate', title: '命中率', width: 90, align: 'right', cell: ({ row }) => `${row.hitRate}%` },
    { colKey: 'blocked', title: '拦截 / 降级', width: 120, align: 'right', cell: ({ row }) => formatNumber(row.blocked) },
  ], []);

  if (!serviceRows.length) {
    return (
      <div className={style.page} ref={pageRef}>
        <Empty title="未找到该服务的监控数据" description={`当前监控仅提供示例数据，未包含 ${namespaceParam}/${serviceParam}。`}
          action={<Button onClick={() => navigate('/metrics/service')}>返回服务监控</Button>} />
      </div>
    );
  }

  return (
    <div className={style.page} ref={pageRef}>
      <Breadcrumb className={style.detailBreadcrumb} maxItemWidth="220px">
        <BreadcrumbItem onClick={() => navigate('/metrics/service')}>服务监控</BreadcrumbItem>
        <BreadcrumbItem>{serviceName}</BreadcrumbItem>
      </Breadcrumb>

      <section className={style.detailHero}>
        <div className={style.headerTitle}>
          <span className={style.headerIcon}><ServiceIcon /></span>
          <div>
            <h1>{serviceName}</h1>
            <Tag variant="outline">示例数据 · 未连接实际服务观测</Tag>
            <p>service.name、service.namespace、service.instance.id 与语言标签共同绑定调用指标、系统指标和治理事件。</p>
          </div>
        </div>
      </section>

      <section className={style.serviceMetaBar}>
        <div><span>语言体系</span><strong>{language.toUpperCase()}</strong></div>
        <div><span>OTel service.name</span><strong>{fallbackRows[0].service}</strong></div>
        <div><span>OTel service.namespace</span><strong>{fallbackRows[0].namespace}</strong></div>
        <div><span>接口数</span><strong>{apis.length}</strong></div>
        <div><span>实例数</span><strong>{new Set(fallbackRows.map((row) => row.instance)).size}</strong></div>
      </section>

      <div className={style.serviceDetailShell}>
        <aside className={style.apiPane}>
          <Input value={apiSearch} clearable placeholder="输入接口名称" prefixIcon={<SearchIcon />} onChange={setApiSearch} />
          <div className={style.apiPanePerspective}>
            <Tag theme="primary" variant="light">服务端调用视角</Tag>
            <span>按当前服务接收的请求聚合</span>
          </div>
          <div className={style.apiListHeader}>
            <span>接口名称</span>
            <span>请求 / 拒绝 / RT / 成功率</span>
          </div>
          <div className={style.apiList}>
            {filteredApis.map((item) => (
              <Button
                className={`${style.apiItem} ${item.api === selectedApi ? style.apiItemActive : ''}`}
                key={item.api}
                type="button"
                variant="text"
                onClick={() => setSelectedApi(item.api)}
              >
                <strong>{item.api}</strong>
                <span>请求量：{formatNumber(item.requests)}</span>
                <span>拒绝量：{formatNumber(item.blocked)}</span>
                <span>RT：{item.p95}ms</span>
                <span>成功率：{item.successRate}%</span>
              </Button>
            ))}
            {!filteredApis.length && <Empty description="暂无匹配接口" />}
          </div>
        </aside>

        <main className={style.detailMain}>
          <Tabs defaultValue="overview" className={style.detailTabs}>
            <TabPanel label="接口概览" value="overview">
              <div className={style.statGrid}>
                <StatPanel title="接口请求量" value={formatNumber(selectedApiData?.requests || 0)} hint={selectedApi || '未选择接口'} />
                <StatPanel title="拒绝 / 降级" value={formatNumber(selectedApiData?.blocked || 0)} hint="限流、鉴权、熔断聚合" tone={selectedApiData?.blocked ? style.panelWarning : undefined} />
                <StatPanel title="P95 / P99" value={`${selectedApiData?.p95 || 0}ms / ${selectedApiData?.p99 || 0}ms`} hint="接口级调用延迟" />
                <StatPanel title="成功率" value={`${selectedApiData?.successRate || 0}%`} hint={`错误率 ${selectedApiData?.errorRate || 0}%`} />
                <StatPanel title="服务请求量" value={formatNumber(totalRequests)} hint={`服务总拒绝 ${formatNumber(totalBlocked)}，关注 ${warningCount}`} />
              </div>
              <section className={style.methodMeta}>
                <div><span>类名</span><strong>com.pole.demo.{fallbackRows[0].service}.Controller</strong></div>
                <div><span>方法</span><strong>{selectedApi}</strong></div>
                <div><span>参数</span><strong>request metadata / body</strong></div>
                <div><span>返回值</span><strong>response payload</strong></div>
              </section>
              <div className={style.detailPanelGrid}>
                <Panel title="QPS 数据（秒级）" subtitle="通过 / 拒绝 / 异常 / 总流量">
                  <TrafficSeries rows={selectedRows} label="接口 QPS 趋势" />
                </Panel>
                <Panel title="RT 数据（ms）" subtitle="平均 RT 与 P95/P99 联动">
                  <TrafficSeries rows={selectedRows.map((row) => ({ ...row, series: row.series.map((value) => Math.max(1, Math.round(value / 2))) }))} label="接口 RT 趋势" />
                </Panel>
              </div>
            </TabPanel>
            <TabPanel label="实例详情" value="instances">
              <div className={style.detailPanelGrid}>
                <Panel title="实例调用概览" subtitle="按 service.instance.id 聚合接口请求、延迟和成功率">
                  <TrafficSeries rows={selectedRows} label="实例调用趋势" />
                </Panel>
                <Panel title="实例延迟与资源" subtitle="service.instance.id 绑定系统资源和调用指标">
                  <Table data={instanceRows} columns={instanceColumns} rowKey="instance" pagination={false} tableLayout="fixed" />
                </Panel>
              </div>
            </TabPanel>
            <TabPanel label="调用流量" value="traffic">
              <Panel title="调用指标明细" subtitle="接口、实例、治理能力和请求指标的统一视图">
                <Table data={selectedRows} columns={signalColumns} rowKey="id" pagination={false} tableLayout="fixed" />
              </Panel>
            </TabPanel>
            <TabPanel label="路由" value="route"><RulePanel kind="route" rows={selectedRows} /></TabPanel>
            <TabPanel label="限流" value="ratelimit"><RulePanel kind="ratelimit" rows={selectedRows} /></TabPanel>
            <TabPanel label="熔断" value="circuitbreaker"><RulePanel kind="circuitbreaker" rows={selectedRows} /></TabPanel>
            <TabPanel label="探测" value="faultdetect"><RulePanel kind="faultdetect" rows={selectedRows} /></TabPanel>
            <TabPanel label="鉴权" value="auth"><RulePanel kind="auth" rows={selectedRows} /></TabPanel>
            <TabPanel label="Mock" value="mock"><RulePanel kind="mock" rows={selectedRows} /></TabPanel>
            <TabPanel label="镜像" value="mirror"><RulePanel kind="mirror" rows={selectedRows} /></TabPanel>
            <TabPanel label="事件时间线" value="events">
              <div className={style.timeline}>
                {selectedRows.map((row) => (
                  <div className={style.timelineItem} key={`${row.id}-event`}>
                    <i />
                    <strong>{GOVERNANCE_LABEL[row.kind]} · {row.label}</strong>
                    <span>{row.api} 在 {row.instance} 命中 {row.hitRate}% ，请求量 {formatNumber(row.requests)}</span>
                  </div>
                ))}
                <Button
                  variant="outline"
                  icon={<ArrowRightIcon />}
                  onClick={() => {
                    const params = new URLSearchParams({
                      namespace: namespaceParam,
                      resource_name: serviceParam,
                    });
                    navigate(`/metrics/event?${params.toString()}`);
                  }}
                >
                  带过滤条件查看事件指标
                </Button>
              </div>
            </TabPanel>
          </Tabs>
        </main>
      </div>
    </div>
  );
}

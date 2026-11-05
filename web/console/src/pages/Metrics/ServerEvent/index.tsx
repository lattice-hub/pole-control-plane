import React from 'react';
import dayjs from 'dayjs';
import { Button, DateRangePicker, DateRangePickerProps, Drawer, Empty, Table, TableColumnData, Tag } from 'components/Fluent';
import { FilterIcon, InfoCircleIcon, RefreshIcon } from 'components/Fluent/icons';
import QueryComposer from 'components/QueryComposer';
import { ResourceHeader } from 'components/ResourceLayout';
import { describeEventLog, EventLog, EventType, getEventTypeInfo } from 'services/observer';
import { describeObservabilityEvents } from 'services/observability';
import { useSearchParams } from 'components/Router';

import style from './index.module.less';

type EventMetricRow = EventLog & {
  rowKey?: string;
  [key: string]: unknown;
};

interface SearchState {
  searchEvent: string;
  searchNamespace: string;
  searchService: string;
  searchResource: string;
  startTime: string;
  endTime: string;
}

const PAGE_SIZE = 10;

const presets: DateRangePickerProps['presets'] = {
  '最近 1 小时': [dayjs().subtract(1, 'hour').toDate(), dayjs().toDate()],
  '最近 6 小时': [dayjs().subtract(6, 'hour').toDate(), dayjs().toDate()],
  '最近 24 小时': [dayjs().subtract(24, 'hour').toDate(), dayjs().toDate()],
};

const defaultSearchState: SearchState = {
  searchEvent: '',
  searchNamespace: '',
  searchService: '',
  searchResource: '',
  startTime: '',
  endTime: '',
};

function resolveEventLabel(eventType: string) {
  return getEventTypeInfo(eventType)?.label || eventType || '-';
}

function resolveEventCategory(eventType: string) {
  return getEventTypeInfo(eventType)?.category || '未知事件';
}

function isRiskEvent(eventType: string) {
  return ['InstanceOffline', 'InstanceTurnUnHealth', 'InstanceOpenIsolate', 'ServiceOpenEmptyPushProtect', 'ServiceExpireEmptyPushProtect'].includes(eventType);
}

function isRecoveryEvent(eventType: string) {
  return ['InstanceOnline', 'InstanceTurnHealth', 'InstanceCloseIsolate', 'ServiceCloseEmptyPushProtect'].includes(eventType);
}

function eventToneClass(eventType: string) {
  if (isRiskEvent(eventType)) return style.toneDanger;
  if (isRecoveryEvent(eventType)) return style.toneSuccess;
  return style.toneInfo;
}

function formatTime(value: string) {
  if (!value) return '-';
  const date = dayjs(value);
  return date.isValid() ? date.format('YYYY-MM-DD HH:mm:ss') : value;
}

function buildCategoryStats(rows: EventMetricRow[]) {
  const total = Math.max(rows.length, 1);
  const categories = rows.reduce<Record<string, number>>((acc, row) => {
    const category = resolveEventCategory(row.event_type);
    acc[category] = (acc[category] || 0) + 1;
    return acc;
  }, {});
  return Object.entries(categories).map(([label, value]) => ({
    label,
    value,
    percent: Math.round((value / total) * 100),
  }));
}

function buildEventSummary(rows: EventMetricRow[]) {
  const affectedServices = new Set(rows.map((row) => `${row.namespace}/${row.service}`));
  const riskCount = rows.filter((row) => isRiskEvent(row.event_type)).length;
  const latestEvent = rows[0];
  return {
    total: rows.length,
    riskCount,
    affectedServices: affectedServices.size,
    latestTime: latestEvent ? formatTime(latestEvent.event_time) : '-',
  };
}

function SummaryCard(props: { title: string; value: React.ReactNode; hint: string; tone?: string; compactValue?: boolean }) {
  const { title, value, hint, tone, compactValue } = props;
  return (
    <section className={`${style.summaryCard} ${tone || ''}`}>
      <span className={style.summaryTitle}>{title}</span>
      <strong className={`${style.summaryValue} ${compactValue ? style.summaryValueCompact : ''}`}>{value}</strong>
      <span className={style.summaryHint}>{hint}</span>
    </section>
  );
}

function DistributionList(props: { data: Array<{ label: string; value: number; percent: number }> }) {
  const { data } = props;
  if (!data.length) return <Empty description="暂无事件分布" />;
  return (
    <div className={style.distributionList}>
      {data.map((item) => (
        <div className={style.distributionItem} key={item.label}>
          <div className={style.distributionMeta}>
            <span>{item.label}</span>
            <strong>{item.value}</strong>
          </div>
          <span className={style.progressTrack}>
            <span className={style.progressFill} style={{ width: `${item.percent}%` }} />
          </span>
        </div>
      ))}
    </div>
  );
}

export default function ServerEvent() {
  const [searchParams] = useSearchParams();
  const initialSearchState = React.useRef<SearchState>({
    ...defaultSearchState,
    searchNamespace: searchParams.get('namespace') || '',
    searchService: searchParams.get('resource_name') || '',
  });
  const [events, setEvents] = React.useState<EventMetricRow[]>([]);
  const [searchState, setSearchState] = React.useState<SearchState>(initialSearchState.current);
  const [isLoading, setIsLoading] = React.useState(true);
  const [hasNext, setHasNext] = React.useState(false);
  const [fetchError, setFetchError] = React.useState(false);
  const [selectedEvent, setSelectedEvent] = React.useState<EventMetricRow | null>(null);
  const [paginationVersion, setPaginationVersion] = React.useState(0);

  const displayEvents = events;

  const summary = React.useMemo(() => buildEventSummary(displayEvents), [displayEvents]);
  const categoryStats = React.useMemo(() => buildCategoryStats(displayEvents), [displayEvents]);

  const lastQuery = React.useRef<SearchState>(initialSearchState.current);

  const requestSequence = React.useRef(0);

  const fetchData = React.useCallback(async (nextSearchState: SearchState = searchState) => {
    const sequence = ++requestSequence.current;
    lastQuery.current = nextSearchState;
    setIsLoading(true);
    try {
      const requestParams = {
        namespace: nextSearchState.searchNamespace,
        service: nextSearchState.searchService,
        resource: nextSearchState.searchResource,
        event_type: nextSearchState.searchEvent,
        start_time: nextSearchState.startTime,
        end_time: nextSearchState.endTime,
        limit: PAGE_SIZE,
        cursor: 0,
        direction: 'next',
      } as const;
      const observabilityRes = await describeObservabilityEvents(requestParams);
      const fallbackRes = (observabilityRes.data || []).length ? observabilityRes : await describeEventLog(requestParams);
      const res = fallbackRes;
      const remoteRows = (res.data || []) as EventMetricRow[];
      if (sequence !== requestSequence.current) return;
      setEvents(remoteRows);
      setHasNext(Boolean(res.has_next));
      setFetchError(false);
    } catch {
      if (sequence !== requestSequence.current) return;
      setEvents([]);
      setHasNext(false);
      setFetchError(true);
    } finally {
      if (sequence === requestSequence.current) setIsLoading(false);
    }
  }, [searchState]);

  React.useEffect(() => {
    fetchData(initialSearchState.current);
  }, []);

  const columns = React.useMemo<TableColumnData<EventMetricRow>[]>(() => [
    {
      colKey: 'event_type',
      title: '事件',
      width: 180,
      cell: ({ row }) => (
        <div className={style.eventCell}>
          <Tag className={eventToneClass(row.event_type)}>{resolveEventLabel(row.event_type)}</Tag>
          <span>{resolveEventCategory(row.event_type)}</span>
        </div>
      ),
    },
    {
      colKey: 'namespace',
      title: '命名空间',
      width: 120,
      ellipsis: true,
    },
    {
      colKey: 'service',
      title: '服务',
      width: 140,
      ellipsis: true,
    },
    {
      colKey: 'resource',
      title: '资源',
      width: 180,
      ellipsis: true,
    },
    {
      colKey: 'server',
      title: '上报节点',
      width: 150,
      ellipsis: true,
    },
    {
      colKey: 'event_time',
      title: '发生时间',
      width: 170,
      cell: ({ row }) => formatTime(row.event_time),
    },
  ], []);

  const updateSearchState = (patch: Partial<SearchState>) => {
    setSearchState((previous) => ({ ...previous, ...patch }));
  };

  const handleReset = () => {
    setSearchState(defaultSearchState);
    setPaginationVersion((version) => version + 1);
    fetchData(defaultSearchState);
  };

  return (
    <div className={style.page}>
      <ResourceHeader
        density="compact"
        placement="app-header"
        eyebrow="监控指标 / 事件指标"
        title="事件指标"
        description="聚合服务实例、服务保护和控制面事件，快速定位事件影响范围与最近变化。"
        actions={<Tag className={style.sourceTag}>{isLoading ? '加载中' : fetchError ? '查询失败' : '真实数据'}</Tag>}
      />

      {!fetchError && !isLoading && <><div className={style.summaryGrid}>
        <SummaryCard title="事件总数" value={summary.total} hint="当前已加载记录" />
        <SummaryCard title="风险事件" value={summary.riskCount} hint="下线、异常、隔离和推空保护" tone={style.summaryDanger} />
        <SummaryCard title="影响服务" value={summary.affectedServices} hint="按 namespace/service 去重" />
        <SummaryCard title="最近事件" value={summary.latestTime} hint={hasNext ? '还有更多历史事件' : '已展示当前结果集'} compactValue />
      </div>

      <div className={style.analysisGrid}>
        <section className={style.panel}>
          <div className={style.panelHeader}>
            <div>
              <h2>事件趋势</h2>
              <span>当前暂不提供趋势统计</span>
            </div>
            <InfoCircleIcon />
          </div>
          <Empty description="暂无趋势数据" />
        </section>

        <section className={style.panel}>
          <div className={style.panelHeader}>
            <div>
              <h2>类型分布</h2>
              <span>当前已加载记录的事件类别占比</span>
            </div>
            <FilterIcon />
          </div>
          <DistributionList data={categoryStats} />
        </section>
      </div>

      </>}

      <section className={style.panel}>
        <div className={style.panelHeader}>
          <div>
            <h2>事件明细</h2>
            <span>筛选服务、资源或事件类型后查看具体事件上下文</span>
          </div>
          <Button icon={<RefreshIcon />} variant="outline" loading={isLoading} onClick={() => fetchData()}>
            刷新
          </Button>
        </div>

        <div className={style.filterGrid}>
          <QueryComposer
            keyword={searchState.searchService}
            keywordPlaceholder="搜索服务名称"
            suggestions={Array.from(new Set(events.map((item) => item.service).filter(Boolean)))}
            fields={[
              {
                key: 'searchEvent',
                label: '事件类型',
                type: 'select',
                options: EventType,
              },
              {
                key: 'searchNamespace',
                label: '命名空间',
                type: 'text',
              },
              {
                key: 'searchResource',
                label: '资源 / 实例',
                type: 'text',
              },
            ]}
            values={{
              searchEvent: searchState.searchEvent,
              searchNamespace: searchState.searchNamespace,
              searchResource: searchState.searchResource,
            }}
            timeRange={(
              <DateRangePicker
                className={style.datePicker}
                value={[searchState.startTime, searchState.endTime]}
                clearable
                allowInput
                format="YYYY-MM-DD HH:mm:ss"
                presets={presets}
                onChange={(value) => updateSearchState({
                  startTime: String(value?.[0] || ''),
                  endTime: String(value?.[1] || ''),
                })}
              />
            )}
            onKeywordChange={(searchService) => updateSearchState({ searchService })}
            onValuesChange={(values) => updateSearchState({
              searchEvent: String(values.searchEvent || ''),
              searchNamespace: String(values.searchNamespace || ''),
              searchResource: String(values.searchResource || ''),
            })}
            onSubmit={({ keyword, values }) => {
              const nextSearchState = {
                ...searchState,
                searchService: keyword,
                searchEvent: String(values.searchEvent || ''),
                searchNamespace: String(values.searchNamespace || ''),
                searchResource: String(values.searchResource || ''),
              };
              setPaginationVersion((version) => version + 1);
              fetchData(nextSearchState);
            }}
            onReset={handleReset}
            loading={isLoading}
          />
        </div>

        {fetchError && !isLoading ? (
          <Empty title="事件查询失败" description="无法加载数据，请重试。" action={<Button onClick={() => fetchData(lastQuery.current)}>重新加载</Button>} />
        ) : <Table
          key={paginationVersion}
          data={displayEvents}
          columns={columns}
          rowKey="cursor"
          loading={isLoading}
          tableLayout="fixed"
          cellEmptyContent="-"
          empty={<Empty description="暂无事件数据" />}
          pagination={{
            pageSize: PAGE_SIZE,
            total: displayEvents.length,
            showJumper: true,
          }}
          onRowClick={({ row }: { row: EventMetricRow }) => setSelectedEvent(row)}
        />}
      </section>

      <Drawer
        visible={Boolean(selectedEvent)}
        size="medium"
        header={selectedEvent ? resolveEventLabel(selectedEvent.event_type) : '事件详情'}
        onClose={() => setSelectedEvent(null)}
      >
        {selectedEvent && (
          <div className={style.detailBody}>
            <div className={style.detailStatus}>
              <Tag className={eventToneClass(selectedEvent.event_type)}>{resolveEventCategory(selectedEvent.event_type)}</Tag>
              <span>{formatTime(selectedEvent.event_time)}</span>
            </div>
            <dl className={style.detailGrid}>
              <dt>命名空间</dt>
              <dd>{selectedEvent.namespace || '-'}</dd>
              <dt>服务</dt>
              <dd>{selectedEvent.service || '-'}</dd>
              <dt>资源</dt>
              <dd>{selectedEvent.resource || '-'}</dd>
              <dt>上报节点</dt>
              <dd>{selectedEvent.server || '-'}</dd>
              <dt>游标</dt>
              <dd>{selectedEvent.cursor || '-'}</dd>
            </dl>
          </div>
        )}
      </Drawer>
    </div>
  );
}

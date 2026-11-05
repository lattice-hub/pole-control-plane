import React from 'react';
import dayjs from 'dayjs';
import { Button, DateRangePicker, DateRangePickerProps, Drawer, Empty, Table, TableColumnData, Tag } from 'components/Fluent';
import { FilterIcon, InfoCircleIcon, RefreshIcon, UserIcon } from 'components/Fluent/icons';
import QueryComposer from 'components/QueryComposer';
import { ResourceHeader } from 'components/ResourceLayout';
import { describeOperationLog, getOperationTypeInfo, getResourceTypeInfo, OperationLog, OperationType, ResourceType } from 'services/observer';
import { describeObservabilityOperations } from 'services/observability';

import style from './index.module.less';

type OperationMetricRow = OperationLog & {
  rowKey?: string;
  [key: string]: unknown;
};

interface SearchState {
  searchResourceType: string;
  searchOperation: string;
  searchNamespace: string;
  searchResource: string;
  searchOperator: string;
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
  searchResourceType: '',
  searchOperation: '',
  searchNamespace: '',
  searchResource: '',
  searchOperator: '',
  startTime: '',
  endTime: '',
};

function resolveResourceLabel(resourceType: string) {
  return getResourceTypeInfo(resourceType).label || resourceType || '-';
}

function resolveOperationLabel(operationType: string) {
  return getOperationTypeInfo(operationType).label || operationType || '-';
}

function normalizeOperationRows(rows: OperationLog[]): OperationMetricRow[] {
  return rows.map((row) => ({
    ...row,
    detail: row.detail || (row as unknown as { operation_detail?: string }).operation_detail || '',
  }));
}

function isRiskOperation(operationType: string) {
  return ['Delete', 'Rollback', 'Disable'].includes(operationType);
}

function operationToneClass(operationType: string) {
  if (operationType === 'Delete' || operationType === 'Rollback') return style.toneDanger;
  if (operationType === 'Disable') return style.toneWarning;
  if (operationType === 'Create' || operationType === 'Enable') return style.toneSuccess;
  return style.toneInfo;
}

function formatTime(value: string) {
  if (!value) return '-';
  const date = dayjs(value);
  return date.isValid() ? date.format('YYYY-MM-DD HH:mm:ss') : value;
}

function buildResourceStats(rows: OperationMetricRow[]) {
  const total = Math.max(rows.length, 1);
  const resources = rows.reduce<Record<string, number>>((acc, row) => {
    const label = resolveResourceLabel(row.resource_type);
    acc[label] = (acc[label] || 0) + 1;
    return acc;
  }, {});
  return Object.entries(resources).map(([label, value]) => ({
    label,
    value,
    percent: Math.round((value / total) * 100),
  }));
}

function buildOperationSummary(rows: OperationMetricRow[]) {
  const operators = new Set(rows.map((row) => row.operator).filter(Boolean));
  const resources = new Set(rows.map((row) => `${row.resource_type}/${row.namespace}/${row.resource_name}`));
  const riskCount = rows.filter((row) => isRiskOperation(row.operation_type)).length;
  return {
    total: rows.length,
    riskCount,
    operators: operators.size,
    resources: resources.size,
  };
}

function SummaryCard(props: { title: string; value: React.ReactNode; hint: string; tone?: string }) {
  const { title, value, hint, tone } = props;
  return (
    <section className={`${style.summaryCard} ${tone || ''}`}>
      <span className={style.summaryTitle}>{title}</span>
      <strong className={style.summaryValue}>{value}</strong>
      <span className={style.summaryHint}>{hint}</span>
    </section>
  );
}

function DistributionList(props: { data: Array<{ label: string; value: number; percent: number }> }) {
  const { data } = props;
  if (!data.length) return <Empty description="暂无资源分布" />;
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

export default function ServerOperation() {
  const [operations, setOperations] = React.useState<OperationMetricRow[]>([]);
  const [searchState, setSearchState] = React.useState<SearchState>(defaultSearchState);
  const [isLoading, setIsLoading] = React.useState(true);
  const [hasNext, setHasNext] = React.useState(false);
  const [fetchError, setFetchError] = React.useState(false);
  const [selectedOperation, setSelectedOperation] = React.useState<OperationMetricRow | null>(null);
  const [paginationVersion, setPaginationVersion] = React.useState(0);

  const displayOperations = operations;

  const summary = React.useMemo(() => buildOperationSummary(displayOperations), [displayOperations]);
  const resourceStats = React.useMemo(() => buildResourceStats(displayOperations), [displayOperations]);

  const lastQuery = React.useRef<SearchState>(defaultSearchState);

  const requestSequence = React.useRef(0);

  const fetchData = React.useCallback(async (nextSearchState: SearchState = searchState) => {
    const sequence = ++requestSequence.current;
    lastQuery.current = nextSearchState;
    setIsLoading(true);
    try {
      const requestParams = {
        resource_type: nextSearchState.searchResourceType,
        operation_type: nextSearchState.searchOperation,
        namespace: nextSearchState.searchNamespace,
        resource_name: nextSearchState.searchResource,
        operator: nextSearchState.searchOperator,
        start_time: nextSearchState.startTime,
        end_time: nextSearchState.endTime,
        limit: PAGE_SIZE,
        cursor: 0,
        direction: 'next',
      } as const;
      const observabilityRes = await describeObservabilityOperations(requestParams);
      const fallbackRes = (observabilityRes.data || []).length ? observabilityRes : await describeOperationLog(requestParams);
      const remoteRows = normalizeOperationRows((fallbackRes.data || []) as OperationLog[]);
      if (sequence !== requestSequence.current) return;
      setOperations(remoteRows);
      setHasNext(Boolean(fallbackRes.has_next));
      setFetchError(false);
    } catch {
      if (sequence !== requestSequence.current) return;
      setOperations([]);
      setHasNext(false);
      setFetchError(true);
    } finally {
      if (sequence === requestSequence.current) setIsLoading(false);
    }
  }, [searchState]);

  React.useEffect(() => {
    fetchData(defaultSearchState);
  }, []);

  const columns = React.useMemo<TableColumnData<OperationMetricRow>[]>(() => [
    {
      colKey: 'operation_type',
      title: '操作',
      width: 150,
      cell: ({ row }) => <Tag className={operationToneClass(row.operation_type)}>{resolveOperationLabel(row.operation_type)}</Tag>,
    },
    {
      colKey: 'resource_type',
      title: '资源类型',
      width: 130,
      cell: ({ row }) => resolveResourceLabel(row.resource_type),
      ellipsis: true,
    },
    {
      colKey: 'resource_name',
      title: '资源名称',
      width: 180,
      ellipsis: true,
    },
    {
      colKey: 'namespace',
      title: '命名空间',
      width: 120,
      ellipsis: true,
    },
    {
      colKey: 'operator',
      title: '操作者',
      width: 120,
      cell: ({ row }) => (
        <span className={style.operatorCell}>
          <UserIcon />
          {row.operator || '-'}
        </span>
      ),
    },
    {
      colKey: 'detail',
      title: '操作详情',
      width: 240,
      ellipsis: true,
    },
    {
      colKey: 'happen_time',
      title: '发生时间',
      width: 170,
      cell: ({ row }) => formatTime(row.happen_time),
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
        eyebrow="监控指标 / 操作审计"
        title="操作审计"
        description="按操作者、资源类型和操作类型追踪审计数据，联动治理动作和变更影响。"
        actions={<Tag className={style.sourceTag}>{isLoading ? '加载中' : fetchError ? '查询失败' : '真实数据'}</Tag>}
      />

      {!fetchError && !isLoading && <><div className={style.summaryGrid}>
        <SummaryCard title="操作总数" value={summary.total} hint="当前已加载记录" />
        <SummaryCard title="高风险操作" value={summary.riskCount} hint="删除、回滚、禁用类操作" tone={style.summaryDanger} />
        <SummaryCard title="活跃操作者" value={summary.operators} hint="按 operator 去重" />
        <SummaryCard title="涉及资源" value={summary.resources} hint={hasNext ? '还有更多历史资源' : '已展示当前结果集'} />
      </div>

      <div className={style.analysisGrid}>
        <section className={style.panel}>
          <div className={style.panelHeader}>
            <div>
              <h2>操作趋势</h2>
              <span>当前暂不提供趋势统计</span>
            </div>
            <InfoCircleIcon />
          </div>
          <Empty description="暂无趋势数据" />
        </section>

        <section className={style.panel}>
          <div className={style.panelHeader}>
            <div>
              <h2>资源分布</h2>
              <span>当前已加载记录的资源类型占比</span>
            </div>
            <FilterIcon />
          </div>
          <DistributionList data={resourceStats} />
        </section>
      </div>

      </>}

      <section className={style.panel}>
        <div className={style.panelHeader}>
          <div>
            <h2>审计明细</h2>
            <span>筛选资源、操作者和操作类型后查看完整审计上下文</span>
          </div>
          <Button icon={<RefreshIcon />} variant="outline" loading={isLoading} onClick={() => fetchData()}>
            刷新
          </Button>
        </div>

        <div className={style.filterGrid}>
          <QueryComposer
            keyword={searchState.searchResource}
            keywordPlaceholder="搜索资源名称"
            suggestions={Array.from(new Set(operations.map((item) => item.resource_name).filter(Boolean)))}
            fields={[
              {
                key: 'searchResourceType',
                label: '资源类型',
                type: 'select',
                options: ResourceType,
              },
              {
                key: 'searchOperation',
                label: '操作类型',
                type: 'select',
                options: OperationType,
              },
              {
                key: 'searchNamespace',
                label: '命名空间',
                type: 'text',
              },
              {
                key: 'searchOperator',
                label: '操作者',
                type: 'text',
              },
            ]}
            values={{
              searchResourceType: searchState.searchResourceType,
              searchOperation: searchState.searchOperation,
              searchNamespace: searchState.searchNamespace,
              searchOperator: searchState.searchOperator,
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
            onKeywordChange={(searchResource) => updateSearchState({ searchResource })}
            onValuesChange={(values) => updateSearchState({
              searchResourceType: String(values.searchResourceType || ''),
              searchOperation: String(values.searchOperation || ''),
              searchNamespace: String(values.searchNamespace || ''),
              searchOperator: String(values.searchOperator || ''),
            })}
            onSubmit={({ keyword, values }) => {
              const nextSearchState = {
                ...searchState,
                searchResource: keyword,
                searchResourceType: String(values.searchResourceType || ''),
                searchOperation: String(values.searchOperation || ''),
                searchNamespace: String(values.searchNamespace || ''),
                searchOperator: String(values.searchOperator || ''),
              };
              setPaginationVersion((version) => version + 1);
              fetchData(nextSearchState);
            }}
            onReset={handleReset}
            loading={isLoading}
          />
        </div>

        {fetchError && !isLoading ? (
          <Empty title="操作查询失败" description="无法加载数据，请重试。" action={<Button onClick={() => fetchData(lastQuery.current)}>重新加载</Button>} />
        ) : <Table
          key={paginationVersion}
          data={displayOperations}
          columns={columns}
          rowKey="cursor"
          loading={isLoading}
          tableLayout="fixed"
          cellEmptyContent="-"
          empty={<Empty description="暂无操作数据" />}
          pagination={{
            pageSize: PAGE_SIZE,
            total: displayOperations.length,
            showJumper: true,
          }}
          onRowClick={({ row }: { row: OperationMetricRow }) => setSelectedOperation(row)}
        />}
      </section>

      <Drawer
        visible={Boolean(selectedOperation)}
        size="medium"
        header={selectedOperation ? `${resolveOperationLabel(selectedOperation.operation_type)} ${selectedOperation.resource_name}` : '操作详情'}
        onClose={() => setSelectedOperation(null)}
      >
        {selectedOperation && (
          <div className={style.detailBody}>
            <div className={style.detailStatus}>
              <Tag className={operationToneClass(selectedOperation.operation_type)}>{resolveResourceLabel(selectedOperation.resource_type)}</Tag>
              <span>{formatTime(selectedOperation.happen_time)}</span>
            </div>
            <dl className={style.detailGrid}>
              <dt>命名空间</dt>
              <dd>{selectedOperation.namespace || '-'}</dd>
              <dt>资源类型</dt>
              <dd>{resolveResourceLabel(selectedOperation.resource_type)}</dd>
              <dt>资源名称</dt>
              <dd>{selectedOperation.resource_name || '-'}</dd>
              <dt>操作者</dt>
              <dd>{selectedOperation.operator || '-'}</dd>
              <dt>上报节点</dt>
              <dd>{selectedOperation.server || '-'}</dd>
              <dt>游标</dt>
              <dd>{selectedOperation.cursor || '-'}</dd>
              <dt>操作详情</dt>
              <dd>{selectedOperation.detail || '-'}</dd>
            </dl>
          </div>
        )}
      </Drawer>
    </div>
  );
}

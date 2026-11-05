import React from 'react';
import { Button, Empty, Loading, Tag } from 'components/Fluent';
import { ChevronRightIcon, RollbackIcon } from 'components/Fluent/icons';
import { ResourceHeader } from 'components/ResourceLayout';
import {
  describePlatformTraces,
  describeTrace,
  TraceDetail,
  TraceSpan,
  TraceSummary,
} from 'services/observability';

import style from './TraceDrawer.module.less';

type TraceTarget = {
  component: string;
  api: string;
  protocol: string;
};

type TraceDrawerProps = {
  target: TraceTarget;
  startTime: number;
  endTime: number;
  onClose: () => void;
};

type TraceHistoryState = {
  depth: 1 | 2;
  target: TraceTarget;
  selectedTrace?: TraceSummary;
};

const TRACE_HISTORY_KEY = 'systemMonitorTrace';
const EMPTY_TRACE_SPANS: TraceSpan[] = [];

function currentTraceHistoryState(): TraceHistoryState | null {
  return window.history.state?.[TRACE_HISTORY_KEY] || null;
}

function formatTimestamp(timestamp: number) {
  if (!timestamp) return '-';
  return new Intl.DateTimeFormat(undefined, {
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
    fractionalSecondDigits: 3,
    hour12: false,
  }).format(new Date(timestamp));
}

function formatDuration(value: number) {
  if (value < 1) return `${Math.max(value * 1000, 0).toFixed(0)}μs`;
  if (value < 1000) return `${value.toFixed(value < 10 ? 2 : 1)}ms`;
  return `${(value / 1000).toFixed(2)}s`;
}

function shortID(value: string) {
  return value.length > 16 ? `${value.slice(0, 8)}…${value.slice(-6)}` : value;
}

type SpanTreeRow = {
  span: TraceSpan;
  depth: number;
  childCount: number;
};

function spanStartUS(span: TraceSpan) {
  return span.start_time_us ?? span.start_time * 1000;
}

function spanEndUS(span: TraceSpan) {
  return span.end_time_us ?? spanStartUS(span) + span.duration_ms * 1000;
}

function compareSpans(left: TraceSpan, right: TraceSpan) {
  return spanStartUS(left) - spanStartUS(right) || right.duration_ms - left.duration_ms || left.name.localeCompare(right.name);
}

function buildSpanTreeRows(spans: TraceSpan[], collapsed: Set<string>): SpanTreeRow[] {
  const byID = new Map(spans.map(span => [span.span_id, span]));
  const children = new Map<string, TraceSpan[]>();
  const roots: TraceSpan[] = [];
  spans.forEach((span) => {
    if (!span.parent_span_id || span.parent_span_id === span.span_id || !byID.has(span.parent_span_id)) {
      roots.push(span);
      return;
    }
    const siblings = children.get(span.parent_span_id) || [];
    siblings.push(span);
    children.set(span.parent_span_id, siblings);
  });
  roots.sort(compareSpans);
  children.forEach(siblings => siblings.sort(compareSpans));

  const connected = new Set<string>();
  const markConnected = (span: TraceSpan, ancestors: Set<string>) => {
    if (connected.has(span.span_id) || ancestors.has(span.span_id)) return;
    connected.add(span.span_id);
    const nextAncestors = new Set(ancestors).add(span.span_id);
    (children.get(span.span_id) || []).forEach(child => markConnected(child, nextAncestors));
  };
  roots.forEach(root => markConnected(root, new Set()));

  const rows: SpanTreeRow[] = [];
  const visited = new Set<string>();
  const walk = (span: TraceSpan, depth: number, ancestors: Set<string>) => {
    if (visited.has(span.span_id) || ancestors.has(span.span_id)) return;
    visited.add(span.span_id);
    const descendants = children.get(span.span_id) || [];
    rows.push({ span, depth, childCount: descendants.length });
    if (collapsed.has(span.span_id)) return;
    const nextAncestors = new Set(ancestors).add(span.span_id);
    descendants.forEach(child => walk(child, depth + 1, nextAncestors));
  };
  roots.forEach(root => walk(root, 0, new Set()));
  [...spans].sort(compareSpans).filter(span => !connected.has(span.span_id)).forEach(span => walk(span, 0, new Set()));
  return rows;
}

function spanKind(span: TraceSpan) {
  return span.kind.replace('SPAN_KIND_', '') || 'INTERNAL';
}

function spanBarTone(span: TraceSpan) {
  if (span.status === 'error') return style.spanBarError;
  if (spanKind(span) === 'SERVER') return style.spanBarServer;
  if (spanKind(span) === 'CLIENT') return style.spanBarClient;
  return style.spanBarInternal;
}

function TraceWaterfall({ detail }: { detail: TraceDetail }) {
  const spans = detail.spans || EMPTY_TRACE_SPANS;
  const [collapsed, setCollapsed] = React.useState<Set<string>>(new Set());
  const [selectedSpanID, setSelectedSpanID] = React.useState('');
  const parentIDs = React.useMemo(() => new Set(spans.map(span => span.parent_span_id).filter(Boolean)), [spans]);
  const rows = React.useMemo(() => buildSpanTreeRows(spans, collapsed), [collapsed, spans]);

  React.useEffect(() => {
    setCollapsed(new Set());
    setSelectedSpanID(buildSpanTreeRows(spans, new Set())[0]?.span.span_id || '');
  }, [spans]);

  if (!spans.length) return <Empty description="该 Trace 暂无 Span 数据" />;
  const startUS = Math.min(...spans.map(spanStartUS));
  const endUS = Math.max(...spans.map(span => Math.max(spanEndUS(span), spanStartUS(span) + span.duration_ms * 1000)));
  const rangeUS = Math.max(endUS - startUS, 1);
  const selectedSpan = spans.find(span => span.span_id === selectedSpanID) || rows[0]?.span;
  const ticks = [0, 0.25, 0.5, 0.75, 1];
  const toggleSpan = (spanID: string) => {
    setCollapsed((current) => {
      const next = new Set(current);
      if (next.has(spanID)) next.delete(spanID);
      else next.add(spanID);
      return next;
    });
  };

  return (
    <div className={style.waterfall}>
      <div className={style.waterfallToolbar}>
        <div className={style.spanLegend} aria-label="Span 类型图例">
          <span><i className={style.legendServer} />SERVER</span>
          <span><i className={style.legendClient} />CLIENT</span>
          <span><i className={style.legendInternal} />INTERNAL</span>
          <span><i className={style.legendError} />ERROR</span>
        </div>
        <div className={style.treeActions}>
          <Button variant="text" type="button" onClick={() => setCollapsed(new Set())}>全部展开</Button>
          <Button variant="text" type="button" onClick={() => setCollapsed(new Set(parentIDs))}>全部折叠</Button>
        </div>
      </div>
      <div className={style.waterfallViewport}>
        <div className={style.overviewRow}>
          <span className={style.overviewLabel}>全局时间概览</span>
          <div className={style.overviewTrack}>
            {spans.map(span => {
              const left = ((spanStartUS(span) - startUS) / rangeUS) * 100;
              const width = Math.max((span.duration_ms * 1000 / rangeUS) * 100, 0.35);
              return <i key={span.span_id} className={spanBarTone(span)} style={{ left: `${left}%`, width: `${Math.min(width, 100 - left)}%` }} />;
            })}
          </div>
          <span className={style.overviewDuration}>{formatDuration(rangeUS / 1000)}</span>
        </div>
        <div className={style.spanGridHeader}>
          <span>调用树（{rows.length}/{spans.length}）</span>
          <div className={style.waterfallAxis}>
            {ticks.map(tick => <span key={tick}>{formatDuration((rangeUS * tick) / 1000)}</span>)}
          </div>
          <span>耗时</span>
        </div>
        <div className={style.spanTree} role="tree" aria-label="Span 调用树">
          {rows.map(({ span, depth, childCount }) => {
            const left = Math.max(((spanStartUS(span) - startUS) / rangeUS) * 100, 0);
            const width = Math.max((span.duration_ms * 1000 / rangeUS) * 100, 0.35);
            const isCollapsed = collapsed.has(span.span_id);
            const isSelected = selectedSpan?.span_id === span.span_id;
            return (
              <div
                className={`${style.spanRow} ${isSelected ? style.spanRowSelected : ''}`}
                key={span.span_id}
                role="treeitem"
                aria-level={depth + 1}
                aria-selected={isSelected}
                aria-expanded={childCount ? !isCollapsed : undefined}
                tabIndex={0}
                onClick={() => setSelectedSpanID(span.span_id)}
                onKeyDown={(event) => {
                  if (event.key === 'Enter' || event.key === ' ') {
                    event.preventDefault();
                    setSelectedSpanID(span.span_id);
                  }
                }}
              >
                <div className={style.spanMeta} style={{ paddingLeft: `${depth * 18 + 8}px` }}>
                  <span className={style.branchGuides} aria-hidden="true">
                    {Array.from({ length: depth }).map((_, index) => <i key={index} style={{ left: `${index * 18 + 16}px` }} />)}
                  </span>
                  {childCount ? (
                    <Button
                      variant="text"
                      className={`${style.spanToggle} ${isCollapsed ? '' : style.spanToggleOpen}`}
                      type="button"
                      aria-label={isCollapsed ? `展开 ${span.name}` : `折叠 ${span.name}`}
                      onClick={(event) => { event.stopPropagation(); toggleSpan(span.span_id); }}
                    ><ChevronRightIcon /></Button>
                  ) : <i className={style.spanLeaf} aria-hidden="true" />}
                  <span className={style.spanMetaText}>
                    <strong title={span.name}>{span.name}</strong>
                    <small>{span.service_name || '-'} · {spanKind(span)}{childCount ? ` · ${childCount} 子 Span` : ''}</small>
                  </span>
                </div>
                <div className={style.spanTrack}>
                  <div
                    className={`${style.spanBar} ${spanBarTone(span)}`}
                    style={{ left: `${left}%`, width: `${Math.min(width, 100 - left)}%` }}
                    title={`${span.name} · +${formatDuration((spanStartUS(span) - startUS) / 1000)} · ${formatDuration(span.duration_ms)}`}
                  />
                </div>
                <div className={style.spanDuration}>{formatDuration(span.duration_ms)}</div>
                {span.status_message ? <div className={style.spanError}>{span.status_message}</div> : null}
              </div>
            );
          })}
        </div>
        {selectedSpan ? (
          <section className={style.spanInspector} aria-label="选中 Span 详情">
            <header>
              <div><span>选中 Span</span><strong>{selectedSpan.name}</strong></div>
              <Tag className={selectedSpan.status === 'error' ? style.errorTag : style.okTag}>{selectedSpan.status === 'error' ? '异常' : '正常'}</Tag>
            </header>
            <dl>
              <div><dt>开始偏移</dt><dd>+{formatDuration((spanStartUS(selectedSpan) - startUS) / 1000)}</dd></div>
              <div><dt>持续时间</dt><dd>{formatDuration(selectedSpan.duration_ms)}</dd></div>
              <div><dt>类型</dt><dd>{spanKind(selectedSpan)}</dd></div>
              <div><dt>服务</dt><dd>{selectedSpan.service_name || '-'}</dd></div>
              <div><dt>Span ID</dt><dd><code>{selectedSpan.span_id}</code></dd></div>
              <div><dt>Parent Span</dt><dd><code>{selectedSpan.parent_span_id || 'Root'}</code></dd></div>
            </dl>
            {selectedSpan.status_message ? <p>{selectedSpan.status_message}</p> : null}
          </section>
        ) : null}
      </div>
    </div>
  );
}

export default function TraceDrawer(props: TraceDrawerProps) {
  const { target, startTime, endTime, onClose } = props;
  const workspaceRef = React.useRef<HTMLElement>(null);
  const [traces, setTraces] = React.useState<TraceSummary[]>([]);
  const [loading, setLoading] = React.useState(false);
  const [error, setError] = React.useState('');
  const [selectedTrace, setSelectedTrace] = React.useState<TraceSummary | null>(null);
  const [detail, setDetail] = React.useState<TraceDetail | null>(null);
  const [detailLoading, setDetailLoading] = React.useState(false);

  React.useEffect(() => {
    let active = true;
    setLoading(true);
    setError('');
    setSelectedTrace(null);
    setDetail(null);
    void describePlatformTraces({
      start_time: String(startTime),
      end_time: String(endTime),
      service: target.component,
      api: target.api,
      limit: '50',
    }).then((response) => {
      if (active) setTraces(response.data || []);
    }).catch(() => {
      if (active) {
        setTraces([]);
        setError('Trace 查询失败，请检查 OTel Trace 数据链路。');
      }
    }).finally(() => {
      if (active) setLoading(false);
    });
    return () => { active = false; };
  }, [endTime, startTime, target]);

  const loadTrace = React.useCallback((trace: TraceSummary) => {
    setSelectedTrace(trace);
    setDetail(null);
    setDetailLoading(true);
    void describeTrace(trace.trace_id).then(setDetail).catch(() => setDetail(null)).finally(() => setDetailLoading(false));
  }, []);

  const openTrace = React.useCallback((trace: TraceSummary) => {
    window.history.pushState({
      ...window.history.state,
      [TRACE_HISTORY_KEY]: { depth: 2, target, selectedTrace: trace },
    }, '');
    loadTrace(trace);
  }, [loadTrace, target]);

  React.useEffect(() => {
    const navigationState = currentTraceHistoryState();
    if (navigationState?.depth === 2 && navigationState.selectedTrace) {
      loadTrace(navigationState.selectedTrace);
    }

    const handlePopState = (event: PopStateEvent) => {
      const nextState = event.state?.[TRACE_HISTORY_KEY] as TraceHistoryState | undefined;
      if (nextState?.depth === 2 && nextState.selectedTrace) {
        loadTrace(nextState.selectedTrace);
        return;
      }
      setSelectedTrace(null);
      setDetail(null);
    };
    window.addEventListener('popstate', handlePopState);
    return () => window.removeEventListener('popstate', handlePopState);
  }, [loadTrace]);

  React.useLayoutEffect(() => {
    workspaceRef.current?.scrollIntoView({ block: 'start' });
  }, []);

  const backToTraceList = React.useCallback(() => {
    if (currentTraceHistoryState()?.depth === 2) {
      window.history.back();
      return;
    }
    setSelectedTrace(null);
    setDetail(null);
  }, []);

  const header = selectedTrace ? `Trace ${shortID(selectedTrace.trace_id)}` : `接口 Trace · ${target?.api || ''}`;

  return (
    <>
      <ResourceHeader
        density="compact"
        placement="app-header"
        eyebrow="监控指标 / 系统监控 / 接口 Trace"
        title={header}
      />
      <main ref={workspaceRef} className={style.workspace} aria-label="接口 Trace 工作区">
          <nav className={style.workspaceNav} aria-label="Trace 页面导航">
            <Button aria-label="返回系统监控" title="返回系统监控" icon={<RollbackIcon />} variant="outline" onClick={onClose}>返回系统监控</Button>
            <span>接口调用链 / {target.api}</span>
          </nav>
          <section className={style.contextBar}>
            <div><span>组件</span><strong>{target.component}</strong></div>
            <div><span>协议</span><strong>{target.protocol}</strong></div>
            <div><span>时间窗</span><strong>{formatTimestamp(startTime)} — {formatTimestamp(endTime)}</strong></div>
            <Tag className={style.liveTag}>OTel Trace</Tag>
          </section>

          {selectedTrace ? (
            <>
              <div className={style.detailToolbar}>
                <Button aria-label="返回 Trace 列表" title="返回 Trace 列表" icon={<RollbackIcon />} variant="outline" onClick={backToTraceList}>返回 Trace 列表</Button>
                <div className={style.traceIdentity}>
                  <code>{selectedTrace.trace_id}</code>
                  <Tag className={selectedTrace.status === 'error' ? style.errorTag : style.okTag}>
                    {selectedTrace.status === 'error' ? '异常' : '正常'}
                  </Tag>
                </div>
              </div>
              <section className={style.traceSummaryGrid}>
                <div><span>入口 Span</span><strong>{selectedTrace.root_span}</strong></div>
                <div><span>开始时间</span><strong>{formatTimestamp(selectedTrace.start_time)}</strong></div>
                <div><span>总耗时</span><strong>{formatDuration(selectedTrace.duration_ms)}</strong></div>
                <div><span>Span 数</span><strong>{selectedTrace.span_count}</strong></div>
              </section>
              <section className={style.waterfallPanel}>
                <header><div><h3>Span 瀑布</h3><p>按父子关系与真实开始时间排列</p></div><code>{detail?.spans?.length || 0} spans</code></header>
                {detailLoading ? <Loading text="加载 Span 树..." /> : detail ? <TraceWaterfall detail={detail} /> : <Empty description="Trace 详情加载失败" />}
              </section>
            </>
          ) : (
            <section className={style.traceListPanel}>
              <header>
                <div><h3>同接口请求链路</h3><p>按开始时间倒序，点击查看完整 Span 树</p></div>
                <span>{loading ? '查询中' : `${traces.length} 条 Trace`}</span>
              </header>
              {loading ? <Loading text="查询 Trace..." /> : error ? <div className={style.queryError}>{error}</div> : traces.length ? (
                <div className={style.traceList}>
                  {traces.map(trace => (
                    <Button variant="text" className={style.traceCard} type="button" key={trace.trace_id} onClick={() => openTrace(trace)}>
                      <div className={style.traceCardStatus}><span className={trace.status === 'error' ? style.statusError : style.statusOk} /></div>
                      <div className={style.traceCardMain}>
                        <strong>{trace.root_span}</strong>
                        <code title={trace.trace_id}>{shortID(trace.trace_id)}</code>
                      </div>
                      <div className={style.traceCardMetric}><span>开始时间</span><strong>{formatTimestamp(trace.start_time)}</strong></div>
                      <div className={style.traceCardMetric}><span>耗时</span><strong>{formatDuration(trace.duration_ms)}</strong></div>
                      <div className={style.traceCardMetric}><span>Span</span><strong>{trace.span_count}</strong></div>
                      <span className={style.traceCardAction}>查看链路</span>
                    </Button>
                  ))}
                </div>
              ) : <Empty description="该接口在当前时间窗暂无 Trace" />}
            </section>
          )}
      </main>
    </>
  );
}

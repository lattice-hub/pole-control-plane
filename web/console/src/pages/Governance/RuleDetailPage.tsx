import React from 'react';
import { Breadcrumb, Button, Empty, Loading, TableRowData } from 'components/Fluent';
import { useNavigate, useSearchParams } from 'components/Router';

import SubscribeTable from 'components/SubscribeTable';
import { useAppDispatch } from 'modules/store';
import {
    editorCustomRoute,
    listCustomRouteVersions,
    listOneCustomRoute,
} from 'modules/governance/route';
import {
    editorRateLimitRule,
    listOneRateLimitRule,
    listRateLimitRuleVersions,
    removeRateLimitRuleVersion,
    rollbackRateLimitRuleVersion,
} from 'modules/governance/ratelimit';
import {
    editorCircuitBreaker,
    listCircuitBreakerVersions,
    listOneCircuitBreaker,
    removeCircuitBreakerRelease,
    rollbackCircuitBreakerRelease,
} from 'modules/governance/circuitbreaker';
import {
    editorFaultDetect,
    listFaultDetectVersions,
    listOneFaultDetect,
    removeFaultDetectVersion,
    rollbackFaultDetectVersion,
} from 'modules/governance/faultdetect';
import {
    editorLosslessRule,
    listLossLessRule,
    listLosslessRuleVersions,
    removeLosslessVersion,
    rollbackLosslessVersion,
} from 'modules/governance/lossless';
import {
    editorLaneGroup,
    listLaneGroupVersions,
    listOneLaneGroup,
    removeLaneGroupVersion,
    rollbackLanGroupVersion,
} from 'modules/governance/lane_group';
import CustomRouteEditor from './Router/CustomRouteEditor';
import RateLimitEditor from './RateLimit/RateLimitEditor';
import CircuitBreakerEditor from './CircuitBreaker/CircuitBreakerEditor';
import FaultDetectEditor from './CircuitBreaker/FaultDetectEditor';
import LossLessEditor from './LossLess/LossLessEditor';
import LaneGroupEdtor from './Router/LaneGroupEdtor';
import TrafficGovernanceEditor from './Security/TrafficGovernanceEditor';
import RuleDetailFrame from './RuleRelease/RuleDetailFrame';
import RuleTabs from './RuleRelease/RuleTabs';
import { LimitType, RateLimitView } from 'services/ratelimit';
import { CustomRouteView } from 'services/router';
import { CircuitBreakerRule } from 'services/circuitbreaker';
import { FaultDetectRule } from 'services/faultdetect';
import { LossLessRuleView } from 'services/lossless';
import { LaneGroupView } from 'services/lane';
import { Op, RuleRelease } from 'services/types';
import { VersionClient } from 'services/version_client';
import {
    deleteTrafficGovernanceRelease,
    describeOneTrafficGovernanceRule,
    describeTrafficGovernanceVersions,
    TrafficGovernanceKind,
    TrafficGovernanceKindLabel,
    TrafficGovernanceRule,
} from 'services/traffic_governance';
import { openErrNotification, openInfoNotification } from 'utils/notifition';
import style from './RuleRelease/RuleDetailDrawer.module.less';

const { BreadcrumbItem } = Breadcrumb;

type RuleKind = 'route' | 'ratelimit-local' | 'ratelimit-global' | 'circuitbreaker' | 'faultdetect' | 'lossless' | 'lane' | 'traffic-security' | 'traffic-mirror' | 'traffic-mock';

interface RuleDetailSelection {
    kind: RuleKind;
    typeLabel: string;
    name: string;
    editable?: boolean;
    deleteable?: boolean;
    raw: TableRowData;
    limitType?: LimitType;
    trafficKind?: TrafficGovernanceKind;
}

const ruleKindLabels: Record<RuleKind, string> = {
    route: '路由',
    'ratelimit-local': '本地限流',
    'ratelimit-global': '全局限流',
    circuitbreaker: '熔断',
    faultdetect: '探测',
    lossless: '无损',
    lane: '泳道',
    'traffic-security': TrafficGovernanceKindLabel.security,
    'traffic-mirror': TrafficGovernanceKindLabel.mirror,
    'traffic-mock': TrafficGovernanceKindLabel.mock,
};

const validRuleKinds = new Set<RuleKind>(Object.keys(ruleKindLabels) as RuleKind[]);

const trafficKindByRuleKind: Partial<Record<RuleKind, TrafficGovernanceKind>> = {
    'traffic-security': 'security',
    'traffic-mirror': 'mirror',
    'traffic-mock': 'mock',
};

const getActionPayload = <T,>(action: unknown): T | undefined => {
    const result = action as { meta?: { requestStatus?: string }, payload?: T };
    if (result.meta?.requestStatus === 'fulfilled') return result.payload;
    return undefined;
};

const isRuleKind = (value?: string | null): value is RuleKind => !!value && validRuleKinds.has(value as RuleKind);

const versionPayload = (action: unknown, page: number, limit: number) => {
    const payload = getActionPayload<{
        datas?: RuleRelease[];
        versions?: RuleRelease[];
        total?: number;
        versionTotal?: number;
        page?: number;
        versionPage?: number;
        limit?: number;
        versionLimit?: number;
    }>(action);
    return {
        list: payload?.datas || payload?.versions || [],
        total: payload?.total || payload?.versionTotal || 0,
        page: payload?.page || payload?.versionPage || page,
        limit: payload?.limit || payload?.versionLimit || limit,
    };
};

const GovernanceRuleDetailPage: React.FC = () => {
    const dispatch = useAppDispatch();
    const navigate = useNavigate();
    const [searchParams] = useSearchParams();
    const kindParam = searchParams.get('kind');
    const id = searchParams.get('id') || '';
    const nameParam = searchParams.get('name') || '';
    const kind = isRuleKind(kindParam) ? kindParam : undefined;
    const [mode, setMode] = React.useState<Op>('view');
    const [loading, setLoading] = React.useState(false);
    const [selection, setSelection] = React.useState<RuleDetailSelection | null>(null);
    const [detailError, setDetailError] = React.useState('');
    const [notFound, setNotFound] = React.useState(false);
    const [versions, setVersions] = React.useState<RuleRelease[]>([]);
    const [versionLoading, setVersionLoading] = React.useState(false);
    const [versionTotal, setVersionTotal] = React.useState(0);
    const [versionPage, setVersionPage] = React.useState(1);
    const [versionLimit, setVersionLimit] = React.useState(10);

    const applyVersionResult = (result: { list: RuleRelease[]; total: number; page: number; limit: number }) => {
        setVersions(result.list);
        setVersionTotal(result.total);
        setVersionPage(result.page);
        setVersionLimit(result.limit);
    };

    const loadRule = React.useCallback(async () => {
        if (!kind || !id) return;
        setLoading(true);
        setDetailError('');
        setNotFound(false);
        try {
            let next: RuleDetailSelection | null = null;
            if (kind === 'route') {
                const action = await dispatch(listOneCustomRoute({ id }));
                const rule = getActionPayload<{ viewRoute?: CustomRouteView }>(action)?.viewRoute;
                if (rule) {
                    dispatch(editorCustomRoute(rule));
                    next = { kind, typeLabel: ruleKindLabels[kind], name: rule.name || nameParam, raw: rule as TableRowData, editable: rule.editable, deleteable: rule.deleteable };
                }
            } else if (kind === 'ratelimit-local' || kind === 'ratelimit-global') {
                const action = await dispatch(listOneRateLimitRule({ id }));
                const rule = getActionPayload<{ viewRule?: RateLimitView }>(action)?.viewRule;
                if (rule) {
                    dispatch(editorRateLimitRule(rule));
                    const limitType = kind === 'ratelimit-global' ? LimitType.GLOBAL : LimitType.LOCAL;
                    next = { kind, typeLabel: ruleKindLabels[kind], name: rule.name || nameParam, raw: rule as TableRowData, editable: rule.editable, deleteable: rule.deleteable, limitType };
                }
            } else if (kind === 'circuitbreaker') {
                const action = await dispatch(listOneCircuitBreaker({ id }));
                const rule = getActionPayload<{ viewRule?: CircuitBreakerRule }>(action)?.viewRule;
                if (rule) {
                    dispatch(editorCircuitBreaker(rule));
                    next = { kind, typeLabel: ruleKindLabels[kind], name: rule.name || nameParam, raw: rule as TableRowData, editable: rule.editable, deleteable: rule.deleteable };
                }
            } else if (kind === 'faultdetect') {
                const action = await dispatch(listOneFaultDetect({ id }));
                const rule = getActionPayload<{ editRule?: FaultDetectRule }>(action)?.editRule;
                if (rule) {
                    dispatch(editorFaultDetect(rule));
                    next = { kind, typeLabel: ruleKindLabels[kind], name: rule.name || nameParam, raw: rule as TableRowData, editable: rule.editable, deleteable: rule.deleteable };
                }
            } else if (kind === 'lossless') {
                const action = await dispatch(listOneLossLessRule({ id }));
                const rule = getActionPayload<{ viewRule?: LossLessRuleView }>(action)?.viewRule;
                if (rule) {
                    dispatch(editorLosslessRule(rule));
                    next = { kind, typeLabel: ruleKindLabels[kind], name: rule.name || nameParam, raw: rule as TableRowData, editable: rule.editable, deleteable: rule.deleteable };
                }
            } else if (kind === 'lane') {
                const action = await dispatch(listOneLaneGroup({ id }));
                const rule = getActionPayload<{ viewGroup?: LaneGroupView }>(action)?.viewGroup;
                if (rule) {
                    dispatch(editorLaneGroup(rule));
                    next = { kind, typeLabel: ruleKindLabels[kind], name: rule.name || nameParam, raw: rule as TableRowData, editable: rule.editable, deleteable: rule.deleteable };
                }
            } else {
                const trafficKind = trafficKindByRuleKind[kind];
                if (trafficKind) {
                    const rule = await describeOneTrafficGovernanceRule<TrafficGovernanceRule>(trafficKind, id);
                    next = { kind, typeLabel: ruleKindLabels[kind], name: rule.name || nameParam, raw: rule as TableRowData, editable: rule.editable, deleteable: rule.deleteable, trafficKind };
                }
            }
            if (!next) {
                setNotFound(true);
            }
            setSelection(next);
            setMode('view');
        } catch (error) {
            setSelection(null);
            setDetailError((error as Error).message || '请求失败');
        } finally {
            setLoading(false);
        }
    }, [dispatch, id, kind, nameParam]);

    React.useEffect(() => {
        loadRule();
    }, [loadRule]);

    const refreshVersions = async (page = 1, limit = 10) => {
        if (!selection) return;
        setVersionLoading(true);
        try {
            if (selection.kind === 'route') {
                const action = await dispatch(listCustomRouteVersions({ param: { offset: (page - 1) * limit, limit, id: selection.raw.id as string } }));
                applyVersionResult(versionPayload(action, page, limit));
            } else if (selection.kind === 'ratelimit-local' || selection.kind === 'ratelimit-global') {
                const action = await dispatch(listRateLimitRuleVersions({ param: { offset: (page - 1) * limit, limit, rule_name: selection.name } }));
                applyVersionResult(versionPayload(action, page, limit));
            } else if (selection.kind === 'circuitbreaker') {
                const action = await dispatch(listCircuitBreakerVersions({ param: { offset: (page - 1) * limit, limit, rule_name: selection.name } }));
                applyVersionResult(versionPayload(action, page, limit));
            } else if (selection.kind === 'faultdetect') {
                const action = await dispatch(listFaultDetectVersions({ param: { offset: (page - 1) * limit, limit, id: selection.raw.id as string } }));
                applyVersionResult(versionPayload(action, page, limit));
            } else if (selection.kind === 'lossless') {
                const action = await dispatch(listLosslessRuleVersions({ param: { offset: (page - 1) * limit, limit, id: selection.raw.id as string } }));
                applyVersionResult(versionPayload(action, page, limit));
            } else if (selection.kind === 'lane') {
                const action = await dispatch(listLaneGroupVersions({ param: { offset: (page - 1) * limit, limit, id: selection.raw.id as string } }));
                applyVersionResult(versionPayload(action, page, limit));
            } else if (selection.trafficKind) {
                const result = await describeTrafficGovernanceVersions(selection.trafficKind, {
                    offset: (page - 1) * limit,
                    limit,
                    id: selection.raw.id as string,
                    rule_name: selection.name,
                });
                applyVersionResult({ list: result.list, total: result.totalCount, page, limit });
            }
        } finally {
            setVersionLoading(false);
        }
    };

    const operateRelease = async (op: Op, row: TableRowData) => {
        if (!selection) return;
        if (selection.kind === 'route') return;
        if (op === 'delete') {
            if (selection.kind === 'ratelimit-local' || selection.kind === 'ratelimit-global') {
                await dispatch(removeRateLimitRuleVersion({ id: row.id }));
            } else if (selection.kind === 'circuitbreaker') {
                await dispatch(removeCircuitBreakerRelease({ id: row.id }));
            } else if (selection.kind === 'faultdetect') {
                await dispatch(removeFaultDetectVersion({ id: row.id }));
            } else if (selection.kind === 'lossless') {
                await dispatch(removeLosslessVersion({ id: row.id }));
            } else if (selection.kind === 'lane') {
                await dispatch(removeLaneGroupVersion({ ids: [row.id] }));
            } else if (selection.trafficKind) {
                await deleteTrafficGovernanceRelease(selection.trafficKind, row.id as string);
            }
            openInfoNotification('请求成功', '删除规则版本成功');
            refreshVersions(versionPage, versionLimit);
        }
        if (op === 'rollback') {
            if (selection.kind === 'ratelimit-local' || selection.kind === 'ratelimit-global') {
                await dispatch(rollbackRateLimitRuleVersion({ id: row.id }));
            } else if (selection.kind === 'circuitbreaker') {
                await dispatch(rollbackCircuitBreakerRelease({ id: row.id }));
            } else if (selection.kind === 'faultdetect') {
                await dispatch(rollbackFaultDetectVersion({ id: row.id }));
            } else if (selection.kind === 'lossless') {
                await dispatch(rollbackLosslessVersion({ id: row.id }));
            } else if (selection.kind === 'lane') {
                await dispatch(rollbackLanGroupVersion({ id: row.id }));
            } else if (selection.trafficKind) {
                return;
            }
            openInfoNotification('请求成功', '回滚规则版本成功');
            refreshVersions(versionPage, versionLimit);
        }
    };

    const commonVersions = selection ? {
        datas: versions,
        action: operateRelease,
        editable: selection.editable ?? true,
        deleteable: selection.deleteable ?? true,
        rollbackable: selection.kind !== 'lossless' && !selection.trafficKind,
        loading: versionLoading,
        pagination: {
            current: versionPage,
            pageSize: versionLimit,
            total: versionTotal,
            showJumper: false,
            onChange(pageInfo: { current: number, pageSize: number }) {
                refreshVersions(pageInfo.current, pageInfo.pageSize);
            },
        },
        onPageChange: (page: { current: number, pageSize: number }) => {
            refreshVersions(page.current, page.pageSize);
        },
    } : undefined;

    const subscribe = selection ? (
        <div style={{ marginLeft: 20, marginTop: 20 }}>
            <SubscribeTable
                title={selection.name}
                editable={selection.editable ?? true}
                deleteable={selection.deleteable ?? true}
                subscribers={Array.isArray((selection.raw as TableRowData & { subscribers?: unknown[] }).subscribers)
                    ? (selection.raw as TableRowData & { subscribers: VersionClient[] }).subscribers
                    : []}
            />
        </div>
    ) : null;

    const afterEditRefresh = (close?: boolean) => {
        if (close) {
            setMode('view');
        }
        loadRule();
    };

    const renderDetail = () => {
        if (!selection || !commonVersions || !subscribe) return null;
        if (selection.kind === 'route') {
            return <RuleTabs op={mode} onVersionView={() => refreshVersions()} view={<CustomRouteEditor op={mode} editable={selection.editable ?? true} refresh={afterEditRefresh} />} versions={commonVersions} subscribe={subscribe} />;
        }
        if (selection.kind === 'ratelimit-local' || selection.kind === 'ratelimit-global') {
            return <RuleTabs op={mode} onVersionView={() => refreshVersions()} view={<RateLimitEditor limitType={selection.limitType || LimitType.LOCAL} visible op={mode} refresh={afterEditRefresh} />} versions={commonVersions} subscribe={subscribe} />;
        }
        if (selection.kind === 'circuitbreaker') {
            return <RuleTabs op={mode} onVersionView={() => refreshVersions()} view={<CircuitBreakerEditor op={mode} refresh={afterEditRefresh} />} versions={commonVersions} subscribe={subscribe} />;
        }
        if (selection.kind === 'faultdetect') {
            return <RuleTabs op={mode} onVersionView={() => refreshVersions()} view={<FaultDetectEditor op={mode} refresh={afterEditRefresh} />} versions={commonVersions} subscribe={subscribe} />;
        }
        if (selection.kind === 'lossless') {
            return <RuleTabs op={mode} onVersionView={() => refreshVersions()} view={<LossLessEditor visible op={mode} refresh={afterEditRefresh} />} versions={commonVersions} subscribe={subscribe} />;
        }
        if (selection.trafficKind) {
            return (
                <RuleTabs
                    op={mode}
                    onVersionView={() => refreshVersions()}
                    view={(
                        <TrafficGovernanceEditor
                            kind={selection.trafficKind}
                            op={mode}
                            data={selection.raw as TrafficGovernanceRule}
                            visible
                            refresh={afterEditRefresh}
                        />
                    )}
                    versions={commonVersions}
                    subscribe={subscribe}
                />
            );
        }
        return <LaneGroupEdtor op={mode} refresh={afterEditRefresh} />;
    };

    return (
        <div className={style.standalonePage}>
            <Breadcrumb className={style.detailBreadcrumb} maxItemWidth="220px">
                <BreadcrumbItem onClick={() => navigate('/governance/workbench')}>治理工作台</BreadcrumbItem>
                <BreadcrumbItem>{selection?.name || nameParam || '规则详情'}</BreadcrumbItem>
            </Breadcrumb>
            {loading && (
                <div className={style.loadingPane}>
                    <Loading text="加载治理规则详情..." />
                </div>
            )}
            {!loading && (!kind || !id) && (
                <div className={style.emptyPane}>
                    <Empty description="缺少治理规则参数" />
                </div>
            )}
            {!loading && kind && id && !selection && (
                <div className={style.emptyPane}>
                    <Empty
                        title={notFound ? '未找到治理规则' : '治理规则加载失败'}
                        description={notFound ? '该规则可能已删除，或当前链接参数已经失效。' : detailError}
                        action={<Button variant="outline" onClick={loadRule}>重新加载</Button>}
                    />
                </div>
            )}
            {!loading && selection && (
                <RuleDetailFrame title={selection.name} subtitle={selection.typeLabel}>
                    {renderDetail()}
                </RuleDetailFrame>
            )}
        </div>
    );
};

export default React.memo(GovernanceRuleDetailPage);

import React from 'react';
import { Button, Dialog, Input, Radio, RadioGroup, Select, Space, Table, Tag, Tooltip } from 'components/Fluent';
import type { PrimaryTableProps, TableRowData } from 'components/Fluent';
import { AddIcon, RefreshIcon } from 'components/Fluent/icons';
import { useNavigate, useSearchParams } from 'components/Router';

import { useAppDispatch } from 'modules/store';
import { listAllNamespaces, selectNamespace } from 'modules/namespace';
import { useAppSelector } from 'modules/store';
import { ConfirmOperationButton, OperationButton } from 'components/OperationButton';
import { ResourceHeader, ResourceToolbar } from 'components/ResourceLayout';
import QueryComposer from 'components/QueryComposer';
import { trafficRuleCount, trafficRuleSummary } from '../Security/TrafficGovernanceEditor';
import { GovernanceServiceContext, GovernanceServiceRole } from '../shared/serviceContext';
import {
    listCustomRoutes,
    removeCustomRoutes,
} from 'modules/governance/route';
import {
    listRateLimitRules,
    removeRateLimitRule,
} from 'modules/governance/ratelimit';
import {
    listCircuitBreakers,
    removeCircuitBreakers,
} from 'modules/governance/circuitbreaker';
import {
    listFaultDetects,
    removeFaultDetects,
} from 'modules/governance/faultdetect';
import {
    listLossLessRules,
    removeLosslessRule,
} from 'modules/governance/lossless';
import {
    listLaneGroups,
    removeLaneGroups,
} from 'modules/governance/lane_group';
import { LimitActionMap, LimitType, LimitTypeMap, RateLimitView } from 'services/ratelimit';
import { CustomRouteView, normalizeRoutingConfigForEditor } from 'services/router';
import { CircuitBreakerRule } from 'services/circuitbreaker';
import { FaultDetectRule } from 'services/faultdetect';
import { LossLessRuleView } from 'services/lossless';
import { LaneGroupView } from 'services/lane';
import {
    deleteTrafficGovernanceRules,
    describeTrafficGovernanceRules,
    TrafficGovernanceKind,
    TrafficGovernanceKindLabel,
    TrafficGovernanceRule,
} from 'services/traffic_governance';
import { openErrNotification, openInfoNotification } from 'utils/notifition';
import ResourceNameLink from 'components/ResourceNameLink';
import style from './index.module.less';

type RuleKind = 'route' | 'ratelimit-local' | 'ratelimit-global' | 'circuitbreaker' | 'faultdetect' | 'lossless' | 'lane' | 'traffic-security' | 'traffic-mirror' | 'traffic-mock';

interface GovernanceRuleRow extends TableRowData {
    key: string;
    kind: RuleKind;
    typeLabel: string;
    name: string;
    description?: string;
    namespace?: string;
    service?: string;
    target: string;
    condition: string;
    status: string;
    release: string;
    ctime?: string;
    mtime?: string;
    editable?: boolean;
    deleteable?: boolean;
    raw: TableRowData;
    limitType?: LimitType;
    trafficKind?: TrafficGovernanceKind;
}

interface GovernanceWorkbenchProps {
    embedded?: boolean;
    serviceContext?: Pick<GovernanceServiceContext, 'namespace' | 'service'>;
}

const typeOptions = [
    { label: '路由', value: 'route', scene: '按主调、接口或标签把流量分配到不同目标服务或版本。' },
    { label: '限流', value: 'ratelimit', scene: '按请求数、Token 数或并发进行请求前准入，并通过窗口表达每秒、每分钟等统计周期。' },
    { label: '熔断', value: 'circuitbreaker', scene: '下游错误率、慢调用或异常升高时，快速切断不稳定依赖。' },
    { label: '探测', value: 'faultdetect', scene: '需要主动探测接口或服务健康，并据此辅助治理决策。' },
    { label: '无损', value: 'lossless', scene: '发布、重启或下线前需要延迟摘除流量，保护存量请求。' },
    { label: '泳道', value: 'lane', scene: '灰度、联调或多环境隔离时，把命中流量导入指定泳道服务。' },
    { label: '鉴权', value: 'traffic-security', scene: '需要按调用方、接口或请求条件控制访问许可。' },
    { label: '镜像', value: 'traffic-mirror', scene: '需要把线上流量复制到影子服务，用于回放、验证或压测。' },
    { label: 'Mock', value: 'traffic-mock', scene: '依赖未就绪或需要固定响应时，按接口返回模拟结果。' },
];

const statusOptions = [
    { label: '全部', value: '' },
    { label: '已启用', value: '启用' },
    { label: '已停用', value: '禁用' },
];

const createRuleTargets: Record<string, { kind: RuleKind; label: string; limitType?: LimitType; trafficKind?: TrafficGovernanceKind }> = {
    route: { kind: 'route', label: '路由' },
    ratelimit: { kind: 'ratelimit-local', label: '限流', limitType: LimitType.LOCAL },
    circuitbreaker: { kind: 'circuitbreaker', label: '熔断' },
    faultdetect: { kind: 'faultdetect', label: '探测' },
    lossless: { kind: 'lossless', label: '无损' },
    lane: { kind: 'lane', label: '泳道' },
    'traffic-security': { kind: 'traffic-security', label: '鉴权', trafficKind: 'security' },
    'traffic-mirror': { kind: 'traffic-mirror', label: '镜像', trafficKind: 'mirror' },
    'traffic-mock': { kind: 'traffic-mock', label: 'Mock', trafficKind: 'mock' },
};

const getActionPayload = <T,>(action: unknown): T | undefined => {
    const result = action as { meta?: { requestStatus?: string }, payload?: T };
    if (result.meta?.requestStatus === 'fulfilled') return result.payload;
    return undefined;
};

const text = (value?: string | number | boolean) => {
    if (value === undefined || value === null || value === '') return '-';
    return String(value);
};

const formatTime = (value?: string) => {
    if (!value) return '-';
    const date = new Date(value);
    if (Number.isNaN(date.getTime())) return value;
    const pad = (num: number) => `${num}`.padStart(2, '0');
    return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())} ${pad(date.getHours())}:${pad(date.getMinutes())}`;
};

const getTypeClassName = (kind: RuleKind) => {
    if (kind === 'route') return style.typeRoute;
    if (kind === 'ratelimit-local' || kind === 'ratelimit-global') return style.typeRatelimit;
    if (kind === 'circuitbreaker') return style.typeCircuitbreaker;
    if (kind === 'faultdetect') return style.typeFaultdetect;
    if (kind === 'lossless') return style.typeLossless;
    if (kind === 'lane') return style.typeLane;
    if (kind === 'traffic-security') return style.typeSecurity;
    if (kind === 'traffic-mirror') return style.typeMirror;
    if (kind === 'traffic-mock') return style.typeMock;
    return '';
};

const routeTarget = (rule: CustomRouteView) => {
    const routingConfig = normalizeRoutingConfigForEditor(rule.routing_config);
    const firstRule = routingConfig?.rules?.[0];
    const source = firstRule?.sources?.[0];
    const destination = firstRule?.destinations?.[0];
    return {
        namespace: routingConfig?.callee?.namespace || destination?.namespace || source?.namespace || '',
        service: routingConfig?.callee?.service || destination?.service || source?.service || '',
        target: `${text(routingConfig?.caller?.namespace || source?.namespace)}/${text(routingConfig?.caller?.service || source?.service)} -> ${text(routingConfig?.callee?.namespace || destination?.namespace)}/${text(routingConfig?.callee?.service || destination?.service)}`,
        condition: firstRule?.name || firstRule?.arguments?.arguments?.map((arg) => `${arg.type}:${arg.key}`).join(' / ') || '按主调 / 被调服务匹配',
    };
};

const normalizeRules = (items: {
    routes: CustomRouteView[];
    localRateLimits: RateLimitView[];
    globalRateLimits: RateLimitView[];
    circuitBreakers: CircuitBreakerRule[];
    faultDetects: FaultDetectRule[];
    losslessRules: LossLessRuleView[];
    laneGroups: LaneGroupView[];
    trafficSecurityRules: TrafficGovernanceRule[];
    trafficMirrorRules: TrafficGovernanceRule[];
    trafficMockRules: TrafficGovernanceRule[];
}): GovernanceRuleRow[] => {
    const routeRows = items.routes.map((rule) => {
        const target = routeTarget(rule);
        return {
            key: `route-${rule.id || rule.name}`,
            kind: 'route' as RuleKind,
            typeLabel: '路由',
            name: rule.name,
            description: rule.description,
            namespace: rule.namespace || target.namespace,
            service: target.service,
            target: target.target,
            condition: target.condition,
            status: rule.enable === false ? '禁用' : '启用',
            release: rule.etime ? '已发布' : '待发布',
            ctime: rule.ctime,
            mtime: rule.mtime,
            editable: rule.editable,
            deleteable: rule.deleteable,
            raw: rule as TableRowData,
        };
    });

    const rateRows = [...items.localRateLimits, ...items.globalRateLimits].map((rule) => {
        const firstRule = rule.rules?.[0];
        return {
            key: `ratelimit-${rule.type}-${rule.id || rule.name}`,
            kind: rule.type === LimitType.GLOBAL ? 'ratelimit-global' as RuleKind : 'ratelimit-local' as RuleKind,
            typeLabel: LimitTypeMap[rule.type] || '限流',
            name: rule.name,
            description: rule.description,
            namespace: rule.namespace,
            service: rule.callee?.service,
            target: `${text(rule.callee?.namespace)}/${text(rule.callee?.service)}`,
            condition: firstRule ? `${text(firstRule.apis?.[0]?.path?.value || firstRule.method?.value || firstRule.resource)} / ${LimitActionMap[firstRule.action] || firstRule.action || '-'}` : '按接口或资源限流',
            status: rule.disable ? '禁用' : '启用',
            release: rule.revision ? '已发布' : '待发布',
            ctime: rule.ctime,
            mtime: rule.mtime,
            editable: rule.editable,
            deleteable: rule.deleteable,
            raw: rule as TableRowData,
            limitType: rule.type,
        };
    });

    const circuitRows = items.circuitBreakers.map((rule) => ({
        key: `circuitbreaker-${rule.id || rule.name}`,
        kind: 'circuitbreaker' as RuleKind,
        typeLabel: '熔断',
        name: rule.name,
        description: rule.description,
        namespace: rule.namespace || rule.ruleMatcher?.destination?.namespace,
        service: rule.ruleMatcher?.destination?.service,
        target: `${text(rule.ruleMatcher?.source?.namespace)}/${text(rule.ruleMatcher?.source?.service)} -> ${text(rule.ruleMatcher?.destination?.namespace)}/${text(rule.ruleMatcher?.destination?.service)}`,
        condition: rule.block_configs?.[0]?.name || '按错误条件和触发条件熔断',
        status: '启用',
        release: rule.etime ? '已发布' : '待发布',
        ctime: rule.ctime,
        mtime: rule.mtime,
        editable: rule.editable,
        deleteable: rule.deleteable,
        raw: rule as TableRowData,
    }));

    const faultRows = items.faultDetects.map((rule) => ({
        key: `faultdetect-${rule.id || rule.name}`,
        kind: 'faultdetect' as RuleKind,
        typeLabel: '探测',
        name: rule.name,
        description: rule.description,
        namespace: rule.namespace || rule.targetService?.namespace,
        service: rule.targetService?.service,
        target: `${text(rule.targetService?.namespace)}/${text(rule.targetService?.service)}`,
        condition: `${text(rule.protocol)} / ${text(rule.targetService?.api?.path?.value || rule.httpConfig?.url)}`,
        status: '启用',
        release: rule.mtime ? '已发布' : '待发布',
        ctime: rule.ctime,
        mtime: rule.mtime,
        editable: rule.editable,
        deleteable: rule.deleteable,
        raw: rule as TableRowData,
    }));

    const losslessRows = items.losslessRules.map((rule) => ({
        key: `lossless-${rule.id || `${rule.namespace}-${rule.service}`}`,
        kind: 'lossless' as RuleKind,
        typeLabel: '无损',
        name: `${rule.namespace}/${rule.service}`,
        description: '无损上线、服务预热与无损下线',
        namespace: rule.namespace,
        service: rule.service,
        target: `${text(rule.namespace)}/${text(rule.service)}`,
        condition: [
            rule.lossless_online?.delay_register?.enable ? '延迟注册' : '',
            rule.lossless_online?.warmup?.enable ? '服务预热' : '',
            rule.lossless_offline?.enable ? '无损下线' : '',
        ].filter(Boolean).join(' / ') || '无损上下线',
        status: '启用',
        release: rule.revision ? '已发布' : '待发布',
        ctime: rule.ctime,
        mtime: rule.mtime,
        editable: rule.editable,
        deleteable: rule.deleteable,
        raw: rule as TableRowData,
    }));

    const laneRows = items.laneGroups.map((rule) => ({
        key: `lane-${rule.id || rule.name}`,
        kind: 'lane' as RuleKind,
        typeLabel: '泳道',
        name: rule.name || '-',
        description: rule.description,
        namespace: rule.namespace || rule.destinations?.[0]?.namespace,
        service: rule.destinations?.[0]?.service,
        target: `${rule.rules?.length || 0} 个泳道 / ${rule.destinations?.length || 0} 个目标服务`,
        condition: rule.entries?.map((entry) => `${text(entry.selector?.namespace)}/${text(entry.selector?.service)}`).join(', ') || '按入口服务匹配',
        status: '启用',
        release: rule.mtime ? '已发布' : '待发布',
        ctime: rule.ctime,
        mtime: rule.mtime,
        editable: rule.editable,
        deleteable: rule.deleteable,
        raw: rule as TableRowData,
    }));

    const trafficRows = ([
        ['traffic-security', 'security', items.trafficSecurityRules],
        ['traffic-mirror', 'mirror', items.trafficMirrorRules],
        ['traffic-mock', 'mock', items.trafficMockRules],
    ] as Array<[RuleKind, TrafficGovernanceKind, TrafficGovernanceRule[]]>).flatMap(([rowKind, trafficKind, rules]) => (
        rules.map((rule) => {
            const caller = rule.caller;
            const callee = rule.callee || rule.target_service || {};
            const callerLabel = caller?.namespace || caller?.service
                ? `${text(caller.namespace)}/${text(caller.service)}`
                : '';
            const calleeLabel = `${text(callee?.namespace)}/${text(callee?.service)}`;
            return {
                key: `${rowKind}-${rule.id || rule.name}`,
                kind: rowKind,
                typeLabel: TrafficGovernanceKindLabel[trafficKind],
                name: rule.name,
                description: rule.description,
                namespace: rule.namespace || callee?.namespace,
                service: callee?.service,
                target: callerLabel ? `${callerLabel} -> ${calleeLabel}` : calleeLabel,
                condition: `${trafficRuleCount(trafficKind, rule)} 条 / ${trafficRuleSummary(trafficKind, rule)}`,
                status: rule.enable ? '启用' : '禁用',
                release: rule.revision ? '已发布' : '待发布',
                ctime: rule.ctime,
                mtime: rule.mtime,
                editable: rule.editable,
                deleteable: rule.deleteable,
                raw: rule as TableRowData,
                trafficKind,
            };
        })
    ));

    return [...routeRows, ...rateRows, ...circuitRows, ...faultRows, ...losslessRows, ...laneRows, ...trafficRows];
};

const matchesEndpoint = (endpoint: unknown, context: Pick<GovernanceServiceContext, 'namespace' | 'service'>) => {
    const value = endpoint as { namespace?: string; service?: string } | undefined;
    return value?.namespace === context.namespace && value?.service === context.service;
};

export const isRuleAssociatedWithService = (
    rule: GovernanceRuleRow,
    context: Pick<GovernanceServiceContext, 'namespace' | 'service'>,
) => {
    const raw = rule.raw as Record<string, any>;
    if (rule.kind === 'route') {
        const config = normalizeRoutingConfigForEditor(raw.routing_config);
        return matchesEndpoint(config?.caller, context)
            || matchesEndpoint(config?.callee, context)
            || (config?.rules || []).some((item) => (
                (item.sources || []).some((source) => matchesEndpoint(source, context))
                || (item.destinations || []).some((destination) => matchesEndpoint(destination, context))
            ));
    }
    if (rule.kind === 'circuitbreaker') {
        return matchesEndpoint(raw.ruleMatcher?.source, context)
            || matchesEndpoint(raw.ruleMatcher?.destination, context);
    }
    if (rule.kind === 'lane') {
        return (raw.entries || []).some((entry: any) => matchesEndpoint(entry.selector, context))
            || (raw.destinations || []).some((destination: any) => matchesEndpoint(destination, context));
    }
    if (rule.trafficKind) {
        return matchesEndpoint(raw.caller, context)
            || matchesEndpoint(raw.callee, context)
            || matchesEndpoint(raw.target_service, context)
            || (raw.namespace === context.namespace && raw.service === context.service);
    }
    return matchesEndpoint(raw.callee || raw.targetService || raw.target_service, context);
};

const GovernanceWorkbench: React.FC<GovernanceWorkbenchProps> = ({ embedded = false, serviceContext }) => {
    const dispatch = useAppDispatch();
    const navigate = useNavigate();
    const [searchParams] = useSearchParams();
    const requestedNamespace = searchParams.get('namespace') || '';
    const { datas: namespaceDatas } = useAppSelector(selectNamespace);
    const [rules, setRules] = React.useState<GovernanceRuleRow[]>([]);
    const [loading, setLoading] = React.useState(false);
    const [search, setSearch] = React.useState('');
    const [typeFilters, setTypeFilters] = React.useState<string[]>([]);
    const [statusFilter, setStatusFilter] = React.useState('');
    const [selectedNamespace, setSelectedNamespace] = React.useState(serviceContext?.namespace || requestedNamespace || '');
    const [paginationVersion, setPaginationVersion] = React.useState(0);
    const [createWizardVisible, setCreateWizardVisible] = React.useState(false);
    const [serviceRole, setServiceRole] = React.useState<GovernanceServiceRole>('caller');

    const createServiceContext = serviceContext ? {
        ...serviceContext,
        role: serviceRole,
    } : undefined;

    React.useEffect(() => {
        dispatch(listAllNamespaces());
    }, [dispatch]);

    React.useEffect(() => {
        if (serviceContext?.namespace) setSelectedNamespace(serviceContext.namespace);
    }, [serviceContext?.namespace]);

    const refreshData = React.useCallback(async (query = search, namespace = selectedNamespace) => {
        setLoading(true);
        try {
            const queryLimit = serviceContext ? 100 : 20;
            const [
                routeAction,
                localRateAction,
                globalRateAction,
                circuitAction,
                faultAction,
                losslessAction,
                laneAction,
                trafficSecurityAction,
                trafficMirrorAction,
                trafficMockAction,
            ] = await Promise.all([
                dispatch(listCustomRoutes({ param: { offset: 0, limit: queryLimit, route_type: 'RulePolicy', name: query, namespace } })),
                dispatch(listRateLimitRules({ param: { offset: 0, limit: queryLimit, name: query, limit_type: LimitType.LOCAL, namespace } })),
                dispatch(listRateLimitRules({ param: { offset: 0, limit: queryLimit, name: query, limit_type: LimitType.GLOBAL, namespace } })),
                dispatch(listCircuitBreakers({ param: { offset: 0, limit: queryLimit, name: query, brief: true, namespace } })),
                dispatch(listFaultDetects({ param: { offset: 0, limit: queryLimit, name: query, brief: true, namespace } })),
                dispatch(listLossLessRules({ param: { offset: 0, limit: queryLimit, name: query, namespace } })),
                dispatch(listLaneGroups({ param: { offset: 0, limit: queryLimit, name: query, brief: true, namespace } })),
                describeTrafficGovernanceRules('security', { offset: 0, limit: queryLimit, name: query, namespace }),
                describeTrafficGovernanceRules('mirror', { offset: 0, limit: queryLimit, name: query, namespace }),
                describeTrafficGovernanceRules('mock', { offset: 0, limit: queryLimit, name: query, namespace }),
            ]);

            const routePayload = getActionPayload<{ datas: CustomRouteView[] }>(routeAction);
            const localRatePayload = getActionPayload<{ datas: RateLimitView[] }>(localRateAction);
            const globalRatePayload = getActionPayload<{ datas: RateLimitView[] }>(globalRateAction);
            const circuitPayload = getActionPayload<{ datas: CircuitBreakerRule[] }>(circuitAction);
            const faultPayload = getActionPayload<{ datas: FaultDetectRule[] }>(faultAction);
            const losslessPayload = getActionPayload<{ datas: LossLessRuleView[] }>(losslessAction);
            const lanePayload = getActionPayload<{ datas: LaneGroupView[] }>(laneAction);
            const trafficSecurityPayload = trafficSecurityAction as { list: TrafficGovernanceRule[] };
            const trafficMirrorPayload = trafficMirrorAction as { list: TrafficGovernanceRule[] };
            const trafficMockPayload = trafficMockAction as { list: TrafficGovernanceRule[] };

            setRules(normalizeRules({
                routes: routePayload?.datas || [],
                localRateLimits: localRatePayload?.datas || [],
                globalRateLimits: globalRatePayload?.datas || [],
                circuitBreakers: circuitPayload?.datas || [],
                faultDetects: faultPayload?.datas || [],
                losslessRules: losslessPayload?.datas || [],
                laneGroups: lanePayload?.datas || [],
                trafficSecurityRules: trafficSecurityPayload.list || [],
                trafficMirrorRules: trafficMirrorPayload.list || [],
                trafficMockRules: trafficMockPayload.list || [],
            }));
        } catch (error) {
            openErrNotification('请求失败', `查询治理规则列表错误: ${(error as Error).message}`);
        } finally {
            setLoading(false);
        }
    }, [dispatch, search, selectedNamespace, serviceContext?.namespace, serviceContext?.service]);

    React.useEffect(() => {
        refreshData('');
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, []);

    const openRule = (rule: GovernanceRuleRow) => {
        const id = rule.raw.id as string | undefined;
        if (!id) {
            openErrNotification('无法打开详情', '当前规则缺少详情 ID');
            return;
        }
        const params = new URLSearchParams({
            kind: rule.kind,
            id,
            name: rule.name,
        });
        if (rule.namespace) params.set('ruleNamespace', rule.namespace);
        navigate(`/governance/rules/detail?${params.toString()}`);
    };

    const deleteRule = async (rule: GovernanceRuleRow) => {
        const id = rule.raw.id as string | undefined;
        if (!id) {
            openErrNotification('删除失败', '当前规则缺少可删除的 ID');
            return;
        }
        try {
            if (rule.kind === 'route') {
                await dispatch(removeCustomRoutes({ ids: [id] }));
            } else if (rule.kind === 'ratelimit-local' || rule.kind === 'ratelimit-global') {
                await dispatch(removeRateLimitRule({ ids: [id] }));
            } else if (rule.kind === 'circuitbreaker') {
                await dispatch(removeCircuitBreakers({ ids: [id] }));
            } else if (rule.kind === 'faultdetect') {
                await dispatch(removeFaultDetects({ ids: [id] }));
            } else if (rule.kind === 'lossless') {
                await dispatch(removeLosslessRule({ ids: [id] }));
            } else if (rule.kind === 'lane') {
                await dispatch(removeLaneGroups({ ids: [id] }));
            } else if (rule.trafficKind) {
                await deleteTrafficGovernanceRules(rule.trafficKind, [{ id }]);
            }
            openInfoNotification('请求成功', '删除治理规则成功');
            refreshData(search);
        } catch (error) {
            openErrNotification('删除失败', `删除治理规则错误: ${(error as Error).message}`);
        }
    };

    const openCreateRule = (type: string) => {
        const target = createRuleTargets[type];
        if (!target) return;
        const params = new URLSearchParams({ kind: target.kind });
        params.set('ruleNamespace', createServiceContext?.namespace || selectedNamespace);
        if (createServiceContext) {
            params.set('namespace', createServiceContext.namespace);
            params.set('service', createServiceContext.service);
            params.set('role', createServiceContext.role);
        }
        navigate(`/governance/rules/create?${params.toString()}`);
    };

    const openCreateWizard = () => setCreateWizardVisible(true);

    const createRuleFromType = (type: string) => {
        setCreateWizardVisible(false);
        openCreateRule(type);
    };

    const scopedRules = serviceContext
        ? rules.filter((rule) => isRuleAssociatedWithService(rule, serviceContext))
        : rules;

    const filteredRules = scopedRules.filter((rule) => {
        const namespaceMatched = !selectedNamespace || rule.namespace === selectedNamespace;
        const typeMatched = typeFilters.length === 0 || typeFilters.some((type) => (type === 'ratelimit' ? rule.kind.startsWith('ratelimit') : rule.kind === type));
        const searchMatched = !search || [rule.name, rule.description, rule.namespace, rule.service, rule.condition].some((item) => item?.toLowerCase().includes(search.toLowerCase()));
        const statusMatched = !statusFilter || rule.status === statusFilter;
        return namespaceMatched && typeMatched && searchMatched && statusMatched;
    });

    const namespaceOptions = namespaceDatas.map((item) => ({ label: item.name, value: item.name }));
    const enabledCount = scopedRules.filter((item) => item.status === '启用').length;
    const pendingCount = scopedRules.filter((item) => item.release === '待发布').length;

    const columns: PrimaryTableProps['columns'] = [
        {
            colKey: 'name',
            title: '规则',
            width: 300,
            cell: ({ row }) => (
                <ResourceNameLink
                    className={style.ruleNameText}
                    name={row.name}
                    onClick={() => openRule(row as GovernanceRuleRow)}
                />
            ),
        },
        {
            colKey: 'typeLabel',
            title: '类型',
            width: 110,
            cell: ({ row }) => <Tag className={`${style.typeTag} ${getTypeClassName(row.kind as RuleKind)}`} variant="light-outline">{row.typeLabel}</Tag>,
        },
        {
            colKey: 'target',
            title: '作用对象',
            minWidth: 280,
            cell: ({ row }) => <div className={style.target}>{row.target}</div>,
        },
        {
            colKey: 'status',
            title: '状态',
            width: 120,
            cell: ({ row }) => (
                <span className={style.statusCell}>
                    <span className={row.status === '启用' ? style.statusEnabled : style.statusDisabled} />
                    {row.status === '启用' ? '已启用' : '已停用'}
                </span>
            ),
        },
        {
            colKey: 'time',
            title: '操作时间',
            width: 190,
            cell: ({ row }) => (
                <div className={style.timeCell}>
                    <span className={style.timeText}>{formatTime(row.mtime)}</span>
                    <span className={style.timeSecondary}>创建: {formatTime(row.ctime)}</span>
                </div>
            ),
        },
        {
            colKey: 'operation',
            title: '操作',
            width: 112,
            fixed: 'right',
            cell: ({ row }) => {
                const rule = row as GovernanceRuleRow;
                return (
                    <Space className={style.rowActions} size={10} onClick={(event) => event.stopPropagation()}>
                        <OperationButton action="viewEdit" onClick={() => openRule(rule)} />
                        <ConfirmOperationButton action="delete" disabled={rule.deleteable === false} disabledLabel="无权限操作" confirmContent={`确认删除规则 ${rule.name}？`} onConfirm={() => deleteRule(rule)} />
                    </Space>
                );
            },
        },
    ];

    const renderCreateWizard = () => (
        <Dialog
            header="新建规则"
            visible={createWizardVisible}
            width={720}
            footer={false}
            onClose={() => setCreateWizardVisible(false)}
        >
                <div className={style.createWizard}>
                <Select
                    label="归属环境"
                    value={selectedNamespace}
                    options={namespaceOptions}
                    disabled={!!serviceContext}
                    onChange={(value) => setSelectedNamespace(value as string)}
                />
                <div className={style.createTypeGrid}>
                    {typeOptions.map((item) => {
                        const target = createRuleTargets[item.value];
                        return (
                            <Button
                                variant="text"
                                key={item.value}
                                className={style.createTypeCard}
                                type="button"
                                aria-label={`创建${item.label}规则`}
                                disabled={!selectedNamespace}
                                onClick={() => createRuleFromType(item.value)}
                            >
                                <Tag className={`${style.typeTag} ${getTypeClassName(target.kind)}`} variant="light-outline">{item.label}</Tag>
                                <p>{item.scene}</p>
                            </Button>
                        );
                    })}
                </div>
            </div>
        </Dialog>
    );

    return (
        <div className={`${style.page} ${embedded ? style.embeddedPage : ''}`}>
            {embedded && serviceContext ? (
                <div className={style.serviceContextBar}>
                    <div className={style.serviceContextIdentity}>
                        <span className={style.serviceContextLabel}>当前服务</span>
                        <strong title={`${serviceContext.namespace}/${serviceContext.service}`}>
                            {serviceContext.namespace}/{serviceContext.service}
                        </strong>
                    </div>
                    <div className={style.serviceContextActions}>
                        <div className={style.serviceRoleControl}>
                            <span>双端规则角色</span>
                            <RadioGroup
                                theme="button"
                                variant="primary-filled"
                                value={serviceRole}
                                onChange={(value) => setServiceRole(value as GovernanceServiceRole)}
                            >
                                <Radio.Button value="caller">作为主调方</Radio.Button>
                                <Radio.Button value="callee">作为被调方</Radio.Button>
                            </RadioGroup>
                        </div>
                    </div>
                </div>
            ) : <ResourceHeader
                density="compact"
                placement="app-header"
                eyebrow="治理管理 / 规则治理"
                title="规则治理工作台"
                description="命名空间用于标识规则归属环境；caller、callee 和目标服务属于独立的运行时作用范围。"
            />}
            {!embedded && <div className={style.summary}>
                <div className={style.summaryItem}>
                    <div className={style.summaryLabel}>规则总数</div>
                    <div className={style.summaryValue}>{scopedRules.length}</div>
                    <div className={style.summaryHint}>当前环境：{selectedNamespace || '全部环境'}</div>
                </div>
                <div className={style.summaryItem}>
                    <div className={style.summaryLabel}>已启用</div>
                    <div className={style.summaryValue}>{enabledCount}</div>
                    <div className={style.summaryHint}>路由、限流、熔断、探测等规则</div>
                </div>
                <div className={style.summaryItem}>
                    <div className={style.summaryLabel}>待发布</div>
                    <div className={style.summaryValue}>{pendingCount}</div>
                    <div className={style.summaryHint}>需进入详情确认版本</div>
                </div>
                <div className={style.summaryItem}>
                    <div className={style.summaryLabel}>规则类型</div>
                    <div className={style.summaryValue}>{new Set(scopedRules.map((item) => item.typeLabel)).size}</div>
                    <div className={style.summaryHint}>按治理场景分类查看</div>
                </div>
            </div>}
            <ResourceToolbar
                density="compact"
                title={embedded ? '关联规则' : '规则清单'}
                count={loading ? '正在同步列表' : `当前显示 ${filteredRules.length} / ${scopedRules.length} 条`}
                className={style.panelToolbar}
                filters={(
                    <>
                        <QueryComposer
                            keyword={search}
                            keywordPlaceholder="搜索规则名、服务或条件"
                            suggestions={scopedRules.map((item) => item.name).filter(Boolean)}
                            fields={[
                                ...(!serviceContext ? [{
                                    key: 'namespace',
                                    label: '归属环境',
                                    type: 'select' as const,
                                    filterable: true,
                                    options: namespaceOptions,
                                }] : []),
                                {
                                    key: 'types',
                                    label: '规则类型',
                                    type: 'multiselect',
                                    filterable: true,
                                    options: typeOptions,
                                },
                                {
                                    key: 'status',
                                    label: '状态',
                                    type: 'select',
                                    options: statusOptions,
                                },
                            ]}
                            values={{
                                ...(!serviceContext ? { namespace: selectedNamespace } : {}),
                                types: typeFilters,
                                status: statusFilter,
                            }}
                            onKeywordChange={setSearch}
                            onValuesChange={(values) => {
                                if (!serviceContext) setSelectedNamespace(String(values.namespace || ''));
                                setTypeFilters(Array.isArray(values.types) ? values.types.map(String) : []);
                                setStatusFilter(String(values.status || ''));
                            }}
                            onSubmit={({ keyword, values }) => {
                                const nextNamespace = serviceContext?.namespace || String(values.namespace || '');
                                setPaginationVersion((version) => version + 1);
                                refreshData(keyword, nextNamespace);
                            }}
                            onReset={() => {
                                const nextNamespace = serviceContext?.namespace || '';
                                setTypeFilters([]);
                                setStatusFilter('');
                                setSearch('');
                                setSelectedNamespace(nextNamespace);
                                setPaginationVersion((version) => version + 1);
                                refreshData('', nextNamespace);
                            }}
                            loading={loading}
                            actions={(
                                <>
                                    <Tooltip content="刷新规则清单">
                                        <Button
                                            aria-label="刷新规则清单"
                                            shape="square"
                                            variant="outline"
                                            icon={<RefreshIcon />}
                                            onClick={() => refreshData(search)}
                                        />
                                    </Tooltip>
                                    <Button theme="primary" icon={<AddIcon />} onClick={openCreateWizard}>
                                        新建规则
                                    </Button>
                                </>
                            )}
                        />
                    </>
                )}
            />
            <div className={style.listPanel}>
                <Table
                    key={paginationVersion}
                    className={style.table}
                    data={filteredRules}
                    columns={columns}
                    loading={loading}
                    rowKey="key"
                    tableLayout="fixed"
                    cellEmptyContent="-"
                    pagination={{
                        pageSize: 10,
                        total: filteredRules.length,
                        showJumper: true,
                    }}
                    onRowClick={({ row }) => openRule(row as GovernanceRuleRow)}
                />
            </div>
            {renderCreateWizard()}
        </div>
    );
};

export default React.memo(GovernanceWorkbench);

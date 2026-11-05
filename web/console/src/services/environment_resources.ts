import { deleteCircuitBreaker, describeCircuitBreakers } from './circuitbreaker';
import { deleteA2AAgents, describeA2AAgents } from './a2a';
import { deleteFaultDetects, describeFaultDetects } from './faultdetect';
import { deleteLaneGroups, describeLaneGroups } from './lane';
import { deleteLossLessRule, describeLossLessRules } from './lossless';
import { deleteMCPServers, describeMCPServers } from './mcp';
import { deleteRateLimit, describeLimitRules, LimitType } from './ratelimit';
import { deleteCustomRoute, describeCustomRoutes } from './router';
import { deleteServices, describeAllServices } from './service';
import {
    deleteTrafficGovernanceRules,
    describeTrafficGovernanceRules,
    TrafficGovernanceKind,
    TrafficGovernanceKindLabel,
} from './traffic_governance';

export type EnvironmentResourceDomain = 'discovery' | 'configuration' | 'governance' | 'ai';

export type EnvironmentResourceKind =
    | 'service'
    | 'route'
    | 'ratelimit-local'
    | 'ratelimit-global'
    | 'circuitbreaker'
    | 'faultdetect'
    | 'lossless'
    | 'lane'
    | 'traffic-security'
    | 'traffic-mirror'
    | 'traffic-mock'
    | 'mcp'
    | 'a2a';

export interface EnvironmentResource {
    key: string
    id?: string
    name: string
    namespace: string
    kind: EnvironmentResourceKind
    domain: EnvironmentResourceDomain
    domainLabel: string
    typeLabel: string
    description: string
    status: string
    statusTone: 'success' | 'warning' | 'neutral'
    metric: string
    mtime?: string
    target: string
    deleteable: boolean
    deleteDisabledReason?: string
}

export interface EnvironmentResourceSnapshot {
    resources: EnvironmentResource[]
    warnings: string[]
}

interface PageResult<T> {
    list: T[]
    totalCount: number
}

const PAGE_SIZE = 100;

const collectAll = async <T,>(loader: (offset: number, limit: number) => Promise<PageResult<T>>) => {
    const result: T[] = [];
    let offset = 0;
    let total = Number.POSITIVE_INFINITY;
    while (offset < total) {
        const page = await loader(offset, PAGE_SIZE);
        result.push(...page.list);
        total = page.totalCount;
        if (!page.list.length || result.length >= total) break;
        offset += page.list.length;
    }
    return result;
};

const asRecord = (value: unknown) => (value || {}) as Record<string, any>;

const ruleTarget = (rule: Record<string, any>) => {
    const service = rule.service
        || rule.callee?.service
        || rule.target_service?.service
        || rule.destination?.service
        || rule.dstService;
    return service ? `服务 ${service}` : '环境级规则';
};

const ruleResource = (
    namespace: string,
    kind: EnvironmentResourceKind,
    typeLabel: string,
    raw: unknown,
): EnvironmentResource => {
    const rule = asRecord(raw);
    const id = String(rule.id || '');
    const name = String(rule.name || typeLabel);
    const enabled = rule.enable !== false && rule.enabled !== false;
    const params = new URLSearchParams({ kind, id, name, ruleNamespace: namespace });
    return {
        key: `governance:${kind}:${id || name}`,
        id: id || undefined,
        name,
        namespace,
        kind,
        domain: 'governance',
        domainLabel: '治理规则',
        typeLabel,
        description: String(rule.description || rule.comment || ruleTarget(rule)),
        status: enabled ? '已启用' : '已停用',
        statusTone: enabled ? 'success' : 'neutral',
        metric: ruleTarget(rule),
        mtime: String(rule.mtime || rule.modifyTime || ''),
        target: id
            ? `/governance/rules/detail?${params.toString()}`
            : `/governance/workbench?namespace=${encodeURIComponent(namespace)}`,
        deleteable: Boolean(id) && rule.deleteable !== false,
    };
};

const loadGovernance = async (namespace: string) => {
    const loaders: Array<{
        key: EnvironmentResourceKind
        typeLabel: string
        load: () => Promise<unknown[]>
    }> = [
        {
            key: 'route',
            typeLabel: '路由',
            load: () => collectAll((offset, limit) => describeCustomRoutes({
                offset, limit, route_type: 'RulePolicy' as any, namespace,
            })),
        },
        {
            key: 'ratelimit-local',
            typeLabel: '本地限流',
            load: () => collectAll((offset, limit) => describeLimitRules({
                offset, limit, limit_type: LimitType.LOCAL, namespace, brief: true,
            })),
        },
        {
            key: 'ratelimit-global',
            typeLabel: '全局限流',
            load: () => collectAll((offset, limit) => describeLimitRules({
                offset, limit, limit_type: LimitType.GLOBAL, namespace, brief: true,
            })),
        },
        {
            key: 'circuitbreaker',
            typeLabel: '熔断',
            load: () => collectAll((offset, limit) => describeCircuitBreakers({
                offset, limit, namespace, brief: true,
            })),
        },
        {
            key: 'faultdetect',
            typeLabel: '主动探测',
            load: () => collectAll((offset, limit) => describeFaultDetects({
                offset, limit, namespace, brief: true,
            })),
        },
        {
            key: 'lossless',
            typeLabel: '无损上下线',
            load: () => collectAll((offset, limit) => describeLossLessRules({ offset, limit, namespace })),
        },
        {
            key: 'lane',
            typeLabel: '泳道',
            load: () => collectAll(async (offset, limit) => {
                const result = await describeLaneGroups({ offset, limit, namespace, brief: true });
                return { list: result.data || [], totalCount: result.amount || 0 };
            }),
        },
        ...(['security', 'mirror', 'mock'] as TrafficGovernanceKind[]).map((trafficKind) => ({
            key: `traffic-${trafficKind}` as EnvironmentResourceKind,
            typeLabel: TrafficGovernanceKindLabel[trafficKind],
            load: () => collectAll((offset, limit) => describeTrafficGovernanceRules(
                trafficKind,
                { offset, limit, namespace },
            )),
        })),
    ];

    const settled = await Promise.allSettled(loaders.map((loader) => loader.load()));
    const resources: EnvironmentResource[] = [];
    const warnings: string[] = [];
    settled.forEach((result, index) => {
        const loader = loaders[index];
        if (result.status === 'rejected') {
            warnings.push(`${loader.typeLabel}查询失败`);
            return;
        }
        resources.push(...result.value.map((item) => ruleResource(namespace, loader.key, loader.typeLabel, item)));
    });
    return { resources, warnings };
};

export async function describeEnvironmentResources(namespace: string): Promise<EnvironmentResourceSnapshot> {
    const [serviceResult, governanceResult, mcpResult, a2aResult] = await Promise.allSettled([
        describeAllServices({ namespace }),
        loadGovernance(namespace),
        collectAll((offset, limit) => describeMCPServers({ offset, limit, namespace })),
        collectAll((offset, limit) => describeA2AAgents({ offset, limit, namespace })),
    ]);

    const resources: EnvironmentResource[] = [];
    const warnings: string[] = [];

    if (serviceResult.status === 'fulfilled') {
        resources.push(...serviceResult.value
            .filter((service) => service.namespace === namespace)
            .map((service) => {
                const total = Number(service.total_instance_count || 0);
                const healthy = Number(service.healthy_instance_count || 0);
                return {
                    key: `discovery:${service.id}`,
                    id: service.id,
                    name: service.name,
                    namespace,
                    kind: 'service' as const,
                    domain: 'discovery' as const,
                    domainLabel: '注册发现',
                    typeLabel: '运行时服务',
                    description: service.comment || service.department || service.business || '服务实例与健康状态',
                    status: total === 0 ? '无实例' : healthy === total ? '健康' : '部分异常',
                    statusTone: total === 0 ? 'neutral' as const : healthy === total ? 'success' as const : 'warning' as const,
                    metric: `${healthy}/${total} 健康实例`,
                    mtime: service.mtime,
                    target: `/discovery/service/instance?namespace=${encodeURIComponent(namespace)}&service=${encodeURIComponent(service.name)}`,
                    deleteable: service.deleteable !== false,
                };
            }));
    } else {
        warnings.push('服务资源查询失败');
    }

    if (governanceResult.status === 'fulfilled') {
        resources.push(...governanceResult.value.resources);
        warnings.push(...governanceResult.value.warnings);
    } else {
        warnings.push('治理资源查询失败');
    }

    if (mcpResult.status === 'fulfilled') {
        resources.push(...mcpResult.value
            .filter((server) => server.namespace === namespace)
            .map((server) => ({
                key: `ai:mcp:${server.id || server.name}`,
                id: server.id,
                name: server.name,
                namespace,
                kind: 'mcp' as const,
                domain: 'ai' as const,
                domainLabel: 'AI 工具',
                typeLabel: 'MCP Server',
                description: server.description || '模型上下文工具服务',
                status: Number(server.flag || 0) === 1 ? '已停用' : '可用',
                statusTone: Number(server.flag || 0) === 1 ? 'neutral' as const : 'success' as const,
                metric: server.protocol || server.backend_type || 'MCP',
                mtime: server.mtime,
                target: `/ai/mcps/detail?id=${encodeURIComponent(String(server.id || ''))}&namespace=${encodeURIComponent(namespace)}&name=${encodeURIComponent(server.name)}`,
                deleteable: Boolean(server.id),
            })));
    } else {
        warnings.push('MCP 资源查询失败');
    }

    if (a2aResult.status === 'fulfilled') {
        resources.push(...a2aResult.value
            .filter((agent) => agent.namespace === namespace)
            .map((agent) => ({
                key: `ai:a2a:${agent.id || agent.name}`,
                id: agent.id,
                name: agent.name,
                namespace,
                kind: 'a2a' as const,
                domain: 'ai' as const,
                domainLabel: 'AI 工具',
                typeLabel: 'A2A Agent',
                description: agent.description || 'Agent 对 Agent 协作资源',
                status: Number(agent.flag || 0) === 1 ? '已停用' : '可用',
                statusTone: Number(agent.flag || 0) === 1 ? 'neutral' as const : 'success' as const,
                metric: agent.preferred_protocol_binding || agent.backend_type || 'A2A',
                mtime: agent.mtime,
                target: `/ai/a2a/detail?id=${encodeURIComponent(String(agent.id || ''))}&namespace=${encodeURIComponent(namespace)}&name=${encodeURIComponent(agent.name)}&tab=card`,
                deleteable: Boolean(agent.id),
            })));
    } else {
        warnings.push('A2A 资源查询失败');
    }

    return {
        resources: resources.sort((left, right) => (
            String(right.mtime || '').localeCompare(String(left.mtime || ''))
            || left.name.localeCompare(right.name)
        )),
        warnings,
    };
}

export async function deleteEnvironmentResource(resource: EnvironmentResource) {
    const id = resource.id || '';
    if (!resource.deleteable) throw new Error('当前账号无权删除该资源');

    switch (resource.kind) {
        case 'service':
            return deleteServices([{ id, name: resource.name, namespace: resource.namespace }]);
        case 'route':
            return deleteCustomRoute([{ id }]);
        case 'ratelimit-local':
        case 'ratelimit-global':
            return deleteRateLimit([{ id }]);
        case 'circuitbreaker':
            return deleteCircuitBreaker([{ id }]);
        case 'faultdetect':
            return deleteFaultDetects([{ id }]);
        case 'lossless':
            return deleteLossLessRule([{ id }]);
        case 'lane':
            return deleteLaneGroups([{ id }]);
        case 'traffic-security':
        case 'traffic-mirror':
        case 'traffic-mock':
            return deleteTrafficGovernanceRules(resource.kind.replace('traffic-', '') as TrafficGovernanceKind, [{ id }]);
        case 'mcp':
            return deleteMCPServers([id]);
        case 'a2a':
            return deleteA2AAgents([id]);
        default:
            throw new Error(`暂不支持删除 ${resource.typeLabel}`);
    }
}

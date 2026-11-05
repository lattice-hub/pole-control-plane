import request, { apiRequest, ApiResponse, getAllList, getApiRequest, putApiRequest } from 'utils/request';
import { API, BaseURL, MatchLogic, MatchString, MatchType, MatchValueType, RuleRelease } from './types';

export interface Lists {
    namespaceList: []
    serviceList: []
}

export enum RateLimitResource {
    QPS = 'QPS',
    Concurrency = 'CONCURRENCY',
    Token = 'TOKEN',
}

// RateLimitRule的资源类型
export const RateLimitResourceOptions = [
    {
        value: RateLimitResource.QPS,
        label: '请求数',
    },
    {
        value: RateLimitResource.Concurrency,
        label: '并发数',
    },
    {
        value: RateLimitResource.Token,
        label: 'Token 数',
    },
]

export const RateLimitResourceMap = RateLimitResourceOptions.reduce((acc, curr) => {
    acc[curr.value] = curr.label
    return acc
}, {} as Record<string, string>)

export interface QuotaLeaseCounterView {
    counterKey: number
    labels?: string
    durationMs: number
    accounting: 'CONSUMABLE' | 'OCCUPANCY' | string
    limit: number
    reserved: number
    consumed: number
    returned: number
    available: number
}

export interface QuotaLeaseView {
    id: string
    clientKey: number
    status: 'ACTIVE' | 'SETTLED' | 'EXPIRED' | string
    sequence: number
    expiresAt: number
    terminalAt?: number
    namespace: string
    service: string
    counters: QuotaLeaseCounterView[]
}

export interface QuotaLeaseSnapshot {
    generatedAt: number
    summary: {
        activeCount: number
        settledCount: number
        expiredCount: number
        activeReserved: number
        activeConsumed: number
        recentlyReturned: number
    }
    leases: QuotaLeaseView[]
}

export function describeQuotaLeases(params: { namespace?: string; service?: string }) {
    return getApiRequest<QuotaLeaseSnapshot>({ action: '/limiter/v1/quota-leases', data: params })
}

// 限流类型，支持LOCAL（单机限流）, GLOBAL（分布式限流）
export enum LimitType {
    GLOBAL = 'GLOBAL',
    LOCAL = 'LOCAL',
}

export const LimitTypeOptions = [
    {
        value: LimitType.LOCAL,
        label: '单机限流',
    },
    {
        value: LimitType.GLOBAL,
        label: '分布式限流',
    },
]

export const LimitTypeMap = LimitTypeOptions.reduce((acc, curr) => {
    acc[curr.value] = curr.label
    return acc
}, {} as Record<string, string>)

// 限流效果，支持REJECT（直接拒绝）,UNIRATE（匀速排队），默认REJECT
export enum LimitAction {
    REJECT = 'REJECT',
    UNIRATE = 'UNIRATE',
}
export const LimitActionOptions = [
    {
        value: LimitAction.REJECT,
        label: '快速失败',
    },
    {
        value: LimitAction.UNIRATE,
        label: '匀速排队',
    },
]

export const LimitActionMap = LimitActionOptions.reduce((acc, curr) => {
    acc[curr.value] = curr.label
    return acc
}, {} as Record<string, string>)

// 接口类型
export enum LimitMethodType {
    EXACT = 'EXACT',
    REGEX = 'REGEX',
    NOT_EQUALS = 'NOT_EQUALS',
    IN = 'IN',
    NOT_IN = 'NOT_IN',
}

export const LimitMethodTypeOptions = [
    {
        value: LimitMethodType.EXACT,
        label: '全匹配',
    },
    {
        value: LimitMethodType.REGEX,
        label: '正则表达式',
    },
    {
        value: LimitMethodType.NOT_EQUALS,
        label: '不等于',
    },
    {
        value: LimitMethodType.IN,
        label: '包含',
    },
    {
        value: LimitMethodType.NOT_IN,
        label: '不包含',
    },
]

export const LimitMethodTypeMap = LimitMethodTypeOptions.reduce((acc, curr) => {
    acc[curr.value] = curr.label
    return acc
}, {} as Record<string, string>)

// 匹配规则类型
export enum LimitArgumentsType {
    CUSTOM = 'CUSTOM',
    METHOD = 'METHOD',
    HEADER = 'HEADER',
    QUERY = 'QUERY',
    CALLER_SERVICE = 'CALLER_SERVICE',
    CALLER_IP = 'CALLER_IP',
}

export const LimitArgumentsTypeOptions = [
    {
        value: LimitArgumentsType.CUSTOM,
        label: '自定义',
    },
    {
        value: LimitArgumentsType.HEADER,
        label: '请求头(HEADER)',
    },
    {
        value: LimitArgumentsType.QUERY,
        label: '请求参数(QUERY)',
    },
    {
        value: LimitArgumentsType.METHOD,
        label: '方法(METHOD)',
    },
    {
        value: LimitArgumentsType.CALLER_SERVICE,
        label: '主调服务',
    },
    {
        value: LimitArgumentsType.CALLER_IP,
        label: '主调IP',
    },
]

export const LimitArgumentsTypeMap = LimitArgumentsTypeOptions.reduce((acc, curr) => {
    acc[curr.value] = curr.label
    return acc
}, {} as Record<string, string>)

// 失败处理策略
export enum LimitFailover {
    FAILOVER_PASS = 'FAILOVER_PASS',
    FAILOVER_LOCAL = 'FAILOVER_LOCAL',
}
export const LimitFailoverOptions = [
    {
        value: LimitFailover.FAILOVER_LOCAL,
        label: '单机限流',
    },
    {
        value: LimitFailover.FAILOVER_PASS,
        label: '直接通过',
    },
]

export const LimitFailoverMap = LimitFailoverOptions.reduce((acc, curr) => {
    acc[curr.value] = curr.label
    return acc
}, {} as Record<string, string>)

// amounts限流阈值，统计窗口时长的单位
export enum LimitAmountsValidationUnit {
    s = 's',
    m = 'm',
    h = 'h',
}

const converToUnit = (s: string): LimitAmountsValidationUnit => {
    switch (s) {
        case 's':
            return LimitAmountsValidationUnit.s;
        case 'm':
            return LimitAmountsValidationUnit.m;
        case 'h':
            return LimitAmountsValidationUnit.h;
        default:
            return LimitAmountsValidationUnit.s;
    }
}

export const LimitAmountsValidationUnitOptions = [
    {
        value: LimitAmountsValidationUnit.s,
        label: '秒',
    },
    {
        value: LimitAmountsValidationUnit.m,
        label: '分钟',
    },
    {
        value: LimitAmountsValidationUnit.h,
        label: '小时',
    },
]

export interface LimitConfig {
    validDuration: string
    maxAmount: number
}

export interface LimitConfigView {
    validDurationUnit: LimitAmountsValidationUnit
    validDuration: number
    maxAmount: number
}

export interface LimitConfigForFormFilling {
    // table row的key值，由于没有唯一标识字段，所以用生成随机数的方法来做生成id做key，在新增列时生成id，避免重复渲染
    id: string
    maxAmount: number
    validDurationNum: number
    validDurationUnit: LimitAmountsValidationUnit
}

export interface LimitArgumentsConfig {
    id?: string // 规则id
    type: LimitArgumentsType
    key: string
    // 由于后台历史原因，这部分的结构不能修改了，这边的value取为复杂类型，value为填充的具体值，type为操作类型
    value: MatchString
}

export interface LimitArgumentsConfigForFormFilling {
    // table row的key值，由于没有唯一标识字段，所以用生成随机数的方法来做生成id做key，在新增列时生成id，避免重复渲染
    id: string
    type: LimitArgumentsType
    key: string
    value: string
    operator: LimitMethodType
}

export interface ConcurrencyAmount {
    maxAmount: number // 最大并发数
}

export interface CustomResponse {
    code?: string
    headers?: Record<string, string>
    body: string
}

// 限流上报方式（按固定周期或达到配额百分比后上报）
export interface Report {
    interval?: string  // 固定周期
    percent?: number   // 配额百分比
}

// 子规则：单条限流触发条件（匹配条件 + 限流阈值 + 行为）
export interface LimitTrigger {
    apis?: API[]
    method?: MatchString
    matchMode?: MatchLogic
    arguments?: LimitArgumentsConfig[]
    amounts: LimitConfig[]
    action: LimitAction
    resource: RateLimitResource
    concurrencyAmount?: ConcurrencyAmount
    regex_combine?: boolean
    failover?: LimitFailover
    max_queue_delay?: number
    customResponse?: CustomResponse
}

// 子规则视图（amounts 等为表单友好格式）
export interface LimitTriggerView {
    apis: API[]
    method?: MatchString
    matchMode: MatchLogic
    arguments: LimitArgumentsConfig[]
    amounts: LimitConfigView[]
    action: LimitAction
    resource: RateLimitResource
    concurrencyAmount?: ConcurrencyAmount
    regex_combine: boolean
    failover: LimitFailover
    max_queue_delay: number
    customResponse?: CustomResponse
}

// 大规则：同一服务下限流规则集合（可包含多条子规则）
export interface RateLimit {
    id?: string
    name: string
    namespace: string
    callee: RateLimitServiceScope
    priority?: number
    type: LimitType
    rules: LimitTrigger[]
    revision?: string
    disable?: boolean
    report?: Report
    cluster?: RateLimitCluster
    ctime?: string
    mtime?: string
    metadata?: Record<string, string>
    editable?: boolean
    deleteable?: boolean
}

// 大规则视图（rules 为 LimitTriggerView[]，用于列表/编辑）
export interface RateLimitView {
    id?: string
    name: string
    namespace: string
    callee: RateLimitServiceScope
    priority?: number
    type: LimitType
    rules: LimitTriggerView[]
    revision?: string
    disable?: boolean
    report?: Report
    cluster?: RateLimitCluster
    ctime?: string
    mtime?: string
    metadata?: Record<string, string>
    editable?: boolean
    deleteable?: boolean
}

export interface RateLimitServiceScope {
    namespace: string
    service: string
}

export interface RateLimitRule {
    id?: string // 规则id
    name: string //规则名
    service: string // 规则所属服务名
    namespace: string // 规则所属命名空间
    type: LimitType // 限流类型
    arguments: LimitArgumentsConfig[]
    // 限流阈值
    // 可以有多个粒度的配置（比如同时针对秒级，分钟级，天级），匹配一个则进行限流
    // 全局限流模式下，该值为服务配额总量；单机限流模式下，该值为单个节点能处理的配额量
    amounts: LimitConfig[]
    action: LimitAction // 限流器的行为
    method: MatchString //接口定义
    // 通配符是否合并计算，默认分开计数
    regex_combine: boolean
    failover: LimitFailover
    max_queue_delay: number
    resource: RateLimitResource // 限流资源
    // 如果 resource == 'Concurrency', 则需要并发限流配置
    concurrencyAmount?: ConcurrencyAmount // 并发限流配置
    metadata?: Record<string, string> // 限流规则元数据
    customResponse?: CustomResponse // 自定义响应内容
    revision?: string
}

export interface RateLimitRuleView {
    id?: string // 规则id
    name: string //规则名
    service: string // 规则所属服务名
    namespace: string // 规则所属命名空间
    type: LimitType // 限流类型
    arguments: LimitArgumentsConfig[]
    // 限流阈值
    // 可以有多个粒度的配置（比如同时针对秒级，分钟级，天级），匹配一个则进行限流
    // 全局限流模式下，该值为服务配额总量；单机限流模式下，该值为单个节点能处理的配额量
    amounts: LimitConfigView[]
    action: LimitAction // 限流器的行为
    method: MatchString //接口定义
    // 通配符是否合并计算，默认分开计数
    regex_combine: boolean
    failover: LimitFailover
    max_queue_delay: number
    resource: RateLimitResource // 限流资源
    // 如果 resource == 'Concurrency', 则需要并发限流配置
    concurrencyAmount?: ConcurrencyAmount // 并发限流配置
    metadata?: Record<string, string> // 限流规则元数据
    customResponse?: CustomResponse // 自定义响应内容
    revision?: string
    editable?: boolean
    deleteable?: boolean
    ctime?: string // 创建时间
    mtime?: string // 修改时间
}

// 将 LimitConfig[] 转为 LimitConfigView[]
function normalizeMatchType(type?: string | number): MatchType {
    const map: Record<number, MatchType> = {
        0: MatchType.EXACT,
        1: MatchType.REGEX,
        2: MatchType.NOT_EQUALS,
        3: MatchType.IN,
        4: MatchType.NOT_IN,
        5: MatchType.RANGE,
    };
    if (typeof type === 'number') return map[type] || MatchType.EXACT;
    return (type as MatchType) || MatchType.EXACT;
}

function normalizeMatchValueType(type?: string | number): MatchValueType {
    const map: Record<number, MatchValueType> = {
        0: MatchValueType.TEXT,
        1: MatchValueType.PARAMETER,
    };
    if (typeof type === 'number') return map[type] || MatchValueType.TEXT;
    return type === MatchValueType.PARAMETER ? MatchValueType.PARAMETER : MatchValueType.TEXT;
}

function normalizeLimitType(type?: LimitType | number): LimitType {
    if (type === 0) return LimitType.GLOBAL;
    if (type === 1) return LimitType.LOCAL;
    return (type as LimitType) || LimitType.LOCAL;
}

function normalizeResource(resource?: RateLimitResource | number | string): RateLimitResource {
    if (resource === 0) return RateLimitResource.QPS;
    if (resource === 1) return RateLimitResource.Concurrency;
    if (resource === 3) return RateLimitResource.Token;
    if (resource === 'Concurrency') return RateLimitResource.Concurrency;
    if (resource === 'Token') return RateLimitResource.Token;
    return (resource as RateLimitResource) || RateLimitResource.QPS;
}

function normalizeArgumentType(type?: LimitArgumentsType | number | string): LimitArgumentsType {
    const map: Record<number, LimitArgumentsType> = {
        0: LimitArgumentsType.CUSTOM,
        1: LimitArgumentsType.METHOD,
        2: LimitArgumentsType.HEADER,
        3: LimitArgumentsType.QUERY,
        4: LimitArgumentsType.CALLER_SERVICE,
        5: LimitArgumentsType.CALLER_IP,
    };
    if (typeof type === 'number') return map[type] || LimitArgumentsType.CUSTOM;
    return (type as LimitArgumentsType) || LimitArgumentsType.CUSTOM;
}

function normalizeMatchMode(mode?: MatchLogic | number | string): MatchLogic {
    if (mode === 1 || mode === MatchLogic.OR || mode === 'OR') return MatchLogic.OR;
    return MatchLogic.AND;
}

function durationToView(duration?: string | { seconds?: number | string; nanos?: number }): Pick<LimitConfigView, 'validDuration' | 'validDurationUnit'> {
    if (duration && typeof duration === 'object') {
        const seconds = Number(duration.seconds || 0);
        if (seconds >= 3600 && seconds % 3600 === 0) {
            return { validDuration: seconds / 3600, validDurationUnit: LimitAmountsValidationUnit.h };
        }
        if (seconds >= 60 && seconds % 60 === 0) {
            return { validDuration: seconds / 60, validDurationUnit: LimitAmountsValidationUnit.m };
        }
        return { validDuration: seconds || 1, validDurationUnit: LimitAmountsValidationUnit.s };
    }
    const [validDuration, validDurationUnit] = (duration || '1s').split(/(\d+)/).filter(Boolean);
    return {
        validDuration: Number.parseInt(validDuration) || 1,
        validDurationUnit: converToUnit(validDurationUnit),
    };
}

function amountsToView(amounts: LimitConfig[]): LimitConfigView[] {
    if (!amounts?.length) return [{ validDuration: 1, validDurationUnit: LimitAmountsValidationUnit.s, maxAmount: 1 }];
    return amounts.map(amount => {
        const duration = durationToView((amount as any).validDuration);
        return {
            validDuration: duration.validDuration,
            validDurationUnit: duration.validDurationUnit,
            maxAmount: amount.maxAmount || 0,
        };
    });
}

function defaultRateLimitAPI(): API {
    return {
        protocol: 'HTTP',
        method: '*',
        path: { value: '', type: MatchType.EXACT, value_type: MatchValueType.TEXT },
    };
}

function apiFromMethod(method?: MatchString): API {
    return {
        ...defaultRateLimitAPI(),
        path: {
            type: normalizeMatchType(method?.type),
            value: method?.value || '',
            value_type: normalizeMatchValueType(method?.value_type),
        },
    };
}

function normalizeRateLimitApis(source: { apis?: API[]; method?: MatchString }): API[] {
    if (source.apis?.length) {
        return source.apis.map(api => ({
            protocol: api.protocol || 'HTTP',
            method: api.method || '*',
            path: {
                type: normalizeMatchType(api.path?.type),
                value: api.path?.value || '',
                value_type: normalizeMatchValueType(api.path?.value_type),
            },
        }));
    }
    return [apiFromMethod(source.method)];
}

// 将单条旧规则 (RateLimitRule) 转为 LimitTriggerView
function ruleToTriggerView(item: RateLimitRule): LimitTriggerView {
    return {
        apis: normalizeRateLimitApis(item as RateLimitRule & { apis?: API[] }),
        matchMode: normalizeMatchMode((item as any).matchMode ?? (item as any).match_mode),
        arguments: item.arguments ? item.arguments.map(arg => ({
            type: normalizeArgumentType(arg.type),
            key: arg.key,
            value: {
                type: normalizeMatchType(arg.value?.type),
                value: arg.value?.value || '',
                value_type: normalizeMatchValueType(arg.value?.value_type),
            },
        })) : [],
        amounts: amountsToView(item.amounts || []),
        action: item.action || LimitAction.REJECT,
        resource: normalizeResource(item.resource),
        concurrencyAmount: item.concurrencyAmount,
        regex_combine: item.regex_combine ?? false,
        failover: item.failover || LimitFailover.FAILOVER_LOCAL,
        max_queue_delay: item.max_queue_delay ?? 1,
        customResponse: item.customResponse ?? (item as any).custom_response,
    };
}

// 将 LimitTrigger 转为 LimitTriggerView
function triggerToView(t: LimitTrigger): LimitTriggerView {
    return {
        apis: normalizeRateLimitApis(t),
        matchMode: normalizeMatchMode(t.matchMode ?? (t as any).match_mode),
        arguments: t.arguments ? t.arguments.map(arg => ({
            type: normalizeArgumentType(arg.type),
            key: arg.key,
            value: {
                type: normalizeMatchType(arg.value?.type),
                value: arg.value?.value || '',
                value_type: normalizeMatchValueType(arg.value?.value_type),
            },
        })) : [],
        amounts: amountsToView(t.amounts || []),
        action: t.action || LimitAction.REJECT,
        resource: normalizeResource(t.resource),
        concurrencyAmount: t.concurrencyAmount,
        regex_combine: t.regex_combine ?? false,
        failover: t.failover || LimitFailover.FAILOVER_LOCAL,
        max_queue_delay: t.max_queue_delay ?? (t as any).maxQueueDelay ?? 1,
        customResponse: t.customResponse ?? (t as any).custom_response,
    };
}

// 后端可能返回新结构 RateLimit（含 rules）或旧结构 RateLimitRule（单条规则）
function itemToRateLimitView(item: RateLimit | RateLimitRule): RateLimitView {
    const hasRules = 'rules' in item && Array.isArray((item as RateLimit).rules);
    if (hasRules) {
        const r = item as RateLimit;
        return {
            id: r.id,
            name: r.name,
            namespace: r.namespace,
            callee: r.callee || { namespace: '', service: '' },
            priority: r.priority,
            type: normalizeLimitType(r.type),
            rules: (r.rules || []).map(triggerToView),
            revision: r.revision,
            disable: r.disable,
            report: r.report,
            cluster: r.cluster,
            ctime: r.ctime,
            mtime: r.mtime,
            metadata: r.metadata,
            editable: r.editable,
            deleteable: r.deleteable,
        };
    }
    const leg = item as RateLimitRule;
    return {
        id: leg.id,
        name: leg.name,
        namespace: leg.namespace,
        callee: { namespace: leg.namespace, service: leg.service },
        type: normalizeLimitType(leg.type),
        rules: [ruleToTriggerView(leg)],
        revision: leg.revision,
        ctime: (leg as RateLimitRuleView).ctime,
        mtime: (leg as RateLimitRuleView).mtime,
        metadata: leg.metadata,
        editable: (leg as RateLimitRuleView).editable,
        deleteable: (leg as RateLimitRuleView).deleteable,
    };
}

// DescribeLimitRulesRequest
export interface DescribeLimitRulesRequest {
    id?: string
    namespace?: string
    service?: string
    method?: string
    //brief为true时，则不返回规则详情，只返回规则列表概要信息，默认为false
    brief?: boolean
    name?: string
    limit_type?: LimitType // 限流类型
    offset: number
    limit: number
}

export interface DescribeLimitRulesResponse {
    data?: (RateLimit | RateLimitRule)[]
    rateLimits?: (RateLimit | RateLimitRule)[]
    amount?: number
    size?: number
}

export async function describeLimitRules(params: DescribeLimitRulesRequest) {
    const res = await getApiRequest<DescribeLimitRulesResponse>({
        action: `${BaseURL.RATELIMIT_RULE}`,
        data: params,
    })
    const rateLimits = res.data ?? res.rateLimits ?? []
    return {
        list: rateLimits.map(itemToRateLimitView),
        totalCount: res.amount ?? rateLimits.length,
    }
}

/** 解包后即为规则详情对象 */
export interface DescribeOneLimitRuleResponse {
    data: RateLimit | RateLimitRule
}

export async function describeOneLimitRules(id: string): Promise<RateLimitView | null> {
    const res = await getApiRequest<RateLimit | RateLimitRule>({
        action: `${BaseURL.RATELIMIT_RULE}/detail`,
        data: { id },
    })

    if (!res || typeof res !== 'object') {
        return null;
    }
    return itemToRateLimitView(res as RateLimit | RateLimitRule);
}

export interface CreateLimitRuleRequest {
    id?: string
    name: string // 规则名
    type: LimitType // 限流类型
    namespace: string // 规则所属命名空间
    service: string // 规则所属服务名
    method: MatchString // 接口定义
    regex_combine: boolean //是否合并计算阈值
    action: LimitAction // 限流器效果【快速失败 ｜ 匀速排队】
    max_queue_delay?: number // 匀速排队的最大排队时长
    failover?: LimitFailover // 失败处理策略
    resource: RateLimitResource // 限流资源类型
    arguments?: LimitArgumentsConfig[] // 请求匹配规则
    amounts: LimitConfig[] // 限流条件
    concurrencyAmount?: ConcurrencyAmount // 并发限流配置
    customResponse?: CustomResponse
    metadata?: Record<string, string> // 限流规则元数据
}

// 新结构：大规则 + 子规则列表 的创建/修改请求
export interface CreateRateLimitRequest {
    id?: string
    name: string
    namespace: string
    callee: RateLimitServiceScope
    priority?: number
    type: LimitType
    disable?: boolean
    report?: Report
    cluster?: RateLimitCluster
    metadata?: Record<string, string>
    rules: LimitTrigger[]
}

export function triggerViewToTrigger(v: LimitTriggerView): LimitTrigger {
    return {
        apis: v.apis,
        matchMode: v.matchMode || MatchLogic.AND,
        arguments: v.arguments,
        amounts: v.amounts.map(a => ({
            validDuration: a.validDuration.toString() + a.validDurationUnit,
            maxAmount: a.maxAmount,
        })),
        action: v.action,
        resource: v.resource,
        concurrencyAmount: v.concurrencyAmount,
        regex_combine: v.regex_combine,
        failover: v.failover,
        max_queue_delay: v.max_queue_delay,
        customResponse: v.customResponse,
    };
}

// 默认一条子规则（用于新建时）
export function defaultLimitTriggerView(): LimitTriggerView {
    return {
        apis: [defaultRateLimitAPI()],
        matchMode: MatchLogic.AND,
        arguments: [],
        amounts: [{ validDuration: 1, validDurationUnit: LimitAmountsValidationUnit.s, maxAmount: 1 }],
        action: LimitAction.REJECT,
        resource: RateLimitResource.QPS,
        regex_combine: false,
        failover: LimitFailover.FAILOVER_LOCAL,
        max_queue_delay: 1,
    };
}

/** 将编辑态大规则转为创建/修改请求体 */
export function rateLimitViewToRequest(v: RateLimitView): CreateRateLimitRequest {
    return {
        id: v.id,
        name: v.name,
        namespace: v.namespace,
        callee: v.callee,
        priority: v.priority,
        type: v.type,
        disable: v.disable,
        report: v.report,
        cluster: v.cluster,
        metadata: v.metadata,
        rules: (v.rules || []).map(triggerViewToTrigger),
    };
}

// createRateLimit（支持新结构：大规则 + rules[]）
export async function createRateLimits(params: CreateRateLimitRequest[]) {
    const res = await apiRequest<any>({
        action: `${BaseURL.RATELIMIT_RULE}`,
        data: params,
    })
    return res
}

// 兼容旧单条规则创建（内部转为新结构一条 rules）
export async function createRateLimitsLegacy(params: CreateLimitRuleRequest[]) {
    const newParams: CreateRateLimitRequest[] = params.map(p => ({
        id: p.id,
        name: p.name,
        service: p.service,
        namespace: p.namespace,
        type: p.type,
        rules: [{
            apis: [apiFromMethod(p.method)],
            matchMode: MatchLogic.AND,
            arguments: p.arguments,
            amounts: p.amounts,
            action: p.action,
            resource: p.resource,
            concurrencyAmount: p.concurrencyAmount,
            regex_combine: p.regex_combine,
            failover: p.failover,
            max_queue_delay: p.max_queue_delay ?? 1,
            customResponse: p.customResponse,
        }],
    }));
    return createRateLimits(newParams);
}

export type ModifyLimitRuleRequest = CreateLimitRuleRequest
export type ModifyRateLimitRequest = CreateRateLimitRequest

// modifyRateLimit（支持新结构）
export async function modifyRateLimits(params: ModifyRateLimitRequest[]) {
    const res = await putApiRequest<any>({
        action: `${BaseURL.RATELIMIT_RULE}`,
        data: params,
    })
    return res
}

// DeleteRateLimitParams .
export interface DeleteRateLimitRequest {
    id: string
}

export async function deleteRateLimit(params: DeleteRateLimitRequest[]) {
    const res = await apiRequest<any>({
        action: `${BaseURL.RATELIMIT_RULE}/delete`,
        data: params,
    })
    return res
}

// publishRateLimit 发布一个限流规则
export async function publishRateLimit(params: RuleRelease[]) {
    const res = await apiRequest<any>({
        action: `${BaseURL.RATELIMIT_RULE}/releases`,
        data: params,
    })
    return res
}

export interface DescribeRateLimitVersionsRequest {
    rule_name: string
    offset: number
    limit: number
}

export interface DescribeRateLimitVersionsResponse {
    total: number
    data: RuleRelease[]
}

export async function describeRateLimitVersions(req: DescribeRateLimitVersionsRequest) {
    const result = await getApiRequest<DescribeRateLimitVersionsResponse>({
        action: `${BaseURL.RATELIMIT_RULE}/releases`,
        data: req,
    });
    return {
        list: result.data,
        totalCount: result.total,
    };
}

export interface RollbackRateLimitReleaseRequest {
    id: string
}

// rollbackRateLimit 回滚某个规则版本
export async function rollbackRateLimit(params: RollbackRateLimitReleaseRequest) {
    const res = await putApiRequest<any>({
        action: `${BaseURL.RATELIMIT_RULE}/releases/rollback`,
        data: [params],
    });
    return res;
}

export interface DeleteRateLimitReleaseRequest {
    id: string
}

// deleteRateLimitRelease 删除某个规则版本
export async function deleteRateLimitRelease(params: DeleteRateLimitReleaseRequest) {
    const res = await apiRequest<any>({
        action: `${BaseURL.RATELIMIT_RULE}/releases/delete`,
        data: [params],
    });
    return res;
}

// 查询分布式限流集群列表
export interface RateLimitCluster {
    id: string
    name: string
    namespace: string
    service: string
    metadata?: Record<string, string> // 限流集群元数据
    revision?: string
    ctime?: string // 创建时间
    mtime?: string // 修改时间
}

export interface DescribeLimitClustersResponse {
    data: RateLimitCluster[]
    amount: number
    size: number
}

export async function describeLimitClusters() {
    const res = await getApiRequest<DescribeLimitClustersResponse>({
        action: '/naming/v1/ratelimits/clusters',
    })
    return {
        list: res.data,
        totalCount: res.data.length,
    }
}

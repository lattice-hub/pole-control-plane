import request, { apiRequest, ApiResponse, getAllList, getApiRequest, putApiRequest } from 'utils/request';
import { SuccessCode } from './const';

export interface AuthMutationResponse {
    code?: number
    result?: boolean
    responses?: ApiResponse[]
}

/**
 * 写接口经过 request 层后可能拿到已解包的 data，也可能拿到无 data 的旧式顶层响应。
 * 因此在一个位置兼容 result、批量 responses 与旧式 code，业务 service 不再各自猜测响应形态。
 */
export function isAuthMutationSuccessful(response: unknown): boolean {
    if (response === true) {
        return true
    }
    if (!response || typeof response !== 'object') {
        return false
    }

    const mutation = response as AuthMutationResponse
    if (typeof mutation.result === 'boolean') {
        return mutation.result
    }
    if (Array.isArray(mutation.responses)) {
        return mutation.responses.length > 0
            && mutation.responses.every(item => Number(item.code) === SuccessCode)
    }
    // 单资源写接口（例如用户/用户组 Token）成功时会把更新后的资源放在 data，
    // request 层解包后只剩资源对象。HTTP/业务错误已由请求层抛出，有资源 ID 即成功。
    if ('id' in mutation && typeof (mutation as { id?: unknown }).id === 'string') {
        return Boolean((mutation as { id: string }).id)
    }
    return Number(mutation.code) === SuccessCode
}

export enum PolicySourceType {
    Namespaces = "Namespaces",
    Services = "Services",
    RouteRules = "RouteRules",
    RateLimitRules = "RateLimitRules",
    CircuitBreakerRules = "CircuitBreakerRules",
    FaultDetectRules = "FaultDetectRules",
    LaneRules = "LaneRules",
    LossLessRules = "LosslessRules",
    SecurityRules = "SecurityRules",
    MirrorRules = "MirrorRules",
    MockRules = "MockRules",
    Users = "Users",
    UserGroups = "UserGroups",
    Roles = "Roles",
    PolicyRules = "PolicyRules",
    MCPServerResources = "MCPServerResources",
    A2AAgentResources = "A2AAgentResources",
}

// 鉴权策略
export interface PolicyRule {
    // 策略唯一ID
    id?: string
    // 策略名称
    name?: string
    // 涉及的用户 or 用户组
    principals?: Principals
    // 资源操作权限
    action?: string
    // 简单描述
    comment?: string
    // 创建时间
    ctime?: string
    // 修改时间
    mtime?: string
    // 策略关联的资源
    resources?: PolicyResources
    // 是否为默认策略
    default_strategy?: boolean
    // 鉴权规则来源
    source?: string
    // 服务端接口
    functions?: string[]
    // 策略生效的资源标签
    resource_labels?: PolicyResourceLabel[]
    // 策略资源标签
    metadata?: Record<string, string>
}

// 鉴权策略涉及的用户 or 用户组信息
export interface Principals {
    // 用户ID列表
    users?: Principal[]
    // 用户组ID列表
    groups?: Principal[]
    // 角色ID列表
    roles?: Principal[]
}

// 鉴权策略资源信息
export interface PolicyResources {
    // 命名空间ID列表
    namespaces?: PolicyResource[]
    // 服务ID列表
    services?: PolicyResource[]
    // 路由规则ID列表
    route_rules?: PolicyResource[]
    // 泳道规则ID列表
    lane_rules?: PolicyResource[]
    // 无损规则ID列表
    lossless_rules?: PolicyResource[]
    // 熔断规则ID列表
    circuitbreaker_rules?: PolicyResource[]
    // 主动探测规则ID列表
    faultdetect_rules?: PolicyResource[]
    // 限流规则ID列表
    ratelimit_rules?: PolicyResource[]
    // 调用鉴权规则ID列表
    security_rules?: PolicyResource[]
    // 流量镜像规则ID列表
    mirror_rules?: PolicyResource[]
    // 流量 Mock 规则ID列表
    mock_rules?: PolicyResource[]
    // 用户ID列表
    users?: PolicyResource[]
    // 用户组ID列表
    user_groups?: PolicyResource[]
    // 资源鉴权规则ID列表
    auth_policies?: PolicyResource[]
    // 角色ID列表
    roles?: PolicyResource[]
    // MCP Server ID列表
    mcp_servers?: PolicyResource[]
    // A2A Agent ID列表
    a2a_agents?: PolicyResource[]
}

// 资源
export interface PolicyResource {
    // 资源Id，如果是全部的话，那么ID就是 *
    id?: string
    // 命名空间
    namespace?: string
    // 服务名｜配置分组名
    name?: string
}

export interface PolicyResourceLabel {
    // 标签键
    key?: string
    // 标签值
    value?: string
    // 比较类型
    compare_type?: string
}

export interface Principal {
    // 用户ID｜用户组ID
    id: string
    // 用户名｜用户组名
    name?: string
}

// 删除鉴权策略
export interface DeleteAuthPoliciesRequest {
    // 鉴权策略ID列表
    id: string
}
export interface DeleteAuthPoliciesResponse {
    // 执行结果
    result: boolean
}

export async function deleteAuthPolicies(params: DeleteAuthPoliciesRequest[]) {
    const result = await apiRequest<DeleteAuthPoliciesResponse & AuthMutationResponse>({
        action: '/auth/v1/policies/delete',
        data: params,
    })
    return isAuthMutationSuccessful(result)
}

// 查询治理中心鉴权策略列表
export interface DescribeAuthPoliciesRequest {
    // 分页查询偏移量
    offset?: number
    // 查询条数
    limit?: number
    // 策略名称，如果需要模糊搜索的话，最后加上一个 *
    name?: string
    // 用户 ID｜用户组 ID
    principal_id?: string
    // 1 为用户，2 为用户组
    principal_type?: number
    // 是否查询默认策略 1 为不查询
    default?: string
    res_id?: string
    res_type?: string
}

export interface DescribeAuthPoliciesResponse {
    // 总数
    amount: number
    size?: number
    // 标准响应策略列表
    data?: PolicyRule[]
    // 策略列表
    authStrategies?: PolicyRule[]
}

export async function describeAuthPolicies(params: DescribeAuthPoliciesRequest) {
    const result = await getApiRequest<DescribeAuthPoliciesResponse>({
        action: '/auth/v1/policies',
        data: params,
    })
    const authStrategies = result.data ?? result.authStrategies ?? []
    return {
        totalCount: result.amount ?? authStrategies.length,
        content: authStrategies,
    }
}

export async function describeAllAuthPolicies() {
    const result = await getAllList(describeAuthPolicies, {
        listKey: 'content',
        totalKey: 'totalCount',
    })({ default: 'false' })
    // 不会查询默认策略
    return {
        totalCount: result.totalCount,
        content: result.list ? result.list : [],
    }
}

// 查询鉴权策略详细
export interface DescribeAuthPolicyDetailRequest {
    // 鉴权策略ID
    id: string
}

export interface DescribeAuthPolicyDetailResponse {
    // 鉴权策略详细
    authStrategy?: PolicyRule
}

export async function describeAuthPolicyDetail(params: DescribeAuthPolicyDetailRequest) {
    const result = await getApiRequest<DescribeAuthPolicyDetailResponse | PolicyRule>({
        action: '/auth/v1/policies/detail',
        data: params,
    })
    const strategy = (result as DescribeAuthPolicyDetailResponse).authStrategy ?? (result as PolicyRule)
    return { strategy }
}

// 批量创建鉴权策略
export interface CreateAuthPoliciesRequest {
    id?: string
    // 策略名称
    name: string
    // 涉及的用户 or 用户组
    principals?: Principals
    // 资源操作权限
    action: string
    // 简单描述
    comment?: string
    // 策略关联的资源
    resources?: PolicyResources
    // 鉴权规则来源
    source?: string
    // 服务端接口
    functions?: string[]
    // 策略生效的资源标签
    resource_labels?: string[]
    // 策略资源标签
    metadata?: Record<string, string>
}

export interface CreateAuthPoliciesResponse {
    // 执行结果
    result: boolean
}

export async function createAuthPolicies(params: CreateAuthPoliciesRequest[]) {
    const result = await apiRequest<CreateAuthPoliciesResponse & AuthMutationResponse>({ action: '/auth/v1/policies', data: params })
    return isAuthMutationSuccessful(result)
}

// 批量修改鉴权策略
export interface ModifyAuthPoliciesRequest {
    id: string
    // 策略名称
    name: string
    // 涉及的用户 or 用户组
    principals?: Principals
    // 资源操作权限
    action?: string
    // 简单描述
    comment?: string
    // 策略关联的资源
    resources?: PolicyResources
    // 鉴权规则来源
    source?: string
    // 服务端接口
    functions?: string[]
    // 策略生效的资源标签
    resource_labels?: string[]
    // 策略资源标签
    metadata?: Record<string, string>
}

export interface ModifyAuthPoliciesResponse {
    // 执行结果
    responses: ApiResponse[]
}

export async function modifyAuthPolicies(params: ModifyAuthPoliciesRequest[]) {
    const result = await putApiRequest<ModifyAuthPoliciesResponse>({
        action: '/auth/v1/policies',
        data: params,
    })
    return isAuthMutationSuccessful(result)
}

// 按照 principal 的维度查询能改操作的资源信息
export interface DescribePrincipalResourcesRequest {
    principal_id?: string
    // 用户名称，模糊搜索最后加上 * 字符
    principal_type?: string
}

export interface DescribePrincipalResourcesReqsponse {
    resources: {
        // 命名空间ID列表
        namespaces?: PolicyResource[]
        // 服务ID列表
        services?: PolicyResource[]
    }
}

export async function describePrincipalResources(params: DescribePrincipalResourcesRequest) {
    const result = await getApiRequest<DescribePrincipalResourcesReqsponse>({
        action: '/auth/v1/principal/resources',
        data: params,
    })
    return result
}


// 按照 resource 的维度查询能改操作的资源信息
export interface DescribeResourcePrincipalsRequest {
    res_id: string
    res_type: string
    action?: string
}

export interface DescribeResourcePrincipalsResponse {
    data: Principals
}

export async function describeResourcePrincipals(params: DescribeResourcePrincipalsRequest) {
    const result = await getApiRequest<DescribeResourcePrincipalsResponse>({
        action: '/auth/v1/resources/principals',
        data: params,
    })
    return result
}

// authorizeResources 资源授权
export interface AuthorizeResourcesRequest {
    // 资源ID
    resource_id: string
    // 资源类型
    resource_type: string
    // 涉及的用户 or 用户组
    principals: Principals
}

export interface AuthorizeResourcesResponse {
    // 执行结果
    result: boolean
}

export async function authorizeResources(params: AuthorizeResourcesRequest[]) {
    const result = await apiRequest<AuthorizeResourcesResponse>({
        action: '/auth/v1/resources/authorize',
        data: params,
    })
    return result
}

// 检查策略是否已开启
export type CheckAuthParams = {}

// 检查策略是否已开启
export interface CheckAuthResult {
    // 执行结果
    optionSwitch: {
        options: { auth: string, clientOpen: string, consoleOpen: string }
    }
}

export async function describeAuthStatus(params: CheckAuthParams) {
    const result = await getApiRequest<CheckAuthResult>({ action: '/auth/v1/status', data: params })
    return result.optionSwitch.options
}

export async function describeServerFunctions() {
    const result = await getApiRequest<ServerFunctionGroup[]>({
        action: '/admin/v1/server/functions',
    })
    for (let i = 0; i < result.length; ++i) {
        result[i].id = result[i].name
    }
    return {
        list: result,
        totalCount: result.length,
    }
}

export interface ServerFunctionGroup {
    id: string
    name: string
    functions: string[]
}

export interface ServerFunction {
    id: string
    name: string
    desc: string
}

export function getServerFunctionDesc(group: string, name: string) {
    for (var i in ServerFunctionZhDesc) {
        const category = ServerFunctionZhDesc[i as keyof typeof ServerFunctionZhDesc];
        if (category.hasOwnProperty(name)) {
            return category[name as keyof typeof category];
        }
    }
    return "未知"
}

export const ServerFunctionZhDesc = {
    "Client": {
        "RegisterInstance": "注册实例",
        "DeregisterInstance": "注销实例",
        "ReportServiceContract": "上报服务契约",
        "DiscoverServices": "查询服务列表",
        "DiscoverInstances": "查询服务实例",
        "UpdateInstance": "更新服务实例",
        "DiscoverRouterRule": "查询自定义路由规则",
        "DiscoverRateLimitRule": "查询限流规则",
        "DiscoverCircuitBreakerRule": "查询熔断规则",
        "DiscoverFaultDetectRule": "查询探测规则",
        "DiscoverServiceContract": "查询服务契约",
        "DiscoverLaneRule": "查询泳道规则",
        "DiscoverConfig": "查询配置"
    },
    "Namespace": {
        "CreateNamespace": "创建单个命名空间",
        "CreateNamespaces": "批量创建命名空间",
        "DeleteNamespaces": "批量删除命名空间",
        "UpdateNamespaces": "批量更新命名空间",
        "DescribeNamespaces": "查询命名空间列表"
    },
    "Service": {
        "CreateServices": "批量创建服务",
        "DeleteServices": "批量删除服务",
        "UpdateServices": "批量更新服务",
        "DescribeAllServices": "查询全部服务列表",
        "DescribeServices": "查询服务列表",
        "DescribeServicesCount": "查询服务总数",
        "CreateServiceAlias": "批量创建服务别名",
        "DeleteServiceAliases": "批量删除服务别名",
        "UpdateServiceAlias": "批量更新服务别名",
        "DescribeServiceAliases": "查询服务别名列表"
    },
    "ServiceContract": {
        "CreateServiceContracts": "批量创建服务契约",
        "DescribeServiceContracts": "查询服务契约列表",
        "DescribeServiceContractVersions": "查询服务契约版本列表",
        "DeleteServiceContracts": "批量删除服务契约",
        "CreateServiceContractInterfaces": "在某个契约版本下创建接口列表",
        "AppendServiceContractInterfaces": "在某个契约版本下追加或覆盖接口列表",
        "DeleteServiceContractInterfaces": "在某个契约版本下删除接口列表"
    },
    "Instance": {
        "CreateInstances": "批量创建服务实例",
        "DeleteInstances": "批量删除服务实例",
        "DeleteInstancesByHost": "根据 IP 批量删除服务实例",
        "UpdateInstances": "批量更新服务实例",
        "UpdateInstancesIsolate": "批量更新服务实例隔离状态",
        "DescribeInstances": "查询服务实例列表",
        "DescribeInstancesCount": "查询服务实例总数",
        "DescribeInstanceLabels": "查询服务实例标签集合",
        "CleanInstance": "清理服务实例",
        "BatchCleanInstances": "批量清理服务实例",
        "DescribeInstanceLastHeartbeat": "查询服务实例的最后一次心跳时间"
    },
    "RouteRule": {
        "CreateRouteRules": "批量创建自定义路由",
        "DeleteRouteRules": "批量删除自定义路由",
        "UpdateRouteRules": "批量更新自定义路由",
        "EnableRouteRules": "批量启用/禁用自定义路由",
        "DescribeRouteRules": "查询自定义路由规则列表"
    },
    "RateLimitRule": {
        "CreateRateLimitRules": "批量创建限流规则",
        "DeleteRateLimitRules": "批量删除批量创建限流规则",
        "UpdateRateLimitRules": "批量更新批量创建限流规则",
        "EnableRateLimitRules": "批量启用/禁用批量创建限流规则",
        "DescribeRateLimitRules": "查询批量创建限流规列表"
    },
    "CircuitBreakerRule": {
        "CreateCircuitBreakerRules": "批量创建熔断规则",
        "DeleteCircuitBreakerRules": "批量删除熔断规则",
        "EnableCircuitBreakerRules": "批量启用/禁用熔断规则",
        "UpdateCircuitBreakerRules": "批量更新熔断规则",
        "DescribeCircuitBreakerRules": "查询熔断规则列表"
    },
    "FaultDetectRule": {
        "CreateFaultDetectRules": "批量创建探测规则",
        "DeleteFaultDetectRules": "批量删除探测规则",
        "UpdateFaultDetectRules": "批量更新探测规则",
        "EnableFaultDetectRules": "批量启用/禁用探测规则",
        "DescribeFaultDetectRules": "查询探测规则列表",
    },
    "LaneRule": {
        "CreateLaneGroups": "批量创建泳道组",
        "DeleteLaneGroups": "批量删除泳道组",
        "UpdateLaneGroups": "批量更新泳道组",
        "EnableLaneGroups": "批量启用/禁用泳道组",
        "DescribeLaneGroups": "查询泳道组规则列表",
    },
    "Config": {
        "InspectConfig": "查看配置状态与历史",
        "ManageConfig": "管理配置草稿与发布",
        "PreviewConfig": "预览配置"
    },
    "User": {
        "CreateUsers": "批量创建用户",
        "DeleteUsers": "批量更新用户",
        "DescribeUsers": "查询用户列表",
        "DescribeUserToken": "查询用户的Token",
        "EnableUserToken": "启用/禁用用户Token",
        "ResetUserToken": "重置用户Token",
        "UpdateUser": "更新用户信息",
        "UpdateUserPassword": "更新用户密码"
    },
    "UserGroup": {
        "CreateUserGroup": "创建用户组",
        "UpdateUserGroups": "更新用户组",
        "DeleteUserGroups": "删除用户组",
        "DescribeUserGroups": "查询用户组列表",
        "DescribeUserGroupDetail": "查询用户组详细信息",
        "DescribeUserGroupToken": "查询用户组的Token",
        "EnableUserGroupToken": "启用/禁用用户组Token",
        "ResetUserGroupToken": "重置用户组Token",
    },
    "AuthPolicy": {
        "CreateAuthPolicy": "创建鉴权策略",
        "UpdateAuthPolicies": "更新鉴权策略",
        "DeleteAuthPolicies": "删除鉴权策略",
        "DescribeAuthPolicies": "查询鉴权策略列表",
        "DescribeAuthPolicyDetail": "查询鉴权策略详细",
        "DescribePrincipalResources": "查询可授权资源列表",
        "AuthorizeResources": "授权资源",
    }
}

export enum BaseURL {
    NAMESPACE = '/core/v1/namespaces',
    SERVICE = '/naming/v1/services',
    LOGICAL_SERVICE = '/naming/v1/logical-services',
    SERVICE_CONTRACT = '/naming/v1/service/contracts',
    SERVICE_CONTRACT_VERSION = '/naming/v1/service/contract/versions',
    SERVICE_CONTRACT_METHOD = '/naming/v1/service/contract/methods',
    SERVICE_SUBSCRIBER = '/naming/v1/service/subscribers',
    ALIAS = '/naming/v1/service/aliases',
    INSTANCE = '/naming/v1/instances',
    CUSTOM_ROUTE = '/naming/v1/routings',
    LANE_GROUP = '/naming/v1/lane/groups',
    RATELIMIT_RULE = '/naming/v1/ratelimits',
    CIRCUIT_BREAKER = '/naming/v1/circuitbreakers',
    FAULT_DETECT = '/naming/v1/faultdetectors',
    LOSSLESS = '/naming/v1/lossless',
    TRAFFIC_SECURITY = '/naming/v1/traffic/security',
    TRAFFIC_MIRROR = '/naming/v1/traffic/mirrors',
    TRAFFIC_MOCK = '/naming/v1/traffic/mocks',
    MCP_SERVER = '/ai/mcp/v1/servers',
    MCP_SERVER_TOOL = '/ai/mcp/v1/server/tools',
    A2A_AGENT = '/ai/a2a/v1/agents',
    A2A_AGENT_SKILL = '/ai/a2a/v1/agent/skills',
}


// MatchString 匹配方式
// type: 'EXACT' | 'REGEX' | 'NOT_EQUALS' | 'IN' | 'NOT_IN' | 'RANGE'
// value_type: 'TEXT' | 'PARAMETER'
export enum MatchType {
    EXACT = 'EXACT',
    REGEX = 'REGEX',
    NOT_EQUALS = 'NOT_EQUALS',
    IN = 'IN',
    NOT_IN = 'NOT_IN',
    RANGE = 'RANGE',
}

export const MatchTypeMap = {
    EXACT: '完全匹配',
    REGEX: '正则匹配',
    NOT_EQUALS: '不匹配',
    IN: '包含',
    NOT_IN: '不包含',
    RANGE: '范围匹配',
}

export const MatchTypeOption = [
    {
        label: MatchTypeMap[MatchType.EXACT],
        value: MatchType.EXACT,
    },
    {
        label: MatchTypeMap[MatchType.REGEX],
        value: MatchType.REGEX,
    },
    {
        label: MatchTypeMap[MatchType.NOT_EQUALS],
        value: MatchType.NOT_EQUALS,
    },
    {
        label: MatchTypeMap[MatchType.IN],
        value: MatchType.IN,
    },
    {
        label: MatchTypeMap[MatchType.NOT_IN],
        value: MatchType.NOT_IN,
    },
    {
        label: MatchTypeMap[MatchType.RANGE],
        value: MatchType.RANGE,
    },
]

export enum MatchValueType {
    TEXT = 'TEXT',
    PARAMETER = 'PARAMETER',
}

export const MatchValueTypeMap = {
    'TEXT': '固定值',
    'PARAMETER': '请求参数',
}

export const MatchValueTypeOption = [
    {
        label: MatchValueTypeMap[MatchValueType.TEXT],
        value: MatchValueType.TEXT,
    },
    {
        label: MatchValueTypeMap[MatchValueType.PARAMETER],
        value: MatchValueType.PARAMETER,
    },
]

export interface MatchString {
    type: string
    value: string
    value_type: string
}

export interface MatcheLabel {
    key: string
    value: MatchString
}

// 匹配规则类型
export enum ClientLabelType {
    CLIENT_ID = 'CLIENT_ID',
    CLIENT_LANGUAGE = 'CLIENT_LANGUAGE',
    CLIENT_IP = 'CLIENT_IP',
    CUSTOM = 'CUSTOM',
}

export const ClientLabelTypeOption = [
    {
        value: ClientLabelType.CLIENT_IP,
        label: '客户端IP',
    },
    {
        value: ClientLabelType.CLIENT_ID,
        label: '客户端ID',
    },
    {
        value: ClientLabelType.CLIENT_LANGUAGE,
        label: '客户端语言',
    },
    // {
    //     value: ClientLabelType.CUSTOM,
    //     label: '自定义',
    // },
]

export const ClientLabelTypeMap = {
    CLIENT_ID: '客户端ID',
    CLIENT_IP: '客户端IP',
    CUSTOM: '自定义',
}

export type Op = '' | 'view' | 'create' | 'edit' | 'delete' | 'authorize' | 'publish' | 'rollback' | 'gray' | 'normal' | 'stopGray' | 'promote';


export interface RuleReleaseRequest {
    id: string
}

export interface RuleRelease {
    id?: string,
    release_name: string,
    rule_id: string,
    rule_name: string,
    resource: string, // 资源类型
    description: string,
    release_type: 'normal' | 'gray',
    client_label: MatcheLabel[],
}

export interface API {
    protocol: string
    method: string
    path: MatchString
}

export enum InterfaceProtocol {
    HTTP = 'HTTP',
    GRPC = 'GRPC',
    DUBBO = 'DUBBO',
}

export const InterfaceProtocolMap = {
    HTTP: 'HTTP',
    GRPC: 'GRPC',
    DUBBO: 'DUBBO',
}

export const InterfaceProtocolOption = [
    {
        label: InterfaceProtocolMap[InterfaceProtocol.HTTP],
        value: InterfaceProtocol.HTTP,
    },
    {
        label: InterfaceProtocolMap[InterfaceProtocol.GRPC],
        value: InterfaceProtocol.GRPC,
    },
    {
        label: InterfaceProtocolMap[InterfaceProtocol.DUBBO],
        value: InterfaceProtocol.DUBBO,
    },
]

export enum HTTPMethod {
    GET = 'GET',
    POST = 'POST',
    PUT = 'PUT',
    DELETE = 'DELETE',
    PATCH = 'PATCH',
    HEAD = 'HEAD',
    OPTIONS = 'OPTIONS',
    TRACE = 'TRACE',
}

export const HTTPMethodMap = {
    GET: 'GET',
    POST: 'POST',
    PUT: 'PUT',
    DELETE: 'DELETE',
    PATCH: 'PATCH',
    HEAD: 'HEAD',
    OPTIONS: 'OPTIONS',
    TRACE: 'TRACE',
}

export const HTTPMethodOption = Object.entries(HTTPMethodMap).map(([key, value]) => ({
    label: value,
    value: key,
}))

export interface Label {
    key: string
    value: string
}

export enum MatchLogic {
    AND = 'AND',
    OR = 'OR',
}

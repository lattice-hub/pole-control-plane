import request, { apiRequest, getAllList, getApiRequest, putApiRequest } from 'utils/request';

export const EventType = [
    {
        category: '实例事件',
        value: 'InstanceOnline',
        label: '实例上线',
    },
    {
        category: '实例事件',
        value: 'InstanceOffline',
        label: '实例下线',
    },
    {
        category: '实例事件',
        value: 'InstanceTurnHealth',
        label: '实例恢复健康',
    },
    {
        category: '实例事件',
        value: 'InstanceTurnUnHealth',
        label: '实例出现异常',
    },
    {
        category: '实例事件',
        value: 'InstanceOpenIsolate',
        label: '实例开启隔离',
    },
    {
        category: '实例事件',
        value: 'InstanceCloseIsolate',
        label: '实例关闭隔离',
    },
    {
        category: '服务事件',
        value: 'ServiceOpenEmptyPushProtect',
        label: '服务实例推空保护开启',
    },
    {
        category: '服务事件',
        value: 'ServiceCloseEmptyPushProtect',
        label: '服务实例推空保护关闭',
    },
    {
        category: '服务事件',
        value: 'ServiceExpireEmptyPushProtect',
        label: '服务实例推空保护开关过期',
    },
];

export const EventTypeMap = EventType.reduce((map, event) => {
    map[event.value] = event;
    return map;
}, {} as Record<string, { category: string; value: string; label: string }>);

export function getEventTypeInfo(value: string) {
    return EventTypeMap[value] || null;
}

// Cursor    string `json:"cursor"`
// Namespace string `json:"namespace"`
// Service   string `json:"service"`
// Resource  string `json:"resource"`
// EventType string `json:"event_type"`
// EventTime string `json:"event_time"`
export interface EventLog {
    cursor: string;
    namespace: string;
    service: string;
    resource: string;
    event_type: string;
    event_time: string;
    server: string; // 服务器名，用于区分生成记录的服务器
}

export interface DescribeEventLogRequest {
    namespace?: string;
    service?: string;
    resource?: string;
    event_type?: string;
    limit: number;
    cursor: number;
    direction: 'next' | 'prev';
    start_time?: string;
    end_time?: string;
}

export interface DescribeEventLogResponse {
    code: number;
    info: string;
    total: number;
    size: number;
    has_next: boolean;
    data: EventLog[];
}

// describeEventLog 查询服务端事件记录
export function describeEventLog(params: DescribeEventLogRequest) {
    return getApiRequest<DescribeEventLogResponse>({
        action: '/metrics/v1/server/events',
        data: params,
    });
}

// ResourceType  Resource      `json:"resource_type"`
// ResourceName  string        `json:"resource_name"`
// Namespace     string        `json:"namespace"`
// Operator      string        `json:"operator"`
// OperationType OperationType `json:"operation_type"`
// Detail        string        `json:"detail"`
// Server        string        `json:"server"` // Server name, used to distinguish the server that generated the record
// HappenTime    time.Time     `json:"happen_time"`
export interface OperationLog {
    cursor: string; // 分页游标
    resource_type: string;
    resource_name: string;
    namespace: string;
    operator: string;
    operation_type: string;
    detail: string;
    server: string; // 服务器名，用于区分生成记录的服务器
    happen_time: string; // 时间字符串
}

export interface DescribeOperationLogRequest {
    resource_type?: string;
    resource_name?: string;
    namespace?: string;
    operator?: string;
    operation_type?: string;
    limit: number;
    cursor: number;
    direction: 'next' | 'prev';
    start_time?: string;
    end_time?: string;
}

export interface DescribeOperationLogResponse {
    code: number;
    info: string;
    total: number;
    size: number;
    has_next: boolean;
    data: OperationLog[];
}

// describeOperationLog 查询服务器操作记录
export function describeOperationLog(params: DescribeOperationLogRequest) {
    return getApiRequest<DescribeOperationLogResponse>({
        action: '/metrics/v1/server/operations',
        data: params,
    });
}

// RNamespace          Resource = "Namespace"
// RService            Resource = "Service"
// RRouting            Resource = "Routing"
// RCircuitBreaker     Resource = "CircuitBreaker"
// RInstance           Resource = "Instance"
// RRateLimit          Resource = "RateLimit"
// RUser               Resource = "User"
// RUserGroup          Resource = "UserGroup"
// RUserGroupRelation  Resource = "UserGroupRelation"
// RAuthStrategy       Resource = "AuthStrategy"
// RAuthRole           Resource = "Role"
// RCircuitBreakerRule Resource = "CircuitBreakerRule"
// RFaultDetectRule    Resource = "FaultDetectRule"
// RServiceContract    Resource = "ServiceContract"
// RLaneGroup          Resource = "LaneGroup"
// RLaneRule           Resource = "LaneRule"
export const ResourceType = [
    {
        value: 'Namespace',
        label: '命名空间',
    },
    {
        value: 'Service',
        label: '服务',
    },
    {
        value: 'Routing',
        label: '路由规则',
    },
    {
        value: 'LaneGroup',
        label: '泳道组规则',
    },
    {
        value: 'LaneRule',
        label: '泳道规则',
    },
    {
        value: 'CircuitBreaker',
        label: '熔断规则',
    },
    {
        value: 'Instance',
        label: '实例',
    },
    {
        value: 'RateLimit',
        label: '限流规则',
    },
    {
        value: 'CircuitBreakerRule',
        label: '熔断规则',
    },
    {
        value: 'FaultDetectRule',
        label: '主动探测',
    },
    {
        value: 'ServiceContract',
        label: '服务契约',
    },
    {
        value: 'User',
        label: '用户',
    },
    {
        value: 'UserGroup',
        label: '用户组',
    },
    {
        value: 'AuthStrategy',
        label: '认证策略',
    },
    {
        value: 'Role',
        label: '角色',
    },
]

export const ResourceTypeMap = ResourceType.reduce((map, event) => {
    map[event.value] = event;
    return map;
}, {} as Record<string, { value: string; label: string }>);

export function getResourceTypeInfo(value: string) {
    const info = ResourceTypeMap[value];
    if (info) {
        return info;
    }
    return {
        value: 'Unknown',
        label: '未知资源',
    };
}

export const OperationType = [
    {
        value: 'Create',
        label: '创建',
    },
    {
        value: 'Update',
        label: '更新',
    },
    {
        value: 'Delete',
        label: '删除',
    },
    {
        value: 'Release',
        label: '发布',
    },
    {
        value: 'Rollback',
        label: '回滚',
    },
    {
        value: 'Enable',
        label: '启用',
    },
    {
        value: 'Disable',
        label: '禁用',
    }
]

export const OperationTypeMap = OperationType.reduce((map, event) => {
    map[event.value] = event;
    return map;
}, {} as Record<string, { value: string; label: string }>);

export function getOperationTypeInfo(value: string) {
    const info = OperationTypeMap[value];
    if (info) {
        return info;
    }
    return {
        value: 'Unknown',
        label: '未知操作',
    };
}

import request, { apiRequest, getAllList, getApiRequest, putApiRequest } from 'utils/request';
import { BaseURL } from './types';

export interface InstanceLocation {
    region: string
    zone: string
    campus: string
}

export interface Instance {
    id: string
    namespace: string
    service: string
    host: string
    port: number
    protocol: string
    version: string
    weight: number
    healthy: boolean
    isolate: boolean
    enableHealthCheck: boolean
    healthCheck?: HEALTH_CHECK_STRUCT
    metadata: Record<string, string>
    location: InstanceLocation
}

export interface InstanceView extends Instance {
    ctime: string
    mtime: string
    editable: boolean
    deleteable: boolean
    location: InstanceLocation
}

export interface HEALTH_CHECK_STRUCT {
    type: number
    heartbeat?: {
        ttl: number
    }
    tcp?: {
        interval: number
    }
    http?: {
        interval: number
        path: string
    }
}

export enum HEALTH_STATUS {
    HEALTH = 'true',
    ABNORMAL = 'false',
    METRIC_HEALTH = 'health',
    METRIC_ABNORMAL = 'unhealth',
    METRIC_OFFLINE = 'offline',
}

export enum ISOLATE_STATUS {
    ISOLATE = 'true',
    UNISOLATED = 'false',
}

export enum HEALTH_CHECK_METHOD {
    HEARTBEAT = 'HEARTBEAT',
}

export const HEALTH_CHECK_METHOD_MAP = {
    [HEALTH_CHECK_METHOD.HEARTBEAT]: {
        text: '心跳上报',
    },
}

export const HEALTH_STATUS_MAP = {
    [HEALTH_STATUS.HEALTH]: {
        text: '健康',
        theme: 'success',
    },
    [HEALTH_STATUS.ABNORMAL]: {
        text: '异常',
        theme: 'danger',
    },
    [HEALTH_STATUS.METRIC_HEALTH]: {
        text: '健康',
        theme: 'success',
    },
    [HEALTH_STATUS.METRIC_ABNORMAL]: {
        text: '异常',
        theme: 'danger',
    },
    [HEALTH_STATUS.METRIC_OFFLINE]: {
        text: '下线',
        theme: 'danger',
    },
}

export const HEALTH_STATUS_OPTIONS = [
    {
        text: HEALTH_STATUS_MAP[HEALTH_STATUS.HEALTH].text,
        value: HEALTH_STATUS.HEALTH,
    },
    {
        text: HEALTH_STATUS_MAP[HEALTH_STATUS.ABNORMAL].text,
        value: HEALTH_STATUS.ABNORMAL,
    },
]

export const HEALTH_CHECK_METHOD_OPTIONS = [
    {
        text: HEALTH_CHECK_METHOD_MAP[HEALTH_CHECK_METHOD.HEARTBEAT].text,
        value: HEALTH_CHECK_METHOD.HEARTBEAT,
    },
]

export const ISOLATE_STATUS_MAP = {
    [ISOLATE_STATUS.ISOLATE]: {
        text: '隔离',
        theme: 'danger',
    },
    [ISOLATE_STATUS.UNISOLATED]: {
        text: '不隔离',
        theme: 'success',
    },
}

export interface DescribeInstancesRequest{
    offset: number
    limit: number
    service: string
    namespace: string
    host?: string
    port?: number
    weight?: number
    protocol?: string
    version?: string
    keys?: string
    values?: string
    healthy?: boolean
    isolate?: boolean
}

export interface DescribeInstancesResponse {
    amount: number
    size: number
    data?: Array<InstanceView>
    instances?: Array<InstanceView>
}

const healthCheckTypeValue: Record<string, number> = {
    UNKNOWN: 0,
    HEARTBEAT: 1,
    TCP: 2,
    HTTP: 3,
}

const normalizeHealthCheck = (value: any): HEALTH_CHECK_STRUCT | undefined => {
    if (!value) return undefined
    return {
        ...value,
        type: typeof value.type === 'string' ? healthCheckTypeValue[value.type] ?? 0 : value.type,
    }
}

const normalizeInstance = (value: any): InstanceView => ({
    ...value,
    enableHealthCheck: value.enableHealthCheck ?? value.enable_health_check ?? false,
    healthCheck: normalizeHealthCheck(value.healthCheck ?? value.health_check),
})

const toApiInstance = <T extends CreateInstanceRequest | ModifyInstanceRequest>(value: T) => {
    const { enableHealthCheck, healthCheck, ...rest } = value
    return {
        ...rest,
        enable_health_check: enableHealthCheck,
        health_check: healthCheck,
    }
}

export async function describeInstances(params: DescribeInstancesRequest) {
    const res = await getApiRequest<DescribeInstancesResponse>({
        action: `${BaseURL.INSTANCE}`,
        data: params,
    })
    const instances = (res.data ?? res.instances ?? []).map(normalizeInstance)
    return {
        list: instances,
        totalCount: res.amount ?? instances.length,
    }
}

export interface CreateInstanceRequest {
    namespace: string
    service: string
    host: string
    port: number
    protocol: string
    version: string
    weight: number
    healthy: boolean
    isolate: boolean
    enableHealthCheck: boolean
    healthCheck?: HEALTH_CHECK_STRUCT
    metadata: Record<string, string>
    location: InstanceLocation
}

export async function createInstances(params: CreateInstanceRequest[]) {
    const res = await apiRequest({
        action: `${BaseURL.INSTANCE}`,
        data: params.map(toApiInstance),
    })

    return res
}

export interface ModifyInstanceRequest {
    id: string
    namespace: string
    service: string
    healthy: boolean
    isolate: boolean
    protocol: string
    version: string
    weight: number
    metadata: Record<string, string>
    location: InstanceLocation
    enableHealthCheck: boolean
    healthCheck?: HEALTH_CHECK_STRUCT
}

export async function modifyInstances(params: ModifyInstanceRequest[]) {
    const res = await putApiRequest({
        action: `${BaseURL.INSTANCE}`,
        data: params.map(toApiInstance),
    })

    return res
}

export interface DeleteInstancesRequest {
    id: string
    namespace: string
    service: string
    host: string
    port: number
}

export async function deleteInstances(params: DeleteInstancesRequest[]) {
    const res = await apiRequest<void>({
        action: `${BaseURL.INSTANCE}/delete`,
        data: params,
    })

    return res
}

export interface DescribeInstanceLabelsRequest {
    namespace: string
    service: string
}

export interface DescribeInstanceLabelsResponse {
    instanceLabels: {
        labels: Record<string, { values: string[] }>
    }
}

export async function describeInstanceLabels(params: DescribeInstanceLabelsRequest) {
    const res = await getApiRequest<DescribeInstanceLabelsResponse>({
        action: `${BaseURL.INSTANCE}/labels`,
        data: params,
    })
    return res.instanceLabels.labels
}

import request, { apiRequest, getAllList, getApiRequest, putApiRequest } from 'utils/request';
import { BaseURL } from './types';

export interface ServiceAlias {
    /** 服务别名 */
    alias: string
    /** 服务别名命名空间 */
    alias_namespace: string
    /** 服务别名指向的服务名 */
    service: string
    /** 服务别名指向的服务命名空间 */
    namespace: string
    /** 服务别名的描述信息 */
    comment?: string
}

export interface ServiceAliasView extends ServiceAlias {
    /** 服务别名创建时间 */
    ctime?: string
    /** 服务别名修改时间 */
    mtime?: string
    /** 服务别名是否可编辑 */
    editable?: boolean
    /** 服务别名是否可删除 */
    deleteable?: boolean
}

export interface DescribeServiceAliasRequest {
    /** 服务别名所指向的服务名。 */
    service?: string
    /** 服务别名所指向的命名空间名。 */
    namespace?: string
    /** 服务别名。 */
    alias?: string
    /** 服务别名命名空间。 */
    alias_namespace?: string
    /** 服务别名描述。 */
    comment?: string
    /** 偏移量，默认为0。 */
    offset: number
    /** 返回数量，默认为20，最大值为100。 */
    limit: number
}

export interface DescribeServiceAliasResponse {
    /** 服务别名总数量。 */
    amount: number
    size?: number
    /** 标准响应服务别名列表。 */
    data?: ServiceAlias[]
    /** 服务别名列表。 */
    aliases?: ServiceAlias[]
}

export async function describeServiceAlias(params: DescribeServiceAliasRequest) {
    const result = await getApiRequest<DescribeServiceAliasResponse>({
        action: `${BaseURL.ALIAS}`,
        data: params,
    })
    const aliases = result.data ?? result.aliases ?? []
    return {
        totalCount: result.amount ?? aliases.length,
        content: aliases,
    }
}

export interface CreateServiceAliasRequest {
    /** 服务别名 */
    alias: string
    /** 服务别名命名空间 */
    alias_namespace: string
    /** 服务别名所指向的服务名 */
    service: string
    /** 服务别名所指向的命名空间 */
    namespace: string
    /** 服务别名描述 */
    comment?: string
}

export interface CreateServiceAliasResponse {
    /** 创建是否成功。 */
    result: boolean
}

export function createServiceAlias(params: CreateServiceAliasRequest) {
    return apiRequest<CreateServiceAliasResponse>({
        action: '/naming/v1/service/alias',
        data: params,
    })
}


export type DeleteServiceAliasRequest = { alias: string; alias_namespace: string }[]

export interface DeleteServiceAliasResponse {
    /** 创建是否成功。 */
    result: boolean
}

export function deleteServiceAlias(params: DeleteServiceAliasRequest) {
    return apiRequest<DeleteServiceAliasResponse>({
        action: `${BaseURL.ALIAS}/delete`,
        data: params,
    })
}


export interface ModifyServiceAliasRequest {
    /** 服务别名 */
    alias: string
    /** 服务别名命名空间 */
    alias_namespace: string
    /** 服务别名所指向的服务名 */
    service: string
    /** 服务别名所指向的命名空间 */
    namespace: string
    /** 服务别名描述 */
    comment?: string
}

export interface ModifyServiceAliasResponse {
    /** 创建是否成功。 */
    result: boolean
}

export function modifyServiceAlias(params: ModifyServiceAliasRequest) {
    return putApiRequest<ModifyServiceAliasResponse>({
        action: '/naming/v1/service/alias',
        data: params,
    })
}

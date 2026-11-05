import request, { apiRequest, getAllList, getApiRequest, putApiRequest } from 'utils/request';
import { User } from './users';
import { AuthMutationResponse, isAuthMutationSuccessful } from './auth_policy';


// 用户组
export interface UserGroup {
    // 用户组ID
    id: string
    // 用户组名称
    name: string
    // 该用户组的授权Token
    auth_token: string
    // 该用户组的授权Token是否可用
    token_enable?: boolean
    // 简单描述
    comment: string
    // 用户组来源
    source?: string
    // 该用户组下的用户ID列表信息
    relation: GroupRelation
    // 创建时间
    ctime: string
    // 修改时间
    mtime: string
    // 用户组下的用户数量
    user_count: number
    // 用户组标签
    metadata: Record<string, string>
}

// 用户-用户组关系
export interface SimpleGroupRelation {
    // 用户组ID
    group_id?: string
    // 用户ID数组
    users?: { id: string }[]
}

/** 用户-用户组关系 */
export interface GroupRelation {
    // 用户组ID
    group_id?: string
    // 用户ID数组
    users?: { id: string, name?: string }[]
}

// 重置用户组Token
export interface ResetUserGroupTokenRequest {
    // 用户组ID
    id: string
}

export interface ResetUserGroupTokenResponse {
    // 执行结果
    result: boolean
}

export async function resetUserGroupToken(params: ResetUserGroupTokenRequest) {
    const result = await putApiRequest<ResetUserGroupTokenResponse & AuthMutationResponse>({
        action: '/auth/v1/usergroup/token/refresh',
        data: params,
    })
    return isAuthMutationSuccessful(result)
}

// 查询用户组列表
export interface DescribeUserGroupsRequest {
    // 用户组ID
    id?: string
    // 用户名称，模糊搜索最后加上 * 字符
    name?: string

    // 主账户ID
    owner?: string

    // 分页偏移量
    offset?: number

    // 查询条数
    limit?: number

    // 账户来源，QCloud | Polaris
    source?: string

    // 用户ID，填写用户ID时为查询该用户下的所有group
    user_id?: string
}

export interface DescribeUserGroupsResponse {
    // 总数
    amount: number
    size?: number
    // 标准响应用户组列表
    data?: UserGroup[]

    // 用户组列表
    userGroups?: UserGroup[]
}

export async function describeUserGroups(params: DescribeUserGroupsRequest) {
    const result = await getApiRequest<DescribeUserGroupsResponse>({ action: '/auth/v1/usergroups', data: params })
    const userGroups = result.data ?? result.userGroups ?? []
    return {
        totalCount: result.amount ?? userGroups.length,
        content: userGroups,
    }
}

export async function describeAllUserGroups() {
    const { list: userGroups } = await getAllList(describeUserGroups, {
        listKey: 'content',
        totalKey: 'totalCount',
    })({})
    return userGroups
}

// 查询治理中心用户组详细
export interface DescribeUserGroupDetailRequest {
    // 用户组ID
    id: string
}

export interface DescribeUserGroupDetailResponse {
    // 用户组详细
    userGroup: UserGroup
}

export async function describeUserGroupDetail(params: DescribeUserGroupDetailRequest) {
    const result = await getApiRequest<DescribeUserGroupDetailResponse | UserGroup>({
        action: '/auth/v1/usergroup/detail',
        data: params,
    })
    const userGroup = (result as DescribeUserGroupDetailResponse).userGroup ?? (result as UserGroup)
    return { userGroup }
}

// 批量删除用户分组
export interface DeleteUserGroupsRequest {
    // 用户组ID列表
    id: string
}

export interface DeleteUserGroupsResponse {
    // 执行结果
    result: boolean
}

export async function deleteUserGroups(params: DeleteUserGroupsRequest[]) {
    const result = await apiRequest<DeleteUserGroupsResponse & AuthMutationResponse>({ action: '/auth/v1/usergroups/delete', data: params })
    return isAuthMutationSuccessful(result)
}

// 查询治理中心用户组Token
export interface DescribeUserGroupTokenRequest {
    // 用户组ID
    id: string
}

export interface DescribeUserGroupTokenResponse {
    // 用户组
    userGroup: UserGroup
}

export async function describeUserGroupToken(params: DescribeUserGroupTokenRequest) {
    const result = await getApiRequest<DescribeUserGroupTokenResponse | UserGroup>({
        action: '/auth/v1/usergroup/token',
        data: params,
    })
    const userGroup = (result as DescribeUserGroupTokenResponse).userGroup ?? (result as UserGroup)
    return { userGroup }
}

// 批量创建用户组
export interface UpsertUserGroupRequest {
    // 用户组ID
    id?: string
    // 用户组名称
    name: string
    // 简单描述
    comment: string
    // 用户组下的用户ID列表信息
    relation?: SimpleGroupRelation
    // 用户组标签
    metadata: Record<string, string>
}

export interface UpsertUserGroupResponse {
    // 请求结果
    result: boolean
}

export async function createUserGroup(params: UpsertUserGroupRequest[]) {
    const result = await apiRequest<UpsertUserGroupResponse & AuthMutationResponse>({ action: '/auth/v1/usergroups', data: params })
    return isAuthMutationSuccessful(result)
}

// 批量修改用户组
export async function modifyUserGroup(params: UpsertUserGroupRequest[]) {
    const result = await putApiRequest<UpsertUserGroupResponse & AuthMutationResponse>({ action: '/auth/v1/usergroups', data: params })
    return isAuthMutationSuccessful(result)
}

// 批量修改用户组的资源访问 token
export interface ModifyUserGroupTokenRequest {
    // 用户组Token信息
    id: string
    token_enable: boolean
}
export interface ModifyUserGroupTokenResponse {
    // 执行结果
    result: boolean
}

export async function modifyUserGroupToken(params: ModifyUserGroupTokenRequest) {
    const result = await putApiRequest<ModifyUserGroupTokenResponse & AuthMutationResponse>({
        action: '/auth/v1/usergroup/token/enable',
        data: params,
    })
    return isAuthMutationSuccessful(result)
}


// 修改用户Token
export interface RefreshUserGroupTokenRequest {
    // 用户Token信息
    id: string
}

export interface RefreshUserGroupTokenResponse {
    // 执行结果
    result: boolean
}

export async function refreshUserGroupToken(params: RefreshUserGroupTokenRequest) {
    const result = await putApiRequest<RefreshUserGroupTokenResponse & AuthMutationResponse>({
        action: '/auth/v1/usergroup/token/refresh',
        data: params,
    })
    return isAuthMutationSuccessful(result)
}

import axios, { AxiosRequestConfig, AxiosResponse } from 'axios';
import proxy from '../configs/host';
import { v4 as uuidv4 } from 'uuid';

const env = import.meta.env.MODE || 'development';
const API_HOST = proxy[env].API;

const TIMEOUT = 5000;

export const PoleTokenKey = 'pole_token'
export const LoginUserIdKey = 'login-user-id'

export const instance = axios.create({
  baseURL: API_HOST,
  timeout: TIMEOUT,
  withCredentials: true,
});

// instance.interceptors.response.use(
//   // eslint-disable-next-line consistent-return
//   (response) => {
//     if (response.status === 200) {
//       return response;
//     }
//     return Promise.reject(response?.data.info);
//   },
//   (e) => Promise.reject(e),
// );

export default instance;


export interface APIRequestOption {
  action: string
  data?: any
  opts?: AxiosRequestConfig
}

export interface ApiResponse {
  code: number
  info: string
  requestId?: string
  data?: any
  amount?: number
  size?: number
}

/**
 * 前端可跨 Redux 和 UI 边界传递的请求错误数据。
 * `message` 只承载后端业务信息；业务码和 Request ID 始终使用独立字段。
 */
export interface RequestErrorPayload {
  code?: number
  message: string
  requestId?: string
}

export class RequestError extends Error implements RequestErrorPayload {
  readonly code?: number
  readonly requestId: string

  constructor(payload: RequestErrorPayload, options?: ErrorOptions) {
    super(payload.message, options)
    this.name = 'RequestError'
    this.code = payload.code
    this.requestId = payload.requestId || ''
  }
}

export const toRequestErrorPayload = (error: unknown): RequestErrorPayload | string => {
  if (error instanceof RequestError) {
    return { code: error.code, message: error.message, requestId: error.requestId || undefined }
  }
  if (error instanceof Error) return error.message
  return typeof error === 'string' ? error : String(error || '')
}

/** 标准 API 响应格式 { code, data, info, amount?, size? }，解包后返回 data 并合并顶层 amount/size */
function unwrapResponse(body: unknown): unknown {
  if (body && typeof body === 'object' && 'data' in body && (body as ApiResponse).data !== undefined) {
    const b = body as ApiResponse
    if (Array.isArray(b.data)) {
      return { data: b.data, amount: b.amount ?? b.data.length, size: b.size ?? b.data.length }
    }
    return { ...(b.data as object), amount: b.amount ?? (b.data as any)?.amount, size: b.size ?? (b.data as any)?.size }
  }
  return body
}

export const SuccessCode = 299999
export const LegacySessionInvalidCode = 407
export const TokenDisabledCode = 401003
export const TokenNotExistCode = 401004

const clearLoginState = () => {
  window.localStorage.removeItem(PoleTokenKey)
  window.localStorage.removeItem(LoginUserIdKey)
  window.localStorage.removeItem('login-name')
  window.localStorage.removeItem('login-role')
  window.localStorage.removeItem('login-owner-id')
}

const handleTokenNotExist = () => {
  clearLoginState()
  if (window.location.pathname !== '/login') {
    window.location.replace('/login')
  }
}

const shouldResetLogin = (error: any, action: string) => {
  if (action === '/auth/v1/user/login') {
    return false
  }
  const status = error.response?.status
  const code = error.response?.data?.code
  if (action === '/auth/v1/user/session' && status === 401) {
    return true
  }
  return status === LegacySessionInvalidCode || code === LegacySessionInvalidCode ||
    code === TokenDisabledCode || code === TokenNotExistCode
}

const toRequestError = (reqId: string, error: unknown) => {
  const response = axios.isAxiosError(error) ? error.response?.data as Partial<ApiResponse> | undefined : undefined
  const detail = typeof response?.info === 'string' && response.info.trim() ? response.info.trim() : ''
  const code = typeof response?.code === 'number' ? response.code : undefined
  return new RequestError({
    code,
    message: detail || '请求失败',
    requestId: typeof response?.requestId === 'string' && response.requestId.trim() ? response.requestId.trim() : reqId,
  }, { cause: error })
}

export async function apiRequest<T>(options: APIRequestOption) {
  const { action, data = {}, opts } = options
  const reqId = uuidv4()
  try {
    const res = await instance.post<T & ApiResponse>(action, data, {
      ...opts,
      headers: {
        'Authorization': window.localStorage.getItem(PoleTokenKey),
        'X-Pole-User': window.localStorage.getItem(LoginUserIdKey),
        'X-Request-Id': reqId,
        ...(opts?.headers ?? {}),
      },
    }) as AxiosResponse<T & ApiResponse>

    return unwrapResponse(res.data as ApiResponse) as T
  } catch (error) {
    if (shouldResetLogin(error, action)) {
      handleTokenNotExist()
    }
    throw toRequestError(reqId, error)
  }
}

export async function getApiRequest<T>(options: APIRequestOption) {
  const { action, data = {}, opts } = options
  const reqId = uuidv4()
  try {
    const res = await instance.get<T & ApiResponse>(action, {
      params: data,
      ...opts,
      headers: {
        'Authorization': window.localStorage.getItem(PoleTokenKey),
        'X-Pole-User': window.localStorage.getItem(LoginUserIdKey),
        'X-Request-Id': reqId,
        ...(opts?.headers ?? {}),
      },
    }) as AxiosResponse<T & ApiResponse>
    return unwrapResponse(res.data as ApiResponse) as T
  } catch (error) {
    if (shouldResetLogin(error, action)) {
      handleTokenNotExist()
    }
    throw toRequestError(reqId, error)
  }
}

export async function putApiRequest<T>(options: APIRequestOption) {
  const { action, data = {}, opts } = options
  const reqId = uuidv4()
  try {
    const res = await axios.put<T & ApiResponse>(action, data, {
      ...opts,
      headers: {
        'Authorization': window.localStorage.getItem(PoleTokenKey),
        'X-Pole-User': window.localStorage.getItem(LoginUserIdKey),
        'X-Request-Id': reqId,
      },
    }) as AxiosResponse<T & ApiResponse>

    return unwrapResponse(res.data as ApiResponse) as T
  } catch (error) {
    if (shouldResetLogin(error, action)) {
      handleTokenNotExist()
    }
    throw toRequestError(reqId, error)
  }
}

export async function deleteApiRequest<T>(options: APIRequestOption) {
  const { action, data = {}, opts } = options
  const reqId = uuidv4()
  try {
    const res = await axios.delete<T & ApiResponse>(action, {
      params: data,
      ...opts,
      headers: {
        'Authorization': window.localStorage.getItem(PoleTokenKey),
        'X-Pole-User': window.localStorage.getItem(LoginUserIdKey),
        'X-Request-Id': reqId,
      },
    }) as AxiosResponse<T & ApiResponse>

    return unwrapResponse(res.data as ApiResponse) as T
  } catch (error) {
    if (shouldResetLogin(error, action)) {
      handleTokenNotExist()
    }
    throw toRequestError(reqId, error)
  }
}

export interface FetchAllOptions {
  listKey?: string
  totalKey?: string
  limitKey?: string
  offsetKey?: string
}

const DefaultOptions = {
  listKey: 'list',
  totalKey: 'totalCount',
  limitKey: 'limit',
  offsetKey: 'offset',
}

/**
 * 获取所有的列表
 * @param fetchFun 模板函数需要支持pageNo,pageSize参数
 * @param listKey 返回结果中列表的键名称 默认list
 */
export function getAllList(fetchFun: (params?: any) => Promise<any>, options: FetchAllOptions = {}) {
  return async function (params: any) {
    const fetchOptions = { ...DefaultOptions, ...options }
    let allList: any[] = [],
      pageNo = 0
    const pageSize = 50
    while (true) {
      // 每次获取获取50条
      params = { ...params }

      const result = await fetchFun({
        ...params,
        [fetchOptions.offsetKey]: pageNo * pageSize,
        [fetchOptions.limitKey]: pageSize,
      } as any)

      const cur = result[fetchOptions.listKey]
      if (cur && cur.length !== 0) {
        allList = allList.concat(cur)
      }
      if (allList.length >= result[fetchOptions.totalKey]) {
        // 返回
        break
      } else {
        pageNo++
      }
    }
    return {
      list: allList,
      totalCount: allList.length,
    }
  }
}

import type { ApiBody } from '@dlidli/shared'
import { ErrCode } from '@dlidli/shared'

export type HttpMethod = 'GET' | 'POST' | 'PUT' | 'PATCH' | 'DELETE'

export interface RequestOptions {
  method?: HttpMethod
  /** query 参数 */
  params?: Record<string, string | number | boolean | undefined>
  /** JSON 请求体 */
  data?: unknown
  headers?: Record<string, string>
}

/**
 * 请求适配器：Web/H5 用 fetch；小程序端注入 uni.request 实现。
 * 返回值为后端统一响应体。
 */
export type RequestAdapter = (
  url: string,
  options: Required<Pick<RequestOptions, 'method'>> &
    RequestOptions & { body?: string | FormData | Blob },
) => Promise<{
  status: number
  body: ApiBody<unknown>
}>

export class ApiError extends Error {
  constructor(
    public readonly code: number,
    message: string,
    public readonly traceId?: string,
  ) {
    super(message)
    this.name = 'ApiError'
  }
}

export interface ClientConfig {
  /** 例如 '' （走 Vite 代理）或 'https://api.dlidli.com' */
  baseURL?: string
  /** 每次请求前获取 token */
  getToken?: () => string | null
  /**
   * access token 过期（10003）时的静默续期钩子：
   * 用 refresh_token 换新令牌并持久化，返回 true 则自动重试原请求。
   */
  onTokenExpired?: () => Promise<boolean>
  /** 续期失败/无法续期时回调（应清空全部登录态并跳登录页） */
  onUnauthorized?: () => void
  adapter?: RequestAdapter
}

export interface DownloadOptions {
  /** query 参数 */
  params?: RequestOptions['params']
  /** 响应无 Content-Disposition 时的文件名兜底 */
  fallbackName: string
}

export interface DownloadResult {
  blob: Blob
  /** 优先取 Content-Disposition filename，否则为 fallbackName */
  filename: string
}

/** 默认 fetch 适配器（Web/H5） */
export const fetchAdapter: RequestAdapter = async (url, options) => {
  const res = await fetch(url, {
    method: options.method,
    headers: options.headers,
    body: options.body,
  })
  const body = (await res.json()) as ApiBody<unknown>
  return { status: res.status, body }
}

export class HttpClient {
  /** 单飞：并发 401 时只触发一次刷新 */
  private refreshing: Promise<boolean> | null = null

  constructor(private readonly cfg: ClientConfig = {}) {}

  request<T>(path: string, options: RequestOptions = {}): Promise<T> {
    return this.doRequest<T>(path, options, false)
  }

  private async doRequest<T>(path: string, options: RequestOptions, retried: boolean): Promise<T> {
    const { getToken, onUnauthorized, adapter = fetchAdapter } = this.cfg

    const url = this.buildUrl(path, options.params)

    const headers: Record<string, string> = { ...options.headers }
    const token = getToken?.()
    if (token) headers.Authorization = `Bearer ${token}`

    let body: string | FormData | Blob | undefined
    if (options.data instanceof FormData || options.data instanceof Blob) {
      body = options.data // Content-Type 由浏览器/调用方处理
    } else if (options.data !== undefined) {
      headers['Content-Type'] = 'application/json'
      body = JSON.stringify(options.data)
    }

    const { body: resBody } = await adapter(url, {
      ...options,
      method: options.method ?? 'GET',
      headers,
      body,
    })

    if (resBody.code !== ErrCode.OK) {
      if (resBody.code === ErrCode.Unauthorized) {
        // 先尝试用 refresh_token 静默续期并重试一次
        if (!retried && (await this.tryRefresh())) {
          return this.doRequest<T>(path, options, true)
        }
        onUnauthorized?.()
      }
      throw new ApiError(resBody.code, resBody.message, resBody.trace_id)
    }
    return resBody.data as T
  }

  /**
   * 下载非 JSON 响应（CSV 导出等）：自动带鉴权；401 同样走静默续期与
   * onUnauthorized 流程，避免调用方绕过封装手拼 token。
   */
  async download(path: string, opts: DownloadOptions, retried = false): Promise<DownloadResult> {
    const { getToken, onUnauthorized } = this.cfg
    const url = this.buildUrl(path, opts.params)

    const headers: Record<string, string> = {}
    const token = getToken?.()
    if (token) headers.Authorization = `Bearer ${token}`

    const res = await fetch(url, { headers })
    if (!res.ok) {
      let code: number = ErrCode.Internal
      let message = `HTTP ${res.status}`
      try {
        // 错误体为后端统一响应时透出业务码，否则保留 HTTP 状态
        const body = (await res.json()) as ApiBody<unknown>
        if (typeof body.code === 'number') {
          code = body.code
          message = body.message
        }
      } catch {
        // 非 JSON 错误体
      }
      if (code === ErrCode.Unauthorized) {
        if (!retried && (await this.tryRefresh())) {
          return this.download(path, opts, true)
        }
        onUnauthorized?.()
      }
      throw new ApiError(code, message)
    }
    const blob = await res.blob()
    const match = (res.headers.get('Content-Disposition') ?? '').match(/filename="?([^";]+)"?/)
    return { blob, filename: match?.[1] ?? opts.fallbackName }
  }

  private buildUrl(path: string, params?: RequestOptions['params']): string {
    let url = (this.cfg.baseURL ?? '') + path
    if (params) {
      const qs = new URLSearchParams()
      for (const [k, v] of Object.entries(params)) {
        if (v !== undefined) qs.append(k, String(v))
      }
      const s = qs.toString()
      if (s) url += (url.includes('?') ? '&' : '?') + s
    }
    return url
  }

  private tryRefresh(): Promise<boolean> {
    const { onTokenExpired } = this.cfg
    if (!onTokenExpired) return Promise.resolve(false)
    if (!this.refreshing) {
      this.refreshing = onTokenExpired()
        .catch(() => false)
        .finally(() => {
          this.refreshing = null
        })
    }
    return this.refreshing
  }

  get<T>(path: string, params?: RequestOptions['params']) {
    return this.request<T>(path, { method: 'GET', params })
  }

  post<T>(path: string, data?: unknown) {
    return this.request<T>(path, { method: 'POST', data })
  }

  patch<T>(path: string, data?: unknown) {
    return this.request<T>(path, { method: 'PATCH', data })
  }

  /** multipart 表单上传 */
  postForm<T>(path: string, form: FormData) {
    return this.request<T>(path, { method: 'POST', data: form })
  }

  /** 二进制分片上传 */
  putRaw<T>(path: string, blob: Blob) {
    return this.request<T>(path, {
      method: 'PUT',
      data: blob,
      headers: { 'Content-Type': 'application/octet-stream' },
    })
  }

  put<T>(path: string, data?: unknown) {
    return this.request<T>(path, { method: 'PUT', data })
  }

  delete<T>(path: string, params?: RequestOptions['params']) {
    return this.request<T>(path, { method: 'DELETE', params })
  }
}

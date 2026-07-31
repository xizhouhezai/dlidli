// DliDli H5 端 API 客户端：复用 @dlidli/api-client，注入 uni.request 适配器
import { createApiClient, type RequestAdapter } from '@dlidli/api-client'
import type { ApiBody } from '@dlidli/shared'

const TOKEN_KEY = 'dlidli_token'

/** uni.request 适配器：把 api-client 请求语义映射到 uni.request，返回统一响应体 */
const uniAdapter: RequestAdapter = (url, options) => {
  return new Promise((resolve, reject) => {
    uni.request({
      url,
      method: (options.method as UniApp.RequestOptions['method']) ?? 'GET',
      header: options.headers as Record<string, string>,
      data: options.body as string | undefined,
      success: (res) => {
        resolve({ status: res.statusCode, body: res.data as ApiBody<unknown> })
      },
      fail: (err) => reject(new Error(err.errMsg || '网络请求失败')),
    })
  })
}

export const api = createApiClient({
  baseURL: '',
  getToken: () => uni.getStorageSync(TOKEN_KEY) || null,
  adapter: uniAdapter,
})

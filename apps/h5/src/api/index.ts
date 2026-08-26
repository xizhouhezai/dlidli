// DliDli H5 端 API 客户端：复用 @dlidli/api-client，注入 uni.request 适配器
import { createApiClient, refreshTokens, type RequestAdapter, type TokenStorage } from '@dlidli/api-client'
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

/** H5 端令牌存取：实现 api-client 的 TokenStorage 抽象（uni storage）。 */
const tokens: TokenStorage = {
  getAccess: () => (uni.getStorageSync(TOKEN_KEY) as string) || null,
  getRefresh: () => (uni.getStorageSync('dlidli_refresh') as string) || null,
  save: (access, refresh) => {
    uni.setStorageSync(TOKEN_KEY, access)
    uni.setStorageSync('dlidli_refresh', refresh)
  },
  clear: () => {
    uni.removeStorageSync(TOKEN_KEY)
    uni.removeStorageSync('dlidli_refresh')
  },
}

/** access token 过期时静默续期（api-client 共享实现；H5 无 fetch 时降级为不可续期） */
async function refreshAccess(): Promise<boolean> {
  if (typeof fetch !== 'function') return false
  return refreshTokens(tokens).then((pair) => pair !== null)
}

export const api = createApiClient({
  baseURL: '',
  getToken: () => tokens.getAccess(),
  onTokenExpired: refreshAccess,
  adapter: uniAdapter,
})

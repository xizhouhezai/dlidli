import { createApiClient } from '@dlidli/api-client'
import { clearTokens, readRefreshToken, readToken, saveTokens } from '@/utils/token'
import router from '@/router'

/**
 * access token 过期时用 refresh_token 静默续期。
 * 用原生 fetch 而非 api 实例，避免刷新请求再次进入 401 重试链。
 */
async function refreshTokens(): Promise<boolean> {
  const refresh = readRefreshToken()
  if (!refresh) return false
  try {
    const res = await fetch('/api/v1/auth/refresh', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ refresh_token: refresh }),
    })
    const body = await res.json()
    if (body.code !== 0) return false
    saveTokens(body.data.access_token, body.data.refresh_token)
    return true
  } catch {
    return false
  }
}

/** 续期失败：清空全部登录信息（缓存 + store）并跳登录页 */
async function handleUnauthorized() {
  clearTokens()
  // 动态引入避免与 stores/user 的静态循环依赖
  const { useUserStore } = await import('@/stores/user')
  useUserStore().forceLogout()
  router.push({ name: 'login' })
}

/** 全局 API 客户端：开发环境经 Vite 代理转发到 :8000 */
export const api = createApiClient({
  baseURL: '',
  getToken: readToken,
  onTokenExpired: refreshTokens,
  onUnauthorized: () => {
    void handleUnauthorized()
  },
})

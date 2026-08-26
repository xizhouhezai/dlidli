import { createApiClient, createLocalStorageTokens, refreshTokens } from '@dlidli/api-client'
import { clearTokens, readToken } from '@/utils/token'
import router from '@/router'

/** C 端令牌存取（key 与既有 localStorage 约定保持一致） */
const tokens = createLocalStorageTokens('dlidli')

/**
 * access token 过期时用 refresh_token 静默续期（api-client 共享实现：
 * 原生 fetch 刷新避免进入 401 重试链；成功自动持久化新令牌对）。
 */
function refreshAccess(): Promise<boolean> {
  return refreshTokens(tokens).then((pair) => pair !== null)
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
  onTokenExpired: refreshAccess,
  onUnauthorized: () => {
    void handleUnauthorized()
  },
})

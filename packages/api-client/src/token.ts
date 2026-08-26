import type { TokenPair } from './apis/auth'

/**
 * 跨端令牌存取抽象（M3-ENG-13）：
 * Web/Admin 用 localStorage，H5/小程序注入 uni storage 实现。
 */
export interface TokenStorage {
  getAccess(): string | null
  getRefresh(): string | null
  save(access: string, refresh: string): void
  clear(): void
}

/** localStorage 实现（Web/Admin；keyPrefix 隔离 C 端与管理员会话）。 */
export function createLocalStorageTokens(keyPrefix = 'dlidli'): TokenStorage {
  const ACCESS_KEY = `${keyPrefix}_token`
  const REFRESH_KEY = `${keyPrefix}_refresh`
  return {
    getAccess: () => localStorage.getItem(ACCESS_KEY),
    getRefresh: () => localStorage.getItem(REFRESH_KEY),
    save: (access, refresh) => {
      localStorage.setItem(ACCESS_KEY, access)
      localStorage.setItem(REFRESH_KEY, refresh)
    },
    clear: () => {
      localStorage.removeItem(ACCESS_KEY)
      localStorage.removeItem(REFRESH_KEY)
    },
  }
}

/**
 * 静默续期：用原生 fetch 而非 HttpClient，避免刷新请求再次进入 401 重试链。
 * 成功返回新令牌对并持久化；失败返回 null（调用方走 onUnauthorized）。
 */
export async function refreshTokens(
  storage: Pick<TokenStorage, 'getRefresh' | 'save'>,
  endpoint = '/api/v1/auth/refresh',
): Promise<TokenPair | null> {
  const refresh = storage.getRefresh()
  if (!refresh) return null
  try {
    const res = await fetch(endpoint, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ refresh_token: refresh }),
    })
    const body = (await res.json()) as { code: number; data?: TokenPair }
    if (body.code !== 0 || !body.data) return null
    storage.save(body.data.access_token, body.data.refresh_token)
    return body.data
  } catch {
    return null
  }
}

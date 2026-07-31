// token 本地存取（独立模块，避免 store 与 api-client 循环依赖）
const TOKEN_KEY = 'dlidli_token'
const REFRESH_KEY = 'dlidli_refresh'

export function readToken(): string | null {
  return localStorage.getItem(TOKEN_KEY)
}

export function readRefreshToken(): string | null {
  return localStorage.getItem(REFRESH_KEY)
}

export function saveTokens(access: string, refresh: string) {
  localStorage.setItem(TOKEN_KEY, access)
  localStorage.setItem(REFRESH_KEY, refresh)
}

export function clearTokens() {
  localStorage.removeItem(TOKEN_KEY)
  localStorage.removeItem(REFRESH_KEY)
}

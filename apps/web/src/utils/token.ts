// token 本地存取：委托 api-client 的 TokenStorage（M3-ENG-13），
// key 约定仍为 dlidli_token / dlidli_refresh；本模块只保留既有导入路径与单例。
import { createLocalStorageTokens } from '@dlidli/api-client'

export const tokens = createLocalStorageTokens('dlidli')

export function readToken(): string | null {
  return tokens.getAccess()
}

export function readRefreshToken(): string | null {
  return tokens.getRefresh()
}

export function saveTokens(access: string, refresh: string) {
  tokens.save(access, refresh)
}

export function clearTokens() {
  tokens.clear()
}

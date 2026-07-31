// 后台管理员令牌（与 C 端用户会话完全隔离）
const ADMIN_TOKEN_KEY = 'dlidli_admin_token'
const ADMIN_INFO_KEY = 'dlidli_admin_info'

export interface AdminInfo {
  username: string
  role: string
}

export function readAdminToken(): string | null {
  return localStorage.getItem(ADMIN_TOKEN_KEY)
}

export function saveAdminToken(token: string) {
  localStorage.setItem(ADMIN_TOKEN_KEY, token)
}

export function saveAdminInfo(info: AdminInfo) {
  localStorage.setItem(ADMIN_INFO_KEY, JSON.stringify(info))
}

export function readAdminInfo(): AdminInfo | null {
  const raw = localStorage.getItem(ADMIN_INFO_KEY)
  if (!raw) return null
  try {
    return JSON.parse(raw) as AdminInfo
  } catch {
    return null
  }
}

export function clearAdminToken() {
  localStorage.removeItem(ADMIN_TOKEN_KEY)
  localStorage.removeItem(ADMIN_INFO_KEY)
}

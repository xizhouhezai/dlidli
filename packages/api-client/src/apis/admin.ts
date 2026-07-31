import type { HttpClient } from '../http'
import type { VideoCard } from './video'

export interface AdminLoginResp {
  token: string
  username: string
  role: string
}

export interface ReviewItem extends VideoCard {
  description: string
  play_url: string
}

export interface SensitiveWord {
  id: string
  word: string
  created_at: string
}

export interface AdminUserItem {
  id: string
  nickname: string
  avatar: string
  level: number
  status: number
  muted_until: string | null
  banned_until: string | null
  created_at: string
}

export type PunishAction = 'mute' | 'unmute' | 'ban' | 'unban'

export interface AdminPermission {
  id: string
  code: string
  name: string
  type: string
  parent: string
  path: string
  icon: string
  sort: number
}

export interface AdminMenuItem {
  code: string
  name: string
  path: string
  icon: string
}

export interface CurrentPerm {
  is_super: boolean
  perms: string[]
  menus: AdminMenuItem[]
}

export interface AdminRole {
  id: string
  code: string
  name: string
  remark: string
  is_builtin: number
  members: number
  perms: string[]
}

export interface AdminAccount {
  id: string
  username: string
  nickname: string
  status: number
  role_ids: string[]
  role_names: string
  created_at: string
  last_login_at: string | null
}

export interface SaveRolePayload {
  name: string
  code?: string
  remark?: string
  perms: string[]
}

export interface SaveAdminPayload {
  username: string
  nickname?: string
  password?: string
  role_ids: string[]
}

export interface AdminCategory {
  id: number
  parent_id: number
  name: string
  sort: number
  status: number
}

export interface SaveCategoryPayload {
  parent_id: number
  name: string
  sort: number
  status: number
}

/** 后台管理接口（对应 /api/v1/admin，需管理员令牌）。 */
export function createAdminApi(http: HttpClient) {
  return {
    login: (username: string, password: string) =>
      http.post<AdminLoginResp>('/api/v1/admin/login', { username, password }),

    reviewList: (page = 1, pageSize = 20) =>
      http.get<{ list: ReviewItem[]; total: number }>('/api/v1/admin/videos/review', {
        page,
        page_size: pageSize,
      }),

    review: (bvid: string, approve: boolean, reason = '') =>
      http.post<null>(`/api/v1/admin/videos/${bvid}/review`, { approve, reason }),

    /** 敏感词列表 */
    sensitiveWords: () =>
      http.get<{ list: SensitiveWord[] }>('/api/v1/admin/sensitive-words'),

    /** 新增敏感词 */
    addSensitiveWord: (word: string) =>
      http.post<SensitiveWord>('/api/v1/admin/sensitive-words', { word }),

    /** 删除敏感词（id 字符串避免精度丢失） */
    deleteSensitiveWord: (id: string) =>
      http.delete<null>(`/api/v1/admin/sensitive-words/${id}`),

    /** 后台用户查询（keyword 支持 UID/手机号/昵称；status -1 全部） */
    users: (params: { keyword?: string, status?: number, page?: number, page_size?: number }) =>
      http.get<{ list: AdminUserItem[], total: number }>('/api/v1/admin/users', params),

    /** 用户处罚/解除（action: mute|unmute|ban|unban；ban days=0 为永久） */
    punishUser: (id: string, action: PunishAction, days = 0, reason = '') =>
      http.post<null>(`/api/v1/admin/users/${id}/punish`, { action, days, reason }),

    // ---- RBAC ----
    /** 当前登录者权限码与可见菜单 */
    myPermissions: () =>
      http.get<CurrentPerm>('/api/v1/admin/me/permissions'),

    /** 权限点全集 */
    permissions: () =>
      http.get<{ list: AdminPermission[] }>('/api/v1/admin/permissions'),

    /** 角色列表 */
    roles: () =>
      http.get<{ list: AdminRole[] }>('/api/v1/admin/roles'),

    createRole: (payload: SaveRolePayload) =>
      http.post<AdminRole>('/api/v1/admin/roles', payload),

    updateRole: (id: string, payload: SaveRolePayload) =>
      http.put<null>(`/api/v1/admin/roles/${id}`, payload),

    deleteRole: (id: string) =>
      http.delete<null>(`/api/v1/admin/roles/${id}`),

    /** 管理员账号列表 */
    admins: (page = 1, pageSize = 20) =>
      http.get<{ list: AdminAccount[], total: number }>('/api/v1/admin/admins', { page, page_size: pageSize }),

    createAdmin: (payload: SaveAdminPayload) =>
      http.post<{ id: string }>('/api/v1/admin/admins', payload),

    updateAdmin: (id: string, payload: SaveAdminPayload) =>
      http.put<null>(`/api/v1/admin/admins/${id}`, payload),

    toggleAdmin: (id: string, status: number) =>
      http.post<null>(`/api/v1/admin/admins/${id}/toggle`, { status }),

    resetAdminPassword: (id: string, password: string) =>
      http.post<null>(`/api/v1/admin/admins/${id}/reset-password`, { password }),

    deleteAdmin: (id: string) =>
      http.delete<null>(`/api/v1/admin/admins/${id}`),

    // ---- 分区管理 ----
    /** 分区列表（含停用） */
    categories: () =>
      http.get<{ list: AdminCategory[] }>('/api/v1/admin/categories'),

    createCategory: (payload: SaveCategoryPayload) =>
      http.post<AdminCategory>('/api/v1/admin/categories', payload),

    updateCategory: (id: number, payload: SaveCategoryPayload) =>
      http.put<null>(`/api/v1/admin/categories/${id}`, payload),

    deleteCategory: (id: number) =>
      http.delete<null>(`/api/v1/admin/categories/${id}`),
  }
}

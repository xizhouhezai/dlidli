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
  phone: string
  signature: string
  gender: number
  level: number
  exp: number
  coin: number
  status: number
  muted_until: string | null
  banned_until: string | null
  created_at: string
}

export type PunishAction = 'mute' | 'unmute' | 'ban' | 'unban'

/** 举报队列项 */
export interface ReportItem {
  id: string
  target_type: number
  target_name: string
  target_desc: string
  target_id: string
  reporter_name: string
  reason_type: number
  reason_name: string
  reason: string
  status: number
  created_at: string
}

export interface HandleReportPayload {
  action: 'ignore' | 'delete' | 'punish'
  note?: string
  punish?: 'mute' | 'ban'
  days?: number
}

/** 审计日志项 */
export interface AuditLogItem {
  id: string
  admin_id: string
  admin_name: string
  action: string
  action_name: string
  obj_type: string
  obj_name: string
  oid: string
  detail: string
  created_at: string
}

/** 系统配置项 */
export interface SystemConfigItem {
  id: string
  config_key: string
  name: string
  value: string
  remark: string
  created_at: string
  updated_at: string
}

/** 数据字典项 */
export interface DataDictItem {
  id: string
  dict_type: string
  label: string
  value: string
  sort: number
  remark: string
  created_at: string
}

/** Banner 运营位 */
export interface BannerItem {
  id: string
  title: string
  image: string
  bvid: string
  sort: number
  status: number
  created_at: string
  updated_at: string
}

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

export interface SavePermissionPayload {
  code?: string
  name: string
  type: 'menu' | 'button'
  parent?: string
  path?: string
  icon?: string
  sort?: number
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

/** 管理端稿件项（复用 ReviewItem 结构：Card + Description） */
export type AdminVideoItem = ReviewItem

/** 数据大盘（M3-OPS-02） */
export interface DashboardStats {
  today: { dau: number, new_users: number, uploads: number, views: number }
  trend: Array<{ date: string, dau: number, new_users: number, uploads: number, views: number }>
  review_hours: number
  pending_review: number
}

/** 后台管理接口（对应 /api/v1/admin，需管理员令牌）。 */
export function createAdminApi(http: HttpClient) {
  return {
    login: (username: string, password: string) =>
      http.post<AdminLoginResp>('/api/v1/admin/login', { username, password }),

    /** 数据大盘（今日实时 + 近 7 日趋势 + 审核时效） */
    dashboardStats: () => http.get<DashboardStats>('/api/v1/admin/dashboard/stats'),

    /** 稿件管理列表（全状态 + 状态/分区/关键词筛选） */
    videoList: (params: { status?: number, category_id?: number, keyword?: string, page?: number, page_size?: number }) =>
      http.get<{ list: AdminVideoItem[]; total: number }>('/api/v1/admin/videos', params),

    /** 稿件下架/恢复（4 发布 / 6 锁定） */
    updateVideoStatus: (bvid: string, status: number) =>
      http.put<null>(`/api/v1/admin/videos/${bvid}/status`, { status }),

    /** 删除稿件（软删除） */
    deleteVideo: (bvid: string) =>
      http.delete<null>(`/api/v1/admin/videos/${bvid}`),

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

    createPermission: (payload: SavePermissionPayload) =>
      http.post<AdminPermission>('/api/v1/admin/permissions', payload),

    updatePermission: (id: string, payload: SavePermissionPayload) =>
      http.put<null>(`/api/v1/admin/permissions/${id}`, payload),

    deletePermission: (id: string) =>
      http.delete<null>(`/api/v1/admin/permissions/${id}`),

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

    // ---- 举报处理 ----
    /** 举报队列（status -1 全部，默认待处理） */
    reports: (params: { status?: number, page?: number, page_size?: number }) =>
      http.get<{ list: ReportItem[], total: number }>('/api/v1/admin/reports', params),

    /** 处理举报（ignore/delete/punish） */
    handleReport: (id: string, payload: HandleReportPayload) =>
      http.post<null>(`/api/v1/admin/reports/${id}/handle`, payload),

    // ---- 审计日志（M2-SYS-01） ----
    /** 审计日志分页查询（action/obj_type 可为空；from/to 为 YYYY-MM-DD） */
    auditLogs: (params: { admin_id?: string, action?: string, obj_type?: string, from?: string, to?: string, page?: number, page_size?: number }) =>
      http.get<{ list: AuditLogItem[]; total: number }>('/api/v1/admin/audit-logs', params),

    // ---- 系统配置（M2-SYS-02） ----
    configs: () => http.get<{ list: SystemConfigItem[] }>('/api/v1/admin/configs'),

    createConfig: (payload: { config_key: string, name?: string, value?: string, remark?: string }) =>
      http.post<null>('/api/v1/admin/configs', payload),

    updateConfig: (id: string, payload: { config_key?: string, name?: string, value?: string, remark?: string }) =>
      http.put<null>(`/api/v1/admin/configs/${id}`, payload),

    deleteConfig: (id: string) =>
      http.delete<null>(`/api/v1/admin/configs/${id}`),

    // ---- 数据字典（M2-SYS-02） ----
    dicts: () => http.get<{ groups: Record<string, DataDictItem[]> }>('/api/v1/admin/dicts'),

    createDict: (payload: { dict_type: string, label: string, value: string, sort?: number, remark?: string }) =>
      http.post<null>('/api/v1/admin/dicts', payload),

    updateDict: (id: string, payload: { dict_type?: string, label: string, value: string, sort?: number, remark?: string }) =>
      http.put<null>(`/api/v1/admin/dicts/${id}`, payload),

    deleteDict: (id: string) =>
      http.delete<null>(`/api/v1/admin/dicts/${id}`),

    // ---- Banner 运营位（M3-OPS-01） ----
    banners: () => http.get<{ list: BannerItem[] }>('/api/v1/admin/banners'),

    createBanner: (payload: { title?: string, image?: string, bvid?: string, sort?: number, status?: number }) =>
      http.post<null>('/api/v1/admin/banners', payload),

    updateBanner: (id: string, payload: { title?: string, image?: string, bvid?: string, sort?: number, status?: number }) =>
      http.put<null>(`/api/v1/admin/banners/${id}`, payload),

    deleteBanner: (id: string) =>
      http.delete<null>(`/api/v1/admin/banners/${id}`),
  }
}

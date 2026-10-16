import type { HttpClient } from '../http'

// 管理端：直播巡查（M4-LIV-05）。对应 /api/v1/admin/live/*，权限点 live:audit。
// 与 C 端 live.ts 分开：同一域的管理端与观众端字段口径不同（例如房间项带最近截帧判定），
// 混在一个模块里会让「这个字段是给谁看的」失去约束。

/** 最近一次截帧机审判定摘要（无记录时为 null）。 */
export interface AdminLiveAuditBrief {
  id: number
  pass: boolean
  /** true=机审不可用（fail-open 未切断），与「判定通过」必须区分 */
  degraded: boolean
  hit_words: string
  /** 截帧图存储键（**裸文件名**，不是可访问 URL；后端未开放取图接口） */
  image_key: string
  width: number
  height: number
  cost_ms: number
  created_at: string
}

/** 巡查列表房间项。 */
export interface AdminLiveRoom {
  id: number
  /** 字符串避免 snowflake ID 超出 JS 安全整数范围 */
  anchor_id: string
  title: string
  status: number
  /** 1=已被切断/封禁 */
  ban_status: number
  ban_reason: string
  online: number
  started_at?: string
  last_audit: AdminLiveAuditBrief | null
}

/** 切断结果。 */
export interface AdminLiveCutResult {
  kicked: boolean
  /** 1=SRS API 已踢流 / 2=降级（未确认踢流，需复检） */
  cut_mode: number
  violation_id: number
}

/** 违规记录（含操作人用户名）。 */
export interface AdminLiveViolation {
  id: number
  room_id: number
  anchor_id: string
  /** 1=机审 2=人工巡查 */
  source: number
  source_text: string
  reason: string
  cut_mode: number
  cut_mode_text: string
  /** 人工切断的操作人用户名；机审为空 */
  operator: string
  created_at: string
}

/** 截帧机审运行态。 */
export interface AdminLiveAuditStatus {
  enabled: boolean
  frame_interval_seconds: number
  srs_api_base: string
  frame_dir: string
  black_yavg_max: number
  flat_range_max: number
}

/** 直播巡查接口（管理端）。 */
export function createAdminLiveApi(http: HttpClient) {
  return {
    /** 直播中房间巡查列表（含在线人数与最近截帧判定） */
    rooms: () => http.get<{ list: AdminLiveRoom[]; total: number }>('/api/v1/admin/live/rooms'),

    /** 违规记录（人工切断 + 机审命中） */
    violations: (params: { room_id?: number; page?: number; size?: number } = {}) =>
      http.get<{ list: AdminLiveViolation[]; total: number; page: number; size: number }>(
        '/api/v1/admin/live/violations',
        params,
      ),

    /** 手动切断直播（人工巡查；reason 为空则走默认文案） */
    cut: (roomId: number, reason = '') =>
      http.post<AdminLiveCutResult>(`/api/v1/admin/live/rooms/${roomId}/cut`, { reason }),

    /** 恢复直播（解封） */
    restore: (roomId: number) =>
      http.post<{ restored: boolean }>(`/api/v1/admin/live/rooms/${roomId}/restore`, {}),

    /** 截帧机审运行态（机审关闭时后端返回 91027，调用方需按错误处理） */
    auditStatus: () => http.get<AdminLiveAuditStatus>('/api/v1/admin/live/audit/status'),
  }
}

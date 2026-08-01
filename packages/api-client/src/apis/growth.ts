import type { HttpClient } from '../http'

/** 今日任务状态 */
export interface GrowthTask {
  reason: string
  name: string
  delta: number
  current: number
  limit: number
  done: boolean
}

/** 成长总览：等级/经验进度/今日任务 */
export interface GrowthSummary {
  level: number
  exp: number
  next_level: number
  next_exp: number
  progress: number
  tasks: GrowthTask[]
}

/** 经验/硬币流水项 */
export interface AssetLogItem {
  id: string
  delta: number
  reason: string
  reason_name: string
  created_at: string
}

/** 成长接口：经验/等级/权益（对应后端 growth 模块）。 */
export function createGrowthApi(http: HttpClient) {
  return {
    /** 成长总览（等级/经验进度/今日任务） */
    summary: () => http.get<GrowthSummary>('/api/v1/growth/summary'),

    /** 经验流水分页 */
    expLogs: (page = 1, pageSize = 20) =>
      http.get<{ list: AssetLogItem[]; total: number }>('/api/v1/growth/exp-logs', {
        page,
        page_size: pageSize,
      }),

    /** 硬币流水分页 */
    coinLogs: (page = 1, pageSize = 20) =>
      http.get<{ list: AssetLogItem[]; total: number }>('/api/v1/users/me/coin-logs', {
        page,
        page_size: pageSize,
      }),
  }
}

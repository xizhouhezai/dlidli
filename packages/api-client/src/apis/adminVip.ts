import type { HttpClient } from '../http'

// 管理端：会员与套餐（M4-VIP-01）。对应 /api/v1/admin/vip/*。
// 权限点：vip:view（页面）、vip:plan（改价/上下架）、vip:grant（手工补发硬币）。
//
// **金额单位为分**：PlanView 的 price/original_price/first_month_price 均为分。

/** 套餐（管理端视图；含下架项）。 */
export interface AdminVipPlan {
  code: string
  name: string
  months: number
  /** 分 */
  price: number
  /** 分（0=不展示原价） */
  original_price: number
  /** 分（0=无首月优惠） */
  first_month_price: number
  auto_renew: boolean
  sort: number
  /**
   * 上下架状态由列表接口的 list / all 区分，而不是字段本身：
   * list 只含上架项、all 含下架项，故「是否上架」= code 是否出现在 list 里。
   */
}

/** 套餐保存入参（金额单位分）。 */
export interface AdminVipPlanPayload {
  code: string
  name: string
  months: number
  price: number
  original_price: number
  first_month_price: number
  auto_renew: boolean
  sort: number
  /**
   * 1 上架 / 0 下架（对齐 vip.PlanOn / vip.PlanOff）。
   * **不是 1/2**：后端 AdminUpsertPlan 对非法值会静默改为下架，写错不会报错，
   * 只会表现为「点了上架却仍是下架」，故此处与常量逐字对齐。
   */
  status: number
}

/** 会员项。 */
export interface AdminVipMember {
  user_id: string
  level: number
  expire_at: string
  is_vip: boolean
  auto_renew: boolean
  total_months: number
  started_at: string
}

/** 会员与套餐接口（管理端）。 */
export function createAdminVipApi(http: HttpClient) {
  return {
    /**
     * 套餐列表（管理端，含下架项）。
     * `list` 只含上架项、`all` 含全部 —— 页面用两者之差判断「哪些是下架的」。
     */
    plans: () => http.get<{ list: AdminVipPlan[]; all: AdminVipPlan[] }>('/api/v1/admin/vip/plans'),

    /** 套餐配置（新建或改价/上下架） */
    upsertPlan: (payload: AdminVipPlanPayload) =>
      http.post<{ ok: boolean }>('/api/v1/admin/vip/plans', payload),

    /** 会员查询（only_vip=1 只看生效中） */
    members: (params: { user_id?: number; only_vip?: 1; page?: number; size?: number }) =>
      http.get<{ list: AdminVipMember[]; total: number; page: number; size: number }>(
        '/api/v1/admin/vip/members',
        params,
      ),

    /**
     * 手工补发本月会员硬币（幂等：同一用户同一月只会发放一次，可放心重试）。
     */
    grantCoins: (uid: number | string) =>
      http.post<{ granted: boolean }>(`/api/v1/admin/vip/members/${uid}/grant-coins`, {}),
  }
}

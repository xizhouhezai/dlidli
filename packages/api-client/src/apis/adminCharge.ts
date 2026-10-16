import type { HttpClient } from '../http'

// 管理端：充电、结算与提现审核（M4-PAY-02）。对应 /api/v1/admin/charge/*。
// 权限点：charge:view（页面）、charge:audit（审核）、charge:payout（打款）、charge:settle（月结）。
//
// **「查询」与「动钱」是独立权限点**：能看记录不等于能动款。页面上的审核/打款按钮
// 必须各自挂 v-perm，且后端逐个路由挂 RequirePerm —— 前端隐藏不构成拦截。
//
// **金额单位为分**（amount / fee / platform_amount 等全部为分）。

/** 充电流水项。 */
export interface AdminChargeItem {
  charge_no: string
  order_no: string
  user_id: string
  anchor_id: string
  video_id?: string
  /** 分 */
  amount: number
  anonymous: boolean
  message: string
  nickname: string
  paid_at?: string
  created_at: string
}

/** 结算单项。 */
export interface AdminSettlementItem {
  /** YYYY-MM */
  settle_month: string
  user_id: string
  charge_count: number
  /** 分（创作者合计） */
  amount: number
  /** 分（平台合计） */
  platform_amount: number
  created_at: string
}

/** 提现单。 */
export interface AdminWithdrawItem {
  withdraw_no: string
  user_id: string
  /** 分 */
  amount: number
  /** 分 */
  fee: number
  channel: string
  account: string
  account_name: string
  /** 0 审核中 / 2 已通过 / 3 已拒绝 / 4 已打款 / 5 打款失败 */
  status: number
  status_text: string
  audit_note?: string
  channel_trade_no?: string
  fail_reason?: string
  created_at: string
  audit_at?: string
  paid_at?: string
}

/** 月结结果。 */
export interface AdminSettleResult {
  month: string
  users: number
  charge_count: number
  /** 分 */
  amount: number
  /** 分 */
  platform_amount: number
}

/** 充电、结算与提现接口（管理端）。 */
export function createAdminChargeApi(http: HttpClient) {
  return {
    /** 充电流水查询 */
    charges: (params: {
      user_id?: number
      anchor_id?: number
      video_id?: number
      page?: number
      size?: number
    }) =>
      http.get<{ list: AdminChargeItem[]; total: number; page: number; size: number }>(
        '/api/v1/admin/charge/charges',
        params,
      ),

    /** 结算单查询 */
    settlements: (params: { user_id?: number; month?: string; page?: number; size?: number }) =>
      http.get<{ list: AdminSettlementItem[]; total: number; page: number; size: number }>(
        '/api/v1/admin/charge/settlements',
        params,
      ),

    /** 手动触发月度结算（与月度 Worker 同一实现，幂等可重跑） */
    settleRun: (month = '') =>
      http.post<AdminSettleResult>('/api/v1/admin/charge/settlements/run', { month }),

    /** 提现审核台列表（status 缺省为全部） */
    withdraws: (params: { user_id?: number; status?: number; page?: number; size?: number }) =>
      http.get<{ list: AdminWithdrawItem[]; total: number; page: number; size: number }>(
        '/api/v1/admin/charge/withdraws',
        params,
      ),

    /** 提现审核（approve=false 驳回并解冻返还；note 为审核备注） */
    auditWithdraw: (withdrawNo: string, approve: boolean, note = '') =>
      http.post<AdminWithdrawItem>(`/api/v1/admin/charge/withdraws/${withdrawNo}/audit`, {
        approve,
        note,
      }),

    /** 提现打款（mock 通道；fail=true 模拟打款失败并解冻返还） */
    payout: (withdrawNo: string, fail = false, remark = '') =>
      http.post<AdminWithdrawItem>(`/api/v1/admin/charge/withdraws/${withdrawNo}/payout`, {
        fail,
        remark,
      }),
  }
}

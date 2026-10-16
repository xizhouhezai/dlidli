import type { HttpClient } from '../http'

// 管理端：支付订单与对账（M4-PAY-01）。对应 /api/v1/admin/pay/*。
// 权限点：pay:view（页面）、pay:order（补单）、pay:recon（对账与异常处理）。
//
// **金额单位一律为分**：后端 OrderView/DailyView/IssueView 都按分返回，
// 展示与输入的元/分换算由 apps/admin 的 utils/money.ts 负责，本层不做换算。

/** 订单（管理端视图）。 */
export interface AdminPayOrder {
  order_no: string
  user_id: string
  /** 分 */
  amount: number
  channel: string
  subject: string
  /** 0 待支付 / 1 已支付 / 2 已关闭 / 3 已退款 */
  status: number
  status_text: string
  channel_trade_no?: string
  refund_amount: number
  expire_at: string
  paid_at?: string
  closed_at?: string
  refunded_at?: string
  created_at: string
}

/** 日对账单。 */
export interface AdminPayReconDaily {
  recon_date: string
  channel: string
  order_count: number
  paid_count: number
  /** 分 */
  paid_amount: number
  refund_count: number
  /** 分 */
  refund_amount: number
  /** 分（现金流水净额，退款取负后相加） */
  cash_log_amount: number
  diff_count: number
  /** 0 正常 / 1 有异常 */
  status: number
  status_text: string
  updated_at: string
}

/** 异常项。 */
export interface AdminPayReconIssue {
  id: number
  recon_date: string
  channel: string
  order_no: string
  /** 1 金额不符 / 2 长期挂起 / 3 流水缺失 / 4 重复回调 */
  issue_type: number
  type_text: string
  detail: string
  /** 0 待处理 / 1 已处理 */
  status: number
  handled_by?: string
  handled_at?: string
  handle_note?: string
  created_at: string
}

/** 对账执行结果（重跑幂等）。 */
export interface AdminPayReconResult {
  recon_date: string
  channels: AdminPayReconDaily[]
  issues: AdminPayReconIssue[]
  issue_num: number
}

/** 补单结果。 */
export interface AdminPayCallbackOutcome {
  order_no: string
  /** true=本次真正生效；false=幂等（此前已生效） */
  applied: boolean
  status: number
  message: string
}

/** 订单筛选参数（金额单位分；日期 YYYY-MM-DD，含当日）。 */
export interface AdminPayOrderQuery extends Record<string, string | number | boolean | undefined> {
  order_no?: string
  user_id?: number
  channel?: string
  status?: number
  min_amount?: number
  max_amount?: number
  start_date?: string
  end_date?: string
  page?: number
  size?: number
}

/** 支付订单与对账接口（管理端）。 */
export function createAdminPayApi(http: HttpClient) {
  return {
    /** 订单查询 */
    orders: (params: AdminPayOrderQuery) =>
      http.get<{ list: AdminPayOrder[]; total: number; page: number; size: number }>(
        '/api/v1/admin/pay/orders',
        params,
      ),

    /** 手动补单（人工确认收款后入账） */
    markPaid: (orderNo: string) =>
      http.post<AdminPayCallbackOutcome>(`/api/v1/admin/pay/orders/${orderNo}/mark-paid`, {}),

    /** 手动触发超时关单 */
    closeExpired: () => http.post<{ closed: number }>('/api/v1/admin/pay/close-expired', {}),

    /** 日对账单 */
    reconDaily: (params: { date?: string; channel?: string; page?: number; size?: number }) =>
      http.get<{ list: AdminPayReconDaily[]; total: number; page: number; size: number }>(
        '/api/v1/admin/pay/recon/daily',
        params,
      ),

    /** 执行对账（重跑幂等）。后端从 query 读 date/channel，故拼在路径上。 */
    reconRun: (params: { date: string; channel?: string }) => {
      const qs = new URLSearchParams({ date: params.date })
      if (params.channel) qs.append('channel', params.channel)
      return http.post<AdminPayReconResult>(`/api/v1/admin/pay/recon/run?${qs.toString()}`, {})
    },

    /** 异常订单清单（人工处理台） */
    reconIssues: (params: {
      date?: string
      channel?: string
      status?: number
      issue_type?: number
      page?: number
      size?: number
    }) =>
      http.get<{ list: AdminPayReconIssue[]; total: number; page: number; size: number }>(
        '/api/v1/admin/pay/recon/issues',
        params,
      ),

    /** 异常处理（备注 + 处理人由后端从令牌取） */
    handleIssue: (id: number, note = '') =>
      http.post<AdminPayReconIssue>(`/api/v1/admin/pay/recon/issues/${id}/handle`, { note }),
  }
}

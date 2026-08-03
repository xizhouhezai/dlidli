import type { HttpClient } from '../http'

/** 举报对象类型：1视频 2评论 3弹幕 4动态 5用户 */
export type ReportTargetType = 1 | 2 | 3 | 4 | 5

/** 举报类型：1违法违规 2色情低俗 3人身攻击 4垃圾广告 5剧透 6其他 */
export type ReportReasonType = 1 | 2 | 3 | 4 | 5 | 6

export interface SubmitReportReq {
  target_type: ReportTargetType
  /** 视频传 bvid，其余传对象 ID（字符串） */
  target_id: string
  reason_type: ReportReasonType
  reason?: string
}

/** 举报接口（对应后端 report 模块）。 */
export function createReportApi(http: HttpClient) {
  return {
    /** 提交举报 */
    submit: (req: SubmitReportReq) => http.post<null>('/api/v1/reports', req),
  }
}

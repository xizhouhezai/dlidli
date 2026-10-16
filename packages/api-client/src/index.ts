import { HttpClient, type ClientConfig } from './http'
import { createSystemApi } from './apis/system'
import { createAuthApi } from './apis/auth'
import { createUploadApi } from './apis/upload'
import { createVideoApi } from './apis/video'
import { createDanmakuApi } from './apis/danmaku'
import { createLiveApi } from './apis/live'
import { createInteractionApi } from './apis/interaction'
import { createAdminApi } from './apis/admin'
import { createAdminLiveApi } from './apis/adminLive'
import { createAdminPayApi } from './apis/adminPay'
import { createAdminVipApi } from './apis/adminVip'
import { createAdminChargeApi } from './apis/adminCharge'
import { createRelationApi } from './apis/relation'
import { createDynamicApi } from './apis/dynamic'
import { createSearchApi } from './apis/search'
import { createNotifyApi } from './apis/notify'
import { createGrowthApi } from './apis/growth'
import { createReportApi } from './apis/report'
import { createRecommendApi } from './apis/recommend'
import { createCreatorApi } from './apis/creator'
import { createCollectionApi } from './apis/collection'
import { createMessageApi } from './apis/messages'

export * from './http'
export * from './error'
export * from './token'
export type { TokenPair } from './apis/auth'
export type { UploadInitResp } from './apis/upload'
export type {
  CategoryItem,
  VideoCard,
  VideoDetail,
  StreamItem,
  SubmitVideoReq,
  PartItem,
} from './apis/video'
export type { DanmakuItem, SendDanmakuReq, DanmakuBlockItem } from './apis/danmaku'
export type {
  LiveRoomInfo,
  LivePlayUrls,
  LiveApplyResult,
  LiveRoomListResult,
  LiveMyRoom,
  LiveDanmakuItem,
  LiveDanmakuHistory,
  LiveAdminItem,
  LiveMuteItem,
  LiveWsFrame,
  LiveGiftItem,
  LiveMelonBalance,
  LiveSendGiftResult,
} from './apis/live'
export type {
  CommentItem,
  CommentUser,
  AddCommentReq,
  InteractionState,
  TripleResult,
  CollectionItem,
} from './apis/interaction'
export type {
  AdminLoginResp,
  ReviewItem,
  SensitiveWord,
  AdminUserItem,
  PunishAction,
  AdminPermission,
  AdminMenuItem,
  CurrentPerm,
  AdminRole,
  AdminAccount,
  SaveRolePayload,
  SaveAdminPayload,
  AdminCategory,
  SaveCategoryPayload,
  SavePermissionPayload,
  ReportItem,
  HandleReportPayload,
  AuditLogItem,
  SystemConfigItem,
  DataDictItem,
  BannerItem,
  DashboardStats,
  AdminVideoItem,
  ExperimentItem,
} from './apis/admin'
export type { UserBrief, RelationStat } from './apis/relation'
export type { FeedItem } from './apis/dynamic'
export type { NotifyItem, NotifyWsFrame } from './apis/notify'
export type { GrowthSummary, GrowthTask, AssetLogItem } from './apis/growth'
export type { SubmitReportReq, ReportTargetType, ReportReasonType } from './apis/report'
export type { CreatorOverview, CreatorVideoStat, TrendPoint, SettlementItem } from './apis/creator'
export type { CollectionCard, CollectionDetail } from './apis/collection'
export type {
  AdminLiveRoom,
  AdminLiveAuditBrief,
  AdminLiveCutResult,
  AdminLiveViolation,
  AdminLiveAuditStatus,
} from './apis/adminLive'
export type {
  AdminPayOrder,
  AdminPayReconDaily,
  AdminPayReconIssue,
  AdminPayReconResult,
  AdminPayCallbackOutcome,
  AdminPayOrderQuery,
} from './apis/adminPay'
export type { AdminVipPlan, AdminVipPlanPayload, AdminVipMember } from './apis/adminVip'
export type {
  AdminChargeItem,
  AdminSettlementItem,
  AdminWithdrawItem,
  AdminSettleResult,
} from './apis/adminCharge'
export type { ConversationItem, MessageItem } from './apis/messages'

/**
 * 创建 API 客户端。业务模块接口（account/video/...）随 M1 后端接口逐步补充，
 * 后续接入 openapi-typescript 自动生成类型。
 */
export function createApiClient(cfg: ClientConfig = {}) {
  const http = new HttpClient(cfg)
  return {
    http,
    system: createSystemApi(http),
    auth: createAuthApi(http),
    upload: createUploadApi(http),
    video: createVideoApi(http),
    danmaku: createDanmakuApi(http),
    live: createLiveApi(http),
    interaction: createInteractionApi(http),
    admin: createAdminApi(http),
    /** 管理端：直播巡查（/admin/live），权限点 live:audit */
    adminLive: createAdminLiveApi(http),
    /** 管理端：支付订单与对账（/admin/pay），权限点 pay:view/order/recon */
    adminPay: createAdminPayApi(http),
    /** 管理端：会员与套餐（/admin/vip），权限点 vip:view/plan/grant */
    adminVip: createAdminVipApi(http),
    /** 管理端：充电结算与提现（/admin/charge），权限点 charge:view/audit/payout/settle */
    adminCharge: createAdminChargeApi(http),
    relation: createRelationApi(http),
    dynamic: createDynamicApi(http),
    search: createSearchApi(http),
    notify: createNotifyApi(http),
    growth: createGrowthApi(http),
    report: createReportApi(http),
    recommend: createRecommendApi(http),
    creator: createCreatorApi(http),
    collection: createCollectionApi(http),
    message: createMessageApi(http),
  }
}

export type ApiClient = ReturnType<typeof createApiClient>

// 与后端约定的枚举/常量（对齐 docs/architecture/data-model.md）

/** 稿件状态 */
export const VideoStatus = {
  Draft: 0,
  Uploading: 1,
  Transcoding: 2,
  Reviewing: 3,
  Published: 4,
  Rejected: 5,
  Locked: 6,
  Deleted: 7,
} as const
export type VideoStatus = (typeof VideoStatus)[keyof typeof VideoStatus]

/** 弹幕模式 */
export const DanmakuMode = {
  Scroll: 1,
  Top: 2,
  Bottom: 3,
} as const
export type DanmakuMode = (typeof DanmakuMode)[keyof typeof DanmakuMode]

/** 清晰度档位 */
export const QUALITIES = [360, 720, 1080, 2160] as const

/** 通用错误码（与 server/internal/pkg/errcode 对齐） */
export const ErrCode = {
  OK: 0,
  Internal: 10001,
  InvalidParams: 10002,
  Unauthorized: 10003,
  Forbidden: 10004,
  NotFound: 10005,
  TooManyRequests: 10006,
} as const


/**
 * 视频域端侧模型（字段名以 server 的 OpenAPI/实现为准，见 plan §4）。
 * Card 对应 server/internal/module/video/model.go 的 Card（列表卡片），详情模型随 M4-HMY-06 补齐。
 */
export interface Category {
  id: number;
  parent_id: number;
  name: string;
  sort: number;
  status: number;
}

export interface OwnerBrief {
  id: string;
  nickname: string;
  avatar: string;
}

export interface StatBrief {
  view: number;
  like: number;
  coin: number;
  fav: number;
  danmaku: number;
  comment: number;
  share: number;
}

export interface VideoCard {
  /** 内部 ID（字符串化防精度丢失）；负反馈等场景用 */
  id: string;
  bvid: string;
  title: string;
  cover: string;
  /** 秒 */
  duration: number;
  status: number;
  risk_level: number;
  category_id: number;
  published_at?: string;
  created_at: string;
  owner: OwnerBrief;
  stat: StatBrief;
}

/** /videos、/recommend/videos 的响应包裹：**不返回 total**，满页即视为还有下一页 */
export interface VideoListResult {
  list: VideoCard[];
}

/** /search 的响应包裹（视频与 UP 主共用形态） */
export interface PagedResult<T> {
  list: T[];
  total: number;
}

/** 播放流（对齐 server StreamItem）。quality 为画质档位（0=原画），url 是带签名的播放地址 */
export interface StreamItem {
  quality: number;
  /** hls | mp4 */
  format: string;
  url: string;
}

/**
 * 稿件详情（对齐 server Detail：内嵌 Card 的字段在 JSON 里是**平铺**的，故此处用接口继承表达）。
 * 注意 `streams` 在无可用流时服务端返回 `null`（非空数组），消费方须按可空处理。
 */
export interface VideoDetail extends VideoCard {
  description: string;
  tags: string[];
  /** 1 自制 2 转载 */
  copyright: number;
  reject_reason?: string;
  streams?: StreamItem[];
}

/** 分P（对齐 server PartItem），单P稿件返回空数组 */
export interface VideoPart {
  index: number;
  title: string;
  duration: number;
  streams?: StreamItem[];
}

/** /videos/{bvid}/parts 的响应包裹 */
export interface PartListResult {
  list: VideoPart[];
}

/** /videos/{bvid}/progress 的响应包裹；无记录时 position 为 0 */
export interface ProgressResult {
  position: number;
}

/** /videos/history 的响应包裹（该接口不返回 total） */
export interface HistoryResult {
  list: VideoCard[];
}

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

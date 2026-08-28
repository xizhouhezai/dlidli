import type { HttpClient } from '../http'

/** 创作者总览 */
export interface CreatorOverview {
  video_cnt: number
  total_view: number
  total_like: number
  total_coin: number
  total_fav: number
  fans: number
  week_view: number
  /** 累计收益（分） */
  earnings: number
}

/** 单稿数据 */
export interface CreatorVideoStat {
  bvid: string
  title: string
  cover: string
  status: number
  view: number
  like: number
  coin: number
  fav: number
  comment: number
  danmaku: number
  valid_views: number
  earnings: number
  published_at: string | null
}

/** 播放趋势点 */
export interface TrendPoint {
  date: string
  views: number
}

/** 收益明细项 */
export interface SettlementItem {
  date: string
  bvid: string
  title: string
  valid_views: number
  amount: number
}

/** 趋势指标：play 有效播放 / like 点赞 / coin 投币 / fav 收藏 / fans 涨粉 / earning 收益 / interact 互动 / click 点击 / expose 曝光 */
export type TrendMetric =
  'play' | 'like' | 'coin' | 'fav' | 'fans' | 'earning' | 'interact' | 'click' | 'expose'

/** 创作者中心接口（对应后端 creator 模块）。 */
export function createCreatorApi(http: HttpClient) {
  return {
    /** 总览（播放/互动/粉丝/收益/近7日播放） */
    overview: () => http.get<CreatorOverview>('/api/v1/creator/overview'),

    /** 稿件数据列表 */
    videos: (page = 1, pageSize = 10) =>
      http.get<{ list: CreatorVideoStat[]; total: number }>('/api/v1/creator/videos', {
        page,
        page_size: pageSize,
      }),

    /** 近 N 天数据趋势（指标切换） */
    trend: (days = 7, metric: TrendMetric = 'play') =>
      http.get<{ list: TrendPoint[]; metric: string }>('/api/v1/creator/trend', { days, metric }),

    /** 收益明细分页 */
    settlements: (page = 1, pageSize = 10) =>
      http.get<{ list: SettlementItem[]; total: number }>('/api/v1/creator/settlements', {
        page,
        page_size: pageSize,
      }),
  }
}

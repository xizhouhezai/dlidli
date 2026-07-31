import type { HttpClient } from '../http'
import type { VideoCard } from './video'
import type { UserBrief } from './relation'

/** 动态条目：type 1投稿动态 2图文动态 3转发视频 */
export interface FeedItem {
  id: string
  type: 1 | 2 | 3
  content: string
  user: UserBrief
  video?: VideoCard
  created_at: string
}

/** 动态接口（图文发布 + 关注流）。 */
export function createDynamicApi(http: HttpClient) {
  return {
    post: (content: string) => http.post<FeedItem>('/api/v1/dynamics', { content }),

    /** 转发视频到动态（content 为转发语，可空） */
    shareVideo: (bvid: string, content = '') =>
      http.post<FeedItem>('/api/v1/dynamics/share', { bvid, content }),

    feed: (cursor = '', pageSize = 20) =>
      http.get<{ list: FeedItem[]; next_cursor: string; has_more: boolean }>('/api/v1/feed', {
        cursor: cursor || undefined,
        page_size: pageSize,
      }),
  }
}

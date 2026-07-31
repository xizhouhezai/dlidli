import type { HttpClient } from '../http'
import type { VideoCard } from './video'
import type { UserBrief } from './relation'

/** 搜索接口（MVP：MySQL LIKE；后续切 ES）。 */
export function createSearchApi(http: HttpClient) {
  return {
    videos: (keyword: string, page = 1, pageSize = 20) =>
      http.get<{ list: VideoCard[]; total: number }>('/api/v1/search', {
        keyword,
        type: 'video',
        page,
        page_size: pageSize,
      }),

    users: (keyword: string, page = 1, pageSize = 20) =>
      http.get<{ list: UserBrief[]; total: number }>('/api/v1/search', {
        keyword,
        type: 'user',
        page,
        page_size: pageSize,
      }),
  }
}

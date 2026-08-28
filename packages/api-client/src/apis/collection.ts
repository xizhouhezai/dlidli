import type { HttpClient } from '../http'
import type { VideoCard } from './video'

/** 合集卡片 */
export interface CollectionCard {
  id: string
  title: string
  description: string
  cover: string
  video_count: number
  created_at: string
}

/** 合集详情 */
export interface CollectionDetail {
  id: string
  user_id: string
  title: string
  description: string
  cover: string
  created_at: string
}

/** UP 主合集接口（对应后端 collection 模块）。 */
export function createCollectionApi(http: HttpClient) {
  return {
    /** 某 UP 主的合集列表 */
    list: (uid: string) => http.get<{ list: CollectionCard[] }>('/api/v1/collections', { uid }),

    /** 合集详情（含稿件列表） */
    detail: (id: string) =>
      http.get<{ collection: CollectionDetail; list: VideoCard[] }>(`/api/v1/collections/${id}`),

    /** 创建合集 */
    create: (payload: { title: string; description?: string; cover?: string }) =>
      http.post<null>('/api/v1/collections', payload),

    /** 合集添加稿件（仅本人已发布） */
    addVideo: (id: string, bvid: string) =>
      http.post<null>(`/api/v1/collections/${id}/videos`, { bvid }),

    /** 合集移除稿件 */
    removeVideo: (id: string, bvid: string) =>
      http.delete<null>(`/api/v1/collections/${id}/videos/${bvid}`),

    /** 删除合集 */
    remove: (id: string) => http.delete<null>(`/api/v1/collections/${id}`),
  }
}

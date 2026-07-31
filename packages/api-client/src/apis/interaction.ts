import type { HttpClient } from '../http'
import type { VideoCard } from './video'

export interface CommentUser {
  id: string
  nickname: string
  avatar: string
  level: number
}

export interface CommentItem {
  id: string
  content: string
  user: CommentUser
  like_cnt: number
  reply_cnt: number
  is_top: boolean
  is_self: boolean
  created_at: string
  replies?: CommentItem[]
}

export interface AddCommentReq {
  content: string
  root_id?: string
  parent_id?: string
}

/** 播放页互动状态聚合 */
export interface InteractionState {
  liked: boolean
  coin_count: number
  faved: boolean
}

/** 一键三连结果（delta 供前端本地修正计数） */
export interface TripleResult extends InteractionState {
  like_delta: number
  coin_delta: number
  fav_delta: number
}

/** 收藏夹 */
export interface CollectionItem {
  id: string
  name: string
  is_default: 0 | 1
  created_at: string
}

/** 互动接口：评论 + 点赞 + 投币/收藏/三连（对应后端 interaction 模块）。 */
export function createInteractionApi(http: HttpClient) {
  return {
    comments: (bvid: string, params?: { sort?: 'hot' | 'new'; page?: number; page_size?: number }) =>
      http.get<{ list: CommentItem[]; total: number }>(`/api/v1/videos/${bvid}/comments`, params),

    addComment: (bvid: string, req: AddCommentReq) =>
      http.post<CommentItem>(`/api/v1/videos/${bvid}/comments`, req),

    replies: (commentId: string, page = 1, pageSize = 10) =>
      http.get<{ list: CommentItem[]; total: number }>(`/api/v1/comments/${commentId}/replies`, {
        page,
        page_size: pageSize,
      }),

    deleteComment: (commentId: string) => http.delete<null>(`/api/v1/comments/${commentId}`),

    likeComment: (commentId: string) =>
      http.post<{ liked: boolean }>(`/api/v1/comments/${commentId}/like`),

    likeVideo: (bvid: string) => http.post<{ liked: boolean }>(`/api/v1/videos/${bvid}/like`),

    videoLiked: (bvid: string) => http.get<{ liked: boolean }>(`/api/v1/videos/${bvid}/like`),

    /** 互动状态聚合（赞/币/藏） */
    interactionState: (bvid: string) =>
      http.get<InteractionState>(`/api/v1/videos/${bvid}/interaction`),

    /** 投币（自制最多 2 枚，转载 1 枚） */
    coinVideo: (bvid: string, count: 1 | 2) =>
      http.post<null>(`/api/v1/videos/${bvid}/coin`, { count }),

    /** 收藏开关（可指定收藏夹；ID 字符串传输避免精度丢失） */
    toggleFavorite: (bvid: string, collectionId?: string) =>
      http.post<{ faved: boolean }>(`/api/v1/videos/${bvid}/favorite`, { collection_id: collectionId || '0' }),

    /** 收藏夹列表 */
    listCollections: () => http.get<CollectionItem[]>('/api/v1/users/me/collections'),

    /** 新建收藏夹 */
    createCollection: (name: string) =>
      http.post<CollectionItem>('/api/v1/users/me/collections', { name }),

    /** 重命名收藏夹 */
    renameCollection: (id: string, name: string) =>
      http.put<null>(`/api/v1/users/me/collections/${id}`, { name }),

    /** 删除收藏夹 */
    deleteCollection: (id: string) =>
      http.delete<null>(`/api/v1/users/me/collections/${id}`),

    /** 一键三连 */
    triple: (bvid: string) => http.post<TripleResult>(`/api/v1/videos/${bvid}/triple`),

    /** 我的收藏列表 */
    favorites: (page = 1, pageSize = 20) =>
      http.get<{ list: VideoCard[]; total: number }>('/api/v1/users/me/favorites', {
        page,
        page_size: pageSize,
      }),
  }
}

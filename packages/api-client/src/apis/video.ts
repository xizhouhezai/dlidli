import type { HttpClient } from '../http'

export interface CategoryItem {
  id: number
  parent_id: number
  name: string
}

export interface OwnerBrief {
  id: string
  nickname: string
  avatar: string
}

export interface VideoStatBrief {
  view: number
  like: number
  coin: number
  fav: number
  danmaku: number
  comment: number
  share: number
}

export interface VideoCard {
  /** 内部 ID（字符串化防精度丢失；负反馈等场景用） */
  id: string
  bvid: string
  title: string
  cover: string
  duration: number
  status: number
  published_at: string | null
  created_at: string
  owner: OwnerBrief
  stat: VideoStatBrief
}

export interface StreamItem {
  quality: number // 0=原画
  format: string
  url: string
}

export interface VideoDetail extends VideoCard {
  description: string
  category_id: number
  tags: string[]
  copyright: 1 | 2
  reject_reason?: string
  streams: StreamItem[]
}

export interface SubmitVideoReq {
  file_id: string
  title: string
  description: string
  category_id: number
  tags: string[]
  copyright: 1 | 2
  cover?: string
  /** 多P投稿（PRD VID-05）：每项为分P（file_id + 标题）；空则单P */
  parts?: Array<{ file_id: string; title?: string }>
}

/** 分P播放项 */
export interface PartItem {
  index: number
  title: string
  duration: number
  streams: StreamItem[]
}

/** 首页轮播 Banner（公开运营位） */
export interface BannerItem {
  id: string
  title: string
  image: string
  bvid: string
}

/** 稿件接口（对应后端 /api/v1/videos）。 */
export function createVideoApi(http: HttpClient) {
  return {
    categories: () => http.get<CategoryItem[]>('/api/v1/categories'),

    list: (params?: {
      category_id?: number
      uid?: string
      sort?: 'new' | 'hot'
      page?: number
      page_size?: number
    }) => http.get<{ list: VideoCard[] }>('/api/v1/videos', params),

    detail: (bvid: string) => http.get<VideoDetail>(`/api/v1/videos/${bvid}`),

    /** 有效播放上报（播放 >5s 触发一次） */
    addView: (bvid: string) => http.post<null>(`/api/v1/videos/${bvid}/view`),

    /** 观看进度（登录用户跨端续播） */
    getProgress: (bvid: string) =>
      http.get<{ position: number }>(`/api/v1/videos/${bvid}/progress`),
    saveProgress: (bvid: string, position: number) =>
      http.post<null>(`/api/v1/videos/${bvid}/progress`, { position }),

    submit: (req: SubmitVideoReq) => http.post<VideoDetail>('/api/v1/videos', req),

    /** 稿件分P列表（含各 P 播放流；单P返回空） */
    parts: (bvid: string) => http.get<{ list: PartItem[] }>(`/api/v1/videos/${bvid}/parts`),

    /** 上传封面图（投稿前调用）；Blob 需指定带扩展名的 filename */
    uploadCover: (file: File | Blob, filename?: string) => {
      const form = new FormData()
      if (file instanceof File) form.append('file', file)
      else form.append('file', file, filename ?? 'cover.jpg')
      return http.postForm<{ cover: string }>('/api/v1/videos/cover', form)
    },

    mine: (page = 1, pageSize = 20) =>
      http.get<{ list: VideoCard[]; total: number }>('/api/v1/videos/mine', {
        page,
        page_size: pageSize,
      }),

    remove: (bvid: string) => http.delete<null>(`/api/v1/videos/${bvid}`),

    /** 首页轮播 Banner（运营位配置，空列表时前端回退最热） */
    banners: () => http.get<{ list: BannerItem[] }>('/api/v1/banners'),
  }
}

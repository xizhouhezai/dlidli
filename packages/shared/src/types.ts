// 与后端统一响应结构对齐（server/internal/pkg/response）
export interface ApiBody<T = unknown> {
  code: number
  message: string
  data: T
  trace_id: string
}

export interface HealthData {
  app: string
  env: string
  components: Record<string, 'up' | 'down'>
}

export interface User {
  id: string
  nickname: string
  avatar: string
  signature: string
  gender: 0 | 1 | 2
  level: number
  coin: number
}

export interface Category {
  id: number
  parentId: number
  name: string
}

export interface VideoStat {
  viewCnt: number
  likeCnt: number
  coinCnt: number
  favCnt: number
  danmakuCnt: number
  commentCnt: number
  shareCnt: number
}

export interface Video {
  id: string
  bvid: string
  title: string
  cover: string
  description: string
  categoryId: number
  duration: number
  publishedAt: string
  owner: Pick<User, 'id' | 'nickname' | 'avatar'>
  stat: VideoStat
}

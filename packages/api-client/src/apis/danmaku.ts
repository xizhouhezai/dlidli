import type { HttpClient } from '../http'

export interface DanmakuItem {
  id: string
  content: string
  time_ms: number
  mode: 1 | 2 | 3 // 1滚动 2顶部 3底部
  color: number
  font_size: number
  is_self?: boolean
}

export interface SendDanmakuReq {
  content: string
  time_ms: number
  mode?: 1 | 2 | 3
  color?: number
  font_size?: number
}

/** 弹幕接口（对应后端 /api/v1/videos/:bvid/danmaku）。 */
export function createDanmakuApi(http: HttpClient) {
  return {
    /** 按段拉取（segment_ms = 6 分钟） */
    list: (bvid: string, segment: number) =>
      http.get<{ segment: number; segment_ms: number; list: DanmakuItem[] }>(
        `/api/v1/videos/${bvid}/danmaku`,
        { segment },
      ),

    send: (bvid: string, req: SendDanmakuReq) =>
      http.post<DanmakuItem>(`/api/v1/videos/${bvid}/danmaku`, req),
  }
}

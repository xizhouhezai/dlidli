import type { HttpClient } from '../http'

export interface DanmakuItem {
  id: string
  content: string
  time_ms: number
  mode: 1 | 2 | 3 // 1滚动 2顶部 3底部
  color: number
  font_size: number
  /** 发送者 UID 哈希（供屏蔽用户，不暴露真实 UID） */
  sender_hash?: string
  is_self?: boolean
}

export interface SendDanmakuReq {
  content: string
  time_ms: number
  mode?: 1 | 2 | 3
  color?: number
  font_size?: number
}

/** 弹幕屏蔽项 */
export interface DanmakuBlockItem {
  id: string
  block_type: 1 | 2 // 1关键词 2用户
  keyword?: string
  block_hash?: string
  created_at: string
}

/** 弹幕接口（对应后端 /api/v1/videos/:bvid/danmaku）。 */
export function createDanmakuApi(http: HttpClient) {
  const base = (bvid: string) => `/api/v1/videos/${bvid}/danmaku`
  return {
    /** 按段拉取（segment_ms = 6 分钟；登录用户服务端过滤屏蔽项） */
    list: (bvid: string, segment: number) =>
      http.get<{ segment: number; segment_ms: number; list: DanmakuItem[] }>(base(bvid), {
        segment,
      }),

    /** 全量列表（列表面板，分页） */
    listAll: (bvid: string, page = 1, pageSize = 50) =>
      http.get<{ list: DanmakuItem[]; total: number }>(`${base(bvid)}/list`, {
        page,
        page_size: pageSize,
      }),

    send: (bvid: string, req: SendDanmakuReq) =>
      http.post<DanmakuItem>(base(bvid), req),

    /** 实时弹幕 WS 地址（http → ws 协议换算由调用方处理） */
    wsUrl: (bvid: string) => `${base(bvid)}/ws`,

    // ---- 屏蔽设置 ----
    blocks: () => http.get<{ list: DanmakuBlockItem[] }>(`${base('_')}/blocks`),

    /** 新增屏蔽：type=1 传 keyword；type=2 传 target_uid 或 block_hash（前端无真实 UID 时） */
    addBlock: (req: { block_type: 1 | 2; keyword?: string; target_uid?: string; block_hash?: string }) =>
      http.post<null>(`${base('_')}/blocks`, req),

    /** 删除屏蔽项 */
    deleteBlock: (id: string) =>
      http.delete<null>(`${base('_')}/blocks/${id}`),
  }
}

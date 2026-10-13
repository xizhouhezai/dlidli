import type { HttpClient } from '../http'

/** 房间信息（密钥哈希永不出参） */
export interface LiveRoomInfo {
  id: number
  anchor_id: string // 字符串避免 JS 精度丢失
  title: string
  cover: string
  category_id: number
  status: 0 | 1 | 2 // 0 空闲 1 直播中 2 已下播
  started_at?: string
  ended_at?: string
  online: number
}

/** 申请/重置密钥返回：房间 + 推流地址 + 明文密钥（仅此一次） */
export interface LiveApplyResult {
  room: LiveRoomInfo | null
  push_url: string
  stream_key?: string
}

/** 直播中房间列表 */
export interface LiveRoomListResult {
  list: LiveRoomInfo[]
  total: number
}

/** 我的房间（含推流地址；密钥不回传） */
export interface LiveMyRoom {
  room: LiveRoomInfo | null
  push_url: string
  stream_key?: string
}

/** 弹幕历史条目（与 WS 广播帧同构） */
export interface LiveDanmakuItem {
  id: string
  user_id: string
  nickname: string
  avatar?: string
  content: string
}

/** 弹幕历史（最近 50 条正序 + 当前在线人数） */
export interface LiveDanmakuHistory {
  list: LiveDanmakuItem[]
  online: number
  room_id: number
}

/** 房管条目 */
export interface LiveAdminItem {
  user_id: string
  nickname: string
  avatar?: string
  appointed_at: string
}

/** 禁言条目 */
export interface LiveMuteItem {
  user_id: string
  nickname: string
  avatar?: string
  reason: string
  muted_until: string
  muted: boolean
}

/** WS 下行帧（enter | danmaku | gift | pong） */
export interface LiveWsFrame {
  type: 'enter' | 'danmaku' | 'gift' | 'pong'
  data: {
    uid?: string
    nickname?: string
    avatar?: string
    fan_badge?: number
    id?: string
    user_id?: string
    content?: string
    log_id?: string
    gift_id?: number
    gift_name?: string
    icon?: string
    count?: number
    effect?: number
    level?: number
  }
}

/** 礼物配置项（M4-LIV-04） */
export interface LiveGiftItem {
  id: number
  name: string
  icon: string
  price: number
  effect: 1 | 2 // 1 弹幕区连击 2 全屏特效
  level: 1 | 2 // 1 普通 2 高级
}

/** 金瓜子余额 */
export interface LiveMelonBalance {
  balance: number
}

/** 赠送礼物结果 */
export interface LiveSendGiftResult {
  balance: number
  gift_log: {
    log_id: string
    sender_id: string
    nickname: string
    gift_id: number
    gift_name: string
    icon?: string
    count: number
    effect: 1 | 2
    level: 1 | 2
  }
}

/**
 * 直播接口（对应后端 /api/v1/live/*，M4-LIV-02/03；前端部分 M4-LIV-06）。
 */
export function createLiveApi(http: HttpClient) {
  return {
    /** 申请开播（资格校验 → 建房 → 明文密钥仅此一次返回） */
    apply: () => http.post<LiveApplyResult>('/api/v1/live/apply', {}),

    /** 开播（status 0/2 → 1） */
    start: () => http.post<LiveRoomInfo>('/api/v1/live/room/start', {}),

    /** 下播（1 → 2） */
    stop: () => http.post<LiveRoomInfo>('/api/v1/live/room/stop', {}),

    /** 修改房间信息（标题/封面/分区） */
    updateRoom: (req: { title: string; cover?: string; category_id: number }) =>
      http.put<LiveRoomInfo>('/api/v1/live/room', req),

    /** 重置推流密钥（旧密钥立即失效；明文仅此一次返回） */
    resetStreamKey: () => http.post<LiveApplyResult>('/api/v1/live/room/stream-key/reset', {}),

    /** 我的房间（含推流地址；密钥不回传；无房间 91007） */
    myRoom: () => http.get<LiveMyRoom>('/api/v1/live/room/mine'),

    /** 直播中房间列表（公开） */
    rooms: (params?: { category_id?: number; page?: number; size?: number }) =>
      http.get<LiveRoomListResult>('/api/v1/live/rooms', params),

    /** 房间详情（未开播房间仅主播可见） */
    roomDetail: (id: number | string) => http.get<LiveRoomInfo>(`/api/v1/live/rooms/${id}`),

    /** 进房（在线计数 +1；重复进房不重复计数） */
    enter: (id: number | string) => http.post<LiveRoomInfo>(`/api/v1/live/rooms/${id}/enter`, {}),

    /** 心跳（≤15s 一次） */
    heartbeat: (id: number | string) => http.post<null>(`/api/v1/live/rooms/${id}/heartbeat`, {}),

    /** 离房（计数 −1，幂等） */
    leave: (id: number | string) => http.post<LiveRoomInfo>(`/api/v1/live/rooms/${id}/leave`, {}),

    /** 房管列表 */
    admins: (id: number | string) =>
      http.get<{ list: LiveAdminItem[] }>(`/api/v1/live/rooms/${id}/admins`),

    /** 任命房管（仅主播） */
    addAdmin: (id: number | string, uid: string) =>
      http.post<{ added: boolean }>(`/api/v1/live/rooms/${id}/admins/${uid}`, {}),

    /** 撤销房管（仅主播） */
    removeAdmin: (id: number | string, uid: string) =>
      http.delete<{ removed: boolean }>(`/api/v1/live/rooms/${id}/admins/${uid}`),

    /** 禁言列表 */
    mutes: (id: number | string) =>
      http.get<{ list: LiveMuteItem[] }>(`/api/v1/live/rooms/${id}/mutes`),

    /** 禁言（主播/房管；时长分钟 1 ~ 43200） */
    mute: (id: number | string, uid: string, req: { minutes: number; reason?: string }) =>
      http.post<{ muted: boolean }>(`/api/v1/live/rooms/${id}/mutes/${uid}`, req),

    /** 解禁（主播/房管） */
    unmute: (id: number | string, uid: string) =>
      http.delete<{ unmuted: boolean }>(`/api/v1/live/rooms/${id}/mutes/${uid}`),

    /** 弹幕历史（最近 50 条正序 + 在线人数） */
    danmakuHistory: (id: number | string) =>
      http.get<LiveDanmakuHistory>(`/api/v1/live/rooms/${id}/danmaku/history`),

    /** 直播弹幕 WS 地址（token 走 query；http → ws 协议换算由调用方处理） */
    wsUrl: (id: number | string) => `/api/v1/live/rooms/${id}/ws`,

    /** 礼物列表（上架，M4-LIV-04） */
    gifts: () => http.get<LiveGiftItem[]>('/api/v1/live/gifts'),

    /** 金瓜子余额 */
    melonBalance: () => http.get<LiveMelonBalance>('/api/v1/live/melon/balance'),

    /** 赠送礼物（余额不足 91021） */
    sendGift: (roomId: number | string, giftId: number, count = 1) =>
      http.post<LiveSendGiftResult>(`/api/v1/live/rooms/${roomId}/gifts/${giftId}`, { count }),
  }
}

import type { HttpClient } from '../http'

/** 会话项 */
export interface ConversationItem {
  peer_id: string
  nickname: string
  avatar: string
  last_content: string
  last_at: string
  unread: number
}

/** 消息项 */
export interface MessageItem {
  id: string
  sender_id: string
  content: string
  content_type: number
  created_at: string
  mine: boolean
}

/** 私信接口（对应后端 im 模块，PRD MSG-10~13）。 */
export function createMessageApi(http: HttpClient) {
  return {
    /** 发送私信（机审 + 未互关每日限制） */
    send: (payload: { to_uid: string, content: string, content_type?: number }) =>
      http.post<MessageItem>('/api/v1/messages', payload),

    /** 会话列表（按最新消息排序，含未读数） */
    conversations: () => http.get<{ list: ConversationItem[] }>('/api/v1/messages/conversations'),

    /** 与某用户的消息记录（读取即已读） */
    messages: (peer: string, page = 1, pageSize = 20) =>
      http.get<{ list: MessageItem[]; total: number }>(`/api/v1/messages/${peer}`, {
        page,
        page_size: pageSize,
      }),

    /** 私信总未读数 */
    unreadCount: () => http.get<{ unread: number }>('/api/v1/messages/unread-count'),

    /** 私信实时连接（WS，query token） */
    wsUrl: () => '/api/v1/messages/ws',
  }
}

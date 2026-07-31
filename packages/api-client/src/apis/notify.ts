import type { HttpClient } from '../http'
import type { UserBrief } from './relation'

/** 通知条目：type 1点赞 2评论/回复 3关注 4系统 */
export interface NotifyItem {
  id: string
  type: 1 | 2 | 3 | 4
  content: string
  link: string
  is_read: boolean
  sender: UserBrief
  created_at: string
}

/** 站内通知接口。 */
export function createNotifyApi(http: HttpClient) {
  return {
    list: (cursor = '', pageSize = 20) =>
      http.get<{ list: NotifyItem[]; next_cursor: string; has_more: boolean }>(
        '/api/v1/notifications',
        { cursor: cursor || undefined, page_size: pageSize },
      ),

    unreadCount: () => http.get<{ count: number }>('/api/v1/notifications/unread-count'),

    markAllRead: () => http.post<null>('/api/v1/notifications/read'),
  }
}

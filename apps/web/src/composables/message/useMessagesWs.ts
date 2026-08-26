import type { MessageItem } from '@dlidli/api-client'
import { api } from '@/api'
import { readToken } from '@/utils/token'

interface WsDeps {
  /** 当前会话对方 ID */
  getPeer: () => string
  /** 当前会话收到新消息：追加气泡 */
  pushMessage: (m: MessageItem) => void
  /** 消息追加后滚动到底部 */
  scrollBottom: () => void
  /** 其他会话来消息：刷新会话列表未读 */
  onIncomingOther: () => void
  /** 停留在当前会话收到新消息：立即已读并刷新角标 */
  markCurrentRead: () => void
}

/**
 * 私信 WebSocket 实时接收（MessagesView 拆分，M3-ENG-11）：
 * 连接/断线重连（最多 5 次）/消息分发/全局未读角标事件（M3-IM-02，PRD MSG-13）。
 */
export function useMessagesWs(deps: WsDeps) {
  let ws: WebSocket | null = null
  let wsRetry = 0

  function connectWs() {
    const token = readToken()
    if (!token || ws) return
    const proto = window.location.protocol === 'https:' ? 'wss' : 'ws'
    ws = new WebSocket(`${proto}://${window.location.host}${api.message.wsUrl()}?token=${encodeURIComponent(token)}`)
    ws.onmessage = (e) => {
      try {
        const frame = JSON.parse(e.data)
        if (frame.type !== 'message') return
        const msg: MessageItem = frame.data
        if (msg.sender_id === deps.getPeer()) {
          deps.pushMessage(msg)
          deps.scrollBottom()
          deps.markCurrentRead()
        } else {
          deps.onIncomingOther() // 其他会话来消息：刷新会话列表未读
        }
        window.dispatchEvent(new CustomEvent('msg-unread-changed'))
      } catch {
        // 忽略非 JSON 帧
      }
    }
    ws.onclose = () => {
      ws = null
      if (wsRetry < 5) {
        wsRetry++
        setTimeout(connectWs, 3000)
      }
    }
    ws.onopen = () => {
      wsRetry = 0
    }
  }

  function closeWs() {
    ws?.close()
    ws = null
  }

  return { connectWs, closeWs }
}

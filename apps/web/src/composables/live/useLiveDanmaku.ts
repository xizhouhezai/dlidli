import { ref } from 'vue'
import { ElMessage } from 'element-plus'
import {
  ApiError,
  type LiveDanmakuItem,
  type LiveRoomInfo,
  type LiveWsFrame,
} from '@dlidli/api-client'
import { api } from '@/api'
import { readToken } from '@/utils/token'

/**
 * 直播间实时弹幕连接（M4-LIV-06）：WS 进房 + 应用层心跳 ≤15s（spec live §5B）+
 * 弹幕上行/下行 + 进场消息。与 danmaku/im 的 WS 封装同构，上行错误提示走本地预判
 * （plan live §2B.4：WS 上行不回错误帧，2s 频控前端本地冷却对齐）。
 */
export function useLiveDanmaku(roomId: string) {
  const messages = ref<LiveDanmakuItem[]>([])
  const connected = ref(false)
  const sending = ref(false)
  let ws: WebSocket | null = null
  let hbTimer: number | null = null
  /** 本地 2s 发送冷却（与服务端 live:cd 对齐；服务端拒绝时 WS 无回帧） */
  let lastSendAt = 0
  let leaving = false

  /** 进场/系统提示行（非弹幕） */
  const notices = ref<string[]>([])

  function pushNotice(text: string) {
    notices.value.push(text)
    if (notices.value.length > 30) notices.value.shift()
  }

  function pushDanmaku(item: LiveDanmakuItem) {
    messages.value.push(item)
    if (messages.value.length > 200) messages.value.shift()
  }

  function connect() {
    const token = readToken()
    if (!token) {
      pushNotice('登录后即可参与弹幕互动')
      return
    }
    const proto = location.protocol === 'https:' ? 'wss' : 'ws'
    ws = new WebSocket(
      `${proto}://${location.host}${api.live.wsUrl(roomId)}?token=${encodeURIComponent(token)}`,
    )
    ws.onopen = () => {
      connected.value = true
      // 应用层心跳 ≤15s（服务端 40s 读超时踢出）
      hbTimer = window.setInterval(() => ws?.send(JSON.stringify({ type: 'heartbeat' })), 10000)
    }
    ws.onmessage = (ev) => {
      const m: LiveWsFrame = JSON.parse(ev.data)
      if (m.type === 'enter') {
        pushNotice(`${m.data.nickname || m.data.uid} 进入直播间`)
      } else if (m.type === 'danmaku' && m.data.id) {
        pushDanmaku({
          id: m.data.id,
          user_id: m.data.user_id || '',
          nickname: m.data.nickname || '',
          avatar: m.data.avatar,
          content: m.data.content || '',
        })
      }
      // pong：心跳应答，无 UI
    }
    ws.onclose = () => {
      connected.value = false
      if (hbTimer) {
        window.clearInterval(hbTimer)
        hbTimer = null
      }
      // 下播/被顶替后不重连（离房清理在 unload）
    }
  }

  function send(content: string) {
    const text = content.trim()
    if (!ws || !connected.value || !text) return
    const now = Date.now()
    if (now - lastSendAt < 2000) {
      ElMessage.warning('发言太频繁，请稍后再试')
      return
    }
    if ([...text].length > 50) {
      ElMessage.warning('弹幕最多 50 字')
      return
    }
    lastSendAt = now
    ws.send(JSON.stringify({ type: 'danmaku', content: text }))
    // 乐观回显：WS 广播不含本人标记，直接本地补一条即时反馈
    pushDanmaku({
      id: `local-${now}`,
      user_id: 'self',
      nickname: '我',
      content: text,
    })
  }

  async function leave() {
    if (leaving) return
    leaving = true
    try {
      await api.live.leave(roomId)
    } catch {
      /* silent：断连即离房 */
    }
  }

  function close() {
    ws?.close(1000)
    ws = null
    connected.value = false
    if (hbTimer) {
      window.clearInterval(hbTimer)
      hbTimer = null
    }
  }

  return { messages, notices, connected, sending, connect, close, send, leave }
}

/** 房间详情 + 进房（进房计数 +1；重复进房不重复计数） */
export async function enterRoom(roomId: string): Promise<LiveRoomInfo | null> {
  try {
    return await api.live.enter(roomId)
  } catch (err) {
    if (err instanceof ApiError && (err.code === 91006 || err.code === 91007)) {
      return null
    }
    throw err
  }
}

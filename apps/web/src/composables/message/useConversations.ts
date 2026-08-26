import { computed, nextTick, ref } from 'vue'
import type { RouteLocationNormalizedLoaded, Router } from 'vue-router'
import { ElMessage } from 'element-plus'
import type { ConversationItem, MessageItem } from '@dlidli/api-client'
import { api } from '@/api'

/**
 * 私信会话与消息（MessagesView 拆分，M3-ENG-11）：
 * 会话列表 / 打开会话（含临时会话项拼装）/ 消息加载与发送 / 已读联动。
 */
export function useConversations(route: RouteLocationNormalizedLoaded, router: Router) {
  const convs = ref<ConversationItem[]>([])
  const activePeer = ref('')
  const messages = ref<MessageItem[]>([])
  const input = ref('')
  const sending = ref(false)
  const listBox = ref<HTMLDivElement>()

  // 当前会话对方信息
  const activeConv = computed(() => convs.value.find((c) => c.peer_id === activePeer.value))

  async function loadConvs() {
    try {
      convs.value = (await api.message.conversations()).list ?? []
    } catch {
      convs.value = []
    }
  }

  function scrollBottom() {
    if (listBox.value) listBox.value.scrollTop = listBox.value.scrollHeight
  }

  // 当前会话立即已读：复用 messages 接口（服务端 MarkRead），并刷新会话列表角标
  async function markCurrentRead() {
    if (!activePeer.value) return
    try {
      await api.message.messages(activePeer.value, 1, 1)
      void loadConvs()
    } catch {
      // 已读失败不阻塞展示
    }
  }

  async function openPeer(peer: string) {
    if (activePeer.value === peer) return
    // 同步 URL（query.peer），触发 AppHeader watch 延时刷新头部未读红点
    if (route.query.peer !== peer) {
      await router.replace({ path: '/messages', query: { peer } })
    }
    activePeer.value = peer
    messages.value = []
    // 列表无此对象时（新会话/仅对方发过未读），用对方资料拼临时会话项
    let conv = convs.value.find((c) => c.peer_id === peer)
    if (!conv) {
      try {
        const p = await api.relation.profile(peer)
        conv = {
          peer_id: peer,
          nickname: p.nickname,
          avatar: p.avatar,
          last_content: '',
          last_at: new Date().toISOString(),
          unread: 0,
        }
        convs.value.unshift(conv)
      } catch {
        // 资料获取失败不阻塞消息加载
      }
    }
    if (conv && conv.unread > 0) {
      conv.unread = 0
    }
    try {
      const res = await api.message.messages(peer, 1, 50)
      messages.value = res.list ?? []
    } catch {
      messages.value = []
    }
    await nextTick()
    scrollBottom()
  }

  async function send() {
    const content = input.value.trim()
    if (!content || !activePeer.value) return
    sending.value = true
    try {
      const msg = await api.message.send({ to_uid: activePeer.value, content, content_type: 1 })
      messages.value.push(msg)
      input.value = ''
      scrollBottom()
      void loadConvs()
    } catch (err) {
      ElMessage.error(err instanceof Error ? err.message : '发送失败')
    } finally {
      sending.value = false
    }
  }

  /** 会话/消息时间展示：当天显示 HH:mm，跨天显示 M/D。 */
  function formatTime(t: string) {
    const d = new Date(t)
    const now = new Date()
    if (d.toDateString() === now.toDateString()) {
      return `${String(d.getHours()).padStart(2, '0')}:${String(d.getMinutes()).padStart(2, '0')}`
    }
    return `${d.getMonth() + 1}/${d.getDate()}`
  }

  return {
    convs,
    activePeer,
    activeConv,
    messages,
    input,
    sending,
    listBox,
    loadConvs,
    markCurrentRead,
    openPeer,
    send,
    scrollBottom,
    formatTime,
  }
}

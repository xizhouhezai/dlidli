<script setup lang="ts">
// 直播间观看页（M4-LIV-06，H5 只读版）：拉流播放 + 弹幕滚动 + 发弹幕。
// 开播管理不做（PC 场景，见任务书）。WS 封装与 web 端同构（uni 环境无 WebSocket 差异，
// H5 编译目标原生可用；token 走 query 与后端 live hub 约定一致）。
import { onLoad, onUnload } from '@dcloudio/uni-app'
import { computed, nextTick, ref } from 'vue'
import { api } from '@/api'
import { LIVE_STREAM_CONFIG } from '@/config/live'
import type { LiveDanmakuItem, LiveWsFrame } from '@dlidli/api-client'

const HLS_TEST = 'https://test-streams.mux.dev/x36xhzz/x36xhzz.m3u8'

const roomId = ref('')
const room = ref<{ title: string; online: number; cover: string } | null>(null)
const offline = ref(false)
const loading = ref(true)

/** 无 id 进入时先展示开播中房间列表（H5 无独立列表页，同页承载） */
const roomList = ref<
  { id: number; title: string; cover: string; online: number; anchor_id: string }[]
>([])

const messages = ref<LiveDanmakuItem[]>([])
const notices = ref<string[]>([])
const connected = ref(false)
const draft = ref('')
const scrollInto = ref('')
let ws: WebSocket | null = null
let hbTimer: ReturnType<typeof setInterval> | null = null
let lastSendAt = 0

const playUrl = computed(() =>
  LIVE_STREAM_CONFIG.demo ? HLS_TEST : LIVE_STREAM_CONFIG.mp4Fallback,
)

onLoad((query) => {
  roomId.value = String((query as { id?: string }).id ?? '')
  if (roomId.value) load()
  else loadList()
})

async function loadList() {
  try {
    const res = await api.live.rooms({ page: 1, size: 50 })
    roomList.value = res.list
  } catch {
    uni.showToast({ title: '直播列表加载失败', icon: 'none' })
  } finally {
    loading.value = false
  }
}

function openRoom(id: number) {
  uni.redirectTo({ url: `/pages/live/live?id=${id}` })
}

async function load() {
  try {
    const info = await api.live.enter(roomId.value)
    if (!info) {
      offline.value = true
      return
    }
    room.value = { title: info.title, online: info.online, cover: info.cover }
    uni.setNavigationBarTitle({ title: info.title || '直播间' })
    connect()
    try {
      const hist = await api.live.danmakuHistory(roomId.value)
      messages.value = hist.list
      room.value.online = hist.online
    } catch {
      /* 历史可选 */
    }
  } catch {
    offline.value = true
  } finally {
    loading.value = false
  }
}

function connect() {
  const token = (uni.getStorageSync('dlidli_token') as string) || ''
  if (!token) {
    notices.value.push('登录后即可参与弹幕互动')
    return
  }
  const proto = location.protocol === 'https:' ? 'wss' : 'ws'
  ws = new WebSocket(
    `${proto}://${location.host}${api.live.wsUrl(roomId.value)}?token=${encodeURIComponent(token)}`,
  )
  ws.onopen = () => {
    connected.value = true
    hbTimer = setInterval(() => ws?.send(JSON.stringify({ type: 'heartbeat' })), 10000)
  }
  ws.onmessage = (ev) => {
    const m: LiveWsFrame = JSON.parse(ev.data)
    if (m.type === 'enter') {
      notices.value.push(`${m.data.nickname || m.data.uid} 进入直播间`)
    } else if (m.type === 'danmaku' && m.data.id) {
      push({
        id: m.data.id,
        user_id: m.data.user_id || '',
        nickname: m.data.nickname || '',
        avatar: m.data.avatar,
        content: m.data.content || '',
      })
    }
    scrollToBottom()
  }
  ws.onclose = () => {
    connected.value = false
    if (hbTimer) {
      clearInterval(hbTimer)
      hbTimer = null
    }
  }
}

function push(item: LiveDanmakuItem) {
  messages.value.push(item)
  if (messages.value.length > 100) messages.value.shift()
}

function scrollToBottom() {
  nextTick(() => {
    scrollInto.value = messages.value.length ? `dm-${messages.value.length - 1}` : ''
  })
}

function send() {
  const text = draft.value.trim()
  if (!ws || !connected.value || !text) return
  const now = Date.now()
  if (now - lastSendAt < 2000) {
    uni.showToast({ title: '发言太频繁', icon: 'none' })
    return
  }
  if ([...text].length > 50) {
    uni.showToast({ title: '弹幕最多 50 字', icon: 'none' })
    return
  }
  lastSendAt = now
  ws.send(JSON.stringify({ type: 'danmaku', content: text }))
  push({ id: `local-${now}`, user_id: 'self', nickname: '我', content: text })
  draft.value = ''
  scrollToBottom()
}

onUnload(() => {
  ws?.close(1000)
  ws = null
  if (hbTimer) clearInterval(hbTimer)
  api.live.leave(roomId.value).catch(() => {
    /* silent */
  })
})
</script>

<template>
  <view class="page">
    <view v-if="loading" class="tip">加载中…</view>

    <!-- 房间列表（无 id 入口） -->
    <template v-else-if="!roomId">
      <view v-if="roomList.length === 0" class="tip"><text>暂时没有主播开播</text></view>
      <view v-for="r in roomList" :key="r.id" class="room-card" @tap="openRoom(r.id)">
        <image
          class="room-card__cover"
          :src="r.cover || '/static/default-cover.png'"
          mode="aspectFill"
        />
        <view class="room-card__info">
          <text class="room-card__title">{{ r.title || '未命名直播间' }}</text>
          <view class="room-card__meta">
            <text class="room-card__live">● 直播中</text>
            <text>{{ r.online }} 人在线</text>
          </view>
        </view>
      </view>
    </template>

    <view v-else-if="offline" class="tip">
      <text>该房间未在直播</text>
    </view>

    <template v-else-if="room">
      <!-- 播放器：uni video 原生 HLS；直播流循环演示源 -->
      <video
        class="player"
        :src="playUrl"
        :controls="true"
        :autoplay="true"
        :muted="true"
        :playsinline="true"
        :webkit-playsinline="true"
        object-fit="contain"
        loop
      />

      <!-- 信息条 -->
      <view class="info">
        <text class="info__title">{{ room.title || '未命名直播间' }}</text>
        <view class="info__stat">
          <text class="info__live">● 直播中</text>
          <text class="info__online">{{ room.online }} 人在线</text>
        </view>
      </view>

      <!-- 弹幕滚动区 -->
      <scroll-view class="danmaku" scroll-y :scroll-into-view="scrollInto" scroll-with-animation>
        <view v-for="(n, i) in notices" :key="`n${i}`" class="dm dm--notice">
          <text>{{ n }}</text>
        </view>
        <view
          v-for="(msg, i) in messages"
          :id="`dm-${i}`"
          :key="msg.id"
          class="dm"
          :class="{ 'dm--self': msg.user_id === 'self' }"
        >
          <text class="dm__name">{{ msg.nickname }}：</text>
          <text>{{ msg.content }}</text>
        </view>
      </scroll-view>

      <!-- 发弹幕 -->
      <view class="bar">
        <input
          v-model="draft"
          class="bar__input"
          :placeholder="connected ? '说点什么…（2s 冷却）' : '未连接，登录后参与互动'"
          maxlength="50"
          confirm-type="send"
          :disabled="!connected"
          @confirm="send"
        />
        <view class="bar__btn" :class="{ disabled: !connected || !draft.trim() }" @tap="send"
          >发送</view
        >
      </view>
    </template>
  </view>
</template>

<style lang="scss">
@use '../../styles/variables' as v;

.page {
  display: flex;
  flex-direction: column;
  height: 100vh;
  background: v.$surface;
}

.player {
  width: 100%;
  height: 211px; /* 16:9 */
  background: #000;
}

.info {
  padding: 20rpx 24rpx;
}

.info__title {
  font-size: 30rpx;
  font-weight: 600;
  color: v.$text-1;
}

.info__stat {
  margin-top: 8rpx;
  font-size: 24rpx;
}

.info__live {
  color: v.$primary;
  font-weight: 600;
  margin-right: 16rpx;
}

.info__online {
  color: v.$text-3;
}

.danmaku {
  flex: 1;
  padding: 16rpx 24rpx;
  box-sizing: border-box;
  background: v.$bg;
}

.dm {
  margin: 6rpx 0;
  font-size: 26rpx;
  line-height: 1.6;
  color: v.$text-1;
  word-break: break-all;
}

.dm--self .dm__name {
  color: v.$primary;
}

.dm__name {
  color: v.$text-2;
}

.dm--notice {
  font-size: 22rpx;
  color: v.$text-3;
}

.bar {
  display: flex;
  align-items: center;
  gap: 16rpx;
  padding: 16rpx 24rpx;
  border-top: 1rpx solid v.$border;
}

.bar__input {
  flex: 1;
  height: 64rpx;
  padding: 0 24rpx;
  border-radius: 32rpx;
  background: v.$bg;
  font-size: 26rpx;
}

.bar__btn {
  padding: 12rpx 28rpx;
  border-radius: 32rpx;
  background: v.$primary;
  color: #fff;
  font-size: 26rpx;
  font-weight: 600;

  &.disabled {
    opacity: 0.4;
  }
}

.tip {
  padding: 120rpx;
  text-align: center;
  font-size: 26rpx;
  color: v.$text-3;
}

/* 房间列表卡片 */
.room-card {
  display: flex;
  margin: 20rpx 24rpx;
  border-radius: 16rpx;
  overflow: hidden;
  background: #fff;
  box-shadow: 0 2rpx 12rpx rgba(0, 0, 0, 0.05);
}

.room-card__cover {
  width: 280rpx;
  height: 158rpx;
  flex-shrink: 0;
  background: #f1f2f3;
}

.room-card__info {
  flex: 1;
  min-width: 0;
  padding: 16rpx 20rpx;
  display: flex;
  flex-direction: column;
  justify-content: center;
}

.room-card__title {
  font-size: 28rpx;
  font-weight: 600;
  color: v.$text-1;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.room-card__meta {
  margin-top: 10rpx;
  font-size: 22rpx;
  color: v.$text-3;
}

.room-card__live {
  color: v.$primary;
  margin-right: 12rpx;
}
</style>

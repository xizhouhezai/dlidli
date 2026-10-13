<script setup lang="ts">
// 直播间页（M4-LIV-06，LIV-11/12）：拉流播放器 + 实时弹幕区 + 在线人数。
// 流源：LIV-01 前用演示流（config/live.ts，替换点见该文件注释）；WS 与 LIV-03 hub 同构。
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { RouterLink, useRoute, useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { PlayerCore } from '@dlidli/player'
import { formatCount } from '@dlidli/shared'
import type { LiveRoomInfo } from '@dlidli/api-client'
import { api } from '@/api'
import { readToken } from '@/utils/token'
import { LIVE_STREAM_CONFIG } from '@/config/live'
import { enterRoom, useLiveDanmaku } from '@/composables/live/useLiveDanmaku'
import defaultAvatar from '@/assets/default-avatar.png'

const route = useRoute()
const router = useRouter()
const roomId = computed(() => String(route.params.id ?? ''))

const room = ref<LiveRoomInfo | null>(null)
const offline = ref(false)
const loading = ref(true)

const anchorProfile = ref<{ nickname: string; avatar: string }>({ nickname: '', avatar: '' })
const online = ref(0)

const videoRef = ref<HTMLVideoElement | null>(null)
let player: PlayerCore | null = null
let onlineTimer: number | null = null

const { messages, notices, connected, connect, close, send, leave } = useLiveDanmaku(roomId.value)
const draft = ref('')
const danmakuListRef = ref<HTMLElement | null>(null)

const isSelfAnchor = computed(() => !!room.value && room.value.anchor_id === String(useSelfUid()))
function useSelfUid() {
  return JSON.parse(atob((readToken() || '').split('.')[1] || 'e30=') || '{}').uid ?? ''
}

async function load() {
  loading.value = true
  try {
    const info = await enterRoom(roomId.value)
    if (!info) {
      offline.value = true
      return
    }
    room.value = info
    online.value = info.online
    connect()
    // 弹幕历史（最近 50 条正序）
    try {
      const hist = await api.live.danmakuHistory(roomId.value)
      messages.value = hist.list
      online.value = hist.online
    } catch {
      /* 历史可选 */
    }
    setupPlayer()
    loadAnchor()
    // 在线数 15s 轮询（口径同列表页）
    onlineTimer = window.setInterval(refreshOnline, 15000)
  } catch {
    ElMessage.error('进房失败，请重试')
  } finally {
    loading.value = false
  }
}

async function refreshOnline() {
  try {
    const detail = await api.live.roomDetail(roomId.value)
    if (detail.status !== 1) {
      offline.value = true
      stopOnlineTimer()
      player?.destroy()
      return
    }
    online.value = detail.online
  } catch {
    /* silent */
  }
}

async function loadAnchor() {
  if (!room.value) return
  try {
    const p = await api.relation.profile(room.value.anchor_id)
    anchorProfile.value = { nickname: p.nickname, avatar: p.avatar }
  } catch {
    anchorProfile.value = { nickname: `主播 ${room.value.anchor_id}`, avatar: '' }
  }
}

/**
 * 挂流：demo 走公共 HLS；不可达或非 demo 时落 mp4 循环兜底。
 * LIV-01 后改 config/live.ts 即接真实流（页面不动）。
 */
function setupPlayer() {
  const video = videoRef.value
  if (!video) return
  player = new PlayerCore(video, {})
  if (LIVE_STREAM_CONFIG.demo) {
    player.attach({ quality: 0, format: 'hls', url: LIVE_STREAM_CONFIG.hlsUrl })
  } else {
    player.attach({ quality: 0, format: 'mp4', url: LIVE_STREAM_CONFIG.mp4Fallback })
  }
  video.loop = true
  video.muted = true // 自动播放策略：先静音起播，用户手势后解除
  void video.play().catch(() => {
    /* 浏览器拒绝自动播：显示控制条手动起播 */
  })
}

function unmute() {
  if (videoRef.value) {
    videoRef.value.muted = false
  }
}

function sendDanmaku() {
  const text = draft.value.trim()
  if (!text) return
  send(text)
  draft.value = ''
}

// 弹幕区自动滚底（用户上翻时暂停自动滚）
function onListScroll() {
  const el = danmakuListRef.value
  if (!el) return
  el.dataset.stick = el.scrollHeight - el.scrollTop - el.clientHeight < 60 ? '1' : ''
}

watch(messages, () => {
  const el = danmakuListRef.value
  if (el && el.dataset.stick !== '') {
    requestAnimationFrame(() => {
      el.scrollTop = el.scrollHeight
    })
  }
})

function stopOnlineTimer() {
  if (onlineTimer) {
    window.clearInterval(onlineTimer)
    onlineTimer = null
  }
}

onMounted(load)
onBeforeUnmount(() => {
  stopOnlineTimer()
  close()
  void leave()
  player?.destroy()
})
</script>

<template>
  <div class="live-room">
    <div v-if="loading" class="live-room__tip">加载中…</div>

    <div v-else-if="offline" class="live-room__tip">
      <p class="text-8 i-mingcute-tv-2-line mb-3 text-#c0c4cc" />
      <p>该房间未在直播</p>
      <RouterLink to="/live" class="live-room__back">← 返回直播首页</RouterLink>
    </div>

    <div v-else-if="room" class="live-room__body">
      <!-- 播放器区 -->
      <div class="live-room__player-wrap">
        <div class="live-room__player">
          <video ref="videoRef" controls playsinline class="w-full h-full bg-black" />
          <div class="live-room__topbar">
            <span class="live-room__live-badge"><span class="dot" />直播中</span>
            <span class="live-room__online">
              <span class="i-mingcute-user-2-line mr-1" />{{ formatCount(online) }} 人在线
            </span>
          </div>
          <button
            v-if="LIVE_STREAM_CONFIG.demo"
            class="live-room__demo-tag"
            title="LIV-01 接入真实流后自动切换"
            @click="unmute"
          >
            演示流 · 点击开启声音
          </button>
        </div>

        <div class="live-room__info">
          <img class="live-room__avatar" :src="anchorProfile.avatar || defaultAvatar" alt="" />
          <div class="flex-1 min-w-0">
            <p class="live-room__room-title">{{ room.title || '未命名直播间' }}</p>
            <p class="live-room__anchor">
              <RouterLink :to="`/space/${room.anchor_id}`" class="hover:text-primary">
                {{ anchorProfile.nickname || `主播 ${room.anchor_id}` }}
              </RouterLink>
            </p>
          </div>
          <el-button v-if="isSelfAnchor" size="small" round @click="router.push('/studio/live')">
            开播管理
          </el-button>
        </div>
      </div>

      <!-- 弹幕区 -->
      <div class="live-room__chat">
        <div class="live-room__chat-head">
          <span class="i-mingcute-danmaku-line mr-1" />弹幕
          <span v-if="connected" class="live-room__conn live-room__conn--on">已连接</span>
          <span v-else class="live-room__conn">未连接</span>
        </div>

        <div ref="danmakuListRef" class="live-room__chat-list" @scroll="onListScroll">
          <p v-for="(n, i) in notices" :key="`n${i}`" class="live-room__notice">{{ n }}</p>
          <p
            v-for="msg in messages"
            :key="msg.id"
            class="live-room__dm"
            :class="{ 'live-room__dm--self': msg.user_id === 'self' }"
          >
            <span class="live-room__dm-name">{{ msg.nickname }}：</span>{{ msg.content }}
          </p>
        </div>

        <div class="live-room__chat-input">
          <el-input
            v-model="draft"
            placeholder="发条弹幕见证当下（2s 冷却）"
            maxlength="50"
            :disabled="!connected"
            @keyup.enter="sendDanmaku"
          />
          <el-button type="primary" :disabled="!connected || !draft.trim()" @click="sendDanmaku">
            发送
          </el-button>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped lang="scss">
@use '@/styles/variables' as v;

.live-room__tip {
  padding: 100px 0;
  text-align: center;
  color: v.$text-3;
}

.live-room__back {
  display: inline-block;
  margin-top: 12px;
  color: v.$primary;
  text-decoration: none;
}

.live-room__body {
  display: grid;
  grid-template-columns: 1fr 320px;
  gap: 16px;

  @media (max-width: 960px) {
    grid-template-columns: 1fr;
  }
}

.live-room__player-wrap {
  min-width: 0;
}

.live-room__player {
  position: relative;
  aspect-ratio: 16 / 9;
  border-radius: 8px;
  overflow: hidden;
  background: #000;
}

.live-room__topbar {
  position: absolute;
  top: 10px;
  left: 10px;
  right: 10px;
  display: flex;
  justify-content: space-between;
  pointer-events: none;
}

.live-room__live-badge {
  display: inline-flex;
  align-items: center;
  padding: 2px 10px;
  border-radius: 4px;
  background: v.$primary;
  color: #fff;
  font-size: 13px;
  font-weight: 600;

  .dot {
    width: 6px;
    height: 6px;
    border-radius: 50%;
    background: #fff;
    margin-right: 5px;
    animation: live-pulse 1.2s infinite;
  }
}

@keyframes live-pulse {
  0%,
  100% {
    opacity: 1;
  }
  50% {
    opacity: 0.4;
  }
}

.live-room__online {
  display: inline-flex;
  align-items: center;
  padding: 2px 10px;
  border-radius: 4px;
  background: rgba(0, 0, 0, 0.55);
  color: #fff;
  font-size: 13px;
}

.live-room__demo-tag {
  position: absolute;
  right: 10px;
  bottom: 52px;
  padding: 4px 10px;
  border: none;
  border-radius: 4px;
  background: rgba(0, 0, 0, 0.55);
  color: #fff;
  font-size: 12px;
  cursor: pointer;
}

.live-room__info {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 12px 4px;
}

.live-room__avatar {
  width: 44px;
  height: 44px;
  border-radius: 50%;
  object-fit: cover;
  background: #f1f2f3;
}

.live-room__room-title {
  margin: 0;
  font-size: 16px;
  font-weight: 600;
  @include v.ellipsis(1);
}

.live-room__anchor {
  margin: 2px 0 0;
  font-size: 13px;
  color: v.$text-2;

  a {
    color: inherit;
    text-decoration: none;
  }
}

/* ---- 弹幕区 ---- */
.live-room__chat {
  display: flex;
  flex-direction: column;
  height: 560px;
  border: 1px solid v.$border;
  border-radius: 8px;
  overflow: hidden;

  @media (max-width: 960px) {
    height: 400px;
  }
}

.live-room__chat-head {
  display: flex;
  align-items: center;
  padding: 10px 14px;
  border-bottom: 1px solid v.$border;
  font-size: 14px;
  font-weight: 600;
}

.live-room__conn {
  margin-left: auto;
  font-size: 12px;
  font-weight: 400;
  color: v.$text-3;

  &--on {
    color: #2a9d52;
  }
}

.live-room__chat-list {
  flex: 1;
  overflow-y: auto;
  padding: 10px 14px;
  font-size: 13px;
  line-height: 1.7;
}

.live-room__notice {
  margin: 2px 0;
  color: v.$text-3;
  font-size: 12px;
}

.live-room__dm {
  margin: 2px 0;
  color: v.$text-1;
  word-break: break-all;
}

.live-room__dm--self .live-room__dm-name {
  color: v.$primary;
}

.live-room__dm-name {
  color: v.$text-2;
}

.live-room__chat-input {
  display: flex;
  gap: 8px;
  padding: 10px;
  border-top: 1px solid v.$border;
}
</style>

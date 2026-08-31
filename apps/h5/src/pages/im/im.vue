<script setup lang="ts">
import { onMounted, ref, nextTick } from 'vue'
import { onShow } from '@dcloudio/uni-app'
import { ApiError, type ConversationItem, type MessageItem } from '@dlidli/api-client'
import { api, hasLogin } from '@/api'

const DEFAULT_AVATAR = '/static/default-avatar.png'

const peerId = ref('')
const peer = ref<ConversationItem | null>(null)
const messages = ref<MessageItem[]>([])
const page = ref(1)
const hasMore = ref(true)
const loading = ref(false)
const sending = ref(false)
const input = ref('')

async function load() {
  if (!peerId.value) return
  loading.value = true
  try {
    const res = await api.message.messages(peerId.value, page.value, 20)
    messages.value = page.value === 1 ? res.list : [...messages.value, ...res.list]
    hasMore.value = res.list.length === 20
  } catch (e) {
    uni.showToast({ title: e instanceof ApiError ? e.message : '加载失败', icon: 'none' })
  } finally {
    loading.value = false
  }
}

async function loadPeerProfile() {
  try {
    const res = await api.message.conversations()
    const c = res.list.find((x: ConversationItem) => x.peer_id === peerId.value)
    if (c) peer.value = c
  } catch {
    // 静默
  }
}

async function send() {
  const content = input.value.trim()
  if (!content || sending.value) return
  sending.value = true
  try {
    const item = await api.message.send({ to_uid: peerId.value, content })
    messages.value = [...messages.value, item]
    input.value = ''
    await nextTick()
    uni.pageScrollTo({ scrollTop: 99999, duration: 200 })
  } catch (e) {
    uni.showToast({ title: e instanceof ApiError ? e.message : '发送失败', icon: 'none' })
  } finally {
    sending.value = false
  }
}

function goBack() {
  uni.navigateBack()
}

onMounted(() => {
  if (!hasLogin()) {
    uni.showToast({ title: '请先登录', icon: 'none' })
    setTimeout(() => uni.navigateBack(), 800)
    return
  }
  const pages = getCurrentPages()
  const cur: any = pages[pages.length - 1]
  const q = cur?.options ?? cur?.$page?.options ?? {}
  peerId.value = (q.peer as string) || ''
  if (peerId.value) {
    loadPeerProfile()
    load()
  }
})

onShow(() => {
  if (peerId.value) load()
})
</script>

<template>
  <view class="im">
    <!-- 自定义导航 -->
    <view class="nav">
      <text class="nav__back" @tap="goBack">‹ 返回</text>
      <view class="nav__title">
        <image class="nav__avatar" :src="peer?.avatar || DEFAULT_AVATAR" mode="aspectFill" />
        <text class="nav__name">{{ peer?.nickname || '私信' }}</text>
      </view>
    </view>

    <!-- 消息列表 -->
    <scroll-view class="msgs" :scroll-y="true" :scroll-into-view="'msg-' + (messages.length - 1)">
      <view v-if="loading && messages.length === 0" class="tip">加载中…</view>
      <view v-else-if="messages.length === 0" class="tip">开始你们的对话吧</view>
      <view
        v-for="(m, i) in messages"
        :id="'msg-' + i"
        :key="m.id"
        class="bubble"
        :class="m.mine ? 'bubble--mine' : 'bubble--peer'"
      >
        <text class="bubble__text">{{ m.content }}</text>
        <text class="bubble__time">{{ m.created_at }}</text>
      </view>
    </scroll-view>

    <!-- 输入栏 -->
    <view class="composer">
      <input
        v-model="input"
        class="composer__input"
        placeholder="输入消息"
        :disabled="sending"
        @confirm="send"
      />
      <button class="composer__send" :disabled="sending || !input.trim()" @tap="send">发送</button>
    </view>
  </view>
</template>

<style lang="scss">
@use '../../styles/variables' as v;

.im {
  display: flex;
  flex-direction: column;
  height: 100vh;
  background: v.$bg;
}

.nav {
  display: flex;
  align-items: center;
  padding: 24rpx;
  background: v.$surface;
  border-bottom: 1rpx solid v.$border;
  position: sticky;
  top: 0;
  z-index: 5;
}

.nav__back {
  font-size: 32rpx;
  color: v.$primary;
  padding-right: 16rpx;
}

.nav__title {
  flex: 1;
  display: flex;
  align-items: center;
  gap: 16rpx;
}

.nav__avatar {
  width: 56rpx;
  height: 56rpx;
  border-radius: 50%;
  background: v.$primary;
}

.nav__name {
  font-size: 30rpx;
  font-weight: 600;
}

.msgs {
  flex: 1;
  padding: 16rpx 24rpx;
}

.tip {
  text-align: center;
  font-size: 24rpx;
  color: v.$text-3;
  padding: 32rpx 0;
}

.bubble {
  display: inline-block;
  max-width: 70%;
  padding: 20rpx 24rpx;
  margin: 12rpx 0;
  border-radius: 16rpx;
  font-size: 28rpx;
  line-height: 1.5;
  word-break: break-word;
}

.bubble--peer {
  background: v.$surface;
  color: v.$text-1;
  border: 1rpx solid v.$border;
}

.bubble--mine {
  background: v.$primary;
  color: #fff;
  margin-left: auto;
}

.bubble--mine + .bubble--mine,
.bubble--peer + .bubble--peer {
  margin-top: 8rpx;
}

.bubble__text {
  display: block;
  white-space: pre-wrap;
}

.bubble__time {
  display: block;
  margin-top: 8rpx;
  font-size: 20rpx;
  opacity: 0.65;
}

.composer {
  display: flex;
  gap: 16rpx;
  padding: 16rpx 24rpx;
  background: v.$surface;
  border-top: 1rpx solid v.$border;
  padding-bottom: calc(16rpx + env(safe-area-inset-bottom));
}

.composer__input {
  flex: 1;
  height: 72rpx;
  padding: 0 24rpx;
  border-radius: 36rpx;
  background: v.$bg;
  font-size: 28rpx;
}

.composer__send {
  height: 72rpx;
  line-height: 72rpx;
  padding: 0 28rpx;
  border-radius: 36rpx;
  background: v.$primary;
  color: #fff;
  font-size: 26rpx;
  white-space: nowrap;
}

.composer__send[disabled] {
  opacity: 0.5;
}
</style>

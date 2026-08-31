<script setup lang="ts">
import { onMounted, ref, computed } from 'vue'
import { onShow } from '@dcloudio/uni-app'
import { ApiError } from '@dlidli/api-client'
import { api, hasLogin } from '@/api'
import type { NotifyItem, ConversationItem } from '@dlidli/api-client'

const DEFAULT_AVATAR = '/static/default-avatar.png'
const tab = ref<'notify' | 'message'>('notify')
const loggedIn = ref(false)

// —— 通知 ——
const notifyList = ref<NotifyItem[]>([])
const notifyCursor = ref('')
const notifyHasMore = ref(true)
const notifyLoading = ref(false)
const notifyUnread = ref(0)
const notifyTypeLabel: Record<1 | 2 | 3 | 4, string> = {
  1: '点赞',
  2: '评论',
  3: '关注',
  4: '系统',
}

// —— 私信 ——
const conversations = ref<ConversationItem[]>([])
const msgUnread = ref(0)
const msgLoading = ref(false)

async function loadUnread() {
  if (!loggedIn.value) return
  try {
    const [n, m] = await Promise.all([api.notify.unreadCount(), api.message.unreadCount()])
    notifyUnread.value = n.count
    msgUnread.value = m.unread
  } catch {
    // 静默失败
  }
}

async function loadNotify(reset = true) {
  if (!loggedIn.value) return
  if (reset) {
    notifyCursor.value = ''
    notifyHasMore.value = true
    notifyLoading.value = true
  }
  try {
    const res = await api.notify.list(notifyCursor.value, 20)
    notifyList.value = reset ? res.list : [...notifyList.value, ...res.list]
    notifyCursor.value = res.next_cursor
    notifyHasMore.value = res.has_more
  } catch (e) {
    uni.showToast({ title: e instanceof ApiError ? e.message : '通知加载失败', icon: 'none' })
  } finally {
    notifyLoading.value = false
  }
}

async function loadConversations() {
  if (!loggedIn.value) return
  msgLoading.value = true
  try {
    const res = await api.message.conversations()
    conversations.value = res.list
  } catch (e) {
    uni.showToast({ title: e instanceof ApiError ? e.message : '会话加载失败', icon: 'none' })
  } finally {
    msgLoading.value = false
  }
}

async function markAllNotifyRead() {
  if (!notifyUnread.value) return
  try {
    await api.notify.markAllRead()
    notifyList.value = notifyList.value.map((n) => ({ ...n, is_read: true }))
    notifyUnread.value = 0
  } catch {
    // 静默
  }
}

function switchTab(t: 'notify' | 'message') {
  tab.value = t
  if (t === 'message' && conversations.value.length === 0) loadConversations()
}

function openNotify(item: NotifyItem) {
  if (!item.is_read) {
    item.is_read = true
    if (notifyUnread.value > 0) notifyUnread.value--
  }
  if (item.link) {
    // 站内 link（如 /video/BV...、/users/...）——仅支持视频跳转
    if (item.link.startsWith('/video/')) {
      uni.navigateTo({ url: item.link })
    }
  }
}

function openConversation(peerId: string) {
  uni.navigateTo({ url: '/pages/im/im?peer=' + peerId })
}

const totalUnread = computed(() => notifyUnread.value + msgUnread.value)

onMounted(() => {
  loggedIn.value = hasLogin()
  if (loggedIn.value) {
    loadNotify(true)
    loadUnread()
  }
})

onShow(() => {
  if (loggedIn.value) {
    loadUnread()
    if (tab.value === 'message') loadConversations()
  }
})
</script>

<template>
  <view class="messages">
    <!-- 未登录 -->
    <view v-if="!loggedIn" class="empty">
      <text class="empty__icon i-mingcute-mail-line" />
      <text class="empty__text">请先登录后查看消息</text>
    </view>

    <template v-else>
      <!-- 顶部 Tab + 未读 -->
      <view class="tab-bar">
        <view class="tab" :class="{ active: tab === 'notify' }" @tap="switchTab('notify')">
          <text>通知</text>
          <text v-if="notifyUnread > 0" class="badge">{{ notifyUnread }}</text>
        </view>
        <view class="tab" :class="{ active: tab === 'message' }" @tap="switchTab('message')">
          <text>私信</text>
          <text v-if="msgUnread > 0" class="badge">{{ msgUnread }}</text>
        </view>
        <view v-if="totalUnread > 0" class="tab__action" @tap="markAllNotifyRead">全部已读</view>
      </view>

      <!-- 通知列表 -->
      <view v-if="tab === 'notify'">
        <view v-if="notifyLoading && notifyList.length === 0" class="empty">加载中…</view>
        <view v-else-if="notifyList.length === 0" class="empty">
          <text class="empty__icon i-mingcute-notification-line" />
          <text class="empty__text">暂无通知</text>
        </view>
        <view
          v-for="n in notifyList"
          :key="n.id"
          class="row"
          :class="{ unread: !n.is_read }"
          @tap="openNotify(n)"
        >
          <text class="row__type" :class="'row__type--t' + n.type">{{
            notifyTypeLabel[n.type]
          }}</text>
          <view class="row__body">
            <text class="row__text">{{ n.content }}</text>
            <text class="row__time">{{ n.created_at }}</text>
          </view>
          <text v-if="!n.is_read" class="row__dot" />
        </view>
        <view v-if="!notifyHasMore && notifyList.length > 0" class="tip">没有更多了</view>
      </view>

      <!-- 私信会话 -->
      <view v-else>
        <view v-if="msgLoading && conversations.length === 0" class="empty">加载中…</view>
        <view v-else-if="conversations.length === 0" class="empty">
          <text class="empty__icon i-mingcute-message-3-line" />
          <text class="empty__text">还没有私信</text>
        </view>
        <view
          v-for="c in conversations"
          :key="c.peer_id"
          class="conv"
          @tap="openConversation(c.peer_id)"
        >
          <image class="conv__avatar" :src="c.avatar || DEFAULT_AVATAR" mode="aspectFill" />
          <view class="conv__body">
            <view class="conv__top">
              <text class="conv__name">{{ c.nickname }}</text>
              <text class="conv__time">{{ c.last_at }}</text>
            </view>
            <text class="conv__last">{{ c.last_content }}</text>
          </view>
          <text v-if="c.unread > 0" class="conv__badge">{{ c.unread }}</text>
        </view>
      </view>
    </template>
  </view>
</template>

<style lang="scss">
@use '../../styles/variables' as v;

.messages {
  min-height: 100vh;
  background: v.$bg;
}

.tab-bar {
  display: flex;
  align-items: center;
  background: v.$surface;
  border-bottom: 1rpx solid v.$border;
  position: sticky;
  top: 0;
  z-index: 5;
}

.tab {
  display: flex;
  align-items: center;
  gap: 8rpx;
  padding: 24rpx 32rpx;
  font-size: 28rpx;
  color: v.$text-3;
  position: relative;
}

.tab.active {
  color: v.$primary;
  font-weight: 600;
}

.tab.active::after {
  content: '';
  position: absolute;
  left: 32rpx;
  right: 32rpx;
  bottom: 0;
  height: 4rpx;
  background: v.$primary;
  border-radius: 2rpx;
}

.badge {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-width: 32rpx;
  height: 32rpx;
  padding: 0 8rpx;
  border-radius: 16rpx;
  background: v.$primary;
  color: #fff;
  font-size: 20rpx;
  font-weight: 600;
}

.tab__action {
  margin-left: auto;
  margin-right: 24rpx;
  font-size: 24rpx;
  color: v.$primary;
}

.row {
  display: flex;
  gap: 20rpx;
  padding: 24rpx;
  background: v.$surface;
  border-bottom: 1rpx solid v.$border;
  position: relative;
}

.row.unread {
  background: rgba(v.$primary, 0.04);
}

.row__type {
  flex-shrink: 0;
  width: 60rpx;
  height: 60rpx;
  line-height: 60rpx;
  text-align: center;
  border-radius: 50%;
  font-size: 22rpx;
  color: #fff;
}

.row__type--t1 {
  background: #ff7f50;
}
.row__type--t2 {
  background: #5c8cff;
}
.row__type--t3 {
  background: #ff5ca8;
}
.row__type--t4 {
  background: #999;
}

.row__body {
  flex: 1;
  min-width: 0;
}

.row__text {
  display: block;
  font-size: 28rpx;
  color: v.$text-1;
  line-height: 1.5;
}

.row__time {
  display: block;
  margin-top: 8rpx;
  font-size: 22rpx;
  color: v.$text-3;
}

.row__dot {
  position: absolute;
  top: 28rpx;
  right: 24rpx;
  width: 16rpx;
  height: 16rpx;
  border-radius: 50%;
  background: v.$primary;
}

.conv {
  display: flex;
  align-items: center;
  gap: 20rpx;
  padding: 24rpx;
  background: v.$surface;
  border-bottom: 1rpx solid v.$border;
}

.conv__avatar {
  width: 88rpx;
  height: 88rpx;
  border-radius: 50%;
  background: v.$primary;
  flex-shrink: 0;
}

.conv__body {
  flex: 1;
  min-width: 0;
}

.conv__top {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.conv__name {
  font-size: 28rpx;
  font-weight: 600;
}

.conv__time {
  font-size: 22rpx;
  color: v.$text-3;
}

.conv__last {
  display: block;
  margin-top: 8rpx;
  font-size: 26rpx;
  color: v.$text-3;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.conv__badge {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-width: 32rpx;
  height: 32rpx;
  padding: 0 8rpx;
  border-radius: 16rpx;
  background: v.$primary;
  color: #fff;
  font-size: 20rpx;
}

.empty {
  text-align: center;
  padding: 120rpx 32rpx;
}

.empty__icon {
  display: block;
  font-size: 96rpx;
  color: v.$text-3;
  margin-bottom: 24rpx;
}

.empty__text {
  display: block;
  font-size: 28rpx;
  color: v.$text-3;
}

.tip {
  text-align: center;
  padding: 32rpx 0;
  font-size: 24rpx;
  color: v.$text-3;
}
</style>

<script setup lang="ts">
import { ref } from 'vue'
import { onShow } from '@dcloudio/uni-app'
import { formatCount, formatDuration } from '@dlidli/shared'
import { ApiError, type FeedItem } from '@dlidli/api-client'
import { api, hasLogin } from '@/api'

const DEFAULT_AVATAR = '/static/default-avatar.png'
const DEFAULT_COVER = '/static/default-cover.png'
const PAGE_SIZE = 20

const loggedIn = ref(false)
const list = ref<FeedItem[]>([])
const cursor = ref('')
const hasMore = ref(true)
const loading = ref(false)
const refreshing = ref(false)
const input = ref('')
const posting = ref(false)

/** 动态类型文案 */
function typeLabel(t: FeedItem['type']): string {
  return t === 3 ? '转发视频' : t === 1 ? '投稿' : '动态'
}

async function loadFeed(reset = true) {
  if (!loggedIn.value) return
  if (reset) {
    cursor.value = ''
    hasMore.value = true
    loading.value = true
  }
  try {
    const res = await api.dynamic.feed(cursor.value, PAGE_SIZE)
    list.value = reset ? res.list : [...list.value, ...res.list]
    cursor.value = res.next_cursor
    hasMore.value = res.has_more
  } catch (e) {
    uni.showToast({ title: e instanceof ApiError ? e.message : '加载失败', icon: 'none' })
  } finally {
    loading.value = false
  }
}

/** 列表顶部下拉刷新 */
async function onRefresh() {
  refreshing.value = true
  try {
    await loadFeed(true)
  } finally {
    refreshing.value = false
  }
}

/** 触底加载更多 */
function onScrollLower() {
  if (!hasMore.value || loading.value) return
  loadFeed(false)
}

async function postDynamic() {
  const content = input.value.trim()
  if (!content) {
    uni.showToast({ title: '说点什么吧', icon: 'none' })
    return
  }
  posting.value = true
  try {
    await api.dynamic.post(content)
    input.value = ''
    uni.showToast({ title: '已发布', icon: 'none' })
    await loadFeed(true)
  } catch (e) {
    uni.showToast({ title: e instanceof ApiError ? e.message : '发布失败', icon: 'none' })
  } finally {
    posting.value = false
  }
}

function goVideo(bvid: string) {
  uni.navigateTo({ url: '/pages/video/video?bvid=' + bvid })
}

function goSpace(uid: string) {
  uni.navigateTo({ url: '/pages/space/space?uid=' + uid })
}

onShow(() => {
  loggedIn.value = hasLogin()
  if (loggedIn.value && list.value.length === 0) loadFeed(true)
})
</script>

<template>
  <view class="feed">
    <!-- 未登录 -->
    <view v-if="!loggedIn" class="empty">
      <text class="empty__icon i-mingcute-send-line" />
      <text class="empty__text">请先登录后查看动态</text>
    </view>

    <template v-else>
      <!-- 顶部操作栏：发动态输入（固定，不参与滚动） -->
      <view class="composer">
        <input
          v-model="input"
          class="composer__input"
          placeholder="分享新鲜事…"
          :disabled="posting"
          @confirm="postDynamic"
        />
        <button class="composer__send" :disabled="posting || !input.trim()" @tap="postDynamic">
          发布
        </button>
      </view>

      <!-- 动态流：独立滚动区域 -->
      <scroll-view
        scroll-y
        class="feed-scroll"
        :refresher-enabled="true"
        :refresher-triggered="refreshing"
        @refresherrefresh="onRefresh"
        @scrolltolower="onScrollLower"
      >
        <view v-if="loading && list.length === 0" class="tip">加载中…</view>
        <view v-else-if="list.length === 0" class="empty">
          <text class="empty__icon i-mingcute-send-line" />
          <text class="empty__text">还没有动态，关注更多 UP 主吧</text>
        </view>

        <view v-for="item in list" :key="item.id" class="dyn">
          <!-- 用户信息 -->
          <view class="dyn__head" @tap="goSpace(item.user.id)">
            <image
              class="dyn__avatar"
              :src="item.user.avatar || DEFAULT_AVATAR"
              mode="aspectFill"
            />
            <view class="dyn__info">
              <text class="dyn__name">{{ item.user.nickname }}</text>
              <text class="dyn__meta">{{ typeLabel(item.type) }} · {{ item.created_at }}</text>
            </view>
          </view>

          <!-- 正文 -->
          <text v-if="item.content" class="dyn__content">{{ item.content }}</text>

          <!-- 转发视频卡片 -->
          <view v-if="item.video" class="video-card" @tap="item.video && goVideo(item.video.bvid)">
            <view class="video-card__cover">
              <image
                class="video-card__img"
                :src="item.video.cover || DEFAULT_COVER"
                mode="aspectFill"
              />
              <text v-if="item.video.duration > 0" class="video-card__dur">{{
                formatDuration(item.video.duration)
              }}</text>
            </view>
            <view class="video-card__body">
              <text class="video-card__title">{{ item.video.title }}</text>
              <text class="video-card__views">{{ formatCount(item.video.stat.view) }}观看</text>
            </view>
          </view>
        </view>

        <view v-if="!hasMore && list.length > 0" class="tip">没有更多了</view>
      </scroll-view>
    </template>
  </view>
</template>

<style lang="scss">
@use '../../styles/variables' as v;

.feed {
  height: 100vh;
  overflow: hidden;
  display: flex;
  flex-direction: column;
  background: v.$bg;
}

/* 发动态输入栏（固定） */
.composer {
  flex-shrink: 0;
  display: flex;
  gap: 16rpx;
  padding: 20rpx 24rpx;
  background: v.$surface;
  border-bottom: 1rpx solid v.$border;
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
  padding: 0 32rpx;
  border-radius: 36rpx;
  background: v.$primary;
  color: #fff;
  font-size: 26rpx;
  white-space: nowrap;
}

.composer__send[disabled] {
  opacity: 0.5;
}

/* 动态流独立滚动区 */
.feed-scroll {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  -webkit-overflow-scrolling: touch;
}

.dyn {
  margin: 20rpx 24rpx;
  padding: 24rpx;
  background: v.$surface;
  border-radius: 16rpx;
}

.dyn__head {
  display: flex;
  align-items: center;
  gap: 16rpx;
}

.dyn__avatar {
  width: 72rpx;
  height: 72rpx;
  border-radius: 50%;
  background: v.$primary;
  flex-shrink: 0;
}

.dyn__info {
  flex: 1;
  min-width: 0;
}

.dyn__name {
  display: block;
  font-size: 28rpx;
  font-weight: 600;
}

.dyn__meta {
  display: block;
  margin-top: 4rpx;
  font-size: 22rpx;
  color: v.$text-3;
}

.dyn__content {
  display: block;
  margin-top: 16rpx;
  font-size: 28rpx;
  line-height: 1.6;
  white-space: pre-wrap;
}

/* 转发视频卡片 */
.video-card {
  display: flex;
  gap: 16rpx;
  margin-top: 16rpx;
  padding: 16rpx;
  background: v.$bg;
  border-radius: 12rpx;
}

.video-card__cover {
  position: relative;
  width: 200rpx;
  height: 120rpx;
  border-radius: 8rpx;
  overflow: hidden;
  flex-shrink: 0;
}

.video-card__img {
  width: 100%;
  height: 100%;
}

.video-card__dur {
  position: absolute;
  right: 8rpx;
  bottom: 8rpx;
  padding: 2rpx 8rpx;
  border-radius: 4rpx;
  background: rgba(0, 0, 0, 0.6);
  color: #fff;
  font-size: 20rpx;
}

.video-card__body {
  flex: 1;
  min-width: 0;
}

.video-card__title {
  display: block;
  font-size: 26rpx;
  line-height: 1.4;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.video-card__views {
  display: block;
  margin-top: 12rpx;
  font-size: 22rpx;
  color: v.$text-3;
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

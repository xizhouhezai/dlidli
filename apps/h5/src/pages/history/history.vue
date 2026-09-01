<script setup lang="ts">
import { ref } from 'vue'
import { onShow } from '@dcloudio/uni-app'
import { ApiError, type VideoCard } from '@dlidli/api-client'
import { api, hasLogin } from '@/api'
import { formatCount, formatDuration } from '@dlidli/shared'

const DEFAULT_COVER = '/static/default-cover.svg'
const PAGE_SIZE = 20

const loggedIn = ref(false)
const list = ref<VideoCard[]>([])
const page = ref(1)
const hasMore = ref(true)
const loading = ref(false)

async function load(reset = true) {
  if (!loggedIn.value) return
  if (reset) {
    page.value = 1
    hasMore.value = true
    loading.value = true
  }
  try {
    const res = await api.video.history(page.value, PAGE_SIZE)
    list.value = reset ? res.list : [...list.value, ...res.list]
    hasMore.value = res.list.length === PAGE_SIZE
  } catch (e) {
    uni.showToast({ title: e instanceof ApiError ? e.message : '加载失败', icon: 'none' })
  } finally {
    loading.value = false
  }
}

function goVideo(bvid: string) {
  uni.navigateTo({ url: '/pages/video/video?bvid=' + bvid })
}

onShow(() => {
  loggedIn.value = hasLogin()
  if (loggedIn.value) load(true)
})
</script>

<template>
  <view class="history">
    <view v-if="!loggedIn" class="empty">
      <text class="empty__icon i-mingcute-history-line" />
      <text class="empty__text">请先登录后查看观看历史</text>
    </view>

    <template v-else>
      <view v-if="loading && list.length === 0" class="empty">加载中…</view>
      <view v-else-if="list.length === 0" class="empty">
        <text class="empty__icon i-mingcute-history-line" />
        <text class="empty__text">还没有观看记录</text>
      </view>
      <view v-for="v in list" :key="v.bvid" class="row" @tap="goVideo(v.bvid)">
        <view class="row__cover">
          <image class="row__img" :src="v.cover || DEFAULT_COVER" mode="aspectFill" />
          <text v-if="v.duration > 0" class="row__dur">{{ formatDuration(v.duration) }}</text>
        </view>
        <view class="row__info">
          <text class="row__title">{{ v.title }}</text>
          <text class="row__meta">{{ v.owner.nickname }} · {{ formatCount(v.stat.view) }}观看</text>
        </view>
      </view>
      <view v-if="!hasMore && list.length > 0" class="tip">没有更多了</view>
    </template>
  </view>
</template>

<style lang="scss">
@use '../../styles/variables' as v;

.history {
  min-height: 100vh;
  background: v.$bg;
}

.row {
  display: flex;
  gap: 20rpx;
  padding: 20rpx 24rpx;
  background: v.$surface;
  border-bottom: 1rpx solid v.$border;
}

.row__cover {
  position: relative;
  width: 240rpx;
  height: 140rpx;
  border-radius: 8rpx;
  overflow: hidden;
  flex-shrink: 0;
}

.row__img {
  width: 100%;
  height: 100%;
}

.row__dur {
  position: absolute;
  right: 8rpx;
  bottom: 8rpx;
  padding: 2rpx 8rpx;
  border-radius: 4rpx;
  background: rgba(0, 0, 0, 0.6);
  color: #fff;
  font-size: 20rpx;
}

.row__info {
  flex: 1;
  min-width: 0;
}

.row__title {
  display: block;
  font-size: 28rpx;
  line-height: 1.4;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.row__meta {
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

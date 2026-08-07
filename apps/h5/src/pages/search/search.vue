<script setup lang="ts">
import { ref } from 'vue'
import { formatCount, formatDuration } from '@dlidli/shared'
import type { VideoCard, UserBrief } from '@dlidli/api-client'
import { api } from '@/api'

const DEFAULT_COVER = '/static/default-cover.svg'
const DEFAULT_AVATAR = '/static/default-avatar.png'

const keyword = ref('')
const activeTab = ref<'video' | 'user'>('video')
const videos = ref<VideoCard[]>([])
const users = ref<UserBrief[]>([])
const loading = ref(false)
const searched = ref(false)

async function doSearch() {
  const kw = keyword.value.trim()
  if (!kw) return
  loading.value = true
  searched.value = true
  try {
    if (activeTab.value === 'video') {
      const res = await api.search.videos(kw)
      videos.value = res.list
      users.value = []
    } else {
      const res = await api.search.users(kw)
      users.value = res.list
      videos.value = []
    }
  } catch {
    videos.value = []
    users.value = []
  } finally {
    loading.value = false
  }
}

function switchTab(tab: 'video' | 'user') {
  if (activeTab.value === tab) return
  activeTab.value = tab
  if (searched.value) doSearch()
}

function openVideo(bvid: string) {
  uni.navigateTo({ url: `/pages/video/video?bvid=${bvid}` })
}

function openSpace(_id: string) {
  uni.showToast({ title: '空间页待开发', icon: 'none' })
}
</script>

<template>
  <view class="search-page">
    <!-- 搜索栏 -->
    <view class="search-bar">
      <input
        v-model="keyword"
        class="search-bar__input"
        placeholder="搜索视频、UP 主"
        confirm-type="search"
        @confirm="doSearch"
      >
      <view
        class="search-bar__btn"
        @tap="doSearch"
      >
        <text class="i-mingcute-search-2-line" />
      </view>
    </view>

    <!-- Tab -->
    <view class="tabs">
      <text
        class="tab"
        :class="{ active: activeTab === 'video' }"
        @tap="switchTab('video')"
      >视频</text>
      <text
        class="tab"
        :class="{ active: activeTab === 'user' }"
        @tap="switchTab('user')"
      >用户</text>
    </view>

    <!-- 加载中 -->
    <view
      v-if="loading"
      class="tip"
    >搜索中…</view>

    <!-- 视频结果 -->
    <template v-else-if="activeTab === 'video'">
      <view
        v-if="searched && videos.length === 0"
        class="tip"
      >没有找到相关视频</view>
      <view
        v-for="v in videos"
        :key="v.bvid"
        class="v-card"
        @tap="openVideo(v.bvid)"
      >
        <view class="v-card__cover">
          <image
            class="v-card__img"
            :src="v.cover || DEFAULT_COVER"
            mode="aspectFill"
          />
          <text
            v-if="v.duration > 0"
            class="v-card__dur"
          >{{ formatDuration(v.duration) }}</text>
        </view>
        <view class="v-card__info">
          <text class="v-card__title">{{ v.title }}</text>
          <text class="v-card__meta"><text class="i-mingcute-play-circle-line align-middle mr-1" />{{ formatCount(v.stat.view) }} · {{ v.owner.nickname }}</text>
        </view>
      </view>
    </template>

    <!-- 用户结果 -->
    <template v-else>
      <view
        v-if="searched && users.length === 0"
        class="tip"
      >没有找到相关用户</view>
      <view
        v-for="u in users"
        :key="u.id"
        class="u-card"
        @tap="openSpace(u.id)"
      >
        <image
          class="u-card__avatar"
          :src="u.avatar || DEFAULT_AVATAR"
          mode="aspectFill"
        />
        <view class="u-card__info">
          <text class="u-card__name">{{ u.nickname }}</text>
          <text class="u-card__sign">{{ u.signature || 'TA 还没有签名' }}</text>
        </view>
      </view>
    </template>
  </view>
</template>

<style lang="scss">
@use '../../styles/variables' as v;

.search-page {
  min-height: 100vh;
  background: v.$bg;
}

.search-bar {
  display: flex;
  align-items: center;
  gap: 16rpx;
  padding: 16rpx 24rpx;
  background: v.$surface;
}

.search-bar__input {
  flex: 1;
  height: 64rpx;
  padding: 0 24rpx;
  border-radius: 32rpx;
  background: v.$bg;
  font-size: 28rpx;
}

.search-bar__btn {
  font-size: 36rpx;
  padding: 8rpx;
}

.tabs {
  display: flex;
  gap: 32rpx;
  padding: 20rpx 24rpx;
}

.tab {
  font-size: 28rpx;
  color: v.$text-2;
  padding-bottom: 8rpx;
}

.tab.active {
  color: v.$primary;
  font-weight: 600;
  border-bottom: 4rpx solid v.$primary;
}

.tip {
  padding: 80rpx 0;
  text-align: center;
  font-size: 26rpx;
  color: v.$text-3;
}

/* 视频卡片（横排） */
.v-card {
  display: flex;
  gap: 20rpx;
  padding: 20rpx 24rpx;
  background: v.$surface;
  margin-bottom: 2rpx;
}

.v-card__cover {
  position: relative;
  width: 240rpx;
  height: 150rpx;
  border-radius: 12rpx;
  overflow: hidden;
  flex-shrink: 0;
  background: v.$border;
}

.v-card__img {
  width: 100%;
  height: 100%;
}

.v-card__dur {
  position: absolute;
  right: 8rpx;
  bottom: 8rpx;
  padding: 2rpx 10rpx;
  font-size: 20rpx;
  color: #fff;
  background: rgba(0, 0, 0, 0.6);
  border-radius: 6rpx;
}

.v-card__info {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  justify-content: space-between;
}

.v-card__title {
  font-size: 28rpx;
  line-height: 1.4;
  color: v.$text-1;
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
}

.v-card__meta {
  font-size: 24rpx;
  color: v.$text-3;
}

/* 用户卡片 */
.u-card {
  display: flex;
  align-items: center;
  gap: 20rpx;
  padding: 24rpx;
  background: v.$surface;
  margin-bottom: 2rpx;
}

.u-card__avatar {
  width: 80rpx;
  height: 80rpx;
  border-radius: 50%;
  background: v.$primary;
  flex-shrink: 0;
}

.u-card__info {
  flex: 1;
  min-width: 0;
}

.u-card__name {
  font-size: 28rpx;
  font-weight: 600;
  color: v.$text-1;
}

.u-card__sign {
  display: block;
  margin-top: 6rpx;
  font-size: 24rpx;
  color: v.$text-3;
  overflow: hidden;
  white-space: nowrap;
  text-overflow: ellipsis;
}
</style>

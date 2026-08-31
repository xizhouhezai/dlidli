<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { onPullDownRefresh, onReachBottom } from '@dcloudio/uni-app'
import { ApiError, type UserBrief, type VideoCard } from '@dlidli/api-client'
import { api } from '@/api'
import { formatCount, formatDuration } from '@dlidli/shared'

const DEFAULT_AVATAR = '/static/default-avatar.png'
const DEFAULT_COVER = '/static/default-cover.svg'
const PAGE_SIZE = 10

const uid = ref('')
const profile = ref<UserBrief | null>(null)
const stat = ref<{ follower_cnt: number; following: boolean } | null>(null)
const videos = ref<VideoCard[]>([])
const page = ref(1)
const hasMore = ref(true)
const loading = ref(true)

async function loadProfile() {
  if (!uid.value) return
  try {
    const [p, s] = await Promise.all([
      api.relation.profile(uid.value),
      api.relation.stat(uid.value),
    ])
    profile.value = p
    stat.value = { follower_cnt: s.follower_cnt, following: s.following }
  } catch (e) {
    uni.showToast({ title: e instanceof ApiError ? e.message : '加载失败', icon: 'none' })
  }
}

async function loadVideos(reset = true) {
  if (!uid.value) return
  if (reset) {
    page.value = 1
    hasMore.value = true
    loading.value = true
  }
  try {
    const res = await api.video.list({
      uid: uid.value,
      sort: 'new',
      page: page.value,
      page_size: PAGE_SIZE,
    })
    videos.value = reset ? res.list : [...videos.value, ...res.list]
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

onMounted(() => {
  const pages = getCurrentPages()
  const cur: any = pages[pages.length - 1]
  const q = cur?.options ?? cur?.$page?.options ?? {}
  uid.value = (q.uid as string) || ''
  if (uid.value) {
    loadProfile()
    loadVideos(true)
  }
})

onPullDownRefresh(async () => {
  await loadVideos(true)
  uni.stopPullDownRefresh()
})

onReachBottom(() => {
  if (!hasMore.value || loading.value) return
  page.value++
  loadVideos(false)
})
</script>

<template>
  <view class="space">
    <!-- UP 主卡片 -->
    <view class="up">
      <image class="up__avatar" :src="profile?.avatar || DEFAULT_AVATAR" mode="aspectFill" />
      <view class="up__info">
        <text class="up__name">{{ profile?.nickname }}</text>
        <text class="up__sig">{{ profile?.signature || '这个人很懒，什么都没留下' }}</text>
        <text class="up__meta">
          <text v-if="profile">Lv{{ profile.level }} · </text>
          {{ stat ? formatCount(stat.follower_cnt) + ' 粉丝' : '' }}
        </text>
      </view>
    </view>

    <!-- 投稿 -->
    <view class="grid">
      <view v-if="loading && videos.length === 0" class="tip">加载中…</view>
      <view v-else-if="videos.length === 0" class="tip">还没有投稿</view>
      <view v-for="v in videos" :key="v.bvid" class="card" @tap="goVideo(v.bvid)">
        <view class="card__cover">
          <image class="card__img" :src="v.cover || DEFAULT_COVER" mode="aspectFill" />
          <text v-if="v.duration > 0" class="card__dur">{{ formatDuration(v.duration) }}</text>
        </view>
        <text class="card__title">{{ v.title }}</text>
        <text class="card__views">{{ formatCount(v.stat.view) }}观看</text>
      </view>
    </view>
    <view v-if="!hasMore && videos.length > 0" class="tip">没有更多了</view>
  </view>
</template>

<style lang="scss">
@use '../../styles/variables' as v;

.space {
  min-height: 100vh;
  background: v.$bg;
}

.up {
  display: flex;
  gap: 24rpx;
  padding: 32rpx 24rpx;
  background: v.$surface;
}

.up__avatar {
  width: 128rpx;
  height: 128rpx;
  border-radius: 50%;
  background: v.$primary;
  flex-shrink: 0;
}

.up__info {
  flex: 1;
  min-width: 0;
}

.up__name {
  display: block;
  font-size: 36rpx;
  font-weight: 600;
}

.up__sig {
  display: block;
  margin-top: 8rpx;
  font-size: 24rpx;
  color: v.$text-3;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.up__meta {
  display: block;
  margin-top: 12rpx;
  font-size: 24rpx;
  color: v.$text-2;
}

.grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 20rpx;
  padding: 20rpx 24rpx;
}

.card {
  background: v.$surface;
  border-radius: 12rpx;
  overflow: hidden;
}

.card__cover {
  position: relative;
  width: 100%;
  height: 180rpx;
}

.card__img {
  width: 100%;
  height: 100%;
}

.card__dur {
  position: absolute;
  right: 8rpx;
  bottom: 8rpx;
  padding: 2rpx 8rpx;
  border-radius: 4rpx;
  background: rgba(0, 0, 0, 0.6);
  color: #fff;
  font-size: 20rpx;
}

.card__title {
  display: block;
  font-size: 26rpx;
  padding: 12rpx 16rpx;
  line-height: 1.4;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.card__views {
  display: block;
  padding: 0 16rpx 16rpx;
  font-size: 22rpx;
  color: v.$text-3;
}

.tip {
  text-align: center;
  padding: 32rpx 0;
  font-size: 24rpx;
  color: v.$text-3;
}
</style>

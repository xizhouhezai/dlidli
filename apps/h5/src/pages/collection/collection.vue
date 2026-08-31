<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { onShow } from '@dcloudio/uni-app'
import { ApiError, type VideoCard, type CollectionItem } from '@dlidli/api-client'
import { api, hasLogin } from '@/api'
import { formatCount, formatDuration } from '@dlidli/shared'

const DEFAULT_COVER = '/static/default-cover.svg'
const PAGE_SIZE = 20

const tab = ref<'fav' | 'folder'>('fav')
const loggedIn = ref(false)

const favList = ref<VideoCard[]>([])
const favPage = ref(1)
const favHasMore = ref(true)
const favLoading = ref(false)

const folders = ref<CollectionItem[]>([])
const folderLoading = ref(false)

async function loadFavorites(reset = true) {
  if (!loggedIn.value) return
  if (reset) {
    favPage.value = 1
    favHasMore.value = true
    favLoading.value = true
  }
  try {
    const res = await api.interaction.favorites(favPage.value, PAGE_SIZE)
    favList.value = reset ? res.list : [...favList.value, ...res.list]
    favHasMore.value = res.list.length === PAGE_SIZE
  } catch (e) {
    uni.showToast({ title: e instanceof ApiError ? e.message : '加载失败', icon: 'none' })
  } finally {
    favLoading.value = false
  }
}

async function loadFolders() {
  if (!loggedIn.value) return
  folderLoading.value = true
  try {
    folders.value = await api.interaction.listCollections()
  } catch (e) {
    uni.showToast({ title: e instanceof ApiError ? e.message : '加载失败', icon: 'none' })
  } finally {
    folderLoading.value = false
  }
}

function switchTab(t: 'fav' | 'folder') {
  tab.value = t
  if (t === 'folder' && folders.value.length === 0) loadFolders()
}

function goVideo(bvid: string) {
  uni.navigateTo({ url: '/pages/video/video?bvid=' + bvid })
}

onMounted(() => {
  loggedIn.value = hasLogin()
  if (loggedIn.value) {
    loadFavorites(true)
  }
})

onShow(() => {
  if (loggedIn.value) loadFavorites(true)
})
</script>

<template>
  <view class="colpage">
    <view v-if="!loggedIn" class="empty">
      <text class="empty__icon i-mingcute-star-line" />
      <text class="empty__text">请先登录后查看收藏</text>
    </view>

    <template v-else>
      <view class="tab-bar">
        <view class="tab" :class="{ active: tab === 'fav' }" @tap="switchTab('fav')">收藏视频</view>
        <view class="tab" :class="{ active: tab === 'folder' }" @tap="switchTab('folder')"
          >收藏夹</view
        >
      </view>

      <!-- 收藏视频 -->
      <view v-if="tab === 'fav'">
        <view v-if="favLoading && favList.length === 0" class="empty">加载中…</view>
        <view v-else-if="favList.length === 0" class="empty">
          <text class="empty__icon i-mingcute-star-line" />
          <text class="empty__text">还没有收藏视频</text>
        </view>
        <view v-for="v in favList" :key="v.bvid" class="card" @tap="goVideo(v.bvid)">
          <view class="card__cover">
            <image class="card__img" :src="v.cover || DEFAULT_COVER" mode="aspectFill" />
            <text v-if="v.duration > 0" class="card__dur">{{ formatDuration(v.duration) }}</text>
          </view>
          <text class="card__title">{{ v.title }}</text>
          <view class="card__meta">
            <text class="card__views">{{ formatCount(v.stat.view) }}观看</text>
            <text class="card__up">{{ v.owner.nickname }}</text>
          </view>
        </view>
        <view v-if="!favHasMore && favList.length > 0" class="tip">没有更多了</view>
      </view>

      <!-- 收藏夹 -->
      <view v-else>
        <view v-if="folderLoading && folders.length === 0" class="empty">加载中…</view>
        <view v-else-if="folders.length === 0" class="empty">
          <text class="empty__icon i-mingcute-folder-line" />
          <text class="empty__text">还没有收藏夹</text>
        </view>
        <view v-for="c in folders" :key="c.id" class="folder" @tap="switchTab('fav')">
          <text class="folder__icon i-mingcute-folder-2-line" />
          <text class="folder__name">{{ c.name }}</text>
          <text class="folder__meta">{{ c.is_default === 1 ? '默认' : '' }}</text>
        </view>
      </view>
    </template>
  </view>
</template>

<style lang="scss">
@use '../../styles/variables' as v;

.colpage {
  min-height: 100vh;
  background: v.$bg;
}

.tab-bar {
  display: flex;
  background: v.$surface;
  border-bottom: 1rpx solid v.$border;
  position: sticky;
  top: 0;
}

.tab {
  flex: 1;
  text-align: center;
  padding: 24rpx 0;
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
  left: 40rpx;
  right: 40rpx;
  bottom: 0;
  height: 4rpx;
  background: v.$primary;
  border-radius: 2rpx;
}

.card {
  margin: 20rpx 24rpx;
  background: v.$surface;
  border-radius: 12rpx;
  overflow: hidden;
}

.card__cover {
  position: relative;
  width: 100%;
  height: 360rpx;
}

.card__img {
  width: 100%;
  height: 100%;
}

.card__dur {
  position: absolute;
  right: 16rpx;
  bottom: 16rpx;
  padding: 4rpx 12rpx;
  border-radius: 6rpx;
  background: rgba(0, 0, 0, 0.6);
  color: #fff;
  font-size: 22rpx;
}

.card__title {
  display: block;
  font-size: 28rpx;
  padding: 16rpx 20rpx 4rpx;
  line-height: 1.4;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.card__meta {
  display: flex;
  justify-content: space-between;
  padding: 0 20rpx 20rpx;
  font-size: 22rpx;
  color: v.$text-3;
}

.folder {
  display: flex;
  align-items: center;
  gap: 20rpx;
  margin: 20rpx 24rpx;
  padding: 28rpx 24rpx;
  background: v.$surface;
  border-radius: 12rpx;
}

.folder__icon {
  font-size: 44rpx;
  color: v.$primary;
}

.folder__name {
  flex: 1;
  font-size: 28rpx;
}

.folder__meta {
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

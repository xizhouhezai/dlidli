<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { onPullDownRefresh, onReachBottom } from '@dcloudio/uni-app'
import { formatCount, formatDuration } from '@dlidli/shared'
import type { CategoryItem, VideoCard } from '@dlidli/api-client'
import { api } from '@/api'

const PAGE_SIZE = 10
const DEFAULT_COVER = '/static/default-cover.svg'
const DEFAULT_AVATAR = '/static/default-avatar.png'

const categories = ref<CategoryItem[]>([])
const activeCategory = ref(0)
const sort = ref<'new' | 'hot'>('new')
const videos = ref<VideoCard[]>([])
const page = ref(1)
const hasMore = ref(true)
const loading = ref(true)

async function loadList(reset = true) {
  if (reset) {
    page.value = 1
    hasMore.value = true
    loading.value = true
  }
  try {
    const res = await api.video.list({
      category_id: activeCategory.value || undefined,
      sort: sort.value,
      page: page.value,
      page_size: PAGE_SIZE,
    })
    videos.value = reset ? res.list : [...videos.value, ...res.list]
    hasMore.value = res.list.length === PAGE_SIZE
  } catch {
    // 静默失败
  } finally {
    loading.value = false
  }
}

onMounted(async () => {
  try {
    categories.value = await api.video.categories()
  } catch {
    categories.value = []
  }
  loadList()
})

onPullDownRefresh(async () => {
  await loadList(true)
  uni.stopPullDownRefresh()
})

onReachBottom(() => {
  if (!hasMore.value || loading.value) return
  page.value++
  loadList(false)
})

function pickCategory(id: number) {
  if (activeCategory.value === id) return
  activeCategory.value = id
  loadList()
}

function switchSort(s: 'new' | 'hot') {
  if (sort.value === s) return
  sort.value = s
  loadList()
}

function openVideo(bvid: string) {
  uni.navigateTo({ url: `/pages/video/video?bvid=${bvid}` })
}

function goSearch() {
  uni.navigateTo({ url: '/pages/search/search' })
}
</script>

<template>
  <view class="home">
    <!-- 搜索入口 -->
    <view
      class="search-entry"
      @tap="goSearch"
    >
      <text class="search-entry__icon i-mingcute-search-2-line" />
      <text class="search-entry__text">搜索视频、UP 主</text>
    </view>

    <!-- 分区 + 排序 -->
    <scroll-view
      class="cat-bar"
      scroll-x
      :show-scrollbar="false"
    >
      <text
        class="cat-chip"
        :class="{ active: activeCategory === 0 }"
        @tap="pickCategory(0)"
      >推荐</text>
      <text
        v-for="c in categories"
        :key="c.id"
        class="cat-chip"
        :class="{ active: activeCategory === c.id }"
        @tap="pickCategory(c.id)"
      >{{ c.name }}</text>
    </scroll-view>

    <view class="sort-bar">
      <text
        class="sort-tab"
        :class="{ active: sort === 'new' }"
        @tap="switchSort('new')"
      >最新</text>
      <text
        class="sort-tab"
        :class="{ active: sort === 'hot' }"
        @tap="switchSort('hot')"
      >最热</text>
    </view>

    <!-- 视频双列网格 -->
    <view class="grid">
      <view
        v-for="v in videos"
        :key="v.bvid"
        class="card"
        @tap="openVideo(v.bvid)"
      >
        <view class="card__cover">
          <image
            class="card__img"
            :src="v.cover || DEFAULT_COVER"
            mode="aspectFill"
          />
          <text
            v-if="v.duration > 0"
            class="card__dur"
          >{{ formatDuration(v.duration) }}</text>
        </view>
        <text class="card__title">{{ v.title }}</text>
        <view class="card__meta">
          <image
            class="card__avatar"
            :src="v.owner.avatar || DEFAULT_AVATAR"
            mode="aspectFill"
          />
          <text class="card__up">{{ v.owner.nickname }}</text>
          <text class="card__views">{{ formatCount(v.stat.view) }}观看</text>
        </view>
      </view>
    </view>

    <view
      v-if="loading"
      class="tip"
    >加载中…</view>
    <view
      v-else-if="videos.length === 0"
      class="tip"
    >这里还没有视频</view>
    <view
      v-else-if="!hasMore"
      class="tip"
    >没有更多了</view>
  </view>
</template>

<style lang="scss">
@use '../../styles/variables' as v;

.home {
  min-height: 100vh;
  background: v.$bg;
}

/* 搜索入口 */
.search-entry {
  display: flex;
  align-items: center;
  gap: 12rpx;
  margin: 16rpx 24rpx;
  padding: 16rpx 24rpx;
  border-radius: 32rpx;
  background: v.$surface;
  box-shadow: 0 2rpx 8rpx rgba(0, 0, 0, 0.06);
}

.search-entry__icon {
  font-size: 28rpx;
}

.search-entry__text {
  font-size: 26rpx;
  color: v.$text-3;
}

/* 分区 */
.cat-bar {
  white-space: nowrap;
  padding: 16rpx 20rpx;
  background: v.$surface;
}

.cat-chip {
  display: inline-block;
  padding: 10rpx 28rpx;
  margin-right: 16rpx;
  font-size: 26rpx;
  color: v.$text-2;
  background: #f1f2f3;
  border-radius: 28rpx;
}

.cat-chip.active {
  color: #fff;
  background: v.$primary;
  font-weight: 600;
}

.sort-bar {
  display: flex;
  gap: 32rpx;
  padding: 16rpx 24rpx;
}

.sort-tab {
  font-size: 26rpx;
  color: v.$text-2;
}

.sort-tab.active {
  color: v.$primary;
  font-weight: 600;
}

/* 视频网格 */
.grid {
  display: flex;
  flex-wrap: wrap;
  padding: 0 12rpx;
}

.card {
  width: 50%;
  box-sizing: border-box;
  padding: 12rpx;
}

.card__cover {
  position: relative;
  width: 100%;
  height: 200rpx;
  border-radius: 12rpx;
  overflow: hidden;
  background: v.$border;
}

.card__img {
  width: 100%;
  height: 100%;
}

.card__dur {
  position: absolute;
  right: 10rpx;
  bottom: 10rpx;
  padding: 2rpx 10rpx;
  font-size: 22rpx;
  color: #fff;
  background: rgba(0, 0, 0, 0.6);
  border-radius: 6rpx;
}

.card__title {
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
  margin: 12rpx 0 8rpx;
  font-size: 26rpx;
  line-height: 1.4;
  color: v.$text-1;
}

.card__meta {
  display: flex;
  align-items: center;
}

.card__avatar {
  width: 32rpx;
  height: 32rpx;
  border-radius: 50%;
  background: v.$primary;
  margin-right: 8rpx;
}

.card__up {
  flex: 1;
  font-size: 22rpx;
  color: v.$text-3;
  overflow: hidden;
  white-space: nowrap;
  text-overflow: ellipsis;
}

.card__views {
  font-size: 22rpx;
  color: v.$text-3;
}

.tip {
  padding: 40rpx;
  text-align: center;
  font-size: 24rpx;
  color: #9499a0;
}
</style>

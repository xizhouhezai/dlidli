<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { formatCount, formatDuration, formatPubdate } from '@dlidli/shared'
import type { CategoryItem, VideoCard } from '@dlidli/api-client'
import { api } from '@/api'
import defaultCover from '@/assets/default-cover.svg'
import defaultAvatar from '@/assets/default-avatar.png'

const router = useRouter()

const PAGE_SIZE = 24

const categories = ref<CategoryItem[]>([])
const activeCategory = ref(0) // 0 = 全部
const sort = ref<'new' | 'hot'>('new')
const videos = ref<VideoCard[]>([])
const banners = ref<VideoCard[]>([]) // 首页推荐轮播（左侧大图）
const sideRecos = ref<VideoCard[]>([]) // 右侧 2×2 推荐网格
const page = ref(1)
const hasMore = ref(false)
const loading = ref(true)
const loadingMore = ref(false)
// 无限滚动状态
const backendDown = ref(false)

// 无限滚动：监听窗口滚动，距底 <400px 自动加载下一页
function onScroll() {
  const doc = document.documentElement
  if (doc.scrollTop + window.innerHeight >= doc.scrollHeight - 400) loadMore()
}

async function loadList(reset = true) {
  if (reset) {
    page.value = 1
    loading.value = true
  } else {
    loadingMore.value = true
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
    backendDown.value = true
  } finally {
    loading.value = false
    loadingMore.value = false
  }
}

// 触底自动加载下一页（防并发：加载中/无更多时不触发）
function loadMore() {
  if (!hasMore.value || loading.value || loadingMore.value) return
  page.value++
  loadList(false)
}

function switchSort(s: 'new' | 'hot') {
  if (sort.value === s) return
  sort.value = s
  loadList()
}

onMounted(async () => {
  try {
    categories.value = (await api.video.categories()).filter((c) => c.parent_id === 0)
  } catch {
    backendDown.value = true
  }
  loadBanners()
  await loadList()

  // 监听滚动实现无限加载（passive 不阻塞滚动）
  window.addEventListener('scroll', onScroll, { passive: true })
})

// 推荐区：取最热前 8，左侧轮播 4 张 + 右侧 2×2 网格 4 个（后续可接后台 Banner 配置 M3-OPS-01）
async function loadBanners() {
  try {
    const res = await api.video.list({ sort: 'hot', page: 1, page_size: 8 })
    banners.value = res.list.slice(0, 4)
    sideRecos.value = res.list.slice(4, 8)
  } catch {
    banners.value = []
    sideRecos.value = []
  }
}

onBeforeUnmount(() => {
  window.removeEventListener('scroll', onScroll)
})

function onCategoryClick(id: number) {
  activeCategory.value = id
  loadList()
}

function onCardClick(v: VideoCard) {
  router.push(`/video/${v.bvid}`)
}
</script>

<template>
  <div>
    <el-alert
      v-if="backendDown"
      type="error"
      title="后端服务未连接，请确认 dlidli-api（:8000）已启动"
      :closable="false"
      class="mb-4"
    />

    <!-- 分区导航 + 排序 -->
    <div class="flex flex-wrap items-center gap-2 mb-5">
      <span
        class="category-chip"
        :class="{ 'is-active': activeCategory === 0 }"
        @click="onCategoryClick(0)"
      >
        首页
      </span>
      <span
        v-for="c in categories"
        :key="c.id"
        class="category-chip"
        :class="{ 'is-active': activeCategory === c.id }"
        @click="onCategoryClick(c.id)"
      >
        {{ c.name }}
      </span>

      <span class="ml-auto flex items-center gap-2 text-3.25">
        <span
          class="sort-tab"
          :class="{ 'is-active': sort === 'new' }"
          @click="switchSort('new')"
        >最新</span>
        <span class="text-border">|</span>
        <span
          class="sort-tab"
          :class="{ 'is-active': sort === 'hot' }"
          @click="switchSort('hot')"
        >最热</span>
      </span>
    </div>

    <!-- 推荐区：左大轮播 + 右 2×2 网格（仅首页 Tab 展示） -->
    <div
      v-if="activeCategory === 0 && banners.length > 0"
      class="reco mb-5"
    >
      <el-carousel
        class="reco__carousel"
        height="320px"
        :interval="5000"
        arrow="hover"
      >
        <el-carousel-item
          v-for="b in banners"
          :key="b.bvid"
          @click="onCardClick(b)"
        >
          <div class="banner__slide">
            <img
              class="banner__img"
              :src="b.cover || defaultCover"
              :alt="b.title"
            >
            <div class="banner__mask">
              <p class="banner__title">
                {{ b.title }}
              </p>
              <p class="banner__meta flex items-center">
                <span class="i-mingcute-play-circle-line mr-1" />{{ formatCount(b.stat.view) }} · {{ b.owner.nickname }}
              </p>
            </div>
          </div>
        </el-carousel-item>
      </el-carousel>

      <!-- 右侧 2×2 推荐网格 -->
      <div class="reco__side">
        <div
          v-for="r in sideRecos"
          :key="r.bvid"
          class="reco-card"
          @click="onCardClick(r)"
        >
          <div class="reco-card__cover">
            <img
              :src="r.cover || defaultCover"
              :alt="r.title"
              loading="lazy"
            >
            <span
              v-if="r.duration > 0"
              class="absolute right-1.5 bottom-1.5 px-1.5 py-0.25 rounded-4px bg-black/60 text-white text-3"
            >{{ formatDuration(r.duration) }}</span>
          </div>
          <p class="reco-card__title">
            {{ r.title }}
          </p>
          <p class="flex items-center m-0 text-3 text-text-2">
            <span class="i-mingcute-play-circle-line mr-1" />{{ formatCount(r.stat.view) }} · {{ r.owner.nickname }}
          </p>
        </div>
      </div>
    </div>

    <!-- 稿件网格 -->
    <el-skeleton
      v-if="loading"
      :rows="6"
      animated
    />

    <el-empty
      v-else-if="videos.length === 0"
      description="这个分区还没有稿件，点击右上角「投稿」发布第一个视频吧！"
    />

    <div
      v-else
      class="video-grid"
    >
      <div
        v-for="v in videos"
        :key="v.bvid"
        class="video-card cursor-pointer"
        @click="onCardClick(v)"
      >
        <div class="video-card__cover relative aspect-video rounded-8px overflow-hidden">
          <img
            :src="v.cover || defaultCover"
            :alt="v.title"
            loading="lazy"
          >
          <span
            v-if="v.duration > 0"
            class="absolute right-1.5 bottom-1.5 px-1.5 py-0.25 rounded-4px bg-black/60 text-white text-3"
          >{{ formatDuration(v.duration) }}</span>
        </div>
        <div class="pt-2 px-0.5">
          <p
            class="video-card__title m-0 text-3.5 leading-[1.4]"
            :title="v.title"
          >
            {{ v.title }}
          </p>
          <p class="flex items-center gap-1.5 min-w-0 mt-1 mb-0 text-3 text-text-2">
            <el-avatar
              :size="20"
              :src="v.owner.avatar || defaultAvatar"
              class="shrink-0 bg-primary text-white text-3 font-600"
            >
              {{ v.owner.nickname?.slice(0, 1) ?? 'U' }}
            </el-avatar>
            <span class="video-card__up truncate">{{ v.owner.nickname }}</span>
            <span
              v-if="v.published_at"
              class="mx-1"
            >·</span>
            <span v-if="v.published_at">{{ formatPubdate(v.published_at) }}</span>
          </p>
          <p class="flex items-center mt-1 mb-0 text-3 text-text-2">
            <span class="i-mingcute-play-circle-line mr-1" />{{ formatCount(v.stat.view) }}
            <span class="inline-block w-2.5" />
            <span class="i-mingcute-danmaku-line mr-1" />{{ formatCount(v.stat.danmaku) }}
          </p>
        </div>
      </div>
    </div>

    <!-- 无限滚动：滚动触底自动加载；载入中/到底提示 -->
    <div
      v-if="!loading && videos.length > 0"
      class="text-center py-4 text-3.25 text-text-3"
    >
      <span v-if="loadingMore">加载中…</span>
      <span v-else-if="!hasMore">没有更多了</span>
    </div>
  </div>
</template>

<style scoped lang="scss">
@use '@/styles/variables' as v;

// 推荐区：左大轮播 + 右 2×2 网格；定高 320px——
// 否则 stretch 会把 el-carousel 根拉高于图片（右侧网格大屏更高），
// 导致绝对定位的指示器贴在被拉高的根底部 → dot 与图片间距随屏幕变大。
.reco {
  display: flex;
  gap: 16px;
  height: 320px;
}

.reco__carousel {
  flex: 0 0 60%;
  max-width: 60%;
  height: 100%;
  border-radius: v.$radius-lg;
  overflow: hidden;
  cursor: pointer;
}

.banner__slide {
  position: relative;
  width: 100%;
  height: 100%;
  background: v.$border;
}

.banner__img {
  width: 100%;
  height: 100%;
  object-fit: cover;
  display: block;
}

.banner__mask {
  position: absolute;
  left: 0;
  right: 0;
  bottom: 0;
  padding: 48px 24px 16px;
  background: linear-gradient(transparent, rgba(0, 0, 0, 0.72));
  color: #fff;
}

.banner__title {
  margin: 0;
  font-size: 18px;
  font-weight: 600;
  @include v.ellipsis(1);
}

.banner__meta {
  margin: 6px 0 0;
  font-size: 13px;
  opacity: 0.85;
}

// 指示器：拉回轮播内部底部（消除 outside 大间距）；flex 居中对齐防激活 dot 偏位
.reco__carousel :deep(.el-carousel__indicators) {
  position: absolute !important;
  bottom: 12px;
  left: 50%;
  transform: translateX(-50%);
  display: flex;
  align-items: center;
  margin: 0;
}

.reco__carousel :deep(.el-carousel__indicator) {
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 6px 7px; // 横向加宽，防激活圆两侧被相邻/容器截断
}

// 普通 dot：半透明白小圆（带投影保证浅色封面上可见）。
// 排除激活项，避免与激活规则同特异性相持（激活靠伪元素渲染，button 本体需透明）。
.reco__carousel :deep(.el-carousel__indicator:not(.is-active) .el-carousel__button) {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: rgba(255, 255, 255, 0.65);
  box-shadow: 0 0 2px rgba(0, 0, 0, 0.3);
  opacity: 1;
}

// 激活 dot：吃豆人张嘴——上下半圆绕中缝旋转张合，激活时咬一口（对标 B 站）。
// 不与组件抢 button 尺寸，用固定 12px 居中伪元素拼圆，以 button 中心线为中缝。
.reco__carousel :deep(.el-carousel__indicator.is-active .el-carousel__button) {
  position: relative;
  width: 12px;
  height: 12px;
  background: transparent;
  box-shadow: none;
  overflow: visible;

  &::before,
  &::after {
    content: '';
    position: absolute;
    left: 50%;
    width: 12px;
    height: 6px;
    margin-left: -6px;
    background: #{v.$primary};
  }

  // 上半圆：坐在中缝之上，绕中缝中心向上旋开（咬一口）
  &::before {
    bottom: 50%;
    border-radius: 6px 6px 0 0;
    transform-origin: center bottom;
    animation: eat-chomp-up 0.5s ease-in-out;
  }

  // 下半圆：坐在中缝之下，绕中缝中心向下旋开（咬一口）
  &::after {
    top: 50%;
    border-radius: 0 0 6px 6px;
    transform-origin: center top;
    animation: eat-chomp-down 0.5s ease-in-out;
  }
}

// 上半圆：闭合 → 向上旋 40° 张嘴 → 闭合（右侧开口，吃豆人）
@keyframes eat-chomp-up {
  0%, 100% { transform: rotate(0); }
  50% { transform: rotate(-40deg); }
}

// 下半圆：反向旋转
@keyframes eat-chomp-down {
  0%, 100% { transform: rotate(0); }
  50% { transform: rotate(40deg); }
}

// 无障碍：偏好减少动效时关闭张合动画
@media (prefers-reduced-motion: reduce) {
  .reco__carousel :deep(.el-carousel__indicator.is-active .el-carousel__button)::before,
  .reco__carousel :deep(.el-carousel__indicator.is-active .el-carousel__button)::after {
    animation: none;
  }
}

// 左右切换箭头：加深背景保证可见
.reco__carousel :deep(.el-carousel__arrow) {
  width: 40px;
  height: 40px;
  background: rgba(0, 0, 0, 0.4);
  color: #fff;
  font-size: 18px;

  &:hover {
    background: rgba(0, 0, 0, 0.62);
  }
}

// 右侧 2×2 推荐网格（高度锁定与轮播一致，行高均分）
.reco__side {
  flex: 1;
  min-width: 0;
  height: 100%;
  display: grid;
  grid-template-columns: repeat(2, 1fr);
  grid-template-rows: repeat(2, 1fr);
  gap: 12px;
}

.reco-card {
  min-width: 0;
  min-height: 0;
  cursor: pointer;
  display: flex;
  flex-direction: column;
}

.reco-card__cover {
  position: relative;
  flex: 1;
  min-height: 0;
  border-radius: v.$radius-md;
  overflow: hidden;
  background: v.$border;

  img {
    width: 100%;
    height: 100%;
    object-fit: cover;
    display: block;
    transition: transform 0.25s;
  }
}

.reco-card:hover .reco-card__cover img {
  transform: scale(1.05);
}

.reco-card__title {
  margin: 6px 0 2px;
  font-size: 13px;
  line-height: 1.4;
  @include v.ellipsis(1);
}

.reco-card:hover .reco-card__title {
  color: v.$primary;
}

// 分区 chip（hover/active 交互态）
.category-chip {
  padding: 5px 14px;
  border-radius: v.$radius-sm;
  font-size: 13px;
  color: v.$text-2;
  background: v.$surface;
  cursor: pointer;
  transition: all 0.15s;

  &:hover {
    color: v.$primary;
  }

  &.is-active {
    background: v.$primary;
    color: #fff;
    font-weight: 600;
  }
}

// 排序 Tab
.sort-tab {
  color: v.$text-2;
  cursor: pointer;

  &.is-active {
    color: v.$primary;
    font-weight: 600;
  }
}

// 视频卡片网格（auto-fill minmax）
.video-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(230px, 1fr));
  gap: 20px 16px;
}

.video-card__cover {
  transition: transform 0.15s;

  img {
    width: 100%;
    height: 100%;
    object-fit: cover;
    display: block;
  }
}

.video-card__title {
  height: 2.8em;
  @include v.ellipsis(2);
}

// hover 联动
.video-card:hover {
  .video-card__cover {
    transform: translateY(-2px);
  }

  .video-card__title {
    color: v.$primary;
  }
}

.video-card__up:hover {
  color: v.$primary;
}
</style>

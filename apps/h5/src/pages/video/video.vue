<script setup lang="ts">
import { ref } from 'vue'
import { onLoad } from '@dcloudio/uni-app'
import { formatCount, formatDuration, formatPubdate } from '@dlidli/shared'
import type { VideoDetail } from '@dlidli/api-client'
import { api } from '@/api'

const DEFAULT_AVATAR = '/static/default-avatar.png'

const detail = ref<VideoDetail | null>(null)
const playUrl = ref('')
const loading = ref(true)
const notFound = ref(false)
let bvid = ''
let viewReported = false

onLoad((query) => {
  bvid = (query as { bvid?: string }).bvid ?? ''
  if (bvid) load()
})

async function load() {
  loading.value = true
  try {
    const d = await api.video.detail(bvid)
    detail.value = d
    // 选默认清晰度（streams 按 quality 降序，取首个）
    const def = d.streams?.[0]
    if (def) playUrl.value = def.url
    uni.setNavigationBarTitle({ title: d.title })
  } catch {
    notFound.value = true
  } finally {
    loading.value = false
  }
}

// 有效播放上报（>5s，一次）
function onTimeUpdate(e: { detail: { currentTime: number } }) {
  if (viewReported || e.detail.currentTime < 5 || !detail.value) return
  viewReported = true
  api.video.addView(detail.value.bvid).catch(() => {
    viewReported = false
  })
}

// ---------- 互动操作 ----------
const liked = ref(false)
const faved = ref(false)

async function doLike() {
  if (!detail.value) return
  try {
    const res = await api.interaction.likeVideo(detail.value.bvid)
    liked.value = res.liked
    detail.value.stat.like += res.liked ? 1 : -1
  } catch { /* silent */ }
}

async function doCoin() {
  if (!detail.value) return
  try {
    await api.interaction.coinVideo(detail.value.bvid, 1)
    detail.value.stat.coin += 1
    uni.showToast({ title: '投币成功', icon: 'success' })
  } catch { /* silent */ }
}

async function doFav() {
  if (!detail.value) return
  try {
    const res = await api.interaction.toggleFavorite(detail.value.bvid)
    faved.value = res.faved
    detail.value.stat.fav += res.faved ? 1 : -1
  } catch { /* silent */ }
}

function doShare() {
  if (!detail.value) return
  uni.setClipboardData({
    data: `DliDli - ${detail.value.title}`,
    success: () => uni.showToast({ title: '链接已复制', icon: 'success' }),
  })
}

// ---------- 弹幕发送 ----------
const danmakuText = ref('')
const danmakuSending = ref(false)

async function sendDanmaku() {
  const text = danmakuText.value.trim()
  if (!text || !detail.value) return
  danmakuSending.value = true
  try {
    await api.danmaku.send(detail.value.bvid, { content: text, time_ms: 0, color: 16777215 })
    danmakuText.value = ''
    uni.showToast({ title: '发送成功', icon: 'success' })
  } catch {
    uni.showToast({ title: '发送失败', icon: 'none' })
  } finally {
    danmakuSending.value = false
  }
}
</script>

<template>
  <view class="page">
    <view
      v-if="notFound"
      class="tip"
    >视频不存在或已下架</view>

    <template v-else-if="detail">
      <!-- 播放器：uni video 原生支持 HLS -->
      <video
        v-if="playUrl"
        class="player"
        :src="playUrl"
        autoplay
        :controls="true"
        :enable-danmu="false"
        object-fit="contain"
        @timeupdate="(e: any) => onTimeUpdate(e)"
      />
      <view
        v-else
        class="player player--empty"
      >
        <text>转码中，稍后再来～</text>
      </view>

      <!-- 信息区 -->
      <view class="info">
        <text class="info__title">{{ detail.title }}</text>
        <view class="info__stat">
          <text><text class="i-mingcute-play-circle-line align-middle mr-1" />{{ formatCount(detail.stat.view) }}</text>
          <text>· 弹幕 {{ formatCount(detail.stat.danmaku) }}</text>
          <text>· {{ formatPubdate(detail.published_at || detail.created_at) }}</text>
        </view>
      </view>

      <!-- UP 主 -->
      <view class="up">
        <image
          class="up__avatar"
          :src="detail.owner.avatar || DEFAULT_AVATAR"
          mode="aspectFill"
        />
        <view class="up__info">
          <text class="up__name">{{ detail.owner.nickname }}</text>
        </view>
      </view>

      <!-- 三连数据 + 互动操作栏 -->
      <view class="stats">
        <view
          class="stat-item"
          @tap="doLike"
        >
          <text
            class="stat-item__icon"
            :class="[liked ? 'i-mingcute-thumb-up-2-fill' : 'i-mingcute-thumb-up-2-line', { active: liked }]"
          />
          <text
            class="stat-item__label"
            :class="{ active: liked }"
          >{{ formatCount(detail.stat.like) }}</text>
        </view>
        <view
          class="stat-item"
          @tap="doCoin"
        >
          <text class="stat-item__icon i-mingcute-coin-2-line" />
          <text class="stat-item__label">{{ formatCount(detail.stat.coin) }}</text>
        </view>
        <view
          class="stat-item"
          @tap="doFav"
        >
          <text
            class="stat-item__icon"
            :class="[faved ? 'i-mingcute-star-2-fill' : 'i-mingcute-star-2-line', { active: faved }]"
          />
          <text
            class="stat-item__label"
            :class="{ active: faved }"
          >{{ formatCount(detail.stat.fav) }}</text>
        </view>
        <view
          class="stat-item"
          @tap="doShare"
        >
          <text class="stat-item__icon i-mingcute-share-forward-line" />
          <text class="stat-item__label">{{ formatCount(detail.stat.share) }}</text>
        </view>
      </view>

      <!-- 弹幕发送栏 -->
      <view class="danmaku-bar">
        <input
          v-model="danmakuText"
          class="danmaku-bar__input"
          placeholder="发条弹幕吧~"
          maxlength="100"
          confirm-type="send"
          @confirm="sendDanmaku"
        >
        <view
          class="danmaku-bar__btn"
          :class="{ disabled: !danmakuText.trim() || danmakuSending }"
          @tap="sendDanmaku"
        >发送</view>
      </view>

      <!-- 简介 -->
      <view
        v-if="detail.description"
        class="desc"
      >
        <text>{{ detail.description }}</text>
      </view>
    </template>

    <view
      v-else-if="loading"
      class="tip"
    >加载中…</view>
  </view>
</template>

<style lang="scss">
@use '../../styles/variables' as v;

.page {
  min-height: 100vh;
  background: v.$surface;
}

.player {
  width: 100%;
  height: 220px;
  background: #000;
}

.player--empty {
  display: flex;
  align-items: center;
  justify-content: center;
  color: v.$text-3;
  font-size: 26rpx;
}

.info {
  padding: 24rpx;
}

.info__title {
  font-size: 32rpx;
  font-weight: 600;
  line-height: 1.4;
  color: v.$text-1;
}

.info__stat {
  margin-top: 12rpx;
  font-size: 24rpx;
  color: v.$text-3;
}

.info__stat text {
  margin-right: 8rpx;
}

.up {
  display: flex;
  align-items: center;
  padding: 20rpx 24rpx;
  border-top: 1rpx solid #f1f2f3;
  border-bottom: 1rpx solid #f1f2f3;
}

.up__avatar {
  width: 72rpx;
  height: 72rpx;
  border-radius: 50%;
  background: v.$primary;
  margin-right: 16rpx;
}

.up__name {
  font-size: 28rpx;
  color: v.$text-1;
  margin-right: 12rpx;
}

.up__lv {
  font-size: 22rpx;
  color: v.$primary;
}

.stats {
  display: flex;
  padding: 28rpx 0;
}

.stat-item {
  flex: 1;
  display: flex;
  flex-direction: column;
  align-items: center;
}

.stat-item__num {
  font-size: 30rpx;
  font-weight: 600;
  color: v.$text-1;
}

.stat-item__icon {
  font-size: 44rpx;
  color: v.$text-2;
}

.stat-item__icon.active {
  color: v.$primary;
}

.stat-item__label {
  margin-top: 6rpx;
  font-size: 24rpx;
  color: v.$text-2;
}

.desc {
  padding: 24rpx;
  font-size: 26rpx;
  line-height: 1.6;
  color: v.$text-2;
  border-top: 1rpx solid #f1f2f3;
}

.tip {
  padding: 80rpx;
  text-align: center;
  font-size: 26rpx;
  color: v.$text-3;
}

/* 互动激活态 */
.stat-item__label.active {
  color: v.$primary;
  font-weight: 600;
}

/* 弹幕发送栏 */
.danmaku-bar {
  display: flex;
  align-items: center;
  gap: 16rpx;
  padding: 16rpx 24rpx;
  border-top: 1rpx solid v.$border;
}

.danmaku-bar__input {
  flex: 1;
  height: 64rpx;
  padding: 0 24rpx;
  border-radius: 32rpx;
  background: v.$bg;
  font-size: 26rpx;
}

.danmaku-bar__btn {
  padding: 12rpx 28rpx;
  border-radius: 32rpx;
  background: v.$primary;
  color: #fff;
  font-size: 26rpx;
  font-weight: 600;

  &.disabled {
    opacity: 0.4;
  }
}
</style>

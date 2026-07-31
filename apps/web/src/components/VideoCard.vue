<script setup lang="ts">
// 通用视频卡片（网格布局）：封面 + 时长角标 + 标题(2行省略) + 播放量/UP/日期 meta。
// hover 联动（封面缩放 + 标题变色）内置，点击整卡跳播放页。
import { computed } from 'vue'
import { RouterLink } from 'vue-router'
import { formatCount, formatDuration, formatPubdate } from '@dlidli/shared'
import type { VideoCard } from '@dlidli/api-client'
import defaultCover from '@/assets/default-cover.svg'

const props = defineProps<{
  video: VideoCard
  /** meta 行是否显示 UP 主昵称 */
  showOwner?: boolean
  /** meta 行是否显示发布日期 */
  showDate?: boolean
}>()

// meta 后缀（UP、日期），拼成单一文本避免模板内联 v-if
const metaSuffix = computed(() => {
  const parts: string[] = []
  if (props.showOwner) parts.push(props.video.owner.nickname)
  if (props.showDate) parts.push(formatPubdate(props.video.published_at || props.video.created_at))
  return parts.length ? ' · ' + parts.join(' · ') : ''
})
</script>

<template>
  <RouterLink
    :to="`/video/${video.bvid}`"
    class="video-card"
  >
    <div class="video-card__cover">
      <img
        :src="video.cover || defaultCover"
        :alt="video.title"
        loading="lazy"
      >
      <span
        v-if="video.duration > 0"
        class="video-card__dur"
      >{{ formatDuration(video.duration) }}</span>
    </div>
    <p class="video-card__title">
      {{ video.title }}
    </p>
    <p class="video-card__meta">
      <span class="i-mingcute-play-circle-line mr-1" />{{ formatCount(video.stat.view) }}{{ metaSuffix }}
    </p>
  </RouterLink>
</template>

<style scoped lang="scss">
@use '@/styles/variables' as v;

.video-card {
  display: block;
  cursor: pointer;
  text-decoration: none;
  color: inherit;
}

.video-card__cover {
  position: relative;
  aspect-ratio: 16 / 9;
  border-radius: 8px;
  overflow: hidden;
  background: #f1f2f3;

  img {
    width: 100%;
    height: 100%;
    object-fit: cover;
    display: block;
    transition: transform 0.25s;
  }
}

.video-card__dur {
  position: absolute;
  right: 6px;
  bottom: 6px;
  padding: 1px 6px;
  border-radius: 4px;
  background: rgba(0, 0, 0, 0.6);
  color: #fff;
  font-size: 12px;
}

.video-card__title {
  margin: 8px 0 2px;
  font-size: 14px;
  line-height: 1.4;
  @include v.ellipsis(2);
}

.video-card__meta {
  display: flex;
  align-items: center;
  margin: 0;
  font-size: 12px;
  color: v.$text-2;
}

.video-card:hover {
  .video-card__cover img {
    transform: scale(1.05);
  }

  .video-card__title {
    color: v.$primary;
  }
}
</style>

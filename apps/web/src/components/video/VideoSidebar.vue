<script setup lang="ts">
// 播放页右侧栏（B 站风格）：UP 主卡片 / 分P 列表 / 弹幕列表 / 相关推荐。
// player/dm/acts 为父级 useVideoPlayer/useDanmakuController/useVideoActions 的返回对象，
// 此处解构其 ref 成员（保持同一引用，响应性不变）。
import { useRouter } from 'vue-router'
import { formatCount, formatDuration } from '@dlidli/shared'
import type { VideoCard, VideoDetail } from '@dlidli/api-client'
import type { useDanmakuController } from '@/composables/video/useDanmakuController'
import type { useVideoActions } from '@/composables/video/useVideoActions'
import type { useVideoPlayer } from '@/composables/video/useVideoPlayer'
import defaultCover from '@/assets/default-cover.svg'
import defaultAvatar from '@/assets/default-avatar.png'
import { useUserStore } from '@/stores/user'

type PlayerCtl = ReturnType<typeof useVideoPlayer>
type DmCtl = ReturnType<typeof useDanmakuController>
type ActsCtl = ReturnType<typeof useVideoActions>

const props = defineProps<{
  detail: VideoDetail | null
  related: VideoCard[]
  player: PlayerCtl
  dm: DmCtl
  acts: ActsCtl
}>()

const router = useRouter()
const userStore = useUserStore()

const { partList, currentPart, switchPart } = props.player
const { dmList, dmListTotal, dmListPage, dmListLoading, loadDmList, dmSeekTo, dmTextColor } =
  props.dm
const { followerCnt, following, followPending, toggleFollow } = props.acts

// 弹幕列表加载更多（多语句逻辑收敛为具名方法，避免内联多语句 handler）
function loadMoreDm() {
  dmListPage.value++
  loadDmList()
}
</script>

<template>
  <aside v-if="detail" class="play-side">
    <!-- UP 主卡片 -->
    <div class="up-card">
      <el-avatar
        :size="44"
        :src="detail.owner.avatar || defaultAvatar"
        class="up-card__avatar is-clickable"
        @click="$router.push(`/space/${detail.owner.id}`)"
      >
        {{ detail.owner.nickname?.slice(0, 1) ?? 'U' }}
      </el-avatar>
      <div class="up-card__info">
        <p class="up-card__name is-clickable" @click="$router.push(`/space/${detail.owner.id}`)">
          {{ detail.owner.nickname }}
        </p>
        <p class="up-card__sign">{{ formatCount(followerCnt) }} 粉丝</p>
      </div>
      <el-button
        v-if="userStore.profile?.id !== detail.owner.id"
        round
        class="up-card__follow"
        :class="{ 'is-following': following }"
        :loading="followPending"
        @click="toggleFollow"
      >
        {{ following ? '已关注' : '+ 关注' }}
      </el-button>
    </div>

    <!-- 分P列表（竖排，多P投稿 PRD VID-05） -->
    <div v-if="partList.length > 0" class="side-block">
      <p class="side-block__title">分P列表</p>
      <div
        v-for="(p, i) in partList"
        :key="p.index"
        class="part-list__item"
        :class="{ 'is-active': currentPart === i }"
        @click="switchPart(i)"
      >
        <span class="part-list__idx">P{{ i + 1 }}</span>
        <span class="part-list__title">{{ p.title || `分P${i + 1}` }}</span>
        <span class="part-list__dur">{{ p.duration ? formatDuration(p.duration) : '' }}</span>
      </div>
    </div>

    <!-- 弹幕列表面板（内嵌，点击跳转进度） -->
    <div class="side-block">
      <p class="side-block__title">
        弹幕列表
        <span class="side-block__count">{{ dmListTotal }}</span>
      </p>
      <div v-if="dmList.length === 0" class="dm-panel__empty">还没有弹幕，来发第一条吧</div>
      <div v-else class="dm-panel">
        <div v-for="d in dmList" :key="d.id" class="dm-panel__item" @click="dmSeekTo(d.time_ms)">
          <span class="dm-panel__time">{{ formatDuration(d.time_ms / 1000) }}</span>
          <span class="dm-panel__text" :style="{ color: dmTextColor(d.color) }">{{
            d.content
          }}</span>
        </div>
        <div v-if="dmList.length < dmListTotal" class="text-center py-1">
          <el-button link size="small" :loading="dmListLoading" @click="loadMoreDm">
            加载更多（{{ dmList.length }}/{{ dmListTotal }}）
          </el-button>
        </div>
      </div>
    </div>

    <!-- 相关推荐 -->
    <p class="side-title">相关推荐</p>
    <el-empty v-if="related.length === 0" description="暂无相关视频" :image-size="64" />
    <div
      v-for="v in related"
      :key="v.bvid"
      class="side-card"
      @click="router.push(`/video/${v.bvid}`)"
    >
      <div class="side-card__cover">
        <img :src="v.cover || defaultCover" :alt="v.title" loading="lazy" />
        <span v-if="v.duration > 0" class="side-card__duration">{{
          formatDuration(v.duration)
        }}</span>
      </div>
      <div class="side-card__info">
        <p class="side-card__title">
          {{ v.title }}
        </p>
        <p class="side-card__meta">
          {{ v.owner.nickname }}
        </p>
        <p class="side-card__meta flex items-center">
          <span class="i-mingcute-play-circle-line mr-1" />{{ formatCount(v.stat.view) }}
        </p>
      </div>
    </div>
  </aside>
</template>

<style scoped lang="scss">
@use '@/styles/variables' as v;

/* 分P列表（竖排，侧栏内） */
.part-list__item {
  display: flex;
  align-items: center;
  gap: 8px;
  width: 100%;
  padding: 8px 10px;
  border-radius: 8px;
  border: 1px solid var(--dli-border);
  font-size: 13px;
  color: var(--dli-text-2);
  cursor: pointer;
  transition: all 0.15s;
  margin-bottom: 6px;

  &:hover {
    color: var(--dli-primary);
    border-color: var(--dli-primary);
  }

  &.is-active {
    background: var(--dli-primary);
    border-color: var(--dli-primary);
    color: #fff;
  }
}

.part-list__idx {
  font-weight: 700;
  flex-shrink: 0;
}

.part-list__title {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.part-list__dur {
  color: var(--dli-text-3);
  font-size: 11.5px;
  flex-shrink: 0;
}

.part-list__item.is-active .part-list__dur {
  color: rgba(255, 255, 255, 0.8);
}

/* 侧栏信息块（分P / 弹幕列表） */
.side-block {
  margin-top: 16px;
}

.side-block__title {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 14px;
  font-weight: 600;
  margin: 0 0 10px;
}

.side-block__count {
  font-size: 12px;
  font-weight: 400;
  color: var(--dli-text-3);
}

/* 弹幕列表面板（内嵌，浅色底；白色弹幕由 dmTextColor 映射为深色保证可读） */
.dm-panel {
  max-height: 280px;
  overflow-y: auto;
  background: #f8f9fa;
  border: 1px solid var(--dli-border);
  border-radius: 8px;
  padding: 4px 6px;
}

.dm-panel__item {
  display: flex;
  align-items: baseline;
  gap: 8px;
  padding: 6px 8px;
  border-radius: 6px;
  font-size: 12.5px;
  color: var(--dli-text-1);
  cursor: pointer;
  transition: background 0.15s;

  &:hover {
    background: #eef0f1;
  }
}

.dm-panel__time {
  flex-shrink: 0;
  font-size: 11px;
  color: var(--dli-text-3);
  min-width: 34px;
}

.dm-panel__text {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.dm-panel__empty {
  padding: 14px 0;
  text-align: center;
  font-size: 12.5px;
  color: var(--dli-text-3);
  border: 1px dashed var(--dli-border);
  border-radius: 8px;
}

/* UP 主卡片（侧栏置顶） */
.up-card {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 12px;
  background: #fff;
  border: 1px solid var(--dli-border);
  border-radius: 10px;
}

.up-card__avatar {
  background: var(--dli-primary);
  color: #fff;
  font-weight: 600;
  flex-shrink: 0;
}

.up-card__info {
  flex: 1;
  min-width: 0;
}

.up-card__name {
  margin: 0;
  font-size: 15px;
  font-weight: 600;
}

.up-card .is-clickable {
  cursor: pointer;
}

.up-card__name.is-clickable:hover {
  color: var(--dli-primary);
}

.up-card__sign {
  margin: 2px 0 0;
  font-size: 12px;
  color: var(--dli-text-2);
}

.up-card__follow {
  --el-button-bg-color: #{v.$primary};
  --el-button-border-color: #{v.$primary};
  --el-button-text-color: #fff;
  --el-button-hover-bg-color: #{v.$primary-hover};
  --el-button-hover-border-color: #{v.$primary-hover};
  --el-button-hover-text-color: #fff;
  min-width: 92px;
}

.up-card__follow.is-following {
  --el-button-bg-color: #f1f2f3;
  --el-button-border-color: #e3e5e7;
  --el-button-text-color: var(--dli-text-2);
  --el-button-hover-bg-color: #e9eaeb;
  --el-button-hover-border-color: #e3e5e7;
  --el-button-hover-text-color: var(--dli-text-2);
}

/* 相关推荐 */
.side-title {
  margin: 0 0 12px;
  font-size: 15px;
  font-weight: 600;
}

.side-card {
  display: flex;
  gap: 10px;
  margin-bottom: 12px;
  cursor: pointer;
}

.side-card__cover {
  position: relative;
  width: 140px;
  aspect-ratio: 16 / 10;
  border-radius: 6px;
  overflow: hidden;
  flex-shrink: 0;
}

.side-card__cover img {
  width: 100%;
  height: 100%;
  object-fit: cover;
  display: block;
}

.side-card__duration {
  position: absolute;
  right: 4px;
  bottom: 4px;
  padding: 0 5px;
  border-radius: 4px;
  background: rgba(0, 0, 0, 0.6);
  color: #fff;
  font-size: 11px;
}

.side-card__info {
  min-width: 0;
}

.side-card__title {
  margin: 0;
  font-size: 13px;
  line-height: 1.4;
  display: -webkit-box;
  -webkit-line-clamp: 2;
  line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
}

.side-card:hover .side-card__title {
  color: var(--dli-primary);
}

.side-card__meta {
  margin: 4px 0 0;
  font-size: 12px;
  color: var(--dli-text-2);
}
</style>

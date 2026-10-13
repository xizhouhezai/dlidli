<script setup lang="ts">
// 直播首页（M4-LIV-06，LIV-10）：开播中房间卡片网格，点击进直播间。
// 推荐排序即后端默认列表序；分区过滤待后续卡（列表接口已支持 category_id）。
import { onBeforeUnmount, ref } from 'vue'
import { RouterLink } from 'vue-router'
import { ElMessage } from 'element-plus'
import { formatCount } from '@dlidli/shared'
import type { LiveRoomInfo } from '@dlidli/api-client'
import { api } from '@/api'
import defaultCover from '@/assets/default-cover.png'

const rooms = ref<LiveRoomInfo[]>([])
const loading = ref(true)
let timer: number | null = null

/** 主播昵称/头像（Briefs 批量补全；取不到降级「主播 {uid}」） */
const anchorProfiles = ref<Record<string, { nickname: string; avatar: string }>>({})

async function load() {
  try {
    const res = await api.live.rooms({ page: 1, size: 50 })
    rooms.value = res.list
    await loadProfiles(res.list)
  } catch {
    ElMessage.error('直播列表加载失败')
  } finally {
    loading.value = false
  }
}

async function loadProfiles(list: LiveRoomInfo[]) {
  const uids = [...new Set(list.map((r) => r.anchor_id))]
  if (uids.length === 0) return
  const entries = await Promise.all(
    uids.map(async (uid) => {
      try {
        const p = await api.relation.profile(uid)
        return [uid, { nickname: p.nickname, avatar: p.avatar }] as const
      } catch {
        return [uid, { nickname: `主播 ${uid}`, avatar: '' }] as const
      }
    }),
  )
  anchorProfiles.value = Object.fromEntries(entries)
}

function anchorName(uid: string) {
  return anchorProfiles.value[uid]?.nickname || `主播 ${uid}`
}

load()
// 列表 15s 轮询：开播/下播/在线数变化及时反映（房间列表无推送通道，轮询口径与在线数一致）
timer = window.setInterval(load, 15000)
onBeforeUnmount(() => {
  if (timer) window.clearInterval(timer)
})
</script>

<template>
  <div class="live-home">
    <div class="live-home__head">
      <h2 class="live-home__title">
        <span class="i-mingcute-live-line mr-2 text-6 text-primary" />直播
      </h2>
      <RouterLink to="/studio/live" class="live-home__studio-link">
        <span class="i-mingcute-broadcast-line mr-1" />我要开播
      </RouterLink>
    </div>

    <div v-if="loading" class="live-home__loading">加载中…</div>
    <div v-else-if="rooms.length === 0" class="live-home__empty">
      <p class="text-8 i-mingcute-tv-2-line mb-3 text-#c0c4cc" />
      <p>暂时没有主播开播，去看看视频吧～</p>
    </div>

    <div v-else class="live-grid">
      <RouterLink v-for="room in rooms" :key="room.id" :to="`/live/${room.id}`" class="live-card">
        <div class="live-card__cover">
          <img :src="room.cover || defaultCover" :alt="room.title" loading="lazy" />
          <span class="live-card__badge"> <span class="live-card__dot" />直播中 </span>
          <span class="live-card__online">
            <span class="i-mingcute-user-2-line mr-0.5" />{{ formatCount(room.online) }}
          </span>
        </div>
        <p class="live-card__title">{{ room.title || '未命名直播间' }}</p>
        <p class="live-card__meta">
          <img
            class="live-card__avatar"
            :src="anchorProfiles[room.anchor_id]?.avatar || '/default-avatar.png'"
            alt=""
          />
          <span class="truncate">{{ anchorName(room.anchor_id) }}</span>
        </p>
      </RouterLink>
    </div>
  </div>
</template>

<style scoped lang="scss">
@use '@/styles/variables' as v;

.live-home__head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 16px;
}

.live-home__title {
  display: flex;
  align-items: center;
  font-size: 20px;
  font-weight: 700;
}

.live-home__studio-link {
  display: inline-flex;
  align-items: center;
  padding: 6px 14px;
  border-radius: 18px;
  background: v.$primary;
  color: #fff;
  font-size: 13px;
  text-decoration: none;
  transition: background 0.15s;

  &:hover {
    background: v.$primary-hover;
  }
}

.live-home__loading,
.live-home__empty {
  padding: 80px 0;
  text-align: center;
  color: v.$text-3;
  font-size: 14px;
}

.live-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(240px, 1fr));
  gap: 16px;
}

.live-card {
  display: block;
  cursor: pointer;
  text-decoration: none;
  color: inherit;
}

.live-card__cover {
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

.live-card:hover .live-card__cover img {
  transform: scale(1.05);
}

.live-card__badge {
  position: absolute;
  left: 8px;
  top: 8px;
  display: inline-flex;
  align-items: center;
  padding: 2px 8px;
  border-radius: 4px;
  background: v.$primary;
  color: #fff;
  font-size: 12px;
  font-weight: 600;
}

.live-card__dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: #fff;
  margin-right: 4px;
  animation: live-pulse 1.2s infinite;
}

@keyframes live-pulse {
  0%,
  100% {
    opacity: 1;
  }
  50% {
    opacity: 0.4;
  }
}

.live-card__online {
  position: absolute;
  right: 8px;
  bottom: 8px;
  display: inline-flex;
  align-items: center;
  padding: 1px 8px;
  border-radius: 4px;
  background: rgba(0, 0, 0, 0.6);
  color: #fff;
  font-size: 12px;
}

.live-card__title {
  margin: 8px 0 2px;
  font-size: 14px;
  line-height: 1.4;
  @include v.ellipsis(1);
}

.live-card__meta {
  display: flex;
  align-items: center;
  gap: 6px;
  margin: 0;
  font-size: 12px;
  color: v.$text-2;
}

.live-card__avatar {
  width: 18px;
  height: 18px;
  border-radius: 50%;
  object-fit: cover;
  background: #f1f2f3;
}
</style>

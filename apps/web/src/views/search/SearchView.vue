<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import type { UserBrief, VideoCard as VideoCardData } from '@dlidli/api-client'
import { api } from '@/api'
import defaultAvatar from '@/assets/default-avatar.png'
import VideoCard from '@/components/VideoCard.vue'

const route = useRoute()
const router = useRouter()

const PAGE_SIZE = 20

const keyword = computed(() => String(route.query.kw ?? '').trim())
const activeTab = ref<'video' | 'user'>('video')

const videos = ref<VideoCardData[]>([])
const users = ref<UserBrief[]>([])
const total = ref(0)
const page = ref(1)
const loading = ref(false)

async function load() {
  if (!keyword.value) return
  loading.value = true
  try {
    if (activeTab.value === 'video') {
      const res = await api.search.videos(keyword.value, page.value, PAGE_SIZE)
      videos.value = res.list
      total.value = res.total
    } else {
      const res = await api.search.users(keyword.value, page.value, PAGE_SIZE)
      users.value = res.list
      total.value = res.total
    }
    document.title = `${keyword.value} - 搜索 - DliDli`
  } finally {
    loading.value = false
  }
}

function reset() {
  page.value = 1
  load()
}

onMounted(load)
watch(keyword, reset)
watch(activeTab, reset)
</script>

<template>
  <div class="mx-auto max-w-1100px">
    <p class="m-0 mb-3 text-4">
      「<span class="text-primary font-600">{{ keyword }}</span>」的搜索结果
      <span class="ml-2.5 text-3.25 text-text-2">共 {{ total }} 条</span>
    </p>

    <div class="flex gap-1.5 mb-3.5">
      <span
        class="search__tab"
        :class="{ 'is-active': activeTab === 'video' }"
        @click="activeTab = 'video'"
      >视频</span>
      <span
        class="search__tab"
        :class="{ 'is-active': activeTab === 'user' }"
        @click="activeTab = 'user'"
      >用户</span>
    </div>

    <el-skeleton
      v-if="loading"
      :rows="5"
      animated
    />

    <!-- 视频结果 -->
    <template v-else-if="activeTab === 'video'">
      <el-empty
        v-if="videos.length === 0"
        description="没有找到相关视频，换个关键词试试"
      />
      <div
        v-else
        class="search-grid"
      >
        <VideoCard
          v-for="v in videos"
          :key="v.bvid"
          :video="v"
          show-owner
          show-date
        />
      </div>
    </template>

    <!-- 用户结果 -->
    <template v-else>
      <el-empty
        v-if="users.length === 0"
        description="没有找到相关用户"
      />
      <div
        v-else
        class="search-users"
      >
        <div
          v-for="u in users"
          :key="u.id"
          class="search-user flex items-center gap-3 p-4 rounded-10px bg-white cursor-pointer"
          @click="router.push(`/space/${u.id}`)"
        >
          <el-avatar
            :size="48"
            :src="u.avatar || defaultAvatar"
            class="shrink-0 bg-primary text-white font-600"
          >
            {{ u.nickname?.slice(0, 1) ?? 'U' }}
          </el-avatar>
          <div class="flex-1 min-w-0">
            <p class="search-user__name m-0 text-3.5 font-600">
              {{ u.nickname }}
            </p>
            <p class="mt-0.75 mb-0 text-3 text-text-2 truncate">
              {{ u.signature || 'TA 还没有签名' }}
            </p>
          </div>
          <el-tag
            size="small"
            effect="plain"
          >
            Lv{{ u.level }}
          </el-tag>
        </div>
      </div>
    </template>

    <div
      v-if="total > PAGE_SIZE"
      class="flex justify-center py-3.5"
    >
      <el-pagination
        v-model:current-page="page"
        layout="prev, pager, next"
        :total="total"
        :page-size="PAGE_SIZE"
        @current-change="load"
      />
    </div>
  </div>
</template>

<style scoped lang="scss">
@use '@/styles/variables' as v;

// Tab（含 hover/active 交互态，保留 SCSS）
.search__tab {
  padding: 6px 18px;
  border-radius: v.$radius-md;
  font-size: 14px;
  cursor: pointer;
  color: v.$text-2;
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

// 自适应网格（Uno 难表达 auto-fill minmax）
.search-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(200px, 1fr));
  gap: 16px;
}

.search-users {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(320px, 1fr));
  gap: 12px;
}

.search-user {
  transition: box-shadow 0.15s;

  &:hover {
    box-shadow: v.$shadow-card;

    .search-user__name {
      color: v.$primary;
    }
  }
}
</style>

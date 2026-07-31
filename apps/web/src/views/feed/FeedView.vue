<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { formatCount, formatDuration, formatPubdate } from '@dlidli/shared'
import { ApiError, type FeedItem } from '@dlidli/api-client'
import { api } from '@/api'
import { useUserStore } from '@/stores/user'
import defaultCover from '@/assets/default-cover.svg'
import defaultAvatar from '@/assets/default-avatar.png'

const router = useRouter()
const userStore = useUserStore()

const list = ref<FeedItem[]>([])
const cursor = ref('')
const hasMore = ref(false)
const loading = ref(true)
const loadingMore = ref(false)

const draft = ref('')
const posting = ref(false)

async function load(reset = true) {
  if (reset) {
    cursor.value = ''
    loading.value = true
  } else {
    loadingMore.value = true
  }
  try {
    const res = await api.dynamic.feed(reset ? '' : cursor.value, 20)
    list.value = reset ? res.list : [...list.value, ...res.list]
    cursor.value = res.next_cursor
    hasMore.value = res.has_more
  } finally {
    loading.value = false
    loadingMore.value = false
  }
}

onMounted(() => load())

async function post() {
  const content = draft.value.trim()
  if (!content) {
    ElMessage.warning('说点什么吧')
    return
  }
  posting.value = true
  try {
    const item = await api.dynamic.post(content)
    list.value = [item, ...list.value]
    draft.value = ''
    ElMessage.success('发布成功')
  } catch (err) {
    ElMessage.error(err instanceof ApiError ? err.message : '发布失败')
  } finally {
    posting.value = false
  }
}
</script>

<template>
  <div class="mx-auto max-w-160">
    <!-- 发布器 -->
    <div class="flex gap-3 p-4 mb-3.5 rounded-10px bg-white">
      <el-avatar
        :size="40"
        :src="userStore.profile?.avatar || defaultAvatar"
        class="shrink-0 bg-primary text-white font-600"
      >
        {{ userStore.profile?.nickname?.slice(0, 1) ?? 'U' }}
      </el-avatar>
      <div class="flex-1 min-w-0">
        <el-input
          v-model="draft"
          type="textarea"
          :rows="3"
          maxlength="1000"
          show-word-limit
          placeholder="有什么想和大家分享的？"
        />
        <div class="feed-editor__ops mt-2.5 flex justify-end">
          <el-button
            type="primary"
            :loading="posting"
            :disabled="!draft.trim()"
            @click="post"
          >
            发布
          </el-button>
        </div>
      </div>
    </div>

    <el-skeleton
      v-if="loading"
      :rows="6"
      animated
    />
    <el-empty
      v-else-if="list.length === 0"
      description="动态空空如也，去关注一些有趣的 UP 主吧"
    >
      <el-button
        type="primary"
        @click="router.push('/')"
      >
        去逛逛
      </el-button>
    </el-empty>

    <!-- 动态流 -->
    <div
      v-for="item in list"
      :key="item.id"
      class="flex gap-3 p-4 mb-3 rounded-10px bg-white"
    >
      <el-avatar
        :size="44"
        :src="item.user.avatar || defaultAvatar"
        class="shrink-0 bg-primary text-white font-600 cursor-pointer"
        @click="router.push(`/space/${item.user.id}`)"
      >
        {{ item.user.nickname?.slice(0, 1) ?? 'U' }}
      </el-avatar>
      <div class="flex-1 min-w-0">
        <p class="m-0 flex items-baseline gap-2.5">
          <span
            class="feed-card__name text-3.75 font-600 cursor-pointer"
            @click="router.push(`/space/${item.user.id}`)"
          >{{ item.user.nickname }}</span>
          <span class="text-3 text-text-2">
            {{ formatPubdate(item.created_at) }} · {{ item.type === 1 ? '投稿了视频' : item.type === 3 ? '转发了视频' : '发布了动态' }}
          </span>
        </p>
        <p
          v-if="item.content"
          class="mt-2 mb-0 text-3.5 leading-[1.7] break-words whitespace-pre-wrap"
        >
          {{ item.content }}
        </p>

        <!-- 投稿动态：视频卡片 -->
        <div
          v-if="item.video"
          class="feed-video flex gap-3 mt-2.5 p-2.5 rounded-8px border border-border cursor-pointer"
          @click="router.push(`/video/${item.video.bvid}`)"
        >
          <div class="feed-video__cover relative w-180px shrink-0 aspect-video rounded-6px overflow-hidden bg-#f1f2f3">
            <img
              :src="item.video.cover || defaultCover"
              :alt="item.video.title"
              loading="lazy"
            >
            <span
              v-if="item.video.duration > 0"
              class="absolute right-1.5 bottom-1.5 px-1.5 py-0.25 rounded-4px bg-black/60 text-white text-3"
            >{{ formatDuration(item.video.duration) }}</span>
          </div>
          <div class="flex-1 min-w-0 flex flex-col justify-between py-1">
            <p class="feed-video__title m-0 text-3.5 font-600 leading-[1.4]">
              {{ item.video.title }}
            </p>
            <p class="flex items-center m-0 text-3 text-text-2">
              <span class="i-mingcute-play-circle-line mr-1" />{{ formatCount(item.video.stat.view) }} ·
              <span class="i-mingcute-danmaku-line mx-1" />{{ formatCount(item.video.stat.comment) }} ·
              <span class="i-mingcute-thumb-up-2-line mx-1" />{{ formatCount(item.video.stat.like) }}
            </p>
          </div>
        </div>
      </div>
    </div>

    <div
      v-if="hasMore && !loading"
      class="text-center py-3.5"
    >
      <el-button
        link
        :loading="loadingMore"
        @click="load(false)"
      >
        加载更多
      </el-button>
    </div>
  </div>
</template>

<style scoped lang="scss">
@use '@/styles/variables' as v;

// 发布按钮品牌色（变量覆盖 Element Plus）
.feed-editor__ops .el-button {
  --el-button-bg-color: #{v.$primary};
  --el-button-border-color: #{v.$primary};
  --el-button-hover-bg-color: #{v.$primary-hover};
  --el-button-hover-border-color: #{v.$primary-hover};
}

.feed-card__name:hover {
  color: v.$primary;
}

.feed-video__title {
  @include v.ellipsis(2);
}

.feed-video {
  transition: background 0.15s;

  &:hover {
    background: v.$bg;

    .feed-video__title {
      color: v.$primary;
    }
  }
}

.feed-video__cover img {
  width: 100%;
  height: 100%;
  object-fit: cover;
  display: block;
}
</style>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { formatPubdate } from '@dlidli/shared'
import type { NotifyItem } from '@dlidli/api-client'
import { api } from '@/api'
import defaultAvatar from '@/assets/default-avatar.png'

const router = useRouter()

const TYPE_META: Record<number, { icon: string; label: string }> = {
  1: { icon: 'i-mingcute-thumb-up-2-line', label: '点赞' },
  2: { icon: 'i-mingcute-comment-line', label: '评论' },
  3: { icon: 'i-mingcute-user-add-line', label: '关注' },
  4: { icon: 'i-mingcute-notification-line', label: '系统' },
}

const list = ref<NotifyItem[]>([])
const cursor = ref('')
const hasMore = ref(false)
const loading = ref(true)
const loadingMore = ref(false)

async function load(reset = true) {
  if (reset) {
    cursor.value = ''
    loading.value = true
  } else {
    loadingMore.value = true
  }
  try {
    const res = await api.notify.list(reset ? '' : cursor.value, 20)
    list.value = reset ? res.list : [...list.value, ...res.list]
    cursor.value = res.next_cursor
    hasMore.value = res.has_more
  } finally {
    loading.value = false
    loadingMore.value = false
  }
}

onMounted(async () => {
  await load()
  // 进页即全部已读（本地不改 is_read 样式，保留"新消息"视觉一轮）
  api.notify.markAllRead().catch(() => {})
})

function open(n: NotifyItem) {
  if (n.link) router.push(n.link)
}
</script>

<template>
  <div class="mx-auto max-w-160">
    <h2 class="m-0 mb-3.5 text-5">
      消息通知
    </h2>

    <el-skeleton
      v-if="loading"
      :rows="5"
      animated
    />
    <el-empty
      v-else-if="list.length === 0"
      description="还没有收到任何消息"
    />

    <div
      v-for="n in list"
      :key="n.id"
      class="notify-card flex items-center gap-3 p-4 mb-2.5 rounded-10px bg-white cursor-pointer"
      :class="{ 'is-unread': !n.is_read }"
      @click="open(n)"
    >
      <el-avatar
        :size="42"
        :src="n.sender.avatar || defaultAvatar"
        class="shrink-0 bg-primary text-white font-600"
        @click.stop="router.push(`/space/${n.sender.id}`)"
      >
        {{ n.sender.nickname?.slice(0, 1) ?? 'U' }}
      </el-avatar>
      <div class="flex-1 min-w-0">
        <p class="m-0 text-3.5 leading-normal truncate">
          <span class="font-600 mr-1">{{ n.sender.nickname }}</span>
          {{ n.content }}
        </p>
        <p class="flex items-center mt-1 mb-0 text-3 text-text-3">
          <span
            class="mr-1"
            :class="TYPE_META[n.type]?.icon"
          />{{ TYPE_META[n.type]?.label }} · {{ formatPubdate(n.created_at) }}
        </p>
      </div>
      <span
        v-if="!n.is_read"
        class="shrink-0 w-2 h-2 rounded-full bg-primary"
      />
    </div>

    <div
      v-if="hasMore && !loading"
      class="text-center py-3"
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

// hover / 状态类仍用 SCSS（原子类处理交互态不如局部选择器简洁）
.notify-card {
  transition: box-shadow 0.15s;

  &:hover {
    box-shadow: v.$shadow-card;
  }

  &.is-unread {
    background: #fff7f9;
  }
}
</style>

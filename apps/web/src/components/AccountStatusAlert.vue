<script setup lang="ts">
import { computed } from 'vue'
import { formatDateTime, type User } from '@dlidli/shared'

const props = defineProps<{ user: User | null }>()

// 仅异常状态（禁言/封禁/注销）展示；正常不渲染
const meta = computed(() => {
  const u = props.user
  if (!u) return null
  const fmt = (s?: string) => (s ? formatDateTime(s) : '')
  switch (u.status) {
    case 1:
      return {
        type: 'warning' as const,
        title: '账号禁言中',
        desc: u.muted_until
          ? `禁言至 ${fmt(u.muted_until)}，期间无法发布弹幕/评论/动态/投稿。`
          : '期间无法发布弹幕/评论/动态/投稿。',
      }
    case 2:
      return {
        type: 'error' as const,
        title: '账号封禁中',
        desc: u.banned_until
          ? `封禁至 ${fmt(u.banned_until)}，期间无法进行任何发布与互动操作。`
          : '账号已被永久封禁，无法进行任何发布与互动操作。',
      }
    case 3:
      return { type: 'info' as const, title: '账号已注销', desc: '该账号已注销。' }
    default:
      return null
  }
})
</script>

<template>
  <el-alert
    v-if="meta"
    :type="meta.type"
    :title="meta.title"
    :description="meta.desc"
    show-icon
    :closable="false"
    class="mb-4"
  />
</template>

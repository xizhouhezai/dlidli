<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { ElMessageBox } from 'element-plus'
import type { SensitiveWord } from '@dlidli/api-client'
import { adminApi } from '@/api'
import { useApiAction } from '@/composables/useApiAction'
import PageHead from '@/components/PageHead.vue'

const words = ref<SensitiveWord[]>([])
const loading = ref(false)
const newWord = ref('')
const { loading: adding, run } = useApiAction()

async function load() {
  loading.value = true
  try {
    words.value = (await adminApi.admin.sensitiveWords()).list
  } finally {
    loading.value = false
  }
}

onMounted(load)

async function add() {
  const w = newWord.value.trim()
  if (!w) return
  const ok = await run(() => adminApi.admin.addSensitiveWord(w), {
    success: '已添加，词库即时生效',
    fallback: '添加失败',
  })
  if (ok) {
    newWord.value = ''
    load()
  }
}

async function remove(word: SensitiveWord) {
  await run(
    async () => {
      await ElMessageBox.confirm(`确定删除敏感词「${word.word}」吗？`, '删除', { type: 'warning' })
      await adminApi.admin.deleteSensitiveWord(word.id)
      load()
    },
    { success: '已删除，词库即时生效', fallback: '删除失败' },
  )
}
</script>

<template>
  <div>
    <PageHead title="敏感词库" :sub="`共 ${words.length} 个自定义词（另含内置默认词）`" />

    <div class="page-card max-w-720px">
      <!-- 新增 -->
      <div class="flex gap-2 mb-4">
        <el-input
          v-model="newWord"
          maxlength="64"
          placeholder="输入要屏蔽的敏感词，回车或点添加"
          @keyup.enter="add"
        />
        <el-button
          v-perm="'sensitive:edit'"
          type="primary"
          class="pink-btn"
          :loading="adding"
          @click="add"
        >
          添加
        </el-button>
      </div>

      <el-alert
        type="info"
        :closable="false"
        title="命中敏感词的评论、动态、投稿标题将被拦截；增删后即时热加载生效，无需重启服务。"
        class="mb-4"
      />

      <el-skeleton v-if="loading" :rows="4" animated />
      <el-empty v-else-if="words.length === 0" description="还没有自定义敏感词" />
      <div v-else class="flex flex-wrap gap-2">
        <el-tag v-for="w in words" :key="w.id" size="large" closable @close="remove(w)">
          {{ w.word }}
        </el-tag>
      </div>
    </div>
  </div>
</template>

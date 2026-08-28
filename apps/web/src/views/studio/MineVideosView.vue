<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { formatCount, formatDuration, formatPubdate } from '@dlidli/shared'
import { ApiError, type VideoCard } from '@dlidli/api-client'
import { api } from '@/api'
import defaultCover from '@/assets/default-cover.svg'

const router = useRouter()

const PAGE_SIZE = 10
const list = ref<VideoCard[]>([])
const total = ref(0)
const page = ref(1)
const loading = ref(false)

const STATUS_MAP: Record<
  number,
  { text: string; type: 'info' | 'warning' | 'success' | 'danger' | 'primary' }
> = {
  0: { text: '草稿', type: 'info' },
  1: { text: '上传中', type: 'info' },
  2: { text: '转码中', type: 'primary' },
  3: { text: '审核中', type: 'warning' },
  4: { text: '已发布', type: 'success' },
  5: { text: '已驳回', type: 'danger' },
  6: { text: '已锁定', type: 'danger' },
}

async function load() {
  loading.value = true
  try {
    const res = await api.video.mine(page.value, PAGE_SIZE)
    list.value = res.list
    total.value = res.total
  } finally {
    loading.value = false
  }
}

onMounted(load)

async function remove(v: VideoCard) {
  try {
    await ElMessageBox.confirm(`确定删除《${v.title}》吗？删除后不可恢复。`, '删除稿件', {
      type: 'warning',
    })
  } catch {
    return
  }
  try {
    await api.video.remove(v.bvid)
    ElMessage.success('已删除')
    load()
  } catch (err) {
    ElMessage.error(err instanceof ApiError ? err.message : '删除失败')
  }
}

function open(v: VideoCard) {
  if (v.status === 4) router.push(`/video/${v.bvid}`)
}
</script>

<template>
  <div class="mx-auto max-w-900px">
    <div class="flex items-center gap-3 mb-4">
      <h2 class="m-0 text-5">稿件管理</h2>
      <span class="flex-1 text-3.25 text-text-2">共 {{ total }} 个稿件</span>
      <el-button type="primary" class="mine__upload" @click="router.push('/upload')">
        + 投稿
      </el-button>
    </div>

    <el-skeleton v-if="loading" :rows="5" animated />
    <el-empty v-else-if="list.length === 0" description="还没有投过稿，点击右上角开始创作吧" />

    <el-card v-for="v in list" :key="v.bvid" class="mb-3 rounded-10px" shadow="never">
      <div class="flex gap-4 items-start">
        <div
          class="mine-card__cover relative w-180px shrink-0 aspect-video rounded-8px overflow-hidden"
          :class="{ 'is-clickable': v.status === 4 }"
          @click="open(v)"
        >
          <img :src="v.cover || defaultCover" :alt="v.title" />
          <span
            v-if="v.duration > 0"
            class="absolute right-1.5 bottom-1.5 px-1.5 py-0.25 rounded-4px bg-black/60 text-white text-3"
            >{{ formatDuration(v.duration) }}</span
          >
        </div>
        <div class="flex-1 min-w-0">
          <p
            class="mine-card__title m-0 text-3.75 font-600 truncate"
            :class="{ 'is-clickable': v.status === 4 }"
            @click="open(v)"
          >
            {{ v.title }}
          </p>
          <p class="flex items-center gap-3 my-2 text-3 text-text-2">
            <el-tag size="small" :type="STATUS_MAP[v.status]?.type ?? 'info'">
              {{ STATUS_MAP[v.status]?.text ?? '未知' }}
            </el-tag>
            <span>投稿于 {{ formatPubdate(v.created_at) }}</span>
          </p>
          <p v-if="v.status === 4" class="flex items-center m-0 text-3.25 text-text-2">
            <span class="i-mingcute-play-circle-line mr-1" />{{ formatCount(v.stat.view) }}
            <span class="inline-block w-3" /><span class="i-mingcute-thumb-up-2-line mr-1" />{{
              formatCount(v.stat.like)
            }}
            <span class="inline-block w-3" /><span class="i-mingcute-comment-line mr-1" />{{
              formatCount(v.stat.comment)
            }}
            <span class="inline-block w-3" /><span class="i-mingcute-danmaku-line mr-1" />{{
              formatCount(v.stat.danmaku)
            }}
          </p>
          <p v-else-if="v.status === 2" class="m-0 text-3.25 text-text-2">
            转码处理中，完成后将自动进入审核…
          </p>
          <p v-else-if="v.status === 3" class="m-0 text-3.25 text-text-2">
            等待审核，通过后自动发布
          </p>
          <p v-else-if="v.status === 5" class="m-0 text-3.25 text-#f56c6c">
            稿件被驳回，可修改后重新投稿
          </p>
        </div>
        <div class="shrink-0">
          <el-button size="small" type="danger" plain @click="remove(v)"> 删除 </el-button>
        </div>
      </div>
    </el-card>

    <div v-if="total > PAGE_SIZE" class="flex justify-center py-3">
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

.mine__upload {
  --el-button-bg-color: #{v.$primary};
  --el-button-border-color: #{v.$primary};
  --el-button-hover-bg-color: #{v.$primary-hover};
  --el-button-hover-border-color: #{v.$primary-hover};
}

.mine-card__cover img {
  width: 100%;
  height: 100%;
  object-fit: cover;
  display: block;
}

.mine-card__title {
  &.is-clickable:hover {
    color: v.$primary;
  }
}

.is-clickable {
  cursor: pointer;

  &:hover {
    opacity: 0.9;
  }
}
</style>

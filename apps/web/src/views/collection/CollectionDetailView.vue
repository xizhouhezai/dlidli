<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useRoute } from 'vue-router'
import {
  ApiError,
  type CollectionDetail,
  type VideoCard as VideoCardData,
} from '@dlidli/api-client'
import { api } from '@/api'
import VideoCard from '@/components/VideoCard.vue'

const route = useRoute()
const col = ref<CollectionDetail | null>(null)
const videos = ref<VideoCardData[]>([])
const loading = ref(true)
const notFound = ref(false)

onMounted(async () => {
  try {
    const res = await api.collection.detail(route.params.id as string)
    col.value = res.collection
    videos.value = res.list
    document.title = `${res.collection.title} - DliDli`
  } catch (err) {
    if (err instanceof ApiError && err.code === 10005) notFound.value = true
    else notFound.value = true
  } finally {
    loading.value = false
  }
})
</script>

<template>
  <div class="mx-auto max-w-1100px">
    <el-skeleton v-if="loading" :rows="6" animated />

    <el-result v-else-if="notFound" icon="warning" title="合集不存在" />

    <template v-else-if="col">
      <div class="col-head">
        <div class="min-w-0">
          <h2 class="col-head__title">
            {{ col.title }}
          </h2>
          <p v-if="col.description" class="col-head__desc">
            {{ col.description }}
          </p>
          <p class="col-head__meta">共 {{ videos.length }} 个视频</p>
        </div>
      </div>

      <el-empty v-if="videos.length === 0" description="合集还没有视频" />
      <div v-else class="grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-4 gap-4">
        <VideoCard v-for="v in videos" :key="v.bvid" :video="v" />
      </div>
    </template>
  </div>
</template>

<style scoped lang="scss">
@use '@/styles/variables' as v;

.col-head {
  background: v.$surface;
  border-radius: v.$radius-lg;
  padding: 18px 20px;
  margin-bottom: 16px;
}

.col-head__title {
  margin: 0;
  font-size: 18px;
  font-weight: 700;
}

.col-head__desc {
  margin: 6px 0 0;
  font-size: 13px;
  color: v.$text-2;
}

.col-head__meta {
  margin: 6px 0 0;
  font-size: 12px;
  color: v.$text-3;
}
</style>

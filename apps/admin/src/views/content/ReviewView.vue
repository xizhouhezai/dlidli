<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { formatDuration, formatPubdate } from '@dlidli/shared'
import { type ReviewItem } from '@dlidli/api-client'
import { adminApi } from '@/api'
import { useApiAction } from '@/composables/useApiAction'
import PageHead from '@/components/PageHead.vue'
import defaultCover from '@/assets/default-cover.svg'

const list = ref<ReviewItem[]>([])
const total = ref(0)
const loading = ref(false)
const previewing = ref<ReviewItem | null>(null)
const { run } = useApiAction()

async function load() {
  loading.value = true
  try {
    const res = await adminApi.admin.reviewList(1, 50)
    list.value = res.list
    total.value = res.total
  } finally {
    loading.value = false
  }
}

onMounted(load)

async function approve(item: ReviewItem) {
  const ok = await run(() => adminApi.admin.review(item.bvid, true))
  if (ok) {
    ElMessage.success(`已通过《${item.title}》`)
    list.value = list.value.filter((v) => v.bvid !== item.bvid)
    total.value--
  }
}

async function reject(item: ReviewItem) {
  const ok = await run(async () => {
    const res = await ElMessageBox.prompt('请填写驳回原因（将展示给 UP 主）', '驳回稿件', {
      inputPlaceholder: '如：内容与标题不符 / 画质过低 / 涉嫌搬运',
      inputValidator: (v) => (v?.trim() ? true : '驳回必须填写原因'),
      type: 'warning',
    })
    await adminApi.admin.review(item.bvid, false, res.value.trim())
  }, { success: '已驳回' })
  if (ok) {
    list.value = list.value.filter((v) => v.bvid !== item.bvid)
    total.value--
  }
}
</script>

<template>
  <div>
    <PageHead
      title="审核工作台"
      :sub="`待审 ${total} 件`"
    >
      <template #actions>
        <el-button @click="load">
          <span class="i-mingcute-refresh-2-line mr-1" />刷新
        </el-button>
      </template>
    </PageHead>

    <div class="page-card">
      <el-skeleton
        v-if="loading"
        :rows="5"
        animated
      />
      <el-empty
        v-else-if="list.length === 0"
        description="审核队列已清空 🎉"
      />

      <el-card
        v-for="item in list"
        :key="item.bvid"
        class="mb-3.5 rounded-10px"
        shadow="never"
      >
        <div class="flex gap-4">
          <img
            class="w-200px shrink-0 rounded-8px object-cover aspect-video"
            :src="item.cover || defaultCover"
            :alt="item.title"
          >
          <div class="flex-1 min-w-0">
            <p class="m-0 text-4 font-600">
              {{ item.title }}
            </p>
            <p class="my-1.5 text-3 text-text-2">
              {{ item.bvid }} · UP：{{ item.owner.nickname }} ·
              时长 {{ formatDuration(item.duration) }} ·
              提交于 {{ formatPubdate(item.created_at) }}
            </p>
            <p
              v-if="item.description"
              class="review-card__desc m-0 mb-2 text-3.25 text-text-2"
            >
              {{ item.description }}
            </p>
            <div class="flex gap-2">
              <el-button
                size="small"
                @click="previewing = previewing?.bvid === item.bvid ? null : item"
              >
                {{ previewing?.bvid === item.bvid ? '收起预览' : '预览视频' }}
              </el-button>
              <el-button
                v-perm="'review:approve'"
                type="success"
                size="small"
                @click="approve(item)"
              >
                通过
              </el-button>
              <el-button
                v-perm="'review:approve'"
                type="danger"
                size="small"
                @click="reject(item)"
              >
                驳回
              </el-button>
            </div>
          </div>
        </div>
        <video
          v-if="previewing?.bvid === item.bvid"
          class="w-full max-h-400px mt-3 rounded-8px bg-black"
          :src="item.play_url"
          controls
          autoplay
          muted
        />
      </el-card>
    </div>
  </div>
</template>

<style scoped lang="scss">
@use '@/styles/variables' as v;

// 多行省略（Uno line-clamp 需额外配置，用 mixin 更直接）
.review-card__desc {
  @include v.ellipsis(2);
}
</style>

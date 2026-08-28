<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { type AdminVideoItem, type CategoryItem, apiErrorMessage } from '@dlidli/api-client'
import { VideoStatus } from '@dlidli/shared'
import { adminApi } from '@/api'
import { useApiAction } from '@/composables/useApiAction'
import { saveBlob } from '@/utils/download'
import PageHead from '@/components/PageHead.vue'

const list = ref<AdminVideoItem[]>([])
const total = ref(0)
const page = ref(1)
const pageSize = 10
const loading = ref(false)
const { run } = useApiAction()

// 筛选
const status = ref(0)
const categoryId = ref(0)
const keyword = ref('')
const categories = ref<CategoryItem[]>([])

const STATUS_MAP: Record<
  number,
  { text: string; type: 'info' | 'warning' | 'success' | 'danger' | 'primary' }
> = {
  [VideoStatus.Draft]: { text: '草稿', type: 'info' },
  [VideoStatus.Uploading]: { text: '上传中', type: 'info' },
  [VideoStatus.Transcoding]: { text: '转码中', type: 'primary' },
  [VideoStatus.Reviewing]: { text: '审核中', type: 'warning' },
  [VideoStatus.Published]: { text: '已发布', type: 'success' },
  [VideoStatus.Rejected]: { text: '已驳回', type: 'danger' },
  [VideoStatus.Locked]: { text: '已锁定', type: 'danger' },
}

// 筛选项：草稿（0）不提供筛选（管理端面向已提交稿件，列表仍展示草稿 tag）
const STATUS_OPTIONS = computed(() =>
  Object.entries(STATUS_MAP)
    .filter(([k]) => Number(k) !== VideoStatus.Draft)
    .map(([k, v]) => ({ value: Number(k), text: v.text })),
)

async function load(reset = false) {
  if (reset) page.value = 1
  loading.value = true
  try {
    const res = await adminApi.admin.videoList({
      status: status.value || undefined,
      category_id: categoryId.value || undefined,
      keyword: keyword.value.trim() || undefined,
      page: page.value,
      page_size: pageSize,
    })
    list.value = res.list ?? []
    total.value = res.total
  } catch (err) {
    ElMessage.error(apiErrorMessage(err, '列表加载失败，请稍后再试'))
  } finally {
    loading.value = false
  }
}

function search() {
  load(true)
}

// 导出 CSV（当前筛选，SYS-06）
async function exportCsv() {
  await run(
    () =>
      adminApi.http
        .download('/api/v1/admin/videos/export', {
          params: {
            status: status.value || undefined,
            category_id: categoryId.value || undefined,
            keyword: keyword.value.trim() || undefined,
          },
          fallbackName: `videos-${Date.now()}.csv`,
        })
        .then(saveBlob),
    { success: '导出成功', fallback: '导出失败，请稍后再试' },
  )
}

function categoryName(id: number) {
  return categories.value.find((c) => c.id === id)?.name ?? '—'
}

async function toggleStatus(item: AdminVideoItem) {
  const isPublished = item.status === VideoStatus.Published
  const action = isPublished ? '下架' : '恢复'
  await ElMessageBox.confirm(
    `确定${action}《${item.title}》吗？${isPublished ? '下架后前台不可见。' : ''}`,
    `${action}稿件`,
    { type: 'warning' },
  )
  const ok = await run(
    () =>
      adminApi.admin.updateVideoStatus(
        item.bvid,
        isPublished ? VideoStatus.Locked : VideoStatus.Published,
      ),
    { success: `已${action}`, fallback: `${action}失败` },
  )
  if (ok) load()
}

async function remove(item: AdminVideoItem) {
  await ElMessageBox.confirm(
    `确定删除《${item.title}》吗？删除后作者与游客均不可见，不可恢复。`,
    '删除稿件',
    { type: 'error' },
  )
  const ok = await run(() => adminApi.admin.deleteVideo(item.bvid), {
    success: '已删除',
    fallback: '删除失败',
  })
  if (ok) load()
}

onMounted(async () => {
  load()
  try {
    categories.value = (await adminApi.admin.categories()).list
  } catch {
    // 分区加载失败不阻塞
  }
})
</script>

<template>
  <div>
    <PageHead title="稿件管理" :sub="`全站稿件 ${total} 条（含全部状态）`" />

    <div class="page-card">
      <!-- 筛选栏 -->
      <div class="flex flex-wrap items-center gap-3 mb-4">
        <el-select v-model="status" placeholder="全部状态" class="w-130px" @change="search">
          <el-option label="全部状态" :value="0" />
          <el-option
            v-for="opt in STATUS_OPTIONS"
            :key="opt.value"
            :label="opt.text"
            :value="opt.value"
          />
        </el-select>
        <el-select v-model="categoryId" placeholder="全部分区" class="w-140px" @change="search">
          <el-option label="全部分区" :value="0" />
          <el-option v-for="c in categories" :key="c.id" :label="c.name" :value="c.id" />
        </el-select>
        <el-input
          v-model="keyword"
          placeholder="标题关键词"
          class="w-200px"
          clearable
          @keyup.enter="search"
          @clear="search"
        />
        <el-button type="primary" class="pink-btn" @click="search"> 查询 </el-button>
        <el-button v-perm="'video:view'" class="pink-btn" @click="exportCsv"> 导出 CSV </el-button>
      </div>

      <el-table v-loading="loading" :data="list" stripe>
        <el-table-column label="稿件" min-width="200">
          <template #default="{ row }">
            <div class="flex items-center gap-2">
              <img
                v-if="row.cover"
                :src="row.cover"
                alt=""
                class="w-70px aspect-video object-cover rounded-4px bg-#f1f2f3"
              />
              <span v-else class="w-70px aspect-video rounded-4px bg-#f1f2f3 inline-block" />
              <div class="min-w-0">
                <p class="m-0 text-3.5 font-600 truncate max-w-260px">
                  {{ row.title }}
                </p>
                <p class="m-0 mt-1 text-3 text-text-3">
                  {{ row.bvid }} · {{ row.owner?.nickname ?? '—' }}
                </p>
              </div>
            </div>
          </template>
        </el-table-column>
        <el-table-column label="分区" width="90">
          <template #default="{ row }">
            {{ categoryName(row.category_id) }}
          </template>
        </el-table-column>
        <el-table-column label="状态" width="90">
          <template #default="{ row }">
            <el-tag size="small" effect="plain" :type="STATUS_MAP[row.status]?.type ?? 'info'">
              {{ STATUS_MAP[row.status]?.text ?? '未知' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="播放" width="80">
          <template #default="{ row }">
            {{ row.stat?.view ?? 0 }}
          </template>
        </el-table-column>
        <el-table-column label="发布时间" width="160">
          <template #default="{ row }">
            {{ row.published_at ? row.published_at.slice(0, 10) : '—' }}
          </template>
        </el-table-column>
        <el-table-column label="操作" width="150" fixed="right">
          <template #default="{ row }">
            <template
              v-if="row.status === VideoStatus.Published || row.status === VideoStatus.Locked"
            >
              <el-button
                v-perm="'video:manage'"
                link
                :type="row.status === VideoStatus.Published ? 'warning' : 'primary'"
                @click="toggleStatus(row)"
              >
                {{ row.status === VideoStatus.Published ? '下架' : '恢复' }}
              </el-button>
              <el-button v-perm="'video:manage'" link type="danger" @click="remove(row)">
                删除
              </el-button>
            </template>
            <span v-else class="text-3 text-text-3"> 不可操作 </span>
          </template>
        </el-table-column>
      </el-table>

      <div class="flex justify-end mt-4">
        <el-pagination
          v-model:current-page="page"
          :total="total"
          :page-size="pageSize"
          layout="total, prev, pager, next"
          @current-change="() => load()"
        />
      </div>
    </div>
  </div>
</template>

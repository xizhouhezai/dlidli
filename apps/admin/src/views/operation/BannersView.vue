<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { type BannerItem } from '@dlidli/api-client'
import { adminApi } from '@/api'
import { useApiAction } from '@/composables/useApiAction'
import PageHead from '@/components/PageHead.vue'

const list = ref<BannerItem[]>([])
const loading = ref(false)
const { run } = useApiAction()

async function load() {
  loading.value = true
  try {
    list.value = (await adminApi.admin.banners()).list
  } finally {
    loading.value = false
  }
}

// 编辑弹窗
const dialogVisible = ref(false)
const editingId = ref('')
const form = reactive({ title: '', image: '', bvid: '', sort: 0, status: 0 })
const saving = ref(false)

function openCreate() {
  editingId.value = ''
  form.title = ''
  form.image = ''
  form.bvid = ''
  form.sort = 0
  form.status = 0
  dialogVisible.value = true
}

function openEdit(item: BannerItem) {
  editingId.value = item.id
  form.title = item.title
  form.image = item.image
  form.bvid = item.bvid
  form.sort = item.sort
  form.status = item.status
  dialogVisible.value = true
}

async function save() {
  if (!form.bvid && !form.image) {
    ElMessage.warning('跳转稿件与图片至少填一项')
    return
  }
  saving.value = true
  try {
    if (editingId.value) {
      await adminApi.admin.updateBanner(editingId.value, form)
    } else {
      await adminApi.admin.createBanner(form)
    }
    dialogVisible.value = false
    ElMessage.success('已保存')
    await load()
  } catch (err) {
    ElMessage.error(err instanceof Error ? err.message : '保存失败')
  } finally {
    saving.value = false
  }
}

async function remove(item: BannerItem) {
  await ElMessageBox.confirm(`确定删除 Banner「${item.title || '#' + item.id}」吗？`, '删除 Banner', { type: 'warning' })
  const ok = await run(() => adminApi.admin.deleteBanner(item.id), { success: '已删除', fallback: '删除失败' })
  if (ok) load()
}

async function toggleStatus(item: BannerItem) {
  const ok = await run(
    () => adminApi.admin.updateBanner(item.id, { status: item.status === 0 ? 1 : 0 }),
    { success: item.status === 0 ? '已停用' : '已启用', fallback: '操作失败' },
  )
  if (ok) load()
}

onMounted(load)
</script>

<template>
  <div>
    <PageHead
      title="Banner 配置"
      :sub="`共 ${list.length} 个 Banner（首页推荐轮播）`"
    />

    <div class="page-card">
      <div class="flex justify-end mb-4">
        <el-button
          v-perm="'banner:edit'"
          type="primary"
          class="pink-btn"
          @click="openCreate"
        >
          新增 Banner
        </el-button>
      </div>

      <el-table
        v-loading="loading"
        :data="list"
        stripe
      >
        <el-table-column
          label="预览"
          width="150"
        >
          <template #default="{ row }">
            <img
              v-if="row.image"
              :src="row.image"
              alt=""
              class="w-120px aspect-video object-cover rounded-6px bg-#f1f2f3"
            >
            <span
              v-else
              class="text-3 text-text-3"
            >
              无图片
            </span>
          </template>
        </el-table-column>
        <el-table-column
          prop="title"
          label="标题"
          min-width="140"
        >
          <template #default="{ row }">
            <span :class="row.title ? '' : 'text-text-3'">{{ row.title || '（未命名）' }}</span>
          </template>
        </el-table-column>
        <el-table-column
          prop="bvid"
          label="跳转稿件"
          width="150"
        >
          <template #default="{ row }">
            <span :class="row.bvid ? '' : 'text-text-3'">{{ row.bvid || '—' }}</span>
          </template>
        </el-table-column>
        <el-table-column
          prop="sort"
          label="排序"
          width="80"
        />
        <el-table-column
          label="状态"
          width="90"
        >
          <template #default="{ row }">
            <el-tag
              :type="row.status === 0 ? 'success' : 'info'"
              size="small"
            >
              {{ row.status === 0 ? '启用' : '停用' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column
          label="操作"
          width="160"
          fixed="right"
        >
          <template #default="{ row }">
            <el-button
              v-perm="'banner:edit'"
              link
              type="primary"
              @click="openEdit(row)"
            >
              编辑
            </el-button>
            <el-button
              v-perm="'banner:edit'"
              link
              @click="toggleStatus(row)"
            >
              {{ row.status === 0 ? '停用' : '启用' }}
            </el-button>
            <el-button
              v-perm="'banner:edit'"
              link
              type="danger"
              @click="remove(row)"
            >
              删除
            </el-button>
          </template>
        </el-table-column>
      </el-table>
    </div>

    <!-- 编辑弹窗 -->
    <el-dialog
      v-model="dialogVisible"
      :title="editingId ? '编辑 Banner' : '新增 Banner'"
      width="480px"
    >
      <el-form
        label-width="80px"
        class="max-w-440px"
      >
        <el-form-item label="标题">
          <el-input
            v-model="form.title"
            placeholder="轮播展示标题（选填）"
            maxlength="64"
          />
        </el-form-item>
        <el-form-item label="图片 URL">
          <el-input
            v-model="form.image"
            placeholder="留空则自动使用稿件封面"
            maxlength="255"
          />
        </el-form-item>
        <el-form-item label="跳转稿件">
          <el-input
            v-model="form.bvid"
            placeholder="如 DV2TqnH0737WC（选填）"
            maxlength="16"
          />
        </el-form-item>
        <el-form-item label="排序">
          <el-input-number
            v-model="form.sort"
            :min="0"
          />
        </el-form-item>
        <el-form-item label="状态">
          <el-radio-group v-model="form.status">
            <el-radio :value="0">
              启用
            </el-radio>
            <el-radio :value="1">
              停用
            </el-radio>
          </el-radio-group>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">
          取消
        </el-button>
        <el-button
          type="primary"
          class="pink-btn"
          :loading="saving"
          @click="save"
        >
          保存
        </el-button>
      </template>
    </el-dialog>
  </div>
</template>

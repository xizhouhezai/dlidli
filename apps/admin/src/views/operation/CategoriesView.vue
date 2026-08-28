<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import type { AdminCategory } from '@dlidli/api-client'
import { adminApi } from '@/api'
import { useApiAction } from '@/composables/useApiAction'
import PageHead from '@/components/PageHead.vue'

const list = ref<AdminCategory[]>([])
const loading = ref(false)
const { loading: saving, run } = useApiAction()

// 一级分区（含各自子分区），按 sort 排
const topCategories = computed(() =>
  list.value.filter((c) => c.parent_id === 0).sort((a, b) => a.sort - b.sort),
)
function childrenOf(pid: number) {
  return list.value.filter((c) => c.parent_id === pid).sort((a, b) => a.sort - b.sort)
}
function parentName(pid: number) {
  return list.value.find((c) => c.id === pid)?.name ?? '—'
}

async function load() {
  loading.value = true
  try {
    list.value = (await adminApi.admin.categories()).list
  } finally {
    loading.value = false
  }
}
onMounted(load)

// 对话框
const dialogVisible = ref(false)
const editing = ref<AdminCategory | null>(null)
const form = ref<{ parent_id: number; name: string; sort: number; status: number }>({
  parent_id: 0,
  name: '',
  sort: 0,
  status: 0,
})

function openCreate(parentId = 0) {
  editing.value = null
  form.value = { parent_id: parentId, name: '', sort: 0, status: 0 }
  dialogVisible.value = true
}
function openEdit(c: AdminCategory) {
  editing.value = c
  form.value = { parent_id: c.parent_id, name: c.name, sort: c.sort, status: c.status }
  dialogVisible.value = true
}

async function save() {
  if (!form.value.name.trim()) {
    ElMessage.warning('请填写分区名')
    return
  }
  const ok = await run(
    async () => {
      if (editing.value) {
        await adminApi.admin.updateCategory(editing.value.id, form.value)
      } else {
        await adminApi.admin.createCategory(form.value)
      }
    },
    { success: editing.value ? '已保存' : '已创建', fallback: '保存失败' },
  )
  if (ok) {
    dialogVisible.value = false
    load()
  }
}

async function remove(c: AdminCategory) {
  const ok = await run(
    async () => {
      await ElMessageBox.confirm(`确定删除分区「${c.name}」吗？`, '删除', { type: 'warning' })
      await adminApi.admin.deleteCategory(c.id)
    },
    { success: '已删除', fallback: '删除失败' },
  )
  if (ok) load()
}
</script>

<template>
  <div>
    <PageHead title="分区管理" :sub="`共 ${topCategories.length} 个一级分区`">
      <template #actions>
        <el-button v-perm="'category:edit'" type="primary" class="pink-btn" @click="openCreate(0)">
          <span class="i-mingcute-add-line mr-1" />新建一级分区
        </el-button>
      </template>
    </PageHead>

    <div v-loading="loading" class="cat-list">
      <div v-for="top in topCategories" :key="top.id" class="page-card mb-3">
        <!-- 一级分区行 -->
        <div class="flex items-center gap-3">
          <span class="i-mingcute-classify-2-line text-5 text-primary" />
          <span class="text-4 font-600">{{ top.name }}</span>
          <el-tag size="small" :type="top.status === 0 ? 'success' : 'info'">
            {{ top.status === 0 ? '启用' : '停用' }}
          </el-tag>
          <span class="text-3 text-text-3">sort {{ top.sort }}</span>
          <div class="flex-1" />
          <el-button v-perm="'category:edit'" size="small" @click="openCreate(top.id)">
            <span class="i-mingcute-add-line mr-1" />子分区
          </el-button>
          <el-button v-perm="'category:edit'" size="small" @click="openEdit(top)"> 编辑 </el-button>
          <el-button v-perm="'category:edit'" size="small" type="danger" @click="remove(top)">
            删除
          </el-button>
        </div>

        <!-- 子分区 -->
        <div v-if="childrenOf(top.id).length" class="mt-3 pl-8 flex flex-wrap gap-2">
          <div v-for="sub in childrenOf(top.id)" :key="sub.id" class="cat-sub">
            <span>{{ sub.name }}</span>
            <el-tag v-if="sub.status !== 0" size="small" type="info"> 停用 </el-tag>
            <span class="i-mingcute-edit-line cat-sub__act" @click="openEdit(sub)" />
            <span class="i-mingcute-delete-2-line cat-sub__act" @click="remove(sub)" />
          </div>
        </div>
      </div>
    </div>

    <!-- 新建/编辑对话框 -->
    <el-dialog
      v-model="dialogVisible"
      :title="editing ? '编辑分区' : form.parent_id ? '新建子分区' : '新建一级分区'"
      width="440px"
    >
      <el-form label-width="72px">
        <el-form-item v-if="form.parent_id !== 0" label="所属分区">
          <span class="text-text-2">{{ parentName(form.parent_id) }}</span>
        </el-form-item>
        <el-form-item label="分区名">
          <el-input v-model="form.name" maxlength="32" />
        </el-form-item>
        <el-form-item label="排序">
          <el-input-number v-model="form.sort" :min="0" :max="9999" />
          <span class="ml-2 text-3 text-text-3">数字越小越靠前</span>
        </el-form-item>
        <el-form-item label="状态">
          <el-switch
            v-model="form.status"
            :active-value="0"
            :inactive-value="1"
            active-text="启用"
            inactive-text="停用"
            inline-prompt
          />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false"> 取消 </el-button>
        <el-button type="primary" class="pink-btn" :loading="saving" @click="save">
          保存
        </el-button>
      </template>
    </el-dialog>
  </div>
</template>

<style scoped lang="scss">
@use '@/styles/variables' as v;

.cat-sub {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 6px 12px;
  border-radius: 8px;
  background: v.$bg;
  font-size: 13px;
}

.cat-sub__act {
  cursor: pointer;
  color: v.$text-3;
  transition: color 0.15s;

  &:hover {
    color: v.$primary;
  }
}
</style>

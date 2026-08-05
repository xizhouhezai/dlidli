<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { type ExperimentItem } from '@dlidli/api-client'
import { adminApi } from '@/api'
import { useApiAction } from '@/composables/useApiAction'
import PageHead from '@/components/PageHead.vue'

const list = ref<ExperimentItem[]>([])
const loading = ref(false)
const { run } = useApiAction()

async function load() {
  loading.value = true
  try {
    list.value = (await adminApi.admin.experiments()).list ?? []
  } finally {
    loading.value = false
  }
}

// 编辑弹窗
const dialogVisible = ref(false)
const editingId = ref('')
const form = reactive({ name: '', target: 'recommend', variant_a: '', variant_b: '', ratio: 50, status: 0, remark: '' })
const saving = ref(false)

const TARGET_HINT: Record<string, string> = {
  recommend: '首页推荐策略（A 组混合召回 / B 组纯热度）',
}

function openCreate() {
  editingId.value = ''
  form.name = ''
  form.target = 'recommend'
  form.variant_a = 'hybrid'
  form.variant_b = 'hot_only'
  form.ratio = 50
  form.status = 0
  form.remark = ''
  dialogVisible.value = true
}

function openEdit(item: ExperimentItem) {
  editingId.value = item.id
  form.name = item.name
  form.target = item.target
  form.variant_a = item.variant_a
  form.variant_b = item.variant_b
  form.ratio = item.ratio
  form.status = item.status
  form.remark = item.remark
  dialogVisible.value = true
}

async function save() {
  if (!form.name || !form.target || !form.variant_a || !form.variant_b) {
    ElMessage.warning('名称、场景与策略标识均为必填')
    return
  }
  saving.value = true
  try {
    if (editingId.value) {
      await adminApi.admin.updateExperiment(editingId.value, form)
    } else {
      await adminApi.admin.createExperiment(form)
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

async function remove(item: ExperimentItem) {
  await ElMessageBox.confirm(`确定删除实验「${item.name}」吗？删除后流量恢复默认策略。`, '删除实验', { type: 'warning' })
  const ok = await run(() => adminApi.admin.deleteExperiment(item.id), { success: '已删除', fallback: '删除失败' })
  if (ok) load()
}

async function toggleStatus(item: ExperimentItem) {
  const ok = await run(
    () => adminApi.admin.updateExperiment(item.id, { ...item, status: item.status === 0 ? 1 : 0 }),
    { success: item.status === 0 ? '已停用' : '已启用', fallback: '操作失败' },
  )
  if (ok) load()
}

onMounted(load)
</script>

<template>
  <div>
    <PageHead
      title="A/B 实验"
      :sub="`共 ${list.length} 个实验（按用户哈希稳定分流）`"
    />

    <div class="page-card">
      <div class="flex justify-end mb-4">
        <el-button
          v-perm="'experiment:edit'"
          type="primary"
          class="pink-btn"
          @click="openCreate"
        >
          新建实验
        </el-button>
      </div>

      <el-table
        v-loading="loading"
        :data="list"
        stripe
      >
        <el-table-column
          prop="name"
          label="实验名称"
          min-width="140"
        />
        <el-table-column
          label="应用场景"
          width="110"
        >
          <template #default="{ row }">
            <el-tag
              size="small"
              effect="plain"
            >
              {{ row.target }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column
          label="A 组策略"
          width="120"
        >
          <template #default="{ row }">
            {{ row.variant_a }}
          </template>
        </el-table-column>
        <el-table-column
          label="B 组策略"
          width="120"
        >
          <template #default="{ row }">
            {{ row.variant_b }}
          </template>
        </el-table-column>
        <el-table-column
          label="B 组占比"
          width="100"
        >
          <template #default="{ row }">
            {{ row.ratio }}%
          </template>
        </el-table-column>
        <el-table-column
          label="状态"
          width="80"
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
          prop="remark"
          label="说明"
          min-width="140"
        >
          <template #default="{ row }">
            <span :class="row.remark ? '' : 'text-text-3'">{{ row.remark || '—' }}</span>
          </template>
        </el-table-column>
        <el-table-column
          label="操作"
          width="160"
          fixed="right"
        >
          <template #default="{ row }">
            <el-button
              v-perm="'experiment:edit'"
              link
              type="primary"
              @click="openEdit(row)"
            >
              编辑
            </el-button>
            <el-button
              v-perm="'experiment:edit'"
              link
              @click="toggleStatus(row)"
            >
              {{ row.status === 0 ? '停用' : '启用' }}
            </el-button>
            <el-button
              v-perm="'experiment:edit'"
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
      :title="editingId ? '编辑实验' : '新建实验'"
      width="480px"
    >
      <el-form
        label-width="80px"
        class="max-w-440px"
      >
        <el-form-item label="实验名称">
          <el-input
            v-model="form.name"
            placeholder="如：推荐策略 A/B 测试"
            maxlength="64"
          />
        </el-form-item>
        <el-form-item label="应用场景">
          <el-select
            v-model="form.target"
            class="w-full"
          >
            <el-option
              label="首页推荐（recommend）"
              value="recommend"
            />
          </el-select>
          <div class="text-3 text-text-3 mt-1">
            {{ TARGET_HINT[form.target] }}
          </div>
        </el-form-item>
        <el-form-item label="A 组策略">
          <el-input
            v-model="form.variant_a"
            placeholder="如 hybrid（混合召回）"
            maxlength="64"
          />
        </el-form-item>
        <el-form-item label="B 组策略">
          <el-input
            v-model="form.variant_b"
            placeholder="如 hot_only（纯热度榜）"
            maxlength="64"
          />
        </el-form-item>
        <el-form-item label="B 组占比">
          <el-slider
            v-model="form.ratio"
            :min="0"
            :max="100"
            show-input
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
        <el-form-item label="说明">
          <el-input
            v-model="form.remark"
            placeholder="实验目的/上线计划（选填）"
            maxlength="200"
          />
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

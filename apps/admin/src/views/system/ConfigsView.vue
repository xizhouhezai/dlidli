<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { type SystemConfigItem } from '@dlidli/api-client'
import { adminApi } from '@/api'
import { useApiAction } from '@/composables/useApiAction'
import PageHead from '@/components/PageHead.vue'

const list = ref<SystemConfigItem[]>([])
const loading = ref(false)
const { run } = useApiAction()

async function load() {
  loading.value = true
  try {
    list.value = (await adminApi.admin.configs()).list
  } finally {
    loading.value = false
  }
}

// 编辑弹窗
const dialogVisible = ref(false)
const editingId = ref('')
const form = reactive({ config_key: '', name: '', value: '', remark: '' })
const saving = ref(false)

function openCreate() {
  editingId.value = ''
  form.config_key = ''
  form.name = ''
  form.value = ''
  form.remark = ''
  dialogVisible.value = true
}

function openEdit(item: SystemConfigItem) {
  editingId.value = item.id
  form.config_key = item.config_key
  form.name = item.name
  form.value = item.value
  form.remark = item.remark
  dialogVisible.value = true
}

async function save() {
  if (!form.config_key.trim()) {
    ElMessage.warning('配置键不能为空')
    return
  }
  saving.value = true
  try {
    if (editingId.value) {
      await adminApi.admin.updateConfig(editingId.value, form)
    } else {
      await adminApi.admin.createConfig(form)
    }
    dialogVisible.value = false
    ElMessage.success('已保存（热更新生效）')
    await load()
  } catch (err) {
    ElMessage.error(err instanceof Error ? err.message : '保存失败')
  } finally {
    saving.value = false
  }
}

async function remove(item: SystemConfigItem) {
  await ElMessageBox.confirm(`确定删除配置「${item.config_key}」吗？`, '删除配置', { type: 'warning' })
  const ok = await run(() => adminApi.admin.deleteConfig(item.id), { success: '已删除', fallback: '删除失败' })
  if (ok) load()
}

onMounted(load)
</script>

<template>
  <div>
    <PageHead
      title="系统配置"
      :sub="`共 ${list.length} 项配置（键值，热更新生效）`"
    />

    <div class="page-card">
      <div class="flex justify-end mb-4">
        <el-button
          v-perm="'config:edit'"
          type="primary"
          class="pink-btn"
          @click="openCreate"
        >
          新增配置
        </el-button>
      </div>

      <el-table
        v-loading="loading"
        :data="list"
        stripe
      >
        <el-table-column
          prop="config_key"
          label="配置键"
          min-width="160"
        >
          <template #default="{ row }">
            <code class="text-3.5">{{ row.config_key }}</code>
          </template>
        </el-table-column>
        <el-table-column
          prop="name"
          label="名称"
          width="140"
        />
        <el-table-column
          prop="value"
          label="值"
          min-width="120"
        >
          <template #default="{ row }">
            <span class="truncate block">{{ row.value || '—' }}</span>
          </template>
        </el-table-column>
        <el-table-column
          prop="remark"
          label="说明"
          min-width="160"
        >
          <template #default="{ row }">
            <span
              class="truncate block text-3 text-text-2"
              :title="row.remark"
            >{{ row.remark || '—' }}</span>
          </template>
        </el-table-column>
        <el-table-column
          label="更新时间"
          width="160"
        >
          <template #default="{ row }">
            {{ new Date(row.updated_at).toLocaleString() }}
          </template>
        </el-table-column>
        <el-table-column
          label="操作"
          width="130"
          fixed="right"
        >
          <template #default="{ row }">
            <el-button
              v-perm="'config:edit'"
              link
              type="primary"
              @click="openEdit(row)"
            >
              编辑
            </el-button>
            <el-button
              v-perm="'config:edit'"
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
      :title="editingId ? '编辑配置' : '新增配置'"
      width="460px"
    >
      <el-form
        label-width="72px"
        class="max-w-420px"
      >
        <el-form-item label="配置键">
          <el-input
            v-model="form.config_key"
            placeholder="如 audit:sampling"
            maxlength="64"
          />
        </el-form-item>
        <el-form-item label="名称">
          <el-input
            v-model="form.name"
            maxlength="64"
          />
        </el-form-item>
        <el-form-item label="值">
          <el-input
            v-model="form.value"
            maxlength="500"
          />
        </el-form-item>
        <el-form-item label="说明">
          <el-input
            v-model="form.remark"
            type="textarea"
            :rows="2"
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

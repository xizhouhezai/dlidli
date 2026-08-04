<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { type DataDictItem } from '@dlidli/api-client'
import { adminApi } from '@/api'
import { useApiAction } from '@/composables/useApiAction'
import PageHead from '@/components/PageHead.vue'

const groups = ref<Record<string, DataDictItem[]>>({})
const loading = ref(false)
const { run } = useApiAction()

async function load() {
  loading.value = true
  try {
    groups.value = (await adminApi.admin.dicts()).groups
  } finally {
    loading.value = false
  }
}

// 编辑弹窗
const dialogVisible = ref(false)
const editingId = ref('')
const form = reactive({ dict_type: '', label: '', value: '', sort: 0, remark: '' })
const saving = ref(false)

function openCreate(dictType = '') {
  editingId.value = ''
  form.dict_type = dictType
  form.label = ''
  form.value = ''
  form.sort = 0
  form.remark = ''
  dialogVisible.value = true
}

function openEdit(item: DataDictItem) {
  editingId.value = item.id
  form.dict_type = item.dict_type
  form.label = item.label
  form.value = item.value
  form.sort = item.sort
  form.remark = item.remark
  dialogVisible.value = true
}

async function save() {
  if (!form.dict_type.trim() || !form.label.trim() || !form.value.trim()) {
    ElMessage.warning('类型、展示名、值均为必填')
    return
  }
  saving.value = true
  try {
    if (editingId.value) {
      await adminApi.admin.updateDict(editingId.value, form)
    } else {
      await adminApi.admin.createDict(form)
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

async function remove(item: DataDictItem) {
  await ElMessageBox.confirm(`确定删除字典项「${item.label}」吗？`, '删除字典项', { type: 'warning' })
  const ok = await run(() => adminApi.admin.deleteDict(item.id), { success: '已删除', fallback: '删除失败' })
  if (ok) load()
}

onMounted(load)
</script>

<template>
  <div>
    <PageHead
      title="数据字典"
      :sub="`共 ${Object.keys(groups).length} 个字典类型`"
    />

    <div class="page-card">
      <div class="flex justify-end mb-4">
        <el-button
          v-perm="'dict:edit'"
          type="primary"
          class="pink-btn"
          @click="openCreate()"
        >
          新增字典项
        </el-button>
      </div>

      <el-skeleton
        v-if="loading"
        :rows="6"
        animated
      />

      <template v-else>
        <el-empty
          v-if="Object.keys(groups).length === 0"
          description="暂无字典数据"
        />
        <div
          v-for="(items, type) in groups"
          :key="type"
          class="mb-6"
        >
          <div class="flex items-center gap-2 mb-2">
            <h3 class="m-0 text-3.75 font-600">
              {{ type }}
            </h3>
            <el-button
              v-perm="'dict:edit'"
              link
              size="small"
              type="primary"
              @click="openCreate(type)"
            >
              + 添加
            </el-button>
          </div>
          <div class="flex flex-wrap gap-2">
            <el-tag
              v-for="item in items"
              :key="item.id"
              closable
              effect="plain"
              @close="remove(item)"
              @click="openEdit(item)"
            >
              {{ item.label }} = {{ item.value }}
              <span
                v-if="item.remark"
                class="ml-1 opacity-60"
              >{{ item.remark }}</span>
            </el-tag>
          </div>
        </div>
      </template>
    </div>

    <!-- 编辑弹窗 -->
    <el-dialog
      v-model="dialogVisible"
      :title="editingId ? '编辑字典项' : '新增字典项'"
      width="460px"
    >
      <el-form
        label-width="72px"
        class="max-w-420px"
      >
        <el-form-item label="类型">
          <el-input
            v-model="form.dict_type"
            placeholder="如 report_reason"
            maxlength="32"
          />
        </el-form-item>
        <el-form-item label="展示名">
          <el-input
            v-model="form.label"
            maxlength="64"
          />
        </el-form-item>
        <el-form-item label="值">
          <el-input
            v-model="form.value"
            maxlength="64"
          />
        </el-form-item>
        <el-form-item label="排序">
          <el-input-number
            v-model="form.sort"
            :min="0"
          />
        </el-form-item>
        <el-form-item label="备注">
          <el-input
            v-model="form.remark"
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

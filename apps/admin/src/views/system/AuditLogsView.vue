<script setup lang="ts">
import { ref } from 'vue'
import { type AuditLogItem } from '@dlidli/api-client'
import { formatDateTime } from '@dlidli/shared'
import { adminApi } from '@/api'
import { usePagedList } from '@/composables/usePagedList'
import { useApiAction } from '@/composables/useApiAction'
import { saveBlob } from '@/utils/download'
import PageHead from '@/components/PageHead.vue'

const adminId = ref('')
const action = ref('')
const objType = ref('')
const dateRange = ref<[string, string] | null>(null)

const ACTIONS: Array<{ value: string; label: string }> = [
  { value: 'approve', label: '审核通过' },
  { value: 'reject', label: '审核驳回' },
  { value: 'mute', label: '禁言' },
  { value: 'unmute', label: '解除禁言' },
  { value: 'ban', label: '封禁' },
  { value: 'unban', label: '解除封禁' },
  { value: 'add_word', label: '新增敏感词' },
  { value: 'del_word', label: '删除敏感词' },
  { value: 'add_category', label: '新增分区' },
  { value: 'edit_category', label: '编辑分区' },
  { value: 'del_category', label: '删除分区' },
  { value: 'add_permission', label: '新增权限点' },
  { value: 'edit_permission', label: '编辑权限点' },
  { value: 'del_permission', label: '删除权限点' },
  { value: 'add_role', label: '新增角色' },
  { value: 'edit_role', label: '编辑角色' },
  { value: 'del_role', label: '删除角色' },
  { value: 'add_admin', label: '新增账号' },
  { value: 'edit_admin', label: '编辑账号' },
  { value: 'del_admin', label: '删除账号' },
  { value: 'reset_pwd', label: '重置密码' },
  { value: 'add_config', label: '新增配置' },
  { value: 'edit_config', label: '编辑配置' },
  { value: 'del_config', label: '删除配置' },
  { value: 'add_dict', label: '新增字典项' },
  { value: 'edit_dict', label: '编辑字典项' },
  { value: 'del_dict', label: '删除字典项' },
]

const OBJ_TYPES: Array<{ value: string; label: string }> = [
  { value: 'video', label: '稿件' },
  { value: 'user', label: '用户' },
  { value: 'sensitive_word', label: '敏感词' },
  { value: 'category', label: '分区' },
  { value: 'permission', label: '权限点' },
  { value: 'role', label: '角色' },
  { value: 'admin', label: '账号' },
  { value: 'config', label: '配置' },
  { value: 'dict', label: '字典项' },
]

function buildParams(p: number, size: number) {
  return {
    admin_id: adminId.value.trim() || undefined,
    action: action.value || undefined,
    obj_type: objType.value || undefined,
    from: dateRange.value?.[0],
    to: dateRange.value?.[1],
    page: p,
    page_size: size,
  }
}

const {
  list: logs,
  total,
  loading,
  page,
  pageSize,
  search,
  onPageChange,
} = usePagedList<AuditLogItem>((p, size) => adminApi.admin.auditLogs(buildParams(p, size)))
const { run } = useApiAction()

// 导出 CSV（当前筛选，SYS-06）
async function exportCsv() {
  const p = buildParams(1, 20)
  await run(
    () =>
      adminApi.http
        .download('/api/v1/admin/audit-logs/export', {
          params: {
            admin_id: p.admin_id,
            action: p.action,
            obj_type: p.obj_type,
            from: p.from,
            to: p.to,
          },
          fallbackName: `audit-log-${Date.now()}.csv`,
        })
        .then(saveBlob),
    { success: '导出成功', fallback: '导出失败，请稍后再试' },
  )
}
</script>

<template>
  <div>
    <PageHead title="审计日志" :sub="`共 ${total} 条操作记录`" />

    <div class="page-card">
      <!-- 筛选栏 -->
      <div class="flex gap-2 mb-4 flex-wrap">
        <el-input
          v-model="adminId"
          class="w-32"
          placeholder="操作者ID"
          clearable
          @keyup.enter="search"
        />
        <el-select v-model="action" class="w-36" placeholder="动作" clearable>
          <el-option v-for="a in ACTIONS" :key="a.value" :value="a.value" :label="a.label" />
        </el-select>
        <el-select v-model="objType" class="w-32" placeholder="对象类型" clearable>
          <el-option v-for="o in OBJ_TYPES" :key="o.value" :value="o.value" :label="o.label" />
        </el-select>
        <el-date-picker
          v-model="dateRange"
          type="daterange"
          value-format="YYYY-MM-DD"
          range-separator="至"
          start-placeholder="开始日期"
          end-placeholder="结束日期"
          class="w-64"
        />
        <el-button type="primary" class="pink-btn" @click="search"> 查询 </el-button>
        <el-button v-perm="'audit:export'" class="ml-auto" @click="exportCsv"> 导出 CSV </el-button>
      </div>

      <el-table v-loading="loading" :data="logs" stripe>
        <el-table-column prop="id" label="ID" width="90" />
        <el-table-column label="操作者" width="120">
          <template #default="{ row }">
            <span :class="row.admin_name ? '' : 'text-text-3'">{{
              row.admin_name || row.admin_id
            }}</span>
          </template>
        </el-table-column>
        <el-table-column label="动作" width="120">
          <template #default="{ row }">
            <el-tag size="small" effect="plain">
              {{ row.action_name }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="对象" width="140">
          <template #default="{ row }"> {{ row.obj_name }} #{{ row.oid }} </template>
        </el-table-column>
        <el-table-column label="详情" min-width="220">
          <template #default="{ row }">
            <span class="truncate block" :title="row.detail">{{ row.detail || '—' }}</span>
          </template>
        </el-table-column>
        <el-table-column label="时间" width="170">
          <template #default="{ row }">
            {{ formatDateTime(row.created_at) }}
          </template>
        </el-table-column>
      </el-table>

      <el-pagination
        v-if="total > pageSize"
        class="mt-4 justify-end"
        layout="prev, pager, next, total"
        :total="total"
        :page-size="pageSize"
        :current-page="page"
        @current-change="onPageChange"
      />
    </div>
  </div>
</template>

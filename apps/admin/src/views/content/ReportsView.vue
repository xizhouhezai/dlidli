<script setup lang="ts">
import { ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { type ReportItem } from '@dlidli/api-client'
import { adminApi } from '@/api'
import { usePagedList } from '@/composables/usePagedList'
import { useApiAction } from '@/composables/useApiAction'
import PageHead from '@/components/PageHead.vue'
import { readAdminToken } from '@/utils/token'

const status = ref(0)

const STATUS_META: Record<number, { label: string, type: '' | 'success' | 'warning' | 'danger' | 'info' }> = {
  0: { label: '待处理', type: 'danger' },
  1: { label: '已处理', type: 'success' },
  2: { label: '已忽略', type: 'info' },
}

const TARGET_TYPE_LABEL: Record<number, string> = { 1: '视频', 2: '评论', 3: '弹幕', 4: '动态', 5: '用户' }

const { list: reports, total, loading, page, pageSize, load, search, onPageChange } = usePagedList<ReportItem>(
  (p, size) => adminApi.admin.reports({ status: status.value, page: p, page_size: size }),
)
const { run } = useApiAction()

function fmtDate(s: string): string {
  return new Date(s).toLocaleString()
}

function truncate(s: string, n = 40): string {
  return s.length > n ? `${s.slice(0, n)}…` : s
}

async function handleIgnore(r: ReportItem) {
  await ElMessageBox.confirm('确定忽略该举报吗？', '忽略举报', { type: 'warning' })
  const ok = await run(
    () => adminApi.admin.handleReport(r.id, { action: 'ignore', note: '无违规' }),
    { success: '已忽略', fallback: '操作失败' },
  )
  if (ok) load()
}

async function handleDelete(r: ReportItem) {
  await ElMessageBox.confirm(`确定删除该${TARGET_TYPE_LABEL[r.target_type]}内容吗？删除后作者与游客均不可见。`, '删除内容', { type: 'warning' })
  const ok = await run(
    () => adminApi.admin.handleReport(r.id, { action: 'delete', note: '违规内容' }),
    { success: '已删除', fallback: '操作失败' },
  )
  if (ok) load()
}

async function handlePunish(r: ReportItem) {
  const { value: punish } = await ElMessageBox.prompt('处罚类型：mute 禁言 / ban 封禁（0=永久）', '删除并处罚作者', {
    inputValue: 'mute',
    inputPattern: /^(mute|ban)$/,
    inputErrorMessage: '仅支持 mute 或 ban',
  })
  const { value: daysStr } = await ElMessageBox.prompt(punish === 'mute' ? '禁言天数（1/3/7/30）' : '封禁天数（0=永久，或 7/30/90）', '处罚时长', {
    inputValue: punish === 'mute' ? '3' : '0',
    inputPattern: /^\d+$/,
    inputErrorMessage: '请输入非负整数',
  })
  const ok = await run(
    () => adminApi.admin.handleReport(r.id, {
      action: 'punish',
      punish: punish as 'mute' | 'ban',
      days: Number(daysStr),
      note: '举报处理',
    }),
    { success: '已删除并处罚', fallback: '操作失败' },
  )
  if (ok) load()
}

// 导出 CSV（当前状态筛选，SYS-06）
async function exportCsv() {
  const qs = new URLSearchParams()
  qs.set('status', String(status.value))
  try {
    const res = await fetch(`/api/v1/admin/reports/export?${qs.toString()}`, {
      headers: { Authorization: `Bearer ${readAdminToken() ?? ''}` },
    })
    if (!res.ok) {
      ElMessage.error('导出失败')
      return
    }
    const blob = await res.blob()
    const match = (res.headers.get('Content-Disposition') ?? '').match(/filename="?([^";]+)"?/)
    const a = document.createElement('a')
    a.href = URL.createObjectURL(blob)
    a.download = match?.[1] ?? `reports-${Date.now()}.csv`
    a.click()
    URL.revokeObjectURL(a.href)
    ElMessage.success('导出成功')
  } catch {
    ElMessage.error('导出失败，请稍后再试')
  }
}
</script>

<template>
  <div>
    <PageHead
      title="举报处理"
      :sub="`共 ${total} 条举报`"
    />

    <div class="page-card">
      <!-- 状态筛选 -->
      <div class="flex gap-2 mb-4">
        <el-radio-group
          v-model="status"
          @change="search"
        >
          <el-radio-button :value="0">
            待处理
          </el-radio-button>
          <el-radio-button :value="1">
            已处理
          </el-radio-button>
          <el-radio-button :value="2">
            已忽略
          </el-radio-button>
          <el-radio-button :value="-1">
            全部
          </el-radio-button>
        </el-radio-group>
        <el-button
          v-perm="'report:view'"
          class="pink-btn ml-auto"
          @click="exportCsv"
        >
          导出 CSV
        </el-button>
      </div>

      <el-table
        v-loading="loading"
        :data="reports"
        stripe
      >
        <el-table-column
          label="对象"
          min-width="200"
        >
          <template #default="{ row }">
            <div class="flex items-center gap-2">
              <el-tag
                size="small"
                effect="plain"
              >
                {{ row.target_name }}
              </el-tag>
              <span
                class="truncate"
                :title="row.target_desc"
              >{{ truncate(row.target_desc) }}</span>
            </div>
          </template>
        </el-table-column>
        <el-table-column
          label="举报类型"
          width="110"
        >
          <template #default="{ row }">
            {{ row.reason_name }}
          </template>
        </el-table-column>
        <el-table-column
          prop="reporter_name"
          label="举报人"
          width="120"
        />
        <el-table-column
          label="补充说明"
          min-width="140"
        >
          <template #default="{ row }">
            <span
              v-if="row.reason"
              class="truncate block"
              :title="row.reason"
            >{{ truncate(row.reason, 30) }}</span>
            <span v-else>—</span>
          </template>
        </el-table-column>
        <el-table-column
          label="时间"
          width="150"
        >
          <template #default="{ row }">
            {{ fmtDate(row.created_at) }}
          </template>
        </el-table-column>
        <el-table-column
          label="状态"
          width="90"
        >
          <template #default="{ row }">
            <el-tag
              :type="STATUS_META[row.status]?.type"
              size="small"
            >
              {{ STATUS_META[row.status]?.label }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column
          label="操作"
          width="200"
          fixed="right"
        >
          <template #default="{ row }">
            <template v-if="row.status === 0">
              <el-button
                v-perm="'report:handle'"
                link
                type="primary"
                @click="handleDelete(row)"
              >
                删除
              </el-button>
              <el-button
                v-perm="'report:handle'"
                link
                type="danger"
                @click="handlePunish(row)"
              >
                删除并处罚
              </el-button>
              <el-button
                v-perm="'report:handle'"
                link
                @click="handleIgnore(row)"
              >
                忽略
              </el-button>
            </template>
            <span v-else>—</span>
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

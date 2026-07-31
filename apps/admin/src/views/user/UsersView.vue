<script setup lang="ts">
import { ref } from 'vue'
import { ElMessageBox } from 'element-plus'
import { type AdminUserItem, type PunishAction } from '@dlidli/api-client'
import { adminApi } from '@/api'
import { usePagedList } from '@/composables/usePagedList'
import { useApiAction } from '@/composables/useApiAction'
import PageHead from '@/components/PageHead.vue'

const keyword = ref('')
const status = ref(-1)

const STATUS_META: Record<number, { label: string, type: '' | 'success' | 'warning' | 'danger' | 'info' }> = {
  0: { label: '正常', type: 'success' },
  1: { label: '禁言', type: 'warning' },
  2: { label: '封禁', type: 'danger' },
  3: { label: '注销', type: 'info' },
}

const { list: users, total, loading, page, pageSize, load, search, onPageChange } = usePagedList<AdminUserItem>(
  (p, size) => adminApi.admin.users({ keyword: keyword.value.trim(), status: status.value, page: p, page_size: size }),
)
const { run } = useApiAction()

async function punish(user: AdminUserItem, action: PunishAction) {
  const ok = await run(async () => {
    let days = 0
    let reason = ''
    if (action === 'mute' || action === 'ban') {
      const dayOptions = action === 'mute'
        ? '禁言天数（1/3/7/30）'
        : '封禁天数（0=永久，或 7/30/90）'
      const { value } = await ElMessageBox.prompt(dayOptions, action === 'mute' ? '禁言用户' : '封禁用户', {
        inputValue: action === 'mute' ? '3' : '0',
        inputPattern: /^\d+$/,
        inputErrorMessage: '请输入非负整数',
      })
      days = Number(value)
      const { value: r } = await ElMessageBox.prompt('处罚原因（选填，审计留痕）', '原因', {
        inputValue: '',
        inputType: 'textarea',
      }).catch(() => ({ value: '' }))
      reason = r || ''
    } else {
      await ElMessageBox.confirm(
        `确定对「${user.nickname}」执行${action === 'unmute' ? '解除禁言' : '解除封禁'}吗？`,
        '确认',
        { type: 'warning' },
      )
    }
    await adminApi.admin.punishUser(user.id, action, days, reason)
  }, { success: '操作成功', fallback: '操作失败' })
  if (ok) load()
}
</script>

<template>
  <div>
    <PageHead
      title="用户管理"
      :sub="`共 ${total} 名用户`"
    />

    <div class="page-card">
      <!-- 查询栏 -->
      <div class="flex gap-2 mb-4">
        <el-input
          v-model="keyword"
          class="max-w-320px"
          placeholder="按 UID / 手机号 / 昵称查询"
          clearable
          @keyup.enter="search"
        />
        <el-select
          v-model="status"
          class="w-32"
          @change="search"
        >
          <el-option
            :value="-1"
            label="全部状态"
          />
          <el-option
            :value="0"
            label="正常"
          />
          <el-option
            :value="1"
            label="禁言"
          />
          <el-option
            :value="2"
            label="封禁"
          />
        </el-select>
        <el-button
          type="primary"
          class="pink-btn"
          @click="search"
        >
          查询
        </el-button>
      </div>

      <el-table
        v-loading="loading"
        :data="users"
        stripe
      >
        <el-table-column
          label="用户"
          min-width="200"
        >
          <template #default="{ row }">
            <div class="flex items-center gap-2">
              <el-avatar
                :size="32"
                :src="row.avatar"
              />
              <div>
                <div class="text-3.5 font-600">
                  {{ row.nickname }}
                </div>
                <div class="text-3 text-text-2">
                  UID {{ row.id }} · Lv{{ row.level }}
                </div>
              </div>
            </div>
          </template>
        </el-table-column>
        <el-table-column
          label="状态"
          width="100"
        >
          <template #default="{ row }">
            <el-tag :type="STATUS_META[row.status]?.type">
              {{ STATUS_META[row.status]?.label ?? '未知' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column
          label="处罚到期"
          min-width="180"
        >
          <template #default="{ row }">
            <span
              v-if="row.status === 1 && row.muted_until"
              class="text-3 text-text-2"
            >禁言至 {{ new Date(row.muted_until).toLocaleString() }}</span>
            <span
              v-else-if="row.status === 2"
              class="text-3 text-text-2"
            >{{ row.banned_until ? '封禁至 ' + new Date(row.banned_until).toLocaleString() : '永久封禁' }}</span>
            <span
              v-else
              class="text-3 text-text-3"
            >—</span>
          </template>
        </el-table-column>
        <el-table-column
          label="操作"
          width="260"
          fixed="right"
        >
          <template #default="{ row }">
            <template v-if="row.status === 0">
              <el-button
                v-perm="'user:punish'"
                size="small"
                type="warning"
                @click="punish(row, 'mute')"
              >
                禁言
              </el-button>
              <el-button
                v-perm="'user:punish'"
                size="small"
                type="danger"
                @click="punish(row, 'ban')"
              >
                封禁
              </el-button>
            </template>
            <el-button
              v-if="row.status === 1"
              v-perm="'user:punish'"
              size="small"
              @click="punish(row, 'unmute')"
            >
              解除禁言
            </el-button>
            <el-button
              v-if="row.status === 2"
              v-perm="'user:punish'"
              size="small"
              @click="punish(row, 'unban')"
            >
              解除封禁
            </el-button>
          </template>
        </el-table-column>
      </el-table>

      <div class="flex justify-end mt-4">
        <el-pagination
          layout="prev, pager, next"
          :total="total"
          :page-size="pageSize"
          :current-page="page"
          @current-change="onPageChange"
        />
      </div>
    </div>
  </div>
</template>

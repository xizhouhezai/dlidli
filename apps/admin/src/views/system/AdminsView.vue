<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { type AdminAccount, type AdminRole } from '@dlidli/api-client'
import { adminApi } from '@/api'
import { usePagedList } from '@/composables/usePagedList'
import { useApiAction } from '@/composables/useApiAction'
import PageHead from '@/components/PageHead.vue'

const roles = ref<AdminRole[]>([])
const {
  list: admins,
  total,
  loading,
  page,
  pageSize,
  load,
  onPageChange,
} = usePagedList<AdminAccount>((p, size) => adminApi.admin.admins(p, size))
onMounted(async () => {
  roles.value = (await adminApi.admin.roles()).list
})

const { loading: saving, run } = useApiAction()

// 对话框
const dialogVisible = ref(false)
const editing = ref<AdminAccount | null>(null)
const form = ref<{ username: string; nickname: string; password: string; roleIds: string[] }>({
  username: '',
  nickname: '',
  password: '',
  roleIds: [],
})

function openCreate() {
  editing.value = null
  form.value = { username: '', nickname: '', password: '', roleIds: [] }
  dialogVisible.value = true
}

function openEdit(a: AdminAccount) {
  editing.value = a
  form.value = {
    username: a.username,
    nickname: a.nickname,
    password: '',
    roleIds: [...a.role_ids],
  }
  dialogVisible.value = true
}

async function save() {
  if (!form.value.username.trim()) {
    ElMessage.warning('请填写用户名')
    return
  }
  if (!editing.value && !form.value.password) {
    ElMessage.warning('请填写初始密码')
    return
  }
  const ok = await run(
    async () => {
      if (editing.value) {
        await adminApi.admin.updateAdmin(editing.value.id, {
          username: form.value.username,
          nickname: form.value.nickname,
          role_ids: form.value.roleIds,
        })
      } else {
        await adminApi.admin.createAdmin({
          username: form.value.username,
          nickname: form.value.nickname,
          password: form.value.password,
          role_ids: form.value.roleIds,
        })
      }
    },
    { success: editing.value ? '已保存' : '已创建', fallback: '保存失败' },
  )
  if (ok) {
    dialogVisible.value = false
    load()
  }
}

async function toggle(a: AdminAccount) {
  const next = a.status === 0 ? 1 : 0
  const ok = await run(() => adminApi.admin.toggleAdmin(a.id, next), {
    success: next === 0 ? '已启用' : '已停用',
  })
  if (ok) load()
}

async function resetPwd(a: AdminAccount) {
  await run(
    async () => {
      const res = await ElMessageBox.prompt(`为「${a.username}」设置新密码（≥ 6 位）`, '重置密码', {
        inputValidator: (v) => (v && v.length >= 6 ? true : '密码至少 6 位'),
      })
      await adminApi.admin.resetAdminPassword(a.id, res.value)
    },
    { success: '密码已重置' },
  )
}

async function remove(a: AdminAccount) {
  const ok = await run(
    async () => {
      await ElMessageBox.confirm(`确定删除账号「${a.username}」吗？`, '删除', { type: 'warning' })
      await adminApi.admin.deleteAdmin(a.id)
    },
    { success: '已删除' },
  )
  if (ok) load()
}
</script>

<template>
  <div>
    <PageHead title="账号管理" :sub="`共 ${total} 个后台账号`">
      <template #actions>
        <el-button v-perm="'admin:edit'" type="primary" class="pink-btn" @click="openCreate">
          <span class="i-mingcute-add-line mr-1" />新建账号
        </el-button>
      </template>
    </PageHead>

    <div class="page-card">
      <el-table v-loading="loading" :data="admins" stripe>
        <el-table-column label="账号" min-width="160">
          <template #default="{ row }">
            <div class="font-600">
              {{ row.nickname || row.username }}
            </div>
            <div class="text-3 text-text-3">@{{ row.username }}</div>
          </template>
        </el-table-column>
        <el-table-column prop="role_names" label="角色" min-width="160">
          <template #default="{ row }">
            <span v-if="row.role_names">{{ row.role_names }}</span>
            <span v-else class="text-text-3">未分配</span>
          </template>
        </el-table-column>
        <el-table-column label="状态" width="90">
          <template #default="{ row }">
            <el-tag :type="row.status === 0 ? 'success' : 'info'">
              {{ row.status === 0 ? '启用' : '停用' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="last_login_at" label="最近登录" min-width="150">
          <template #default="{ row }">
            <span class="text-3 text-text-2">{{ row.last_login_at ?? '—' }}</span>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="280" fixed="right">
          <template #default="{ row }">
            <el-button v-perm="'admin:edit'" size="small" @click="openEdit(row)"> 编辑 </el-button>
            <el-button v-perm="'admin:edit'" size="small" @click="toggle(row)">
              {{ row.status === 0 ? '停用' : '启用' }}
            </el-button>
            <el-button v-perm="'admin:edit'" size="small" @click="resetPwd(row)">
              重置密码
            </el-button>
            <el-button v-perm="'admin:edit'" size="small" type="danger" @click="remove(row)">
              删除
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

    <!-- 新建/编辑对话框 -->
    <el-dialog v-model="dialogVisible" :title="editing ? '编辑账号' : '新建账号'" width="480px">
      <el-form label-width="72px">
        <el-form-item label="用户名">
          <el-input v-model="form.username" :disabled="!!editing" maxlength="32" />
        </el-form-item>
        <el-form-item label="昵称">
          <el-input v-model="form.nickname" maxlength="32" />
        </el-form-item>
        <el-form-item v-if="!editing" label="初始密码">
          <el-input v-model="form.password" type="password" show-password maxlength="64" />
        </el-form-item>
        <el-form-item label="角色">
          <el-select
            v-model="form.roleIds"
            multiple
            placeholder="选择角色（可多选）"
            class="w-full"
          >
            <el-option v-for="r in roles" :key="r.id" :value="r.id" :label="r.name" />
          </el-select>
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

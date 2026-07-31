<script setup lang="ts">
import { onMounted, ref, computed } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { type AdminRole, type AdminPermission } from '@dlidli/api-client'
import { adminApi } from '@/api'
import { useApiAction } from '@/composables/useApiAction'
import PageHead from '@/components/PageHead.vue'

const roles = ref<AdminRole[]>([])
const permissions = ref<AdminPermission[]>([])
const loading = ref(false)
const { loading: saving, run } = useApiAction()

// 权限树：menu 作为父节点，button 挂到 parent 下
interface TreeNode { code: string, name: string, children?: TreeNode[] }
const permTree = computed<TreeNode[]>(() => {
  const menus = permissions.value.filter(p => p.type === 'menu')
  return menus.map(m => ({
    code: m.code,
    name: m.name,
    children: permissions.value.filter(p => p.parent === m.code).map(b => ({ code: b.code, name: b.name })),
  }))
})

// 所有叶子权限码（button，以及无子节点的 menu）——用于 super 全选
const allLeafCodes = computed<string[]>(() => {
  const menuWithChildren = new Set(
    permissions.value.filter(p => p.type === 'menu' && permissions.value.some(x => x.parent === p.code)).map(p => p.code),
  )
  return permissions.value.filter(p => !menuWithChildren.has(p.code)).map(p => p.code)
})

async function load() {
  loading.value = true
  try {
    const [r, p] = await Promise.all([adminApi.admin.roles(), adminApi.admin.permissions()])
    roles.value = r.list
    permissions.value = p.list
  } finally {
    loading.value = false
  }
}
onMounted(load)

// 对话框：mode = create | edit | view
type DialogMode = 'create' | 'edit' | 'view'
const dialogVisible = ref(false)
const dialogMode = ref<DialogMode>('create')
const editing = ref<AdminRole | null>(null)
const form = ref({ name: '', code: '', remark: '' })
const treeRef = ref()
const readonly = computed(() => dialogMode.value === 'view')
// 待回填的勾选码：配合 :key 强制重建树 + default-checked-keys 回填
const pendingChecked = ref<string[]>([])
// 每次打开对话框递增，作为 el-tree 的 key 强制重建新实例（避免上一次勾选残留）
const treeSeq = ref(0)

// 计算某角色应勾选的叶子权限码（super 为全部叶子）
function checkedCodesOf(role: AdminRole | null): string[] {
  if (role?.code === 'super') return allLeafCodes.value
  const codes = role?.perms ?? []
  // 只勾选叶子（button）与无子的 menu，避免父级半选逻辑干扰
  return codes.filter(c => !permTree.value.some(m => m.code === c && m.children?.length))
}

// el-dialog 打开前先设好 pendingChecked，配合 destroy-on-close + default-checked-keys 回填勾选

function openCreate() {
  dialogMode.value = 'create'
  editing.value = null
  form.value = { name: '', code: '', remark: '' }
  pendingChecked.value = []
  treeSeq.value++
  dialogVisible.value = true
}

function openView(role: AdminRole) {
  dialogMode.value = 'view'
  editing.value = role
  form.value = { name: role.name, code: role.code, remark: role.remark }
  pendingChecked.value = checkedCodesOf(role)
  treeSeq.value++
  dialogVisible.value = true
}

function openEdit(role: AdminRole) {
  dialogMode.value = 'edit'
  editing.value = role
  form.value = { name: role.name, code: role.code, remark: role.remark }
  pendingChecked.value = checkedCodesOf(role)
  treeSeq.value++
  dialogVisible.value = true
}

async function save() {
  if (!form.value.name.trim()) {
    ElMessage.warning('请填写角色名')
    return
  }
  if (!editing.value && !form.value.code.trim()) {
    ElMessage.warning('请填写角色编码')
    return
  }
  // 收集勾选（含半选父节点）得到完整权限码
  const checked: string[] = treeRef.value?.getCheckedKeys() ?? []
  const halfChecked: string[] = treeRef.value?.getHalfCheckedKeys() ?? []
  const perms = [...checked, ...halfChecked]
  const ok = await run(async () => {
    if (editing.value) {
      await adminApi.admin.updateRole(editing.value.id, { name: form.value.name, remark: form.value.remark, perms })
    } else {
      await adminApi.admin.createRole({ name: form.value.name, code: form.value.code, remark: form.value.remark, perms })
    }
  }, { success: editing.value ? '已保存' : '已创建', fallback: '保存失败' })
  if (ok) {
    dialogVisible.value = false
    load()
  }
}

async function remove(role: AdminRole) {
  const ok = await run(async () => {
    await ElMessageBox.confirm(`确定删除角色「${role.name}」吗？`, '删除', { type: 'warning' })
    await adminApi.admin.deleteRole(role.id)
  }, { success: '已删除', fallback: '删除失败' })
  if (ok) load()
}
</script>

<template>
  <div>
    <PageHead
      title="角色管理"
      :sub="`共 ${roles.length} 个角色（内置角色权限固定，不可删除）`"
    >
      <template #actions>
        <el-button
          v-perm="'role:edit'"
          type="primary"
          class="pink-btn"
          @click="openCreate"
        >
          <span class="i-mingcute-add-line mr-1" />新建角色
        </el-button>
      </template>
    </PageHead>

    <div class="page-card">
      <el-table
        v-loading="loading"
        :data="roles"
        stripe
      >
        <el-table-column
          label="角色"
          min-width="180"
        >
          <template #default="{ row }">
            <div class="flex items-center gap-2">
              <span class="font-600">{{ row.name }}</span>
              <el-tag
                v-if="row.is_builtin"
                size="small"
                type="info"
              >
                内置
              </el-tag>
            </div>
            <div class="text-3 text-text-3">
              {{ row.code }}
            </div>
          </template>
        </el-table-column>
        <el-table-column
          prop="remark"
          label="说明"
          min-width="180"
        />
        <el-table-column
          label="成员"
          width="80"
        >
          <template #default="{ row }">
            {{ row.members }}
          </template>
        </el-table-column>
        <el-table-column
          label="权限数"
          width="90"
        >
          <template #default="{ row }">
            {{ row.code === 'super' ? '全部' : row.perms.length }}
          </template>
        </el-table-column>
        <el-table-column
          label="操作"
          width="180"
          fixed="right"
        >
          <template #default="{ row }">
            <el-button
              size="small"
              @click="openView(row)"
            >
              查看
            </el-button>
            <el-button
              v-if="row.code !== 'super'"
              v-perm="'role:edit'"
              size="small"
              type="primary"
              @click="openEdit(row)"
            >
              编辑
            </el-button>
            <el-button
              v-if="!row.is_builtin"
              v-perm="'role:edit'"
              size="small"
              type="danger"
              @click="remove(row)"
            >
              删除
            </el-button>
          </template>
        </el-table-column>
      </el-table>
    </div>

    <!-- 新建/编辑/查看对话框 -->
    <el-dialog
      v-model="dialogVisible"
      :title="dialogMode === 'view' ? '查看角色' : (dialogMode === 'edit' ? '编辑角色' : '新建角色')"
      width="560px"
      destroy-on-close
    >
      <el-form label-width="72px">
        <el-form-item label="角色名">
          <el-input
            v-model="form.name"
            :disabled="readonly || !!editing?.is_builtin"
            maxlength="32"
          />
        </el-form-item>
        <el-form-item label="角色编码">
          <el-input
            v-model="form.code"
            :disabled="!!editing"
            placeholder="如 content_editor"
            maxlength="32"
          />
        </el-form-item>
        <el-form-item label="说明">
          <el-input
            v-model="form.remark"
            :disabled="readonly || !!editing?.is_builtin"
            maxlength="128"
          />
        </el-form-item>
        <el-form-item label="权限">
          <el-tree
            ref="treeRef"
            :key="treeSeq"
            :data="permTree"
            :props="{ label: 'name', children: 'children' }"
            :default-checked-keys="pendingChecked"
            node-key="code"
            show-checkbox
            default-expand-all
            class="w-full"
            :class="{ 'tree-readonly': readonly }"
          />
          <div
            v-if="editing?.code === 'super'"
            class="text-3 text-text-3 mt-1"
          >
            超级管理员拥有全部权限，不可编辑
          </div>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">
          {{ readonly ? '关闭' : '取消' }}
        </el-button>
        <el-button
          v-if="!readonly"
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

<style scoped>
/* 只读权限树：禁用交互但保留勾选展示（避免用 node.disabled——会使 setCheckedKeys 失效） */
.tree-readonly {
  pointer-events: none;
  opacity: 0.85;
}
</style>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import type { AdminPermission } from '@dlidli/api-client'
import { adminApi } from '@/api'
import { useApiAction } from '@/composables/useApiAction'
import PageHead from '@/components/PageHead.vue'

const list = ref<AdminPermission[]>([])
const loading = ref(false)
const { loading: saving, run } = useApiAction()

// 页面权限（menu，含其下按钮权限），按 sort 排
const menus = computed(() =>
  list.value.filter((p) => p.type === 'menu').sort((a, b) => a.sort - b.sort),
)
function buttonsOf(menuCode: string) {
  return list.value
    .filter((p) => p.type === 'button' && p.parent === menuCode)
    .sort((a, b) => a.sort - b.sort)
}
// 无归属的按钮权限（parent 指向不存在的 menu，容错展示）
const orphanButtons = computed(() => {
  const menuCodes = new Set(menus.value.map((m) => m.code))
  return list.value.filter((p) => p.type === 'button' && !menuCodes.has(p.parent))
})

async function load() {
  loading.value = true
  try {
    list.value = (await adminApi.admin.permissions()).list
  } finally {
    loading.value = false
  }
}
onMounted(load)

// 对话框
const dialogVisible = ref(false)
const editing = ref<AdminPermission | null>(null)
const form = ref<{
  code: string
  name: string
  type: 'menu' | 'button'
  parent: string
  path: string
  icon: string
  sort: number
}>({
  code: '',
  name: '',
  type: 'menu',
  parent: '',
  path: '',
  icon: '',
  sort: 0,
})

function openCreateMenu() {
  editing.value = null
  form.value = { code: '', name: '', type: 'menu', parent: '', path: '', icon: '', sort: 0 }
  dialogVisible.value = true
}
function openCreateButton(menuCode: string) {
  editing.value = null
  form.value = { code: '', name: '', type: 'button', parent: menuCode, path: '', icon: '', sort: 0 }
  dialogVisible.value = true
}
function openEdit(p: AdminPermission) {
  editing.value = p
  form.value = {
    code: p.code,
    name: p.name,
    type: p.type as 'menu' | 'button',
    parent: p.parent,
    path: p.path,
    icon: p.icon,
    sort: p.sort,
  }
  dialogVisible.value = true
}

async function save() {
  if (!form.value.name.trim()) {
    ElMessage.warning('请填写名称')
    return
  }
  if (!editing.value && !form.value.code.trim()) {
    ElMessage.warning('请填写权限码')
    return
  }
  const payload = {
    code: form.value.code,
    name: form.value.name,
    type: form.value.type,
    parent: form.value.type === 'button' ? form.value.parent : '',
    path: form.value.path,
    icon: form.value.icon,
    sort: form.value.sort,
  }
  const ok = await run(
    async () => {
      if (editing.value) {
        await adminApi.admin.updatePermission(editing.value.id, payload)
      } else {
        await adminApi.admin.createPermission(payload)
      }
    },
    { success: editing.value ? '已保存' : '已创建', fallback: '保存失败' },
  )
  if (ok) {
    dialogVisible.value = false
    load()
  }
}

async function remove(p: AdminPermission) {
  const ok = await run(
    async () => {
      await ElMessageBox.confirm(`确定删除权限点「${p.name}」（${p.code}）吗？`, '删除', {
        type: 'warning',
      })
      await adminApi.admin.deletePermission(p.id)
    },
    { success: '已删除', fallback: '删除失败' },
  )
  if (ok) load()
}
</script>

<template>
  <div>
    <PageHead
      title="权限管理"
      :sub="`页面权限（menu）${menus.length} 个 · 权限点统一 模块:操作 命名`"
    >
      <template #actions>
        <el-button
          v-perm="'permission:edit'"
          type="primary"
          class="pink-btn"
          @click="openCreateMenu"
        >
          <span class="i-mingcute-add-line mr-1" />新建页面权限
        </el-button>
      </template>
    </PageHead>

    <div v-loading="loading" class="perm-list">
      <div v-for="m in menus" :key="m.id" class="page-card mb-3">
        <!-- 页面权限（menu）行 -->
        <div class="flex items-center gap-3">
          <span :class="m.icon || 'i-mingcute-menu-line'" class="text-5 text-primary" />
          <span class="text-4 font-600">{{ m.name }}</span>
          <el-tag size="small" type="success"> 页面 </el-tag>
          <code class="perm-code">{{ m.code }}</code>
          <span v-if="m.path" class="text-3 text-text-3">{{ m.path }}</span>
          <div class="flex-1" />
          <el-button v-perm="'permission:edit'" size="small" @click="openCreateButton(m.code)">
            <span class="i-mingcute-add-line mr-1" />按钮权限
          </el-button>
          <el-button v-perm="'permission:edit'" size="small" @click="openEdit(m)"> 编辑 </el-button>
          <el-button v-perm="'permission:edit'" size="small" type="danger" @click="remove(m)">
            删除
          </el-button>
        </div>

        <!-- 按钮权限 -->
        <div v-if="buttonsOf(m.code).length" class="mt-3 pl-8 flex flex-wrap gap-2">
          <div v-for="b in buttonsOf(m.code)" :key="b.id" class="perm-btn">
            <span>{{ b.name }}</span>
            <code class="perm-code perm-code--sm">{{ b.code }}</code>
            <span
              v-perm="'permission:edit'"
              class="i-mingcute-edit-line perm-btn__act"
              @click="openEdit(b)"
            />
            <span
              v-perm="'permission:edit'"
              class="i-mingcute-delete-2-line perm-btn__act"
              @click="remove(b)"
            />
          </div>
        </div>
      </div>

      <!-- 无归属按钮权限（容错） -->
      <div v-if="orphanButtons.length" class="page-card mb-3">
        <div class="text-3 text-text-3 mb-2">未归属页面的按钮权限（parent 无效）</div>
        <div class="flex flex-wrap gap-2">
          <div v-for="b in orphanButtons" :key="b.id" class="perm-btn">
            <span>{{ b.name }}</span>
            <code class="perm-code perm-code--sm">{{ b.code }}</code>
            <span
              v-perm="'permission:edit'"
              class="i-mingcute-edit-line perm-btn__act"
              @click="openEdit(b)"
            />
            <span
              v-perm="'permission:edit'"
              class="i-mingcute-delete-2-line perm-btn__act"
              @click="remove(b)"
            />
          </div>
        </div>
      </div>
    </div>

    <!-- 新建/编辑对话框 -->
    <el-dialog
      v-model="dialogVisible"
      :title="editing ? '编辑权限点' : form.type === 'button' ? '新建按钮权限' : '新建页面权限'"
      width="480px"
    >
      <el-form label-width="80px">
        <el-form-item label="类型">
          <el-radio-group v-model="form.type" :disabled="!!editing || form.type === 'button'">
            <el-radio value="menu"> 页面权限 </el-radio>
            <el-radio value="button"> 按钮权限 </el-radio>
          </el-radio-group>
        </el-form-item>
        <el-form-item v-if="form.type === 'button'" label="所属页面">
          <el-select v-model="form.parent" placeholder="选择页面权限" class="w-full">
            <el-option
              v-for="m in menus"
              :key="m.code"
              :label="`${m.name}（${m.code}）`"
              :value="m.code"
            />
          </el-select>
        </el-form-item>
        <el-form-item label="权限码">
          <el-input
            v-model="form.code"
            :disabled="!!editing"
            placeholder="模块:操作，如 banner:view"
            maxlength="64"
          />
        </el-form-item>
        <el-form-item label="名称">
          <el-input v-model="form.name" maxlength="64" />
        </el-form-item>
        <template v-if="form.type === 'menu'">
          <el-form-item label="路由路径">
            <el-input v-model="form.path" placeholder="如 /banners" maxlength="128" />
          </el-form-item>
          <el-form-item label="菜单图标">
            <el-input v-model="form.icon" placeholder="如 i-mingcute-pic-line" maxlength="64">
              <template #prepend>
                <span :class="form.icon || 'i-mingcute-menu-line'" />
              </template>
            </el-input>
            <div class="text-3 text-text-3 mt-1">
              新图标需在 uno.config.ts 的 safelist 登记后才会渲染
            </div>
          </el-form-item>
        </template>
        <el-form-item label="排序">
          <el-input-number v-model="form.sort" :min="0" :max="9999" />
          <span class="ml-2 text-3 text-text-3">数字越小越靠前</span>
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

.perm-code {
  padding: 1px 6px;
  border-radius: 4px;
  background: v.$bg;
  font-size: 12px;
  color: v.$text-2;

  &--sm {
    font-size: 11px;
  }
}

.perm-btn {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 6px 12px;
  border-radius: 8px;
  background: v.$bg;
  font-size: 13px;
}

.perm-btn__act {
  cursor: pointer;
  color: v.$text-3;
  transition: color 0.15s;

  &:hover {
    color: v.$primary;
  }
}
</style>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRoute, useRouter, RouterLink, RouterView } from 'vue-router'
import { ElMessage } from 'element-plus'
import { clearAdminToken, readAdminInfo } from '@/utils/token'
import { permissionStore } from '@/stores/permission'

const route = useRoute()
const router = useRouter()

const adminInfo = readAdminInfo()
const username = computed(() => adminInfo?.username ?? '管理员')
const roleLabel = computed(() => {
  const map: Record<string, string> = {
    super: '超级管理员',
    review_lead: '审核主管',
    reviewer: '审核员',
    operator: '运营',
    moderator: '用户治理',
    analyst: '数据分析',
  }
  return map[adminInfo?.role ?? ''] ?? adminInfo?.role ?? ''
})

// 菜单分组：按权限码前缀归组，菜单项来自后端下发（已按权限过滤）
interface MenuItem { name: string, path: string, title: string, icon: string }
interface MenuGroup { title: string, items: MenuItem[] }

// 权限码 → 分组标题 映射
function groupOf(code: string): string {
  if (code.startsWith('dashboard')) return '概览'
  if (code.startsWith('review') || code.startsWith('sensitive') || code.startsWith('video')) return '内容审核'
  if (code.startsWith('user')) return '用户治理'
  if (code.startsWith('category') || code.startsWith('banner')) return '运营管理'
  if (code.startsWith('admin') || code.startsWith('role')) return '系统管理'
  if (code.startsWith('permission')) return '系统管理'
  return '其他'
}
const groupOrder = ['概览', '内容审核', '用户治理', '运营管理', '系统管理', '其他']

// path → 路由 name（与 router 一致）
const pathToName: Record<string, string> = {
  '/dashboard': 'dashboard',
  '/review': 'review',
  '/videos': 'videos',
  '/sensitive-words': 'sensitive-words',
  '/users': 'users',
  '/categories': 'categories',
  '/banners': 'banners',
  '/admins': 'admins',
  '/roles': 'roles',
  '/permissions': 'permissions',
  '/audit-logs': 'audit-logs',
  '/configs': 'configs',
  '/data-dicts': 'data-dicts',
  '/reports': 'reports',
}

const menuGroups = computed<MenuGroup[]>(() => {
  const map = new Map<string, MenuItem[]>()
  for (const m of permissionStore.state.menus) {
    const g = groupOf(m.code)
    if (!map.has(g)) map.set(g, [])
    map.get(g)!.push({ name: pathToName[m.path] ?? m.path, path: m.path, title: m.name, icon: m.icon })
  }
  return groupOrder
    .filter(g => map.has(g))
    .map(g => ({ title: g, items: map.get(g)! }))
})

const activeName = computed(() => route.name as string)

const breadcrumb = computed(() => {
  for (const g of menuGroups.value) {
    const hit = g.items.find(i => i.name === activeName.value)
    if (hit) return [g.title, hit.title]
  }
  return [route.meta.title as string ?? '']
})

const ready = ref(false)
onMounted(async () => {
  if (!permissionStore.state.loaded) {
    try {
      await permissionStore.load()
    } catch {
      // 加载失败不阻塞（菜单为空）
    }
  }
  ready.value = true
})

function logout() {
  clearAdminToken()
  permissionStore.reset()
  ElMessage.success('已退出登录')
  router.push({ name: 'login' })
}
</script>

<template>
  <div class="admin-layout">
    <!-- 侧边栏 -->
    <aside class="admin-sider">
      <div class="admin-sider__logo">
        <span class="i-mingcute-tv-2-line text-5 text-primary" />
        <span class="admin-sider__logo-text">DliDli 管理后台</span>
      </div>
      <nav class="admin-sider__nav">
        <div
          v-for="group in menuGroups"
          :key="group.title"
          class="admin-menu-group"
        >
          <div class="admin-menu-group__title">
            {{ group.title }}
          </div>
          <RouterLink
            v-for="item in group.items"
            :key="item.name"
            :to="item.path"
            class="admin-menu-item"
            :class="{ 'is-active': activeName === item.name }"
          >
            <span
              class="admin-menu-item__icon"
              :class="item.icon"
            />
            <span>{{ item.title }}</span>
          </RouterLink>
        </div>
      </nav>
    </aside>

    <!-- 右侧主体 -->
    <div class="admin-main">
      <header class="admin-header">
        <div class="admin-breadcrumb">
          <span
            v-for="(crumb, i) in breadcrumb"
            :key="i"
            class="admin-breadcrumb__item"
            :class="{ 'is-last': i === breadcrumb.length - 1 }"
          >
            <span
              v-if="i > 0"
              class="admin-breadcrumb__sep"
            >/</span>
            {{ crumb }}
          </span>
        </div>
        <el-dropdown @command="(c: string) => c === 'logout' && logout()">
          <span class="admin-user">
            <el-avatar
              :size="30"
              class="admin-user__avatar"
            >
              {{ username.charAt(0).toUpperCase() }}
            </el-avatar>
            <span class="admin-user__name">{{ username }}</span>
            <span
              v-if="roleLabel"
              class="admin-user__role"
            >{{ roleLabel }}</span>
            <span class="i-mingcute-down-line text-3.5" />
          </span>
          <template #dropdown>
            <el-dropdown-menu>
              <el-dropdown-item command="logout">
                <span class="i-mingcute-exit-line mr-1" />退出登录
              </el-dropdown-item>
            </el-dropdown-menu>
          </template>
        </el-dropdown>
      </header>

      <main class="admin-content">
        <RouterView />
      </main>
    </div>
  </div>
</template>

<style scoped lang="scss">
@use '@/styles/variables' as v;

.admin-layout {
  display: flex;
  min-height: 100vh;
  background: v.$bg;
}

/* ---- 侧边栏 ---- */
.admin-sider {
  width: 220px;
  flex-shrink: 0;
  background: #1e2330;
  display: flex;
  flex-direction: column;
  position: sticky;
  top: 0;
  height: 100vh;
  overflow-y: auto;
}

.admin-sider__logo {
  display: flex;
  align-items: center;
  gap: 8px;
  height: 56px;
  padding: 0 18px;
  color: #fff;
  font-weight: 700;
  font-size: 15px;
  border-bottom: 1px solid rgba(255, 255, 255, 0.08);
}

.admin-sider__nav {
  flex: 1;
  padding: 12px 10px;
}

.admin-menu-group {
  margin-bottom: 8px;
}

.admin-menu-group__title {
  padding: 10px 12px 6px;
  font-size: 11px;
  color: #6b7280;
  letter-spacing: 0.05em;
}

.admin-menu-item {
  display: flex;
  align-items: center;
  gap: 10px;
  height: 42px;
  padding: 0 12px;
  border-radius: 8px;
  color: #c9ccd0;
  font-size: 14px;
  text-decoration: none;
  transition: all 0.15s;
  cursor: pointer;

  &:hover {
    background: rgba(255, 255, 255, 0.06);
    color: #fff;
  }

  &.is-active {
    background: v.$primary;
    color: #fff;
    box-shadow: 0 4px 12px rgba(251, 114, 153, 0.35);
  }
}

.admin-menu-item__icon {
  font-size: 18px;
}

/* ---- 主体 ---- */
.admin-main {
  flex: 1;
  display: flex;
  flex-direction: column;
  min-width: 0;
}

.admin-header {
  height: 56px;
  background: #fff;
  border-bottom: 1px solid v.$border;
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0 24px;
  position: sticky;
  top: 0;
  z-index: 10;
}

.admin-breadcrumb {
  display: flex;
  align-items: center;
  font-size: 14px;
  color: v.$text-2;
}

.admin-breadcrumb__item.is-last {
  color: v.$text-1;
  font-weight: 600;
}

.admin-breadcrumb__sep {
  margin: 0 8px;
  color: v.$text-3;
}

.admin-user {
  display: flex;
  align-items: center;
  gap: 8px;
  cursor: pointer;
  outline: none;
  color: v.$text-1;
}

.admin-user__avatar {
  background: v.$primary;
  color: #fff;
  font-size: 13px;
}

.admin-user__name {
  font-size: 14px;
  font-weight: 600;
}

.admin-user__role {
  font-size: 12px;
  color: v.$text-2;
  background: v.$bg;
  padding: 2px 8px;
  border-radius: 4px;
}

.admin-content {
  flex: 1;
  padding: 20px 24px;
  overflow-x: hidden;
}
</style>

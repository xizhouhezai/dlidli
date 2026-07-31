import { createRouter, createWebHistory } from 'vue-router'
import { readAdminToken } from '@/utils/token'
import { permissionStore } from '@/stores/permission'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/', redirect: '/dashboard' },
    {
      path: '/login',
      name: 'login',
      component: () => import('@/views/auth/LoginView.vue'),
      meta: { title: '后台登录' },
    },
    {
      path: '/',
      component: () => import('@/layouts/AdminLayout.vue'),
      meta: { requiresAuth: true },
      children: [
        {
          path: 'dashboard',
          name: 'dashboard',
          component: () => import('@/views/dashboard/DashboardView.vue'),
          meta: { title: '工作台', requiresAuth: true },
        },
        {
          path: 'review',
          name: 'review',
          component: () => import('@/views/content/ReviewView.vue'),
          meta: { title: '审核工作台', requiresAuth: true },
        },
        {
          path: 'sensitive-words',
          name: 'sensitive-words',
          component: () => import('@/views/content/SensitiveWordsView.vue'),
          meta: { title: '敏感词库', requiresAuth: true },
        },
        {
          path: 'users',
          name: 'users',
          component: () => import('@/views/user/UsersView.vue'),
          meta: { title: '用户管理', requiresAuth: true },
        },
        {
          path: 'admins',
          name: 'admins',
          component: () => import('@/views/system/AdminsView.vue'),
          meta: { title: '账号管理', requiresAuth: true },
        },
        {
          path: 'roles',
          name: 'roles',
          component: () => import('@/views/system/RolesView.vue'),
          meta: { title: '角色管理', requiresAuth: true },
        },
        {
          path: 'categories',
          name: 'categories',
          component: () => import('@/views/operation/CategoriesView.vue'),
          meta: { title: '分区管理', requiresAuth: true },
        },
      ],
    },
    // 后续：审计日志（M2-SYS-01）
  ],
})

router.beforeEach(async (to) => {
  if (to.meta.requiresAuth && !readAdminToken()) {
    return { name: 'login' }
  }
  // 进入受保护页面前先加载权限，确保 v-perm 指令首次挂载即有正确权限（修复刷新后按钮消失）
  if (to.meta.requiresAuth && readAdminToken() && !permissionStore.state.loaded) {
    try {
      await permissionStore.load()
    } catch {
      // 令牌失效等：清理并回登录
      return { name: 'login' }
    }
  }
})

router.afterEach((to) => {
  document.title = to.meta.title ? `${to.meta.title as string} - DliDli 管理后台` : 'DliDli 管理后台'
})

export default router

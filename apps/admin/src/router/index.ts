import { createRouter, createWebHistory } from 'vue-router'
import { readAdminToken } from '@/utils/token'

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
      ],
    },
    // 后续：分区管理（M1-ADM-06）、审计日志（M2-SYS-01）
  ],
})

router.beforeEach((to) => {
  if (to.meta.requiresAuth && !readAdminToken()) {
    return { name: 'login' }
  }
})

router.afterEach((to) => {
  document.title = to.meta.title ? `${to.meta.title as string} - DliDli 管理后台` : 'DliDli 管理后台'
})

export default router

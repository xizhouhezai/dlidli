import { createRouter, createWebHistory } from 'vue-router'
import { readToken } from '@/utils/token'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    // ---- 全屏独立页（无全局头部）----
    {
      path: '/login',
      name: 'login',
      component: () => import('@/views/auth/LoginView.vue'),
      meta: { title: '登录' },
    },
    {
      path: '/reset-password',
      name: 'reset-password',
      component: () => import('@/views/auth/ResetPasswordView.vue'),
      meta: { title: '找回密码' },
    },

    // ---- 主布局（全局头部 + 内容区），业务页面均为子路由 ----
    {
      path: '/',
      component: () => import('@/layouts/MainLayout.vue'),
      children: [
        {
          path: '',
          name: 'home',
          component: () => import('@/views/home/HomeView.vue'),
          meta: { title: '首页' },
        },
        {
          path: 'video/:bvid',
          name: 'video',
          component: () => import('@/views/video/VideoView.vue'),
          meta: { title: '播放' },
        },
        {
          path: 'search',
          name: 'search',
          component: () => import('@/views/search/SearchView.vue'),
          meta: { title: '搜索' },
        },
        {
          path: 'collections/:id',
          name: 'collection-detail',
          component: () => import('@/views/collection/CollectionDetailView.vue'),
          meta: { title: '合集' },
        },
        {
          path: 'space/:uid',
          name: 'space',
          component: () => import('@/views/space/SpaceView.vue'),
          meta: { title: '个人空间' },
        },
        {
          path: 'feed',
          name: 'feed',
          component: () => import('@/views/feed/FeedView.vue'),
          meta: { title: '动态', requiresAuth: true },
        },
        {
          path: 'notifications',
          name: 'notifications',
          component: () => import('@/views/notify/NotifyView.vue'),
          meta: { title: '消息通知', requiresAuth: true },
        },
        {
          path: 'upload',
          name: 'upload',
          component: () => import('@/views/studio/UploadView.vue'),
          meta: { title: '投稿', requiresAuth: true },
        },
        {
          path: 'mine/videos',
          name: 'mine-videos',
          component: () => import('@/views/studio/MineVideosView.vue'),
          meta: { title: '稿件管理', requiresAuth: true },
        },
        {
          path: 'creator',
          name: 'creator',
          component: () => import('@/views/studio/CreatorView.vue'),
          meta: { title: '创作者中心', requiresAuth: true },
        },
        {
          path: 'settings',
          name: 'settings',
          component: () => import('@/views/account/SettingsView.vue'),
          meta: { title: '账号设置', requiresAuth: true },
        },
        {
          path: 'growth',
          name: 'growth',
          component: () => import('@/views/account/GrowthView.vue'),
          meta: { title: '成长中心', requiresAuth: true },
        },
      ],
    },
    // 管理后台已拆为独立应用 apps/admin（dev :5175）
  ],
})

// 登录守卫：未登录访问受保护页 → 跳登录并记录回跳地址
router.beforeEach((to) => {
  if (to.meta.requiresAuth && !readToken()) {
    return { name: 'login', query: { redirect: to.fullPath } }
  }
})

router.afterEach((to) => {
  const title = to.meta.title as string | undefined
  document.title = title ? `${title} - DliDli` : 'DliDli - 视频社区'
})

export default router

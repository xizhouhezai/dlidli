import { createRouter, createWebHistory } from 'vue-router'
import { ElMessage } from 'element-plus'
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
          path: 'videos',
          name: 'videos',
          component: () => import('@/views/content/VideosManageView.vue'),
          meta: { title: '稿件管理', requiresAuth: true },
        },
        {
          path: 'sensitive-words',
          name: 'sensitive-words',
          component: () => import('@/views/content/SensitiveWordsView.vue'),
          meta: { title: '敏感词库', requiresAuth: true },
        },
        {
          path: 'reports',
          name: 'reports',
          component: () => import('@/views/content/ReportsView.vue'),
          meta: { title: '举报处理', requiresAuth: true },
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
          path: 'permissions',
          name: 'permissions',
          component: () => import('@/views/system/PermissionsView.vue'),
          meta: { title: '权限管理', requiresAuth: true },
        },
        {
          path: 'audit-logs',
          name: 'audit-logs',
          component: () => import('@/views/system/AuditLogsView.vue'),
          meta: { title: '审计日志', requiresAuth: true },
        },
        {
          path: 'configs',
          name: 'configs',
          component: () => import('@/views/system/ConfigsView.vue'),
          meta: { title: '系统配置', requiresAuth: true },
        },
        {
          path: 'data-dicts',
          name: 'data-dicts',
          component: () => import('@/views/system/DataDictsView.vue'),
          meta: { title: '数据字典', requiresAuth: true },
        },
        {
          path: 'categories',
          name: 'categories',
          component: () => import('@/views/operation/CategoriesView.vue'),
          meta: { title: '分区管理', requiresAuth: true },
        },
        {
          path: 'banners',
          name: 'banners',
          component: () => import('@/views/operation/BannersView.vue'),
          meta: { title: 'Banner 配置', requiresAuth: true },
        },
        {
          path: 'experiments',
          name: 'experiments',
          component: () => import('@/views/operation/ExperimentsView.vue'),
          meta: { title: 'A/B 实验', requiresAuth: true },
        },
        {
          path: 'live-patrol',
          name: 'live-patrol',
          component: () => import('@/views/live/LivePatrolView.vue'),
          meta: { title: '直播巡查', requiresAuth: true },
        },
        {
          path: 'pay-orders',
          name: 'pay-orders',
          component: () => import('@/views/pay/PayOrdersView.vue'),
          meta: { title: '支付与对账', requiresAuth: true },
        },
        {
          path: 'vip-members',
          name: 'vip-members',
          component: () => import('@/views/vip/VipMembersView.vue'),
          meta: { title: '会员与套餐', requiresAuth: true },
        },
        {
          path: 'charge-settlements',
          name: 'charge-settlements',
          component: () => import('@/views/charge/ChargeSettlementsView.vue'),
          meta: { title: '充电与结算', requiresAuth: true },
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
  // 页面级权限：直连 URL 也必须被拦。
  //
  // 判据用「该路径是否出现在后端下发的菜单里」而不是在路由上另抄一份权限码：
  // menus 是后端按当前账号权限**过滤后**的结果，是唯一权威；把权限码再写进
  // meta 就有了两份清单，新增页面时漏改其中之一会表现为
  // 「菜单不显示、但直连 URL 能进」—— 正是本卡要防的漏洞。
  //
  // 前提：受保护子路由与 builtinPermissions 的 menu path 一一对应
  // （实测 19 个菜单 ↔ 19 个路由，仅差前导斜杠）。login 与根重定向不带
  // requiresAuth，故不会被误拦。
  //
  // 两条边界都在这里处理掉：
  //   · super 短路放行（其 menus 本就是全集，这里只是省一次集合查找）；
  //   · **menus 为空**（账号未绑任何角色）时不得放行 —— 若用「menus 非空才校验」
  //     的写法，这种账号反而能进任意页面，是比漏拦更隐蔽的漏洞。
  //     `/dashboard` 恒放行，避免「无任何菜单 → 重定向到 dashboard → 又被拦」的死循环。
  if (to.meta.requiresAuth && permissionStore.state.loaded && !permissionStore.state.isSuper) {
    const menuPaths = new Set(permissionStore.state.menus.map((m) => m.path))
    if (!menuPaths.has(to.path) && to.path !== '/dashboard') {
      ElMessage.error('无权限访问该页面')
      return { name: 'dashboard' }
    }
  }
})

router.afterEach((to) => {
  document.title = to.meta.title
    ? `${to.meta.title as string} - DliDli 管理后台`
    : 'DliDli 管理后台'
})

export default router

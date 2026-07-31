// 管理后台权限状态：登录后加载当前管理员的权限码与可见菜单，供 v-perm 指令与动态菜单使用。
import { reactive } from 'vue'
import type { AdminMenuItem } from '@dlidli/api-client'
import { adminApi } from '@/api'

interface PermState {
  loaded: boolean
  isSuper: boolean
  perms: string[]
  menus: AdminMenuItem[]
}

const state = reactive<PermState>({
  loaded: false,
  isSuper: false,
  perms: [],
  menus: [],
})

export const permissionStore = {
  state,

  /** 拉取当前登录者权限（登录后 / 布局挂载时调用）。 */
  async load() {
    const res = await adminApi.admin.myPermissions()
    state.isSuper = res.is_super
    state.perms = res.perms
    state.menus = res.menus
    state.loaded = true
  },

  /** 是否拥有某权限码（super 恒 true）。 */
  has(code: string): boolean {
    return state.isSuper || state.perms.includes(code)
  },

  /** 清空（退出登录时）。 */
  reset() {
    state.loaded = false
    state.isSuper = false
    state.perms = []
    state.menus = []
  },
}

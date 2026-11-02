// v-perm 指令：无权限时隐藏元素。用法 v-perm="'user:punish'"
//
// **为什么是「隐藏」而不是「移除」**：
// 早先实现在 mounted 里 `parentNode.removeChild(el)`，把 Vue 管理的真实 DOM 摘掉了，而 vnode 树不变 ——
// 之后任何重渲染（列表刷新、Tab 切换、固定列重组）都可能因「vnode 说有、DOM 里没有」而在 patch 阶段出错；
// 且该指令没有 updated 钩子，权限若晚于首屏到达，被摘掉的元素**永远回不来**（结构上无法自愈）。
// 隐藏不影响安全边界：前端隐藏只是交互收敛，真正的权限校验在后端 `RequirePerm`
//（越权直调接口由后端 403 拦，v0.72.0 已实测 14/14）。
//
// `updated` 里每次重新判定，使「权限数据晚于首屏加载完成」时能自愈回可见状态。
//
// 注：2026-11-02 浏览器走查中「直播巡查」页切 Tab 崩溃曾被怀疑是本指令所致，实测**已排除**
//（真因是 el-table 上挂了 v-loading），见 .dev-logs/debug-tabs/bisect.md；本改动按上述自身理由保留。
import type { App, Directive } from 'vue'
import { permissionStore } from '@/stores/permission'

const HIDDEN_ATTR = 'data-perm-hidden'

function apply(el: HTMLElement, code: unknown) {
  if (!code) return
  if (permissionStore.has(String(code))) {
    // 有权限：若此前被隐藏过则恢复
    if (el.getAttribute(HIDDEN_ATTR) === '1') {
      el.style.removeProperty('display')
      el.removeAttribute(HIDDEN_ATTR)
    }
    return
  }
  if (el.getAttribute(HIDDEN_ATTR) === '1') return
  el.style.setProperty('display', 'none', 'important')
  el.setAttribute(HIDDEN_ATTR, '1')
}

const permDirective: Directive<HTMLElement, string> = {
  mounted(el, binding) {
    apply(el, binding.value)
  },
  updated(el, binding) {
    apply(el, binding.value)
  },
}

export function setupPermDirective(app: App) {
  app.directive('perm', permDirective)
}

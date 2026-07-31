// v-perm 指令：无权限时移除元素。用法 v-perm="'user:punish'"
import type { App, Directive } from 'vue'
import { permissionStore } from '@/stores/permission'

const permDirective: Directive<HTMLElement, string> = {
  mounted(el, binding) {
    if (binding.value && !permissionStore.has(binding.value)) {
      el.parentNode?.removeChild(el)
    }
  },
}

export function setupPermDirective(app: App) {
  app.directive('perm', permDirective)
}

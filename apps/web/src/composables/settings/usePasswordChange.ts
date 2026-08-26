import { reactive, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { ApiError } from '@dlidli/api-client'
import { api } from '@/api'

/**
 * 修改密码（SettingsView 拆分，M3-ENG-12）。
 */
export function usePasswordChange() {
  const pwdForm = reactive({ old: '', next: '', confirm: '' })
  const pwdSaving = ref(false)

  async function onChangePassword() {
    if (pwdForm.next.length < 8 || pwdForm.next.length > 32) {
      ElMessage.warning('新密码长度需 8~32 位')
      return
    }
    if (pwdForm.next !== pwdForm.confirm) {
      ElMessage.warning('两次输入的新密码不一致')
      return
    }
    pwdSaving.value = true
    try {
      await api.auth.changePassword(pwdForm.old, pwdForm.next)
      pwdForm.old = ''
      pwdForm.next = ''
      pwdForm.confirm = ''
      ElMessage.success('密码已更新，下次登录请使用新密码')
    } catch (err) {
      ElMessage.error(err instanceof ApiError ? err.message : '修改失败，请稍后再试')
    } finally {
      pwdSaving.value = false
    }
  }

  return { pwdForm, pwdSaving, onChangePassword }
}

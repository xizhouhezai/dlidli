// 异步操作封装：统一 loading + 成功提示 + 失败 toast 兜底 + 用户取消(ElMessageBox cancel)静默。
import { ref } from 'vue'
import { ElMessage } from 'element-plus'
import { apiErrorMessage } from '@dlidli/api-client'

export function useApiAction() {
  const loading = ref(false)

  /**
   * 执行异步操作：成功可选 toast，失败统一 error toast（兜底文案 fallback），
   * 用户取消（ElMessageBox reject 的 'cancel'/'close'）静默忽略。
   * 返回 true=成功，false=失败/取消。
   */
  async function run(
    fn: () => Promise<unknown>,
    opts: { success?: string, fallback?: string } = {},
  ): Promise<boolean> {
    loading.value = true
    try {
      await fn()
      if (opts.success) ElMessage.success(opts.success)
      return true
    } catch (err) {
      if (err === 'cancel' || err === 'close') return false
      ElMessage.error(apiErrorMessage(err, opts.fallback))
      return false
    } finally {
      loading.value = false
    }
  }

  return { loading, run }
}

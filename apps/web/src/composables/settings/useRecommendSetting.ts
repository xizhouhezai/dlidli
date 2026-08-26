import { ref } from 'vue'
import { ElMessage } from 'element-plus'
import { ApiError } from '@dlidli/api-client'
import { api } from '@/api'

/**
 * 个性化推荐开关（SettingsView 拆分，M3-ENG-12，M3-REC-07 合规）。
 */
export function useRecommendSetting() {
  const recEnabled = ref(true)
  const recSaving = ref(false)

  async function loadRecommendSetting() {
    try {
      recEnabled.value = (await api.recommend.recommendSetting()).enabled
    } catch {
      // 静默失败
    }
  }

  async function toggleRecommend() {
    recSaving.value = true
    try {
      await api.recommend.setRecommendSetting(recEnabled.value)
      ElMessage.success(recEnabled.value ? '已开启个性化推荐' : '已关闭个性化推荐（首页推荐将展示热门内容）')
    } catch (err) {
      recEnabled.value = !recEnabled.value
      ElMessage.error(err instanceof ApiError ? err.message : '操作失败')
    } finally {
      recSaving.value = false
    }
  }

  return { recEnabled, recSaving, loadRecommendSetting, toggleRecommend }
}

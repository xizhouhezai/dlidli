import { ref, onUnmounted } from 'vue'
import { ElMessage } from 'element-plus'
import { ApiError } from '@dlidli/api-client'
import { api } from '@/api'

const YOUTH_LIMIT_MIN = 40

/**
 * 青少年模式（SettingsView 拆分，M3-ENG-12，M2-AUD-04）：
 * 开关 + 每日 40 分钟使用提醒（localStorage 本地计时）。
 */
export function useYouthMode() {
  const youthMode = ref(false)
  const youthSaving = ref(false)
  let youthTimer: ReturnType<typeof setInterval> | null = null

  function youthUsageKey() {
    return `youth_usage_${new Date().toISOString().slice(0, 10)}`
  }

  async function loadYouthMode() {
    try {
      youthMode.value = (await api.auth.youthMode()).enabled
      if (youthMode.value) startYouthTimer()
    } catch {
      // 静默失败，保持关闭态
    }
  }

  async function toggleYouthMode() {
    youthSaving.value = true
    try {
      await api.auth.setYouthMode(youthMode.value)
      if (youthMode.value) {
        localStorage.setItem(youthUsageKey(), '0') // 开启当日重新计时
        startYouthTimer()
      } else {
        stopYouthTimer()
      }
      ElMessage.success(youthMode.value ? '已开启青少年模式' : '已关闭青少年模式')
    } catch (err) {
      youthMode.value = !youthMode.value
      ElMessage.error(err instanceof ApiError ? err.message : '操作失败')
    } finally {
      youthSaving.value = false
    }
  }

  function startYouthTimer() {
    stopYouthTimer()
    youthTimer = setInterval(() => {
      const key = youthUsageKey()
      const used = Number(localStorage.getItem(key) ?? '0')
      const next = used + 1
      localStorage.setItem(key, String(next))
      if (next >= YOUTH_LIMIT_MIN) {
        stopYouthTimer()
        ElMessage.warning('今日青少年模式使用时长已达 40 分钟，请注意休息')
      }
    }, 60_000)
  }

  function stopYouthTimer() {
    if (youthTimer) {
      clearInterval(youthTimer)
      youthTimer = null
    }
  }

  // 计时器随组件卸载自动清理
  onUnmounted(stopYouthTimer)

  return { youthMode, youthSaving, loadYouthMode, toggleYouthMode }
}

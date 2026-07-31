// 倒计时（验证码等场景）：start(seconds) 启动，自动递减到 0，组件卸载时清理定时器。
import { ref, onUnmounted } from 'vue'

export function useCountdown(initial = 60) {
  const count = ref(0)
  let timer: ReturnType<typeof setInterval> | null = null

  function clear() {
    if (timer) {
      clearInterval(timer)
      timer = null
    }
  }

  /** 启动倒计时（默认从 initial 秒开始）。 */
  function start(seconds = initial) {
    clear()
    count.value = seconds
    timer = setInterval(() => {
      count.value--
      if (count.value <= 0) clear()
    }, 1000)
  }

  onUnmounted(clear)

  return { count, start, clear }
}

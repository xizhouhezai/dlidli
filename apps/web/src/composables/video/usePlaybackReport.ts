import { type Ref } from 'vue'
import { api } from '@/api'
import { useUserStore } from '@/stores/user'
import type { VideoDetail } from '@dlidli/api-client'

/**
 * 播放统计与进度上报（VideoView 拆分，M3-ENG-06）：
 * 有效播放计数（>5s 一次，可重试）、10s 节流的进度上报、离开页面时最终落盘。
 */
export function usePlaybackReport(
  detail: Ref<VideoDetail | null>,
  videoEl: Ref<HTMLVideoElement | undefined>,
) {
  const userStore = useUserStore()

  let viewReported = false
  let watchedSeconds = 0
  let lastTime = 0
  let lastProgressSave = 0

  /** 切换稿件后重置统计状态。 */
  function reset() {
    viewReported = false
    watchedSeconds = 0
    lastTime = 0
    lastProgressSave = 0
  }

  /** timeupdate 事件：累计真实观看时长（跳转进度不计入）+ 有效播放 + 进度节流上报。 */
  function onTimeUpdate(e: Event) {
    const video = e.target as HTMLVideoElement
    const delta = video.currentTime - lastTime
    if (delta > 0 && delta < 2) watchedSeconds += delta
    lastTime = video.currentTime

    if (!viewReported && watchedSeconds >= 5 && detail.value) {
      viewReported = true
      api.video.addView(detail.value.bvid).catch(() => {
        viewReported = false // 上报失败允许重试
      })
    }

    // 每 10s 上报观看进度（登录用户，跨端续播）
    if (userStore.token && detail.value && video.currentTime > 2) {
      const now = Date.now()
      if (now - lastProgressSave > 10_000) {
        lastProgressSave = now
        api.video.saveProgress(detail.value.bvid, Math.floor(video.currentTime)).catch(() => {})
      }
    }
  }

  /** 切清晰度/手动定位后同步 lastTime，避免误计观看时长。 */
  function notePosition() {
    lastTime = videoEl.value?.currentTime ?? 0
  }

  /** 离开页面时落盘最终进度。 */
  function flushProgress() {
    const video = videoEl.value
    if (userStore.token && detail.value && video && video.currentTime > 2) {
      api.video.saveProgress(detail.value.bvid, Math.floor(video.currentTime)).catch(() => {})
    }
  }

  return { reset, onTimeUpdate, notePosition, flushProgress }
}

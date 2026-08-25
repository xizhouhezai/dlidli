import { ref, type Ref } from 'vue'
import { bindKeyboard, PlayerCore } from '@dlidli/player'
import { ElMessage } from 'element-plus'
import { formatDuration } from '@dlidli/shared'
import { api } from '@/api'
import { useUserStore } from '@/stores/user'
import type { PartItem, StreamItem, VideoDetail } from '@dlidli/api-client'

/**
 * 播放器控制器（VideoView 拆分，M3-ENG-06）：
 * 视频元素 / PlayerCore 生命周期 / 清晰度 / 倍速 / 分P / 续播 / 自动播放。
 */
export function useVideoPlayer(detail: Ref<VideoDetail | null>) {
  const userStore = useUserStore()

  const videoEl = ref<HTMLVideoElement>()
  const playerBox = ref<HTMLElement>()
  const currentStream = ref<StreamItem | null>(null)
  let player: PlayerCore | null = null
  let unbindKeys: (() => void) | null = null

  // 多P投稿（PRD VID-05）：分P列表与当前 P
  const partList = ref<PartItem[]>([])
  const currentPart = ref(0)

  // 倍速
  const playbackRate = ref(1)
  const rateOptions = [0.5, 0.75, 1, 1.25, 1.5, 2]

  function setRate(rate: number) {
    playbackRate.value = rate
    player?.setRate(rate)
  }

  function switchPart(i: number) {
    if (i === currentPart.value || !partList.value[i]) return
    const part = partList.value[i]
    if (!part.streams.length) {
      ElMessage.warning('该分P暂无可用播放流')
      return
    }
    currentPart.value = i
    // 切换播放源（保留播放器实例，跨 P 进度独立由服务端按 bvid 记忆）
    player?.setSources(part.streams)
  }

  /** 确保 PlayerCore 已创建（绑定到当前 <video>）。 */
  function ensurePlayer(): PlayerCore | null {
    const video = videoEl.value
    if (!video) return null
    if (!player) {
      player = new PlayerCore(video, {
        onSourceChange: (s) => {
          currentStream.value = s as StreamItem
        },
      })
    }
    return player
  }

  function switchTo(stream: StreamItem) {
    player?.switchTo(stream)
  }

  function destroy() {
    player?.destroy()
    player = null
  }

  /** 首次进入时绑定快捷键（需 <video> 已渲染）。 */
  function bindKeys() {
    if (!unbindKeys && videoEl.value) {
      unbindKeys = bindKeyboard(videoEl.value, { container: playerBox.value ?? null })
    }
  }

  function unbindKeyListeners() {
    unbindKeys?.()
    unbindKeys = null
  }

  /** 登录用户续播：定位到上次观看位置（>2s 才续播，接近看完从头播）。 */
  function tryResume(bvid: string) {
    if (!userStore.token) return
    api.video
      .getProgress(bvid)
      .then(({ position }) => {
        const video = videoEl.value
        if (!video || position <= 2) return
        const apply = () => {
          const dur = detail.value?.duration || 0
          if (dur > 0 && position >= dur - 3) return // 接近看完不续播
          video.currentTime = position
          ElMessage.info(`已定位到上次观看位置 ${formatDuration(position)}`)
        }
        if (video.readyState >= 1) apply()
        else video.addEventListener('loadedmetadata', apply, { once: true })
      })
      .catch(() => {})
  }

  /** 进页自动播放；被浏览器拦截时降级为静音自动播放。 */
  async function tryAutoplay() {
    const video = videoEl.value
    if (!video) return
    try {
      await video.play()
    } catch {
      video.muted = true
      try {
        await video.play()
        ElMessage.info('已静音自动播放，点击声音图标开启声音')
      } catch {
        // 仍被拦截则保持暂停，由用户手动播放
      }
    }
  }

  return {
    videoEl,
    playerBox,
    currentStream,
    partList,
    currentPart,
    playbackRate,
    rateOptions,
    setRate,
    switchPart,
    ensurePlayer,
    switchTo,
    destroy,
    bindKeys,
    unbindKeyListeners,
    tryResume,
    tryAutoplay,
  }
}

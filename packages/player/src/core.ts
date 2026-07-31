import Hls from 'hls.js'
import type { PlayerCoreOptions, PlayerSource } from './types'
import { pickDefaultSource } from './quality'

/**
 * PlayerCore —— 框架无关的播放内核，封装 hls.js 生命周期与清晰度切换。
 * - HLS：Safari 走原生，其余浏览器用 hls.js（MSE）
 * - mp4/其它：直接挂 <video>.src
 * - 极端不支持 MSE 时回退到原画 mp4
 */
export class PlayerCore {
  private video: HTMLVideoElement
  private hls: Hls | null = null
  private opts: PlayerCoreOptions
  private sources: PlayerSource[] = []
  /** 当前生效的播放源 */
  current: PlayerSource | null = null

  constructor(video: HTMLVideoElement, opts: PlayerCoreOptions = {}) {
    this.video = video
    this.opts = opts
  }

  /** 设置源列表并自动挂载默认清晰度，返回被挂载的源。 */
  setSources(sources: PlayerSource[], autoAttach = true): PlayerSource | null {
    this.sources = sources
    const def = pickDefaultSource(sources)
    if (def && autoAttach) this.attach(def)
    return def
  }

  /** 挂载指定源；keepPosition=true 时保留当前进度与播放态（切清晰度用）。 */
  attach(source: PlayerSource, keepPosition = false): void {
    const video = this.video
    const pos = keepPosition ? video.currentTime : 0
    const wasPlaying = keepPosition && !video.paused

    this.destroyHls()
    video.removeAttribute('src')

    if (source.format === 'hls') {
      if (video.canPlayType('application/vnd.apple.mpegurl')) {
        video.src = source.url
      } else if (Hls.isSupported()) {
        this.hls = new Hls()
        this.hls.loadSource(source.url)
        this.hls.attachMedia(video)
      } else {
        // 极端兜底：换回原画 mp4
        const raw = this.sources.find((s) => s.quality === 0 && s.format !== 'hls')
        if (raw) {
          video.src = raw.url
          this.setCurrent(raw)
          return
        }
      }
    } else {
      video.src = source.url
    }

    this.setCurrent(source)
    if (keepPosition) {
      video.currentTime = pos
      if (wasPlaying) void video.play()
    }
  }

  /** 切换清晰度（同源忽略）；默认保留进度。 */
  switchTo(source: PlayerSource, keepPosition = true): void {
    if (source.url === this.current?.url) return
    this.attach(source, keepPosition)
  }

  /** 设置倍速。 */
  setRate(rate: number): void {
    this.video.playbackRate = rate
  }

  /** 释放 hls 实例（组件卸载/换稿件时调用）。 */
  destroy(): void {
    this.destroyHls()
    this.current = null
    this.sources = []
  }

  private setCurrent(source: PlayerSource): void {
    this.current = source
    this.opts.onSourceChange?.(source)
  }

  private destroyHls(): void {
    this.hls?.destroy()
    this.hls = null
  }
}

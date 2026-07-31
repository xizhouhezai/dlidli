/** 键盘快捷键选项。 */
export interface KeyboardOptions {
  /** 左右方向键快进/退秒数，默认 5 */
  seekStep?: number
  /** 上下方向键音量步进（0~1），默认 0.1 */
  volumeStep?: number
  /** 全屏目标容器（默认为 video 本身，通常传播放器外层 box 以含弹幕层） */
  container?: HTMLElement | null
}

function toggleFullscreen(el: HTMLElement): void {
  if (document.fullscreenElement) {
    void document.exitFullscreen()
  } else {
    void el.requestFullscreen?.()
  }
}

/**
 * 绑定播放器键盘快捷键，返回解绑函数。
 * 空格/k 播放暂停、←/→ 快退进、↑/↓ 音量、m 静音、f 全屏。
 * 输入框/文本域聚焦时不拦截。
 */
export function bindKeyboard(video: HTMLVideoElement, opts: KeyboardOptions = {}): () => void {
  const seekStep = opts.seekStep ?? 5
  const volStep = opts.volumeStep ?? 0.1

  const handler = (e: KeyboardEvent): void => {
    const target = e.target as HTMLElement | null
    const tag = target?.tagName
    if (tag === 'INPUT' || tag === 'TEXTAREA' || target?.isContentEditable) return

    switch (e.key) {
      case ' ':
      case 'k':
        e.preventDefault()
        if (video.paused) void video.play()
        else video.pause()
        break
      case 'ArrowLeft':
        e.preventDefault()
        video.currentTime = Math.max(0, video.currentTime - seekStep)
        break
      case 'ArrowRight':
        e.preventDefault()
        video.currentTime = Math.min(video.duration || Infinity, video.currentTime + seekStep)
        break
      case 'ArrowUp':
        e.preventDefault()
        video.volume = Math.min(1, video.volume + volStep)
        break
      case 'ArrowDown':
        e.preventDefault()
        video.volume = Math.max(0, video.volume - volStep)
        break
      case 'm':
      case 'M':
        video.muted = !video.muted
        break
      case 'f':
      case 'F':
        toggleFullscreen(opts.container ?? video)
        break
    }
  }

  window.addEventListener('keydown', handler)
  return () => window.removeEventListener('keydown', handler)
}

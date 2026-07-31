// 视频首帧截取（poster 候选）：浏览器可解码的格式（mp4/mov 等）截首帧，
// 不支持的格式（mkv/flv）返回 null，由上层回退到上传封面/默认封面。
export function captureVideoPoster(file: File): Promise<Blob | null> {
  return new Promise((resolve) => {
    const url = URL.createObjectURL(file)
    const video = document.createElement('video')
    video.muted = true
    video.preload = 'auto'

    let settled = false
    const finish = (blob: Blob | null) => {
      if (settled) return
      settled = true
      window.clearTimeout(timer)
      URL.revokeObjectURL(url)
      resolve(blob)
    }
    // 8s 兜底：解码卡住时放弃截帧
    const timer = window.setTimeout(() => finish(null), 8000)

    video.onerror = () => finish(null)
    video.onloadedmetadata = () => {
      // 取 10% 进度处（≤1s），避开纯黑片头
      video.currentTime = Math.min(1, (video.duration || 1) * 0.1)
    }
    video.onseeked = () => {
      const canvas = document.createElement('canvas')
      canvas.width = video.videoWidth
      canvas.height = video.videoHeight
      const ctx = canvas.getContext('2d')
      if (!ctx || canvas.width === 0) {
        finish(null)
        return
      }
      ctx.drawImage(video, 0, 0)
      canvas.toBlob((b) => finish(b), 'image/jpeg', 0.85)
    }
    video.src = url
  })
}

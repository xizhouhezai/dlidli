/** 播放源（对应后端 video 详情的 StreamItem）。 */
export interface PlayerSource {
  /** 清晰度：0=原画，其余如 360/720 */
  quality: number
  /** 封装格式：'hls' | 'mp4' 等 */
  format: string
  /** 播放地址（可能带签名参数） */
  url: string
}

/** PlayerCore 构造选项。 */
export interface PlayerCoreOptions {
  /** 挂载/切换后回调（用于同步 UI 当前清晰度） */
  onSourceChange?: (source: PlayerSource) => void
}

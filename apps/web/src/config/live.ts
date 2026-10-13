/**
 * 直播流配置（M4-LIV-06，M4-LIV-01 起改为「真实流优先」）。
 *
 * 播放地址首选房间详情下发的 `play_urls.hls`（后端按 live.playDomain 拼装：
 * http://<playDomain>/live/room_<id>.m3u8，契约见 .dev-logs/liv01/PLAN.md D4）。
 * 本文件不再持有任何外部公共测试流地址；演示流仅作为 dev 兜底，
 * 由 `demoFallback` 控制，指向本仓自带的 mp4。
 */
export const LIVE_STREAM_CONFIG = {
  /**
   * 真实流（room.play_urls.hls）缺失时的 dev 兜底开关。
   * - true：落 mp4Fallback 本地循环源，页面显示「演示流」标记（dev 环境默认）
   * - false：不播放任何回落源，只提示等待开播
   * 生产环境建议置 false，避免把「无流」误呈现为「有画面」。
   */
  demoFallback: true,
  /** 兜底源：本仓 public/demo-live.mp4 循环播放 */
  mp4Fallback: '/demo-live.mp4',
} as const

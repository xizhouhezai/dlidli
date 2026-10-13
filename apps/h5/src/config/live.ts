/**
 * 直播流配置（M4-LIV-06，H5 端；M4-LIV-01 起改为「真实流优先」）。
 *
 * 播放地址首选房间详情下发的 `room.play_urls.hls`（后端按 live.playDomain 拼装：
 * http://<playDomain>/live/room_<id>.m3u8，契约见 .dev-logs/liv01/PLAN.md D4）。
 * 本文件不再持有任何外部公共测试流地址；演示兜底由 demoFallback 控制。
 * 注意：uni-app 无法跨包引用 apps/web 的 config，这里保持一份镜像常量。
 */
export const LIVE_STREAM_CONFIG = {
  /**
   * 真实流（play_urls.hls）缺失时的 dev 兜底开关。
   * - true：落 mp4Fallback 本地循环源（dev 默认）
   * - false：不播放任何回落源，只提示等待开播
   */
  demoFallback: true,
  /** 兜底源：本端 static 目录下 mp4 循环播放 */
  mp4Fallback: '/static/demo-live.mp4',
} as const

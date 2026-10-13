/**
 * 直播流配置（M4-LIV-06，H5 端）。
 *
 * ⚠️ 替换点：LIV-01（SRS 部署 + RTMP 推拉流）完成前使用演示流；
 * LIV-01 后只需改这里——把 `demo` 置 false 并把 hlsUrl 指向
 * 推流回调生成的 `http://<hls域名>/{room_key}.m3u8`，页面不动。
 * 注意：uni-app 无法跨包引用 apps/web 的 config，这里保持一份镜像常量。
 */
export const LIVE_STREAM_CONFIG = {
  /** false = 走 hlsUrl 真实流（LIV-01 后） */
  demo: true,
  /** 演示流：公共 HLS 测试流 */
  hlsUrl: 'https://test-streams.mux.dev/x36xhzz/x36xhzz.m3u8',
  /** 备用本地源（mp4 循环） */
  mp4Fallback: '/static/demo-live.mp4',
} as const

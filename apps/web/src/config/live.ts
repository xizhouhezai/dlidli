/**
 * 直播流配置（M4-LIV-06）。
 *
 * ⚠️ 替换点：LIV-01（SRS 部署 + RTMP 推拉流）完成前使用演示流；
 * LIV-01 后只需改这里——把 `demo` 置 false 并把 hlsUrl 指向
 * 推流回调生成的 `http://<hls域名>/{room_key}.m3u8`（后端 push_url 的拉流域名对应物），
 * 页面与控制条形态不变。
 */
export const LIVE_STREAM_CONFIG = {
  /** false = 走 hlsUrl 真实流（LIV-01 后） */
  demo: true,
  /** 演示流：公共 HLS 测试流（m3u8 循环切片） */
  hlsUrl: 'https://test-streams.mux.dev/x36xhzz/x36xhzz.m3u8',
  /** 备用本地源：public/demo-live.mp4 循环播放（演示流不可达时兜底） */
  mp4Fallback: '/demo-live.mp4',
} as const

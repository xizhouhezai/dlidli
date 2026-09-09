// 微信内浏览器适配（M2-H5-06）：环境检测 + JSSDK 分享卡片
// JSSDK 签名来自后端 /api/v1/wechat/jssdk-sign（公众号未配置时后端返回未启用，本工具静默降级）

/** 是否微信内置浏览器 */
export function isWeChat(): boolean {
  if (typeof navigator === 'undefined') return false
  return /MicroMessenger/i.test(navigator.userAgent)
}

/** 微信版本号（解析失败返回空串） */
export function weChatVersion(): string {
  const m = /MicroMessenger\/([\d.]+)/i.exec(navigator.userAgent)
  return m ? m[1] : ''
}

let wxLoading: Promise<any> | null = null

/** 动态加载微信 JSSDK（幂等） */
function loadJweixin(): Promise<any> {
  if (typeof window === 'undefined') return Promise.reject(new Error('no window'))
  const w = window as any
  if (w.wx?.config) return Promise.resolve(w.wx)
  if (wxLoading) return wxLoading
  wxLoading = new Promise((resolve, reject) => {
    const s = document.createElement('script')
    s.src = 'https://res.wx.qq.com/open/js/jweixin-1.6.0.js'
    s.onload = () => (w.wx ? resolve(w.wx) : reject(new Error('jweixin 加载异常')))
    s.onerror = () => reject(new Error('jweixin 加载失败'))
    document.head.appendChild(s)
  })
  return wxLoading
}

/** 完整页面 URL（去掉 hash，供签名） */
function signURL(): string {
  return window.location.href.split('#')[0]
}

export interface ShareContent {
  title: string
  desc?: string
  link?: string
  imgUrl?: string
}

/**
 * 配置微信分享卡片（H5 内调用一次即可）。
 * - 非微信环境 / 后端未启用 / wx.config 失败：静默降级（微信内仍可通过右上角菜单分享，
 *   卡片标题自动取 document.title，因此调用方应同步设置好 title）
 */
export async function setupWeChatShare(content: ShareContent): Promise<boolean> {
  if (!isWeChat()) return false
  try {
    const wx = await loadJweixin()
    // 签名走 uni.request 适配的 api 客户端太重，这里直接 fetch（H5 均支持）
    const res = await fetch(`/api/v1/wechat/jssdk-sign?url=${encodeURIComponent(signURL())}`).then(
      (r) => r.json(),
    )
    if (res?.code !== 0 || !res.data?.signature) return false
    const d = res.data
    await new Promise<void>((resolve, reject) => {
      wx.config({
        debug: false,
        appId: d.app_id,
        timestamp: d.timestamp,
        nonceStr: d.nonce_str,
        signature: d.signature,
        jsApiList: ['updateAppMessageShareData', 'updateTimelineShareData'],
      })
      wx.ready(() => resolve())
      wx.error(() => reject(new Error('wx.config 失败')))
    })
    const link = content.link || signURL()
    wx.updateAppMessageShareData({
      title: content.title,
      desc: content.desc || 'DliDli - 你感兴趣的视频都在 DliDli',
      link,
      imgUrl: content.imgUrl || '',
    })
    wx.updateTimelineShareData({
      title: content.title,
      link,
      imgUrl: content.imgUrl || '',
    })
    return true
  } catch {
    return false // 静默降级
  }
}

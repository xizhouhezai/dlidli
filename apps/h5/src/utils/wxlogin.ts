// 微信小程序授权登录（M3-MP-01）：wx.login 取 code → 后端 code2session 换 openid → 登录/自动注册。
// 仅在 mp-weixin 编译目标下可用；H5/App 端编译时整体摇树剔除（条件编译）。

/** wx.login 取临时凭证 code（失败 reject，调用方 toast 兜底） */
export function wxLoginCode(): Promise<string> {
  // #ifdef MP-WEIXIN
  return new Promise<string>((resolve, reject) => {
    uni.login({
      provider: 'weixin',
      success: (res) => {
        if (res.code) resolve(res.code)
        else reject(new Error('微信授权失败'))
      },
      fail: () => reject(new Error('微信授权失败')),
    })
  })
  // #endif
  // eslint-disable-next-line no-unreachable
  return Promise.reject(new Error('当前环境不支持微信授权登录'))
}

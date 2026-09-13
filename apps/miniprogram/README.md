# @dlidli/miniprogram

微信小程序端，对应里程碑 **M3-MP-01~03**。当前与 H5 共用 `apps/h5` 的 uni-app 源码，
通过不同编译目标生成 `mp-weixin` 产物（架构决策见[前端架构](../../docs/architecture/frontend.md)）。

## 开发与构建

```bash
# 根目录执行
pnpm mp:dev       # 微信小程序开发模式
pnpm mp:build     # 生成 apps/h5/dist/build/mp-weixin
```

然后使用微信开发者工具导入 `apps/h5/dist/build/mp-weixin`。首次联调需在
`apps/h5/src/manifest.json` 的 `mp-weixin.appid` 配置真实小程序 AppID；不要把 Secret
提交到仓库，后端使用 `DLIDLI_WECHAT_MPAPPID` 与 `DLIDLI_WECHAT_MPAPPSECRET` 注入。

## 当前能力

- `mp-weixin` 编译目标已打通，主包构建产物约 200KB。
- `wx.login` → 后端 `code2session` → `/api/v1/auth/login/wechat` 微信授权登录已接入。
- 首页、搜索、播放、个人中心等页面复用 H5 uni-app 页面并可生成小程序页面产物。
- 微信卡片分享与类目资质提审仍需真实 AppID、开发者账号及视频类目资质，见 Roadmap 风险登记册。

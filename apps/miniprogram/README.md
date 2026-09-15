# @dlidli/miniprogram

微信小程序端，对应里程碑 **M3-MP-01~03**。当前与 H5 共用 `apps/h5` 的 uni-app 源码，
通过不同编译目标生成 `mp-weixin` 产物（架构决策见[前端架构](../../docs/architecture/frontend.md)）。

> **主体口径**：本项目小程序为**个人主体**（AppID `wx5a8b4b33acc3ec20`），
> 个人主体无法申请**视频类目**，因此**仅做本地测试，不提交平台审核**。
> 对外分发以 H5 兜底。

## 开发与构建

```bash
# 根目录执行
pnpm mp:dev       # 微信小程序开发模式
pnpm mp:build     # 生成 apps/h5/dist/build/mp-weixin
```

然后使用微信开发者工具导入 `apps/h5/dist/build/mp-weixin`。

AppID 已写入 `apps/h5/src/manifest.json` 的 `mp-weixin.appid`（会同步到构建产物的
`project.config.json`）。**Secret 不要提交到仓库**，后端使用
`DLIDLI_WECHAT_MPAPPID` 与 `DLIDLI_WECHAT_MPAPPSECRET` 注入。

## 当前能力

- `mp-weixin` 编译目标已打通，主包构建产物约 203KB。
- `wx.login` → 后端 `code2session` → `/api/v1/auth/login/wechat` 微信授权登录已接入。
- 首页、搜索、播放、个人中心等页面复用 H5 uni-app 页面并可生成小程序页面产物。
- 播放页已接入 `onShareAppMessage`（好友卡片）与 `onShareTimeline`（朋友圈），
  分享路径携带 `bvid`、封面取视频封面并有默认封面兜底。

## 本地验证边界

已通过真实 AppID 完成**构建级**验收（产物 `project.config.json` 带 AppID、页面编译无错误）。
未完成的部分（属外部条件，非代码缺陷）：

- 好友/朋友圈**分享菜单与卡片的真机实测**需在微信开发者工具登录真实账号后进行；
- **平台提审**需视频类目资质，个人主体无法申请，故不做。

详见 [`docs/architecture/miniprogram-release-checklist.md`](../../docs/architecture/miniprogram-release-checklist.md)。

# 开发进度管理

> 更新频率：**每周五更新** ｜ 当前里程碑：`M0 基建` ｜ 更新日期：2026-07-28
>
> 使用说明：本页为项目"单一事实来源"的进度视图。每周例会后由项目负责人更新状态与百分比；任务级勾选在 [开发清单](/project/checklist) 中维护。

## 1. 总体进度仪表盘

| 里程碑 | 计划窗口 | 状态 | 进度 | 备注 |
| --- | --- | --- | :-: | --- |
| M0 基建 | W1-W4 | 🟢 进行中 | 92% | 仅剩 M0-ENG-13 staging 部署 |
| M1 MVP 内测 | W5-W12 | 🟢 进行中 | 100% | player 内核封装（VID-09），M1 功能全部完成 |
| M2 V1.0 公测 | W13-W24 | ✅ 已完成 | 100% | SYS-01/02 收口，M2 全部 40 项完成 |
| M3 V2.0 增长 | W25-W48 | 🟢 进行中 | 54% | 稿件管理 + 数据大盘 + Banner + 创作者中心 + 推荐系统 |
| M4 V3.0 商业化 | W49+ | ⚪ 未开始 | 0% | - |

> 状态图例：⚪ 未开始 ｜ 🟢 进行中 ｜ 🟡 有风险 ｜ 🔴 阻塞 ｜ ✅ 已完成

## 2. 模块进度矩阵

| 模块 | 后端 | Web | H5 | 小程序 | 后台 | 整体状态 |
| --- | :-: | :-: | :-: | :-: | :-: | :-: |
| 工程基建 | 80% | 80% | - | - | 0% | 🟢 |
| 用户账号 | 100% | 100% | 0% | 0% | 0% | 🟢 |
| 视频投稿/转码 | 90% | 85% | - | - | - | 🟢 |
| 视频播放 | 75% | 90% | 0% | 0% | - | 🟢 |
| 弹幕 | 70% | 70% | 0% | 0% | 0% | 🟢 |
| 评论互动 | 80% | 75% | 0% | 0% | 0% | 🟢 |
| 三连（赞/币/藏） | 95% | 95% | 0% | 0% | - | 🟢 |
| 关注/动态 | 75% | 70% | 0% | 0% | - | 🟢 |
| 消息通知 | 60% | 65% | 0% | 0% | - | 🟢 |
| 搜索 | 50% | 60% | 0% | 0% | 0% | 🟢 |
| 推荐 | 0% | 0% | 0% | 0% | - | ⚪ |
| 创作者中心 | 0% | 0% | 0% | - | - | ⚪ |
| 审核/管理后台 | 60% | - | - | - | 55% | 🟢 |
| 会员/支付 | 0% | 0% | 0% | - | 0% | ⚪ |
| 直播 | 0% | 0% | 0% | - | 0% | ⚪ |

> `-` 表示该端不涉及此模块。

## 3. 本周进展（W1：2026-07-27 ~ 07-31）

### 已完成

- [x] Monorepo 框架搭建（pnpm workspace）
- [x] VitePress 文档系统上线
- [x] 产品需求文档 V1.0 草案（11 个 PRD 模块 + NFR）
- [x] 技术架构文档 V1.0 草案（总体/后端/前端/数据模型/视频流水线）
- [x] Go 后端脚手架：cmd/api + config/logger/errcode/response/jwtx + 中间件 + infra(MySQL/Redis) + /health 验证通过
- [x] 数据库迁移工具 cmd/migrate + 初始 schema（11 张核心表 + 分区种子数据）
- [x] docker-compose 开发环境（MySQL/Redis/Kafka/MinIO）
- [x] CI 流水线（GitHub Actions：Go vet+test+build、Web lint+build、文档构建）
- [x] 本地 MySQL（3307）接入：自动建库 + 迁移落库，/health 全组件 up
- [x] packages/shared（类型/常量/格式化工具）与 packages/api-client（请求封装/token 拦截/适配器）骨架
- [x] Web 脚手架：Vue3+Vite+TS+Pinia+Router+Element Plus，首页完成后端联通验证，登录页框架就绪
- [x] 账号模块后端（M1-ACC-01/03）：短信验证码（mock）自动注册登录、密码登录+错误锁定、JWT+refresh 轮换、/users/me，curl 全链路验证通过
- [x] Web 登录页对接真实接口（密码/短信 Tab、验证码倒计时），UI 对标 B 站风格（粉色主题/搜索框头部/视频卡片网格）
- [x] 资料与头像（M1-ACC-05）：PATCH /users/me（昵称唯一校验）、头像上传（Storage 抽象 + 本地磁盘 + /static 静态服务），curl 验证通过
- [x] Web 账号设置页（/settings，登录守卫）：头像上传、昵称/签名/性别编辑
- [x] 登录页优化：去头部全屏布局 + 动态背景（渐变/漂浮弹幕/光斑/入场动画）
- [x] 上传模块（M1-VID-01）：init 秒传/断点续传会话、分片上传（5MB）、合并 SHA-256 校验、upload_file 登记；脚本实测 12MB 三分片全链路 + 秒传命中
- [x] 可观测性增强：访问日志输出错误链（排查出本机老版 Redis 不支持多字段 HSET，已改 HMSET 兼容）
- [x] 稿件模块（M1-VID-02）：投稿/分区/详情/我的稿件/公开列表/软删除，bvid 编码，dev 自动过审；脚本实测秒传→投稿→列表/详情全链路
- [x] Web 投稿页（M1-VID-07）：拖拽选文件、hash-wasm 增量 SHA-256、并发 3 分片+断点续传+秒传、表单提交
- [x] 首页接真实数据（M1-VID-11）：分区筛选 + 最新列表，无封面渐变占位
- [x] 封面体系：DliDli 默认封面（SVG）、投稿首帧截取/自定义上传、封面上传接口
- [x] 播放页（M1-VID-10）：原画播放 + UP主卡片 + 简介/标签 + 同分区相关推荐，首页卡片跳转打通
- [x] 有效播放计数（M1-VID-06 部分）：>5s 真实观看上报，UID/IP 8h Redis 去重，OptionalAuth 游客可计
- [x] 转码上线（M1-VID-03/04）：DB 任务队列（SKIP LOCKED）+ 进程内 Worker，FFmpeg 转 360P/720P HLS、ffprobe 回写时长、自动抽帧封面、状态机推进；E2E 实测 8s 视频全链路通过
- [x] 播放页多清晰度：hls.js 接入（Safari 原生兜底），720P/360P/原画切换保留进度
- [x] 开发机安装 FFmpeg 8.1.2（winget），转码路径写入 dev 配置
- [x] 文档站升级 Teek 主题（保留 hero 首页，面包屑/代码折叠/主题增强，品牌粉主题色）
- [x] 弹幕上线（M1-DM-01~04）：发送（Lv1+5s 频控+影子屏蔽）、分段拉取+Redis 段缓存、DOM 轨道渲染（滚动/顶/底）、输入框/开关/乐观上屏；脚本实测发送/频控 40001/拉取/计数全通过
- [x] 评论+点赞上线（M1-ITR-01~04）：两级评论（热度/最新、楼中楼预览+展开、作者/UP主删除、影子屏蔽）、视频/评论点赞开关；双账号脚本实测发布/回复/点赞/越权 403/计数全通过
- [x] 审核后台上线（M1-ADM-01/02/05）：管理员登录（独立令牌隔离）、待审队列/预览/通过/驳回+审计日志、/admin 工作台页面；autoApprove 已关闭，E2E 实测"投稿→转码→待审隐藏→过审发布"全链路
- [x] 稿件管理页（M1-VID-08）：状态标签/数据概览/删除/分页，头部下拉入口
- [x] M1 收尾：观看进度跨端续播（Redis 90d、前端 10s 节流+离开落盘+定位）、首页最新/最热 Tab + 加载更多分页
- [x] 三连上线（M2-ITR-01~05）：硬币体系（注册送 5/每日首登 +1/流水）、投币（规则校验/幂等/退款补偿）、收藏+我的收藏列表、一键三连；前端长按点赞触发三连、投币弹层、收藏开关；脚本+浏览器双重实测全通过
- [x] 管理后台拆为独立应用 apps/admin（dev :5175，独立路由/令牌/构建，与 C 端 web 完全分离部署；浏览器实测登录→守卫→审核队列全通）
- [x] 关注体系上线（M2-FLW-01）：关注/取关（禁关自己/幂等）、关注数/粉丝数、双列表；播放页 UP 主卡片关注按钮+粉丝数；双账号脚本+浏览器实测全通过
- [x] 个人空间上线（M2-FLW-02 / M1-ACC-07）：/space/:uid 资料头部+关注按钮+关注/粉丝数，投稿/关注/粉丝/收藏(仅本人) Tab；公开资料接口 + 视频列表按 UP 过滤；播放页 UP 卡片/头像下拉入口；浏览器实测全通
- [x] 动态 Feed 上线（M2-DYN-01~03）：投稿发布钩子自动生成动态（转码自动过审与人工过审双触点）、图文动态（敏感词拒发）、拉模式 Feed+游标分页；动态页发布器/关注流/视频卡片；E2E 实测投稿→过审→粉丝 Feed 自动出现投稿动态
- [x] 搜索上线（M2-SRH-03/04）：视频标题/UP 主昵称搜索（MVP MySQL LIKE，接口层预留 ES 切换）；头部搜索框接真 + /search 结果页（视频/用户 Tab+分页）；脚本+浏览器实测全通
- [x] 消息通知上线（M2-MSG-01/03）：赞/评论/回复/关注四类通知投递（自我触发过滤、旁路失败仅日志）、未读数/全部已读/游标列表；头部小铃铛未读角标（60s 轮询）+ 通知中心页（未读红点/点击跳转/进页已读）；脚本+浏览器闭环实测全通
- [x] 账号安全闭环（M1-ACC-08 / ACC-06 找回密码）：修改密码（首次设密免旧密、旧密校验、强度限制）、短信验证码重置密码（重置后解除登录锁定）；设置页改密卡片 + /reset-password 找回密码页 + 登录页入口；脚本 E2E 全通
- [x] 扫尾收官（M1-ACC-04 / M2-ITR-03 完整版）：自研 SVG 图形验证码（crypto/rand、Redis 一次性、密码登录强制校验、点击刷新）；多收藏夹（CRUD/收藏到指定夹/删夹清理/默认夹懒创建）+ 播放页收藏弹层（选夹/新建）；修复雪花 ID 经 JS Number 丢精度问题（全链路字符串化）；脚本+浏览器实测全通
- [x] H5 端启动（M2-H5-01）：uni-app vite-ts 工程 apps/h5（:5176），api-client 注入 uni.request 适配器复用全部接口；移动端首页（双列视频流+分区+最新/最热+下拉刷新+触底加载）+ 播放页（原生 video 播 HLS+有效播放上报+UP主/三连数据/简介）；浏览器移动视口实测首页→播放页全通
- [x] 转发到动态（M2-ITR-06）：dynamic 新增 type=3 转发视频（带转发语+引用视频卡片，敏感词校验，share_cnt 回写）；播放页转发按钮（弹框填转发语）+ Feed 转发标识；脚本+浏览器实测全通（至此互动模块 ITR 全部完成）
- [x] M1 收尾验收（M1-REL-03 + REL-02 起步）：种子脚本（FFmpeg 10 源视频×5 UP 投 50 稿→转码→批量过审，发布总数 56）；Prometheus /metrics 暴露（dlidli_http 请求量/耗时直方图/in-flight，路由模板作 path 标签防高基数）；全流程浏览器走查：首页 24 卡片/播放 readyState=4/赞币评弹全 code=0/搜索命中
- [x] 前端体验优化：Web 路由三层重构（App.vue 入口 / MainLayout 布局 + AppHeader 公用头部 / login、reset 顶级独立路由，守卫带 redirect 回跳）；首页无限滚动（scroll 距底 400px 自动加载，替换手动按钮）；首页推荐轮播（el-carousel 取最热前 6，渐变遮罩+品牌色圆点+5s 自切+点击跳转，仅首页 Tab 展示；后台 Banner 配置待 M3-OPS-01）
- [x] 敏感词库管理（M1-ADM-04）：0009 迁移 sensitive_word 表 + admin CRUD 接口（列表/新增/删除，雪花 ID 字符串化）+ moderate 包改线程安全动态词库（RWMutex + 内置默认词合并），启动加载/增删后 SetWords 即时热刷无需重启；admin 敏感词管理页（tag 列表+添加+删除+导航）；E2E 实测加词→动态发布被拦(10002)→删词→放行全通
- [x] 用户治理（M1-ADM-03）：0010 迁移 user 加 muted_until/banned_until；account/govern.go 处罚（mute/unmute/ban/unban）+ 到期懒解除（登录/发言时检查自动恢复，无需定时任务）；admin 用户查询（UID/手机号/昵称+状态筛选）/处罚接口+审计；发言链路（评论/弹幕/动态/转发/投稿）统一 EnsureCanPublish 禁言拦截，登录 resolveBanOnLogin 封号拦截；admin 用户管理页（表格+状态 tag+禁言/封禁/解除）；E2E 全绿（禁言拦评论 20010、封号拦登录 20006）
- [x] 后台 UI 参考 vue-vben-admin 改造（M1-ADM-07）：新增 AdminLayout（深色侧边栏分组菜单+顶栏面包屑+用户下拉退出）；路由重组为 layout 子路由（登录独立）；新增 Dashboard 工作台（渐变欢迎条+待审/用户/封禁/敏感词 4 统计卡+快捷入口）；审核/敏感词/用户三页去自带 header 改 page-head+page-card 卡片化；装 @iconify-json/mingcute 统一图标；lint/build 通过，浏览器实测菜单/面包屑/统计卡/跳转全正常
- [x] RBAC 权限体系全套（M2-RBAC-01~05）：0011 迁移 admin_role/permission/user_role/role_permission 4 表 + admin_user 加 nickname/last_login_at；seedRBAC 幂等 upsert 11 权限点（menu/button，模块:操作命名）+ 6 内置角色（super 不落权限关联鉴权时短路），默认 admin 绑 super；middleware.RequirePerm(HasPerm, code) 按权限码鉴权，每个业务路由挂对应 code；/me/permissions 下发当前登录者权限码+可见菜单；账号 CRUD/启停/重置密码（禁操作自己）、角色 CRUD+分配权限（内置禁删）；前端 permissionStore + v-perm 指令（无权移除元素）+ AdminLayout 动态菜单（后端下发按权限过滤）+ 账号管理页/角色管理页（el-tree 权限树）；E2E 全绿：super 11权限6菜单、无权访问返10004、内置角色禁删返10002、停用登录20003、moderator 登录只见工作台/用户管理
- [x] 前端公用抽取规范落地（DRY）：新增规范 docs/architecture/frontend.md §4.0（同一逻辑/结构出现第2次就抽公用，分层归属 shared/composables/components/全局样式）；api-client 新增 apiErrorMessage(err,fallback) 错误文案归一化；admin 抽出 PageHead 组件 + useApiAction/usePagedList composable + 全局 .pink-btn，应用到 Review/SensitiveWords/Users/Admins/Roles 5 页（删 4 处 scoped 按钮样式 + 统一 19 处 try/catch 兼底）；web 抽出 useCountdown（修复 LoginView 定时器泄漏，应用 Login/ResetPassword）+ VideoCard 组件（应用 Search/Space）；web/admin lint 0 + build 通过，搜索页 VideoCard 实测渲染正常
- [x] 成长体系上线（M2-GRW-01~03）：0012 迁移 exp_log 流水表；growth 模块规则引擎（5 类经验来源：每日登录/观看 +5 每日一次、投稿 +10 日 2 次、弹幕/评论 +1 日 20 次，Redis 每日去重/限量；等级表 Lv0-Lv6 阈值自动重算；今日任务状态聚合）；触发点接入：登录/有效观看(≥5s)/投稿发布(转码自动过审与人工过审双触点)/发弹幕/发评论；GET /growth/summary + /growth/exp-logs；硬币明细接口 GET /users/me/coin-logs；Lv3 解锁彩色/顶部/底部弹幕（低等级返 40003）；Web 成长中心页 /growth（等级卡徽章/经验进度条/权益预告 + 今日任务 + 经验/硬币明细 Tab 分页）+ 头部下拉入口 + 弹幕栏 Lv3 解锁提示；E2E 实测：登录+5/弹幕+1/评论+1/观看+5/投稿+10 全累计、每日去重、彩色弹幕拦截 40003、升级 Lv1→Lv2 重算全通
- [x] 个人中心禁言/封禁状态可视化（optimize/me-status-display）：Profile/登录返回新增 status/muted_until/banned_until 字段（shared User 类型同步）；新增 AccountStatusAlert 组件（禁言/封禁/注销三态提示）；SettingsView 顶部 + 本人 SpaceView 顶部接入（他人空间不展示）；E2E 实测禁言用户登录→设置页/本人空间展示警告条、他人空间无提示、控制台无报错
- [x] 机审合规四件套上线（M2-AUD-01~04）：contentmod 机审包（敏感词热加载 + 手机号/QQ/微信/外链规则正则）统一接入评论/弹幕/动态/转发/投稿五处；video 风险分级（机审命中→高、新账号→中、默认低，审核队列按风险降序）；0013 迁移 report 表 + 举报体系全闭环（C 端全对象举报（视频/评论/弹幕/动态/用户）防重复 → admin 举报队列页（report:view/handle 权限点）→ 忽略/删除/删除并处罚 → 站内通知反馈举报人）；青少年模式（后端持久化开关 + 设置页卡片 + 每日 40 分钟本地计时提醒）；E2E + 浏览器双端实测全通
- [x] 后台 UI 修复（optimize）：举报处理菜单 icon 补 uno safelist（i-mingcute-flag-3-line，菜单图标由后端动态下发静态扫描不到）；用户管理页无头像用户兜底默认头像（复制 default-avatar.png 至 admin assets + 昵称首字 fallback）；浏览器实测菜单旗帜图标渲染 + 47/49 无头像用户默认头像展示正常
- [x] 弹幕进阶四件套上线（M2-DM-01~04）：发送工具条（滚动/顶/底模式 + 8 色板，Lv3 解锁置灰）+ 重复 30s 去重（40004）；弹幕列表面板（独立弹窗，时间跳转）+ 悬停操作（暂停/复制/举报/屏蔽）；0014 迁移 danmaku_block 屏蔽表（服务端账号级：关键词 ≤200 + 用户哈希）+ 拉取接口登录态过滤 + 设置页屏蔽管理卡片；弹幕设置面板（不透明度/字号/区域/速度/密度，localStorage 持久化）；WS 实时广播（gorilla/websocket 视频房间 + Origin 白名单 + 连接上限 + 前端断线重连 + Vite 代理 ws:true）；浏览器双连接广播实测通过
- [x] 弹幕缺陷修复（fix/dm-duplicate-and-scroll，v0.10.1）：发送弹幕变两条（hub.Broadcast 按 excludeUID 排除发送者本人，广播 IsSelf=false 副本）；滚动弹幕停留左侧不消失（Vue scoped CSS 重命名 @keyframes 导致 JS 内联动画名不匹配，keyframes 移入全局样式块；轨迹改 --dm-from/--dm-to 挂载后按实际宽度计算，右边缘外→左边缘外）；浏览器实测动画对象真实创建、transform 644→-174 单调递减、无重复无残留
- [x] 弹幕缺陷根源修复（fix/dm-dedupe-and-fontscale，v0.10.2）：弹幕重复根源 = WS 连接未带 token 被解析为游客，服务端 excludeUID 排除失效广播回到发送者；middleware 支持 query token（token/access_token）+ 前端 WS URL 拼 ?token=，服务端排除链路真正生效（实测带 token 连接无自广播、游客连接正常收）；前端 inject/WS 双端 shown 去重兜底；字号缩放无效 = JS 内联 font-size 覆盖 CSS calc，改 --dm-font-base 变量参与缩放（实测 1.5→30px、0.8→16px）；Lv3 提示去重（删工具条内重复提示，保留单条并加大间距）
- [x] 弹幕操作条修复（fix/dm-actions-hover，v0.10.3）：贴顶弹幕操作条固定在弹幕上方 -24px 超出弹幕层被 overflow:hidden 裁剪 → 按弹幕位置自适应（距顶 <30px 时 is-below 放下方）；首轨道 TRACK_PAD 4px 留白防贴顶/贴底；操作条是弹幕子元素，鼠标移向按钮触发弹幕 mouseout 导致操作条被移除 → 改弹幕元素直接绑定 mouseenter/mouseleave（进入子元素不算离开），实测移向按钮操作条保持、点击生效
- [x] 系统管理收口（M2-SYS-01/02，M2 全部完成）：0015 迁移 system_config + data_dict 表（3 组 12 项种子）；审计日志中心（查询筛选：操作者/动作/对象/日期 + 中文映射 + CSV 导出 UTF-8 BOM 1 万条上限；audit:view/audit:export 权限点 + admin 审计日志页）；系统配置 CRUD（键唯一、业务侧 GetConfig 热读取）+ 数据字典 CRUD（类型+值唯一；config:view/edit、dict:view/edit 权限点 + admin 系统配置页/数据字典页）；E2E 实测审计 110 条筛选导出、配置增改删、字典三组全通
- [x] 推荐系统起步（M3-REC-01/02/04~07，6 项）：0016 迁移 user_behavior（行为日志，ClickHouse 预留）+ user_dislike（负反馈）+ user.recommend_on（合规开关）；热度榜（加权分：播1赞3币5藏4评4弹幕2转3，全站/分区，Redis 5min 缓存）；推荐服务（混合召回：兴趣分区热度+全站热度+新稿池（48h 播放<100 保底）→ 已看/负反馈过滤 → 打散（同UP≤1、同分区连≤3））；首页「推荐」Tab（默认）+ 卡片「不感兴趣」+ 曝光上报；设置页个性化推荐开关（关闭退化为热度榜）；E2E 实测：榜单/推荐/行为/负反馈后列表移除/开关全通；ItemCF（REC-03）以降级方案替代待规模化
- [x] 创作者中心（M3-CRT-01~04，4 项）：0017 迁移 creator_settlement 日结算表；实时聚合看板（video_stat + 行为日志，替代 T+1 数仓）：/creator/overview（稿件数/总播放/赞/币/藏/粉丝/近7日播放/累计收益）+ /creator/videos（单稿统计+有效播放+收益）+ /creator/trend（7 天补零对齐）+ /creator/settlements；创作激励（有效播放×1分/次，请求时全量结算 INSERT SELECT 幂等 upsert）；Web /creator 页（5 统计卡 + CSS 柱状趋势 + 稿件数据/收益明细 Tab）+ 头部入口；E2E 实测 4 天 17 播放=17 分结算/趋势/明细全通；多P投稿合集（CRT-05）延后
- [x] 运营位 Banner（M3-OPS-01 Banner 部分）：0018 迁移 banner 表；banner 模块（公开 GET /banners image 空回退稿件封面 + admin CRUD bvid 校验）；权限点 banner:view/banner:edit（operator 角色）+ admin Banner 配置页（预览/标题/跳转/排序/启停/CRUD 弹窗）；首页轮播接入（配置优先，右侧 2×2 网格最热填充不空白，无配置回退最热前 8）；AdminLayout 菜单分组/路由映射补 banner；E2E 实测封面兜底/CRUD/启停/轮播展示跳转/菜单 icon 全通
- [x] 数据大盘（M3-OPS-02）：GET /admin/dashboard/stats（今日实时：活跃/新增/投稿/有效播放 + 近 7 日四线趋势补零对齐 + 平均审核时长 + 待审数，实时聚合不做 T+1 数仓）；admin 工作台数据大盘区块（4 渐变卡 + echarts 四线图 legend/tooltip/smooth + 审核时效条，5 分钟自动轮询）；E2E 实测 4 卡/图表渲染/tooltip/审核时效全通
- [x] 稿件管理页（PRD admin.md 2.4 新增 VMG-01~04）：GET /admin/videos（全状态 + 状态/分区/关键词筛选，Card 补 category_id）+ PUT /admin/videos/:bvid/status（下架/恢复 已发布↔已锁定）+ DELETE /admin/videos/:bvid（软删除，均审计留痕）；权限点 video:view/video:manage（审核主管/审核员分配）；admin 稿件管理页（筛选/表格/下架恢复删除/分页）；E2E 实测筛选/下架恢复持久化/权限按钮/分页/分区列/下拉全通

### 进行中

- [ ] staging 部署脚本（M0-ENG-13，M0 收尾项）

### 下阶段计划

- [ ] M3 增长：A/B 实验（M3-OPS-03）、ItemCF 离线计算（M3-REC-03）、多P投稿与合集（M3-CRT-05）、审计日志导出增强（SYS-06）
- [ ] M1/M2 遗留：staging 部署（M0-ENG-13）、k6 压测（REL-01）、邀请码（REL-04）
- [ ] （后置）多端：H5 剩余页、小程序、App

### 开发优先级策略（调整于 2026-07-30）

> ⭐ **优先把 Web C 端 + 管理平台做深做完，多端（H5/小程序/App）整体后置。**
> 理由：先用单一端跑通完整产品闭环与治理能力，接口/交互稳定后多端复用成本更低。

**优先队列（P0，按序推进）**：
1. 管理平台治理闭环：用户查询/封禁/禁言（M1-ADM-03）+ 用户管理页/分区管理页（M1-ADM-06）
2. Web 端 M1 扫尾：播放地址签名下发（M1-VID-05）、player 内核抽包（M1-VID-09）
3. Web 端 M2 功能：成长体系、机审合规、运营位等（按 checklist M2 非-H5 项）

**后置队列（暂缓）**：H5 端剩余页（M2-H5-04/05/06）、小程序、App、邮箱注册（M1-ACC-02）、k6 压测（REL-01）、邀请码（REL-04）。

### 阻塞与风险

| 问题 | 等级 | 处理 |
| --- | --- | --- |
| 本地开发机无 Docker，MinIO/Kafka/ES 不可用 | 中 | 存储已用本地磁盘 Storage 抽象替代；转码队列已用 DB 任务表+进程内 Worker 落地；接口层抽象保证部署环境可无缝切回 |
| 本地未安装 ffmpeg | - | ✅ 已解决：winget 安装 FFmpeg 8.1.2，路径写入 dev.yaml transcode 配置 |

## 4. 周报归档

| 周次 | 里程碑 | 摘要 | 链接 |
| --- | --- | --- | --- |
| W1 (2026-07-27) | M0 | 项目启动：仓库 + 文档系统 + PRD/架构初稿 | 本页 §3 |

## 5. 进度管理规则

1. **看板流转**：任务状态 `待办 → 开发中 → 联调 → 测试 → 完成`；单人在制品（WIP）≤ 2。
2. **周会节奏**：周一计划会（领任务）、周五站会（更新本页 + 风险上报）。
3. **延期处理**：预计延期 > 3 天须在"阻塞与风险"表报备，并给出砍范围/加资源/顺延三选一决策。
4. **验收口径**：功能完成 = 代码合并 + 自测通过 + 联调通过 + 文档更新（接口/清单勾选）。
5. **变更控制**：需求变更须更新对应 PRD 并在 [Roadmap 变更记录](/project/roadmap#_4-变更记录) 登记。

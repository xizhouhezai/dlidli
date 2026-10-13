# tasks：直播

> 对应规格：[spec](/specs/live/spec) ｜ 方案：[plan](/specs/live/plan)
> 任务编号：`{阶段}-{模块}-{序号}`；完成即勾选并追加完成日期。验证结论 ≤ 5 行（conventions V2.1），超长证据归档 `.dev-logs/liv02/`（gitignore，不入库）。

## M4（W49+）

- [ ] M4-LIV-01 RTMP 接入 + 直播转码 + HLS/FLV 分发
  - 覆盖：LIV-02（推流侧）
- [x] M4-LIV-02 直播间服务（房间/在线人数/权限）——2026-10-13 完成
  - 覆盖：LIV-02、LIV-01（资格校验逻辑）、LIV-04（房管/禁言数据与接口）
  - 实现要点（2026-10-13 细化）：迁移 0034（`live_room`/`live_whitelist`/`live_room_admin`/`live_room_mute`）；`live` 模块（handler/service/repo + router 注册）；房间状态机（空闲/直播中/已下播，spec §5A.1）；资格校验三闸（白名单→实名→粉丝≥100，91001-91014 可区分错误码）；推流密钥 crypto/rand 32 位生成、bcrypt 落库、明文一次性返回、重置即失效；在线人数 Redis 双键（online 计数 + hb:{uid} TTL 30s 心跳懒剔除）；房管任命/撤销与禁言/解禁接口（禁言执行归 LIV-03，本卡出 `IsMuted` 判定）
  - 验证结论：接口实测全生命周期（申请→开播→列表/详情→改信息→下播→重置密钥）13 用例 code 逐项符合（91003/91004/91005/91006/91007/91009/91010 拦截分支全命中）；DB 逐态核对（status/started_at/ended_at/live_cnt=0、禁言行 muted_until/reason/created_by）；3 用户进房=3、重复进房判重、离房=2、下播清零；心跳超时校准实测通过（超时后心跳 DECR+INCR 净 0）；`go vet`+`go test ./...`（25 包全绿，live 集成测试 5 个）+ api.exe 重启实测 + `pnpm docs:build` complete。未覆盖项：91002 实名拦截分支无法经真实登录用户触发（手机号登录即产生 user_auth=实名基线），以集成测试 TestApplyQualification 为证据；心跳超时后**重进房**（非心跳路径）重复计入为已知边界（过渡口径，LIV-03 WS hub 断连即离房后消失，plan §2A.3 已登记）；RTMP 连通性/真实白名单数据/多实例计数对账不在本卡。证据：`.dev-logs/liv02/`
- [x] M4-LIV-03 直播弹幕（comet 扩展房间广播）——2026-10-13 完成
  - 覆盖：LIV-12、LIV-04（禁言通道执行）
  - 实现要点（2026-10-13）：独立 live hub（plan §2B.1，不复用 danmaku hub）；迁移 0035 `live_room_msg` 历史窗口 50 条（§2B.2，下播清空）；频控 2s/去重 10s（§2B.3）；校验链 91015/91016/91017/91018 + 机审影子屏蔽；WS 心跳 ≤15s + 40s 读超时踢出 + 5000 连接上限 + 单连接顶替语义；`GET /live/rooms/{id}/ws`、`/danmaku/history` 两端点（swag 注释全）；web lab 极简测试面板 `/lab/live-danmaku-test`（LIV-06 不做正式页面）
  - 验证结论：WS 实测全项 PASS——双连接广播（B 收 A 弹幕原始帧在案）、进场消息（fan_badge）、频控 91016/去重 91018/禁言执行 91015/未进房 91017 逐 code 命中、在线计数 0→2→1→0 与被踢 40s→0 断开即离房、下播清历史；期间实测修掉 4 缺陷（回调参数错位 91007、UserID 双重序列化、hb TTL 先过期计数漏扣、LIMIT 子查询不兼容）。`go vet`+`go test ./...` 25 包全绿（新增 hub 并发回归 + 窗口/校验链集成测试）+ api 重编重启实测 + vue-tsc 过。未覆盖项：`pnpm docs:build` 被会话沙箱 EPERM 拦（esbuild service 进程 spawn，非代码缺陷，文档改动为增量 markdown）；`-race` 本机无 gcc 未跑；git 发布流程被会话沙箱拦（`.git` 写拒绝，改动滞留工作区待后续会话提交发布 v0.62.0）。证据：`.dev-logs/liv03/`
- [ ] M4-LIV-04 礼物系统（金瓜子/特效/分成）
  - 覆盖：LIV-13
- [ ] M4-LIV-05 直播截帧机审 + 快速切断
  - 覆盖：边界（spec §3）、合规（spec §4）
- [x] M4-LIV-06 Web/H5 直播间页面 + 开播管理——2026-10-13 完成
  - 覆盖：LIV-10、LIV-11、LIV-02（前端）、LIV-03、LIV-04、LIV-14
  - 实现要点（2026-10-13）：Web 直播首页 `/live`（开播中卡片 + 15s 轮询）、直播间 `/live/:id`（PlayerCore 播放器 + 演示流兜底、弹幕 WS + 本地 2s 冷却 + 进场消息、在线人数）、开播管理 `/studio/live`（申请→设置→推流密钥一次性展示与复制→开播/下播→房管/禁言管理）；H5 只读直播间 `pages/live/live`（含房间列表态）；api-client 新增 live API；后端补只读端点 `GET /live/rooms/:id/admins|mutes`（swag 全）。真实流替换点：`apps/web|apps/h5` 的 `src/config/live.ts`（demo 标志 + hlsUrl），LIV-01 后只改配置。
  - 验证结论：API 级 E2E 13 步全 PASS（申请 91001/91003/91004 拦截→设置→开播→列表出现→双账号进房 online=2→房管任命/禁言落库→WS 弹幕双向互见→禁言者发弹幕无广播→历史→解禁/移除→重置密钥 32 位→下播 status=2→列表消失→详情 91007 拦截）；期间核实 pwsh 控制台中文编码假象非产品缺陷（fetch 适配器恒 UTF-8）。`go vet`+`go test ./internal/module/live` + 双端 `vue-tsc` + `pnpm docs:build` complete 过。未覆盖项：`pnpm web:build` 的 vite 段被沙箱拦（esbuild service spawn + vite 内部 `exec('net use')` 双子进程 EPERM，管道边界，ESBUILD_WORKER_THREADS 实测无效），vue-tsc 段已过、完整 vite 构建留待可 spawn 会话复跑；页面截图/录屏不可行（同因），UI 证据为 API 级链路走查。已按 git 规范发布 **v0.63.0**（feature/m4-liv-06-live-web → develop → main → 附注 tag + release 分支均已推送，本地分支清理完毕）。证据：`.dev-logs/liv06/`

## 进度

| 里程碑 | 任务数 | 已完成 |
| --- | :-: | :-: |
| M4 | 6 | 3 |
| **合计** | **6** | **3** |

> 勾选任务后同步更新上表与 [开发进度管理](/project/progress) 的模块矩阵。

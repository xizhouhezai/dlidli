# tasks：直播

> 对应规格：[spec](/specs/live/spec) ｜ 方案：[plan](/specs/live/plan)
> 任务编号：`{阶段}-{模块}-{序号}`；完成即勾选并追加完成日期。验证结论 ≤ 5 行（conventions V2.1），超长证据归档 `.dev-logs/liv02/`（gitignore，不入库）。

## M4（W49+）

- [x] M4-LIV-01 RTMP 接入 + 直播转码 + HLS/FLV 分发 —— 2026-10-13 完成
  - 覆盖：LIV-02（推流侧）、LIV-11（拉流侧）
  - 实现要点（2026-10-13）：**SRS 5 单容器**（`server/deploy/srs/`，独立 compose project `dlidli-live`，1935 RTMP / 8180 HTTP / 1985 API）承担收流与 HLS/HTTP-FLV 转封装；后端 `live` 模块新增 `stream.go`（推流寻址与回调）+ `/live/stream/auth`（on_publish 鉴权：共享 token 常量时间比较 + 房间存在 + bcrypt `param.key` + 房间须直播中，四项任一不满足即 403 拒绝推流）+ `/live/stream/stop`（on_unpublish → 房间置已下播）；房间详情下发 `play_urls{hls,flv}`；前端（Web/H5）从演示流改为按 `play_urls.hls` 真实拉流
  - **寻址契约**（plan §2C D1/D4）：推流 `rtmp://<pushDomain>/<app>/room_<id>?key=<streamKey>`（**流名公开不含密钥**，密钥走查询串，避免播放地址外泄密钥被抢推流位）；拉流 `http://<playDomain>/<app>/room_<id>.m3u8|.flv`
  - **本轮实测修复 4 处配置缺陷（本卡的真实产出）**——这些缺陷此前全部被「配置从未被真正加载」掩盖：① compose 只挂载 `srs.conf` 未覆盖 `command`，SRS 镜像默认 CMD 读内置 `conf/docker.conf`（HTTP 8080 而非 8180）→ 配置挂了却从未生效；② `app live { }` 是 SRS 3/4 旧语法，5.x 解析器直接失败（流 app 由推流 URL 路径段决定，无需声明）；③ `http_hooks` URL 里的 `${SRS_CALLBACK_TOKEN}` 花括号被当作块开括号导致解析中断，且 SRS 对 http_hooks **不做环境变量展开**（须写字面 token）；④ `hls_path` 必须落在 `http_server.dir` 之内，写在 `/data/streams` 时切片存在但 8180 一律 404。另删去 5.x 不存在的 `hls_https`/`hls_fragment_naming` 指令、移除不受支持的 `env` 指令
  - **端到端实测（真实 SRS 5.0.225 + FFmpeg 8.1.2 推流，证据 `.dev-logs/liv01/lead-e2e/`）**：开播取密钥 → 推流被 on_publish **接受**（SRS `publish.active=True`，回调入参 `param=?key=…` 在案）→ **HLS 拉流 HTTP 200** 返回标准 master playlist（`#EXTM3U`/`#EXT-X-STREAM-INF`）→ **HTTP-FLV HTTP 200**（`Content-Type: video/x-flv`）；**鉴别器**：错误密钥推流被拒（后端 `91025 推流密钥错误` → SRS 403 → FFmpeg `I/O error`），与正确密钥形成对照；停流触发 on_unpublish（后端响应 `{"stopped":true}`）→ 房间移出直播列表。**配置合法性**用 SRS 自带校验器回归：`./objs/srs -t -c conf/srs.conf` → `syntax is ok / test is successful`
  - **闸门**：`go vet` ✅ · `go test ./...`（25 包全绿）✅ · 后端重编重启实测 ✅ · `pnpm docs:build` → `build complete` ✅ · `pnpm web:build` ✅
  - 未覆盖：**OBS 人工联调**（无 GUI，用 FFmpeg 命令行推流替代，推流协议同一）；**多码率转码集群**（plan 已列演进路径：第一期用 SRS 直出，转码集群为后续项）；**CDN 分发**；**端到端播放延迟的浏览器侧测量**（headless 浏览器在本机沙箱不可用，延迟口径见 plan §2C D6：`hls_fragment 2` + `hls_window 8` 给出 6–15s 目标的理论下界）；**浏览器 FLV 播放**（未引入 flv.js，FLV 腿以后端验证，浏览器侧走 HLS）
- [x] M4-LIV-02 直播间服务（房间/在线人数/权限）——2026-10-13 完成
  - 覆盖：LIV-02、LIV-01（资格校验逻辑）、LIV-04（房管/禁言数据与接口）
  - 实现要点（2026-10-13 细化）：迁移 0034（`live_room`/`live_whitelist`/`live_room_admin`/`live_room_mute`）；`live` 模块（handler/service/repo + router 注册）；房间状态机（空闲/直播中/已下播，spec §5A.1）；资格校验三闸（白名单→实名→粉丝≥100，91001-91014 可区分错误码）；推流密钥 crypto/rand 32 位生成、bcrypt 落库、明文一次性返回、重置即失效；在线人数 Redis 双键（online 计数 + hb:{uid} TTL 30s 心跳懒剔除）；房管任命/撤销与禁言/解禁接口（禁言执行归 LIV-03，本卡出 `IsMuted` 判定）
  - 验证结论：接口实测全生命周期（申请→开播→列表/详情→改信息→下播→重置密钥）13 用例 code 逐项符合（91003/91004/91005/91006/91007/91009/91010 拦截分支全命中）；DB 逐态核对（status/started_at/ended_at/live_cnt=0、禁言行 muted_until/reason/created_by）；3 用户进房=3、重复进房判重、离房=2、下播清零；心跳超时校准实测通过（超时后心跳 DECR+INCR 净 0）；`go vet`+`go test ./...`（25 包全绿，live 集成测试 5 个）+ api.exe 重启实测 + `pnpm docs:build` complete。未覆盖项：91002 实名拦截分支无法经真实登录用户触发（手机号登录即产生 user_auth=实名基线），以集成测试 TestApplyQualification 为证据；心跳超时后**重进房**（非心跳路径）重复计入为已知边界（过渡口径，LIV-03 WS hub 断连即离房后消失，plan §2A.3 已登记）；RTMP 连通性/真实白名单数据/多实例计数对账不在本卡。证据：`.dev-logs/liv02/`
- [x] M4-LIV-03 直播弹幕（comet 扩展房间广播）——2026-10-13 完成
  - 覆盖：LIV-12、LIV-04（禁言通道执行）
  - 实现要点（2026-10-13）：独立 live hub（plan §2B.1，不复用 danmaku hub）；迁移 0035 `live_room_msg` 历史窗口 50 条（§2B.2，下播清空）；频控 2s/去重 10s（§2B.3）；校验链 91015/91016/91017/91018 + 机审影子屏蔽；WS 心跳 ≤15s + 40s 读超时踢出 + 5000 连接上限 + 单连接顶替语义；`GET /live/rooms/{id}/ws`、`/danmaku/history` 两端点（swag 注释全）；web lab 极简测试面板 `/lab/live-danmaku-test`（LIV-06 不做正式页面）
  - 验证结论：WS 实测全项 PASS——双连接广播（B 收 A 弹幕原始帧在案）、进场消息（fan_badge）、频控 91016/去重 91018/禁言执行 91015/未进房 91017 逐 code 命中、在线计数 0→2→1→0 与被踢 40s→0 断开即离房、下播清历史；期间实测修掉 4 缺陷（回调参数错位 91007、UserID 双重序列化、hb TTL 先过期计数漏扣、LIMIT 子查询不兼容）。`go vet`+`go test ./...` 25 包全绿（新增 hub 并发回归 + 窗口/校验链集成测试）+ api 重编重启实测 + vue-tsc 过。未覆盖项：`pnpm docs:build` 被会话沙箱 EPERM 拦（esbuild service 进程 spawn，非代码缺陷，文档改动为增量 markdown）；`-race` 本机无 gcc 未跑；git 发布流程被会话沙箱拦（`.git` 写拒绝，改动滞留工作区待后续会话提交发布 v0.62.0）。证据：`.dev-logs/liv03/`
- [x] M4-LIV-04 礼物系统（金瓜子/特效/分成）——2026-10-13 完成
  - 覆盖：LIV-13
  - 实现要点（2026-10-13）：迁移 0036 五表（`melon_seed_account`/`melon_seed_log`（uk_biz_no 充值幂等）/`gift`/`gift_log`（单价·分成比例下单快照）/`gift_share_log`（uk_gift_log））；强一致 = 事务 + SELECT...FOR UPDATE 行锁（余额/流水/分成同事务，余额不可负）；分成 700‰（对齐 monetization CHG-02 70/30），月度结算归 creator（PAY-02 消费）；live hub 新增 `type=gift` 特效帧；C 端 `GET /live/gifts`、`GET /live/melon/balance`、`POST /live/rooms/{id}/gifts/{giftId}`（91019-91022）；dev 直充 `POST /admin/dev/recharge`（AdminAuth+RequirePerm config:edit，开发期临时通道，PAY-01 替换点见 plan §2C.2）；Web 礼物面板 `GiftPanel.vue` + 特效 `GiftEffects.vue`（普通弹幕区连击浮层/高级全屏 CSS 动画），swag 全量重生成
  - 验证结论：接口实测全链路——送礼×1/连击×3/全屏礼物 余额逐笔正确、`melon_seed_log`/`gift_log`/`gift_share_log` DB 逐条核对（2170+930=3100=Σtotal，70/30 逐笔符）；WS 双连接均收 `type=gift` 帧原始帧在案；余额不足 91021 且 0 流水；直充无 token/用户 token 均 401。**强一致并发**：集成测试 20 goroutine 打 600 余额恰 6 成功/14 拒绝/终 0 余额/无负数/无丢单（TestGiftConcurrentSpend）+ HTTP 接口级 20 并发同样 6/14、余额严格单调、流水 6 条；充值幂等 TestRechargeIdempotent。`go vet ./...`+`go test ./internal/module/live`（8 用例全绿）+ api 重编重启实测 + vue-tsc 过。未覆盖项：`pnpm web:build` vite 段与 `pnpm docs:build` 均被沙箱 esbuild service spawn EPERM 拦（同 LIV-03/06 边界，vue-tsc 已过、构建留待可 spawn 会话复跑）；页面级浏览器走查同因不可行，UI 证据为 API/WS 级链路走查；PAY-01 真实支付替换点=支付回调以支付单号为 biz_no 复用 RechargeTx 幂等、dev/recharge 下线（plan §2C.2）；git 发布被 `.git` 写拒绝（同 LIV-03/06），改动滞留工作区、v0.64.0 命令清单见 `.dev-logs/liv04/git-release.md`。证据：`.dev-logs/liv04/`
- [x] M4-LIV-05 直播截帧机审 + 快速切断 —— 2026-10-15 完成
  - 覆盖：边界（spec §3）、合规（spec §4）；细化见 spec §5E / plan §2D
  - 实现要点（2026-10-15）：迁移 0037（`live_frame_audit` 判定凭据 + `live_violation_log` 违规流水 + `live_room` 四列封禁字段）；`live/audit.go` 截帧 Worker（`time.Ticker` 同转码 worker 风格，dev 默认关）+ HTTP-FLV 单帧截帧（SRS 5 无截图 API）+ `contentmod.CheckImage` 画面机审 + 处置链（先落流水→SRS 踢流→封禁，`cut_mode` 回填）+ `RestoreRoom`；封禁与 `status` 正交，并以**封禁闸**接入 `AuthorizePublish`（被封禁者持正确密钥也拒推）；巡查三接口 + 机审状态接口挂 `AdminAuth`+`RequirePerm("live:audit")`，权限点已落库；错误码 91027-91030。
  - 验证结论：**真实推流端到端全链闭合**——截帧产出真 PNG 且尺寸与推流源一致（640x360）；黑帧推流 → `live_frame_audit.pass=0`（`degraded=0`）→ 违规流水 `cut_mode=1` → **SRS `/streams/` 变空 + ffmpeg 退出** → 房间 `status=2`（on_unpublish 回调）且 `ban_status=1`；封禁闸实测 403/91028（**带正确密钥**仍被拒）、重复切断 91028 幂等、未开播房间切断 91006 且**零副作用**（无流水/未封禁）、`/restore` 与重复恢复 91029 逐项符合；巡查鉴权矩阵无 token 401 / 用户 token 401 / 管理员 200 实测命中。**期间实测修掉 1 处自身严重缺陷**：`StartAuditWorker` 内联跑循环致 `router.New` 永不返回、HTTP server 从未监听（改 `go auditLoop`）；另由独立验收者提出并确认修复 1 处 spec 缺口（`CutRoom` 未校验场次状态）。闸门：`go vet` ✅ · `go test ./...`（**25 包全绿，本卡新增 23 用例，含 DB 集成**）✅ · `bin/api.exe` 重编重启实测 ✅ · `go run ./cmd/migrate`（0037）✅ · `swag init` 重生成 ✅ · `pnpm docs:build` build complete ✅ 证据：`.dev-logs/liv05/`（E2E 全文 `lead-e2e/EVIDENCE.md`，阈值标定 `recon/thresholds.md`）
  - 未覆盖：**外部图审服务未接入**（`CheckImage` 仅本地画面规则「黑屏/纯色遮挡」，语义级审核是包内既有 TODO 替换点）；**机审不可用走 fail-open（不切断仅记 `degraded`）——待替换点**，接外部图审后应改为「无法判定 ≠ 通过」+ degraded 比率告警；**多实例部署未验证**（进程内定时器会重复截帧，需分片+分布式锁）；**60 天留存无 TTL/归档作业**（仅由配置声明策略）；管理端**页面**未做（本卡只交付后端接口）；`-race` 未跑（本机无 gcc）
- [x] M4-LIV-06 Web/H5 直播间页面 + 开播管理——2026-10-13 完成
  - 覆盖：LIV-10、LIV-11、LIV-02（前端）、LIV-03、LIV-04、LIV-14
  - 实现要点（2026-10-13）：Web 直播首页 `/live`（开播中卡片 + 15s 轮询）、直播间 `/live/:id`（PlayerCore 播放器 + 演示流兜底、弹幕 WS + 本地 2s 冷却 + 进场消息、在线人数）、开播管理 `/studio/live`（申请→设置→推流密钥一次性展示与复制→开播/下播→房管/禁言管理）；H5 只读直播间 `pages/live/live`（含房间列表态）；api-client 新增 live API；后端补只读端点 `GET /live/rooms/:id/admins|mutes`（swag 全）。真实流替换点：`apps/web|apps/h5` 的 `src/config/live.ts`（demo 标志 + hlsUrl），LIV-01 后只改配置。
  - 验证结论：API 级 E2E 13 步全 PASS（申请 91001/91003/91004 拦截→设置→开播→列表出现→双账号进房 online=2→房管任命/禁言落库→WS 弹幕双向互见→禁言者发弹幕无广播→历史→解禁/移除→重置密钥 32 位→下播 status=2→列表消失→详情 91007 拦截）；期间核实 pwsh 控制台中文编码假象非产品缺陷（fetch 适配器恒 UTF-8）。`go vet`+`go test ./internal/module/live` + 双端 `vue-tsc` + `pnpm docs:build` complete 过。未覆盖项：`pnpm web:build` 的 vite 段被沙箱拦（esbuild service spawn + vite 内部 `exec('net use')` 双子进程 EPERM，管道边界，ESBUILD_WORKER_THREADS 实测无效），vue-tsc 段已过、完整 vite 构建留待可 spawn 会话复跑；页面截图/录屏不可行（同因），UI 证据为 API 级链路走查。已按 git 规范发布 **v0.63.0**（feature/m4-liv-06-live-web → develop → main → 附注 tag + release 分支均已推送，本地分支清理完毕）。证据：`.dev-logs/liv06/`

## 进度

| 里程碑 | 任务数 | 已完成 |
| --- | :-: | :-: |
| M4 | 6 | 6 |
| **合计** | **6** | **6** |

> 勾选任务后同步更新上表与 [开发进度管理](/project/progress) 的模块矩阵。

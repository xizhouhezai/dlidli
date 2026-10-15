# plan：会员与商业化

> 对应规格：[spec](/specs/monetization/spec) ｜ 技术基线：[总体架构](/architecture/overview)
> 实施状态：V3.3（2026-10-15 PAY-02 细化）；M4-PAY-01、M4-VIP-01、M4-VIP-02、M4-PAY-02 已实现。monetization 模块 4 项任务全部收口。
>
> 变更记录：
> - 2026-08-21 自 PRD 迁移为 SDD，技术预研。
> - 2026-10-15 补 PAY-01 技术决策（§2E）：渠道适配器契约与 mock 渠道、订单状态机与条件 UPDATE、回调幂等与验签、金额精度与现金/虚拟币分离、表结构（迁移 0038）、对账与异常清单、合规钩子、管理端接口、错误码 92001+。
> - 2026-10-15 补 VIP-01 技术决策（§2F）：会员状态与判定口径（不缓存）、顺延语义与订单幂等双闸、权益发放挂在支付入账事务内、套餐价格服务端计算与首月优惠资格、连续包月 CAS 抢权与 mock 扣款换入点、月度赠币幂等键、权益落地点与跨模块最小接口、表结构（迁移 0039）、接口布局、错误码 92101+。
> - 2026-10-15 补 VIP-02 技术决策（§2G）：档位配置数据驱动（迁移 0040）、鉴权位置（下发时过滤）、**签名绑定档位与用户 + 第二道闸（会员复核）**、原画路径的分辨率上限、升级兼容口径（历史签名继续可用）、4K 转码成本与运维开关、档位配置的权限点。
> - 2026-10-15 补 PAY-02 技术决策（§2H）：充电复用支付链路与「先落记录再返凭据」的时序、**组合 PaidHook**（会员 + 充电共用一个入账事务）、分成 70/30 与 LIV-04 同源且下单快照、创作者收益的现金负债建模（`creator_income`/`creator_balance`）、月结按支付时间归属且可重放、提现防超提（行锁）与防重复出款（双闸）、**频次限制必须在持锁事务内**、表结构（迁移 0041，含 `creator_settlement` 唯一键修复）、接口布局与 ID 字符串口径、错误码 92201+/92301+/92401+。

## 1. 方案概览

商业化的技术核心是支付基础设施（统一网关 + 订单状态机 + 对账），其上挂会员权益判定（清晰度/内容/身份）与充电分成结算（月度）。账户体系上现金账户与虚拟币（硬币）严格分离。

## 2. 技术决策（预研）

| 决策点 | 方向 | 理由 | 备选方案 |
| --- | --- | --- | --- |
| 支付网关 | 适配器模式统一封装支付宝/微信/内购 | 渠道可替换；业务侧无感 | 直连某一家（锁定） |
| 权益判定 | 会员状态服务端判定，清晰度在下发播放地址时鉴权（联动 [video plan](/specs/video/plan) playsign） | 权益不可前端绕过 | 前端标记（可破解） |
| 清晰度档位与签名绑定 | 档位配置表数据驱动；转码档位产物的签名覆盖**档位 + 用户**，静态入口再加会员复核；下发时按身份过滤 | 既拦「免费拿不到 4K 地址」，也拦「改 URL 参数」与「转发他人签名」（§2G.3） | 只在下发时过滤（泄漏的 URL 无法兜底）／只签路径（改档位即越权） |
| 分成结算 | 月度批处理：充电流水 → 按稿/按 UP 聚合 → 70/30 分账入 creator_settlement（复用 [creator plan](/specs/creator/plan) 结算表） | 与创作激励共用结算通道 | 独立结算系统（重复） |
| 对账 | 日对账单 + 异常订单人工处理台 | 支付合规硬要求 | 自动冲正（风险高） |
| 合规 | 增值电信许可、支付通道资质、未成年人消费限制、退款通道 | 大陆运营前提 | — |

## 3. 数据模型（预研）

```sql
order        { id, user_id, sku, amount, status(状态机), channel, paid_at }
membership   { user_id, level, expire_at, auto_renew }
charge_log   { id, user_id, up_id, amount, split_up, split_platform }
```

> 上表为 VIP/充电域的预研草案。**PAY-01 的实际数据模型见 §2E.4（迁移 0038），与上表不同名不同形**，以 §2E 为准。

## 4. 接口设计（预研）

支付下单/回调、会员状态查询、充电下单/鸣谢名单等，启动时以 OpenAPI 契约为准。**PAY-01 的实际接口清单见 §2E.8。**

## 5. 关键流程（预研）

```
下单 → 唤起支付 → 渠道回调（幂等）→ 订单终态 → 权益生效（会员/分成流水）
                ↘ 对账（日切）→ 差异订单人工处理台
```

## 6. 风险与待定项

- [ ] 支付通道资质与合规评估（启动前完成）
- [ ] 4K 转码档位成本核算（联动 [video plan](/specs/video/plan)）
- [ ] 广告系统（ADV-01）与推荐流的融合设计

## 2E. M4-PAY-01 技术决策（2026-10-15）

### 2E.1 交付形态：全量机制 + mock 渠道

**决策：机制按真实契约全量实现，渠道层用 mock 跑通全链路，真实渠道为显式换入点。**

理由：真实支付宝/微信需要支付通道资质与增值电信许可（当前不具备，spec §2.5 已写明）。若等资质再建机制，则会员/充电/直播礼物充值全部阻塞。因此把「与渠道无关的部分」（订单状态机、幂等、验签框架、对账、现金账户）按真实契约做完，把「与渠道相关的部分」（下单报文、签名算法、账单文件）隔离在一个接口之后——将来接真渠道只新增一个适配器实现，业务侧与表结构零改动。

与 LIV-04 既有实践一致：直播礼物的开发直充（`POST /admin/dev/recharge`）就是同一思路的临时通道，其 `RechargeTx` 的 `biz_no` 幂等即为本卡支付回调的替换点（spec §2.6）。

### 2E.2 渠道适配器契约

四个方法（**接口定义即契约，真实渠道实现不得改变签名**）：

| 方法 | 语义 | mock 行为 |
| --- | --- | --- |
| `CreatePayment` | 下单：返回渠道预支付凭据/唤起参数 | 生成 `trade_no`，回传「如何模拟支付」的提示串 |
| `QueryPayment` | 查单：按订单号/渠道交易号查询渠道侧状态 | 读本地 mock 交易状态表 |
| `Refund` | 退款：按订单号+金额发起退款 | 直接置为退款成功（可配置为失败以测降级） |
| `VerifyCallback` | 回调验签：校验原始报文 + 签名头，返回解析后的支付结果 | HMAC-SHA256 共享密钥校验（与 SRS 回调 token 同思路） |

- 注册表：`map[channel]Adapter`，`Register`/`Get` 两个动作；**渠道不存在**返回可区分错误码（92003）。
- **mock 验签**：`HMAC-SHA256(secret, rawBody)`，与请求头 `X-Pay-Signature`（hex）做**常量时间比较**（`hmac.Equal`），密钥来自配置 `pay.mock.secret`。选 HMAC 而非"明文 token 相等"，是因为前者能同时覆盖"密钥未泄漏时无法伪造"与"报文被篡改"两个攻击面；与 LIV-01 SRS 回调鉴权的思路一致。
- **支付宝/微信骨架**：`alipay.go` / `wechat.go` 各实现四个方法的签名骨架，方法体返回 92004「渠道未实现」并**以注释分步写明接入步骤**（凭据配置项、签名算法 RSA2/MD5-HMAC、证书管理、回调地址备案、`QueryPayment` 对账文件下载与解析入口）。**不得返回伪造成功**（spec §3 末条）。

### 2E.3 订单状态机与并发安全

状态：`0 待支付 / 1 已支付 / 2 已关闭 / 3 已退款`。迁移只允许 `0→1`、`0→2`、`1→3`。

**核心决策：一切状态迁移用条件 UPDATE，并以受影响行数作为「本次是否真正迁移」的唯一判据。**

```go
res := tx.Model(&PayOrder{}).Where("id = ? AND status = ?", id, PayStatusPending).Update("status", PayStatusPaid)
if res.RowsAffected == 0 { /* 对手已先迁移：按幂等返回，不执行入账副作用 */ }
```

理由：只写 `WHERE id=?` 再读一次状态在并发下会双写（check-then-act 竞态）。条件 UPDATE 把「判定 + 变更」压成一条语句交给 InnoDB 行锁串行化，`RowsAffected` 是数据库给出的权威裁决，比应用层加锁更可靠且不引入额外依赖（对齐 LIV-04 `SendGiftTx` 的事务 + `FOR UPDATE` 口径；此处不需要显式 `FOR UPDATE`，因为条件 UPDATE 自带行锁且无需先读）。

**入账必须在同一事务内**：条件 UPDATE 成功 → 现金账户加余额 → 写现金流水。三者同事务，任一步失败整体回滚，不会出现「订单已支付但没入账」。

**超时关单**：`expire_at` 在下单时写入（`created_at + pay.order.timeout_minutes`，默认 15）。关单不引入独立 Worker，而是**在读取订单/下单/回调三个入口调用前先执行一次惰性关单**（`UPDATE ... WHERE status=0 AND expire_at < NOW()`），理由是：dev 环境 Worker 常被关闭（LIV-05 教训），惰性关单无需进程存活即可保证「过期订单不会被支付」，而回调路径上的关单检查是抗「过期后回调改单」的关键一道。

**并发实测出的死锁与处置（2026-10-15，证据见 `.dev-logs/pay01/recon/deadlock.md`）**：

20 并发同一订单回调时，实测出现过 `Error 1213 Deadlock`。`SHOW ENGINE INNODB STATUS` 给出确凿的加锁顺序反转：

```
(1) UPDATE pay_order SET ... WHERE order_no = ? AND status = ?   -- 走 uk_order_no → PRIMARY
    HOLDS  : PRIMARY 行锁
    WAITING: idx_status_expire 行锁
(2) UPDATE pay_order SET status=2, closed_at=NOW() WHERE status = 0 AND expire_at < NOW()
    HOLDS  : idx_status_expire 行锁          -- 走 idx_status_expire → PRIMARY
    WAITING: PRIMARY 行锁
```

两条语句索引路径相反，同时执行即互为对方的等待者。处置三层：

1. **关单分批**（`closeBatchSize=200`）：单次事务持锁行数有界、窗口缩短，降低相撞概率。
2. **`retryTx` 重试**（最多 5 次，退避 `(i+1)×5ms`）：只对 1213/1205 重试。死锁是 InnoDB 的正常仲裁结果（官方处置就是重试），且**回滚是整体回滚**——失败的事务不会留下部分副作用，故重试安全；`MarkPaidTx` 靠条件 UPDATE 保证重试后要么迁移成功、要么判定「对手已生效」而幂等返回。
3. **如实记录而不宣称消除**：实测重试后**应用层 20 并发零错误**，但 InnoDB 层仍偶发仲裁死锁（5 次复现中 2 次产生新记录）——这是两条相反锁序语句的固有性质，不可能靠应用代码根除。**因此本卡的口径是「应用层不受影响、恰好一次入账」，不是「死锁已消除」**。彻底消除需把关单改成不经 `idx_status_expire` 的路径（如按 id 区间扫），属后续优化项。

### 2E.4 表结构（迁移 0038）

六张表，全 InnoDB / utf8mb4：

- `pay_order`：`order_no`（对外业务单号，唯一）、`user_id`、`amount`（BIGINT 分）、`channel`、`subject`、`status`（0/1/2/3）、`channel_trade_no`、`expire_at`、`paid_at`、`closed_at`、`refunded_at`、`refund_amount`、`client_ip`、时间戳。
  索引：`uk_order_no`、`uk_channel_trade_no`（**可空列的唯一索引**：MySQL 允许多行 NULL，未支付的订单该列为 NULL 不冲突）、`idx_user`、`idx_status_expire`（关单扫）、`idx_created`。
  **`uk_channel_trade_no` 是回调幂等的第二道闸**（第一道是条件 UPDATE）。
- `cash_account`：`user_id` 唯一、`balance`（BIGINT 分，≥0 由事务保证）、时间戳。**与 `melon_seed_account` 结构同形但语义完全独立**（现金 vs 金瓜子），不共享、不互转。
- `cash_log`：`user_id`、`delta`、`balance_after`、`biz_type`（1 充值 / 2 退款）、`biz_no`（**唯一**）、`order_no`、`remark`、时间戳。`uk_biz_no` 承载资金流水幂等（写法对齐 `melon_seed_log.uk_biz_no`）。
- `pay_callback_log`：`channel`、`raw_body`、`signature`、`sign_ok`（1/0）、`order_no`、`channel_trade_no`、`result`（业务处理结果文本）、`created_at`。**验签失败也落库**（spec §6.4 取证要求）。
- `pay_recon_daily`：`recon_date`、`channel`、`order_count`、`paid_count`、`paid_amount`、`refund_count`、`refund_amount`、`cash_log_amount`、`diff_count`、`status`（0 正常 / 1 有异常）、时间戳；`uk_date_channel` 唯一，重跑对账为 UPSERT 覆盖。
- `pay_recon_issue`：`recon_date`、`channel`、`order_no`、`issue_type`（1 金额不符 / 2 长期挂起 / 3 流水缺失 / 4 重复回调）、`detail`、`status`（0 待处理 / 1 已处理）、`handled_by`、`handled_at`、`handle_note`、`created_at`；`uk_date_order_type` 唯一（重跑不重复登记）。

### 2E.5 金额精度

DB `BIGINT`（分），Go `int64`，JSON 整数分。**全链路禁用浮点**。对账聚合用 SQL `SUM()` 回 `int64`。金额相关的除法只有分成比例（PAY-02），本卡无除法。

### 2E.6 对账口径

**决策：做「本地订单 vs 本地现金流水」的一致性核对，不做渠道账单文件解析（无渠道，且解析是换入点）。**

按「渠道 × 日期」聚合：从 `pay_order` 按 `paid_at`/`refunded_at` 落在该日聚合出订单口径，与 `cash_log` 按 `created_at` 落在该日的流水合计比对；差异按订单逐条登记到 `pay_recon_issue`。

四类异常判定：
- **金额不符**：`pay_callback_log` 中该订单成功回调的金额 ≠ `pay_order.amount`（mock/真实渠道报文金额被篡改或实现缺陷）。
- **长期挂起**：`status=0` 且 `created_at` 早于 `now - 挂起阈值`（默认 24h，可配）。
- **流水缺失**：`status=1` 但 `cash_log` 无该 `order_no` 的入账记录（理论不可能，出现即严重缺陷，必须能被查出）。
- **重复回调**：同一 `order_no` 的成功回调条数 > 1。**渠道重复投递是常态**（渠道未收到 200 就会重投），因此该项登记的是「重复投递发生了」这一事实供人工确认幂等生效，`detail` 中同时给出该订单的现金流水条数——**只有流水 > 1 条才是幂等被击穿**，若把正常重投一律判为缺陷，异常清单会被噪声淹没而使人工处理台失去意义。

重跑对账必须幂等（UPSERT + 唯一键），否则人工反复核对会污染清单。

### 2E.7 合规钩子（实现程度）

- `ConsumptionGuard` 接口：`Check(ctx, uid, amount)`，在下单与回调入账两处调用，默认实现放行。
- 退款通道：走适配器 `Refund`，管理端可发起。
- **未覆盖（必须写明）**：`user_auth` 无出生日期字段，实名判定仅「存在手机/邮箱记录」（ACC-23 未落到年龄维度），故**未成年人识别的数据基础不具备**——本卡只交付钩子与调用点，真实规则待 ACC-23 补全。**不得宣称合规达标**。

### 2E.8 API 布局

C 端（`/api/v1/pay`，登录态）：
- `POST /orders` 下单 → `{order_no, amount, channel, pay_params, expire_at}`
- `GET /orders/:orderNo` 查单（仅本人）
- `POST /orders/:orderNo/refund` 申请退款
- `GET /orders` 我的订单列表
- `POST /callback/:channel` 渠道回调（**无用户鉴权**，验签鉴权）

管理端（`/api/v1/admin/pay`，`AdminAuth` + `RequirePerm`）：
- `GET /orders` 订单查询（分条件筛选）
- `POST /orders/:orderNo/mark-paid` 手动补单
- `GET /recon/daily` 日对账单
- `GET /recon/issues` 异常订单清单
- `POST /recon/issues/:id/handle` 异常处理（备注）
- `POST /dev/pay-callback` dev 模拟回调（**与真实回调同一处理链**，spec §6.7）

权限点：`pay:view`（菜单）、`pay:order`（按钮：补单）、`pay:recon`（按钮：异常处理）——落 `builtinPermissions` 并分配给 super（super 短路放行，其余角色不默认授予财务权限）。

### 2E.9 错误码（92001+）

支付子段自 92001 起（直播段 91001-91030 已用）：

| 码 | 语义 |
| --- | --- |
| 92001 | 订单不存在 |
| 92002 | 订单状态不允许该操作（如非待支付退款、非已支付关单） |
| 92003 | 支付渠道不存在或未启用 |
| 92004 | 支付渠道未实现（真实渠道换入点，未接入） |
| 92005 | 回调验签失败 |
| 92006 | 回调报文不合法（解析失败/缺字段） |
| 92007 | 订单已过期（回调拒绝） |
| 92008 | 金额不合法（≤0 / 超出单笔上限） |
| 92009 | 退款金额不合法（> 订单金额 / ≤0） |
| 92010 | 现金账户余额不足（退款回冲场景） |
| 92011 | 对账日期不合法（未来日期/格式错） |
| 92012 | 未成年人消费受限（钩子保留，当前不触发） |

### 2E.10 未覆盖项（交付时同步登记 tasks.md）

真实支付宝/微信渠道联调与凭据、渠道账单文件下载解析、内购（App）、未成年人识别的数据基础（ACC-23）、现金→虚拟币兑换（PAY-02）、自动冲正、对账定时任务（本卡提供可手动触发的对账接口，定时化归运维配置）、支付成功后的权益发放（VIP-01 消费）。

## 2F. M4-VIP-01 技术决策（2026-10-15）

### 2F.1 会员状态存表与判定口径

**决策：独立 `membership` 表，`user_id` 唯一，判定一律 `expire_at > NOW()`，不缓存。**

不把会员标志加在 `user` 表的理由：`user` 是全站最热的行之一（昵称/头像/硬币都在这），把带**时间语义**的会员状态塞进去会让「每次读用户资料」都携带一个需要随时失效的字段；独立表还能让「会员到期扫描」「会员查询」各走自己的索引（`idx_expire`）。

判定口径的三个细节：

- **只看 `expire_at`，不看 `level`**：若两者都参与判定，就可能出现「level=1 但已过期」被判为会员的自相矛盾状态。`level` 仅作将来多档位的预留。
- **时间基准取数据库的 `NOW()`** 而非 Go 的 `time.Now()`：判定与写入必须同一时钟，否则应用与 DB 的时区/时钟漂移会让「刚过期的会员」在两者间得出相反结论。实测已确认本环境 DB `NOW()`（CST）与 Go `time.Now()`（Local）逐秒一致（`tmp_vip01_seed tzprobe`）。
- **过期用户仍返回 `expire_at`**：前端需要它显示「会员已于 X 过期」，与 `is_vip=false` 并不冲突。

### 2F.2 为什么不缓存（含备选与将来口径）

**决策：本卡不缓存会员判定。**

理由：判定本身就是一条走 `uk_user` / `idx_expire` 的点查（`expire_at > NOW()`），成本与「查一次 Redis」同量级；而缓存的代价是**刚支付完成却仍显示免费**（TTL 内的陈旧读），以及「续期/过期/退款」三条路径都要写缓存失效——对一张单行表，这个复杂度换不来收益。

**备选（将来流量需要时）**：Redis 缓存 `vip:{uid}`，**TTL ≤ 60s**，且在「权益生效事务提交后」主动失效；判定仍以 DB 为权威，缓存仅作为降级加速。选择 60s 上限的理由：会员是低频状态变更、高频读取，1 分钟的陈旧窗口在体验上不可感知，且远小于「到期前提醒」的小时级粒度。

### 2F.3 顺延语义与「同一订单只生效一次」双闸

**顺延（用户已敲定）**：

```sql
-- 已是会员：不覆盖剩余时长
UPDATE membership
SET expire_at = DATE_ADD(GREATEST(NOW(), expire_at), INTERVAL ? MONTH)
WHERE user_id = ?;
-- 已过期 / 从未开通：GREATEST 自动退化为 NOW()，即「从 now() 起算」
```

一条 SQL 同时表达两种语义，不给「判空 → 分支 → 两条 UPDATE」留出中间态。

**幂等双闸**（与 PAY-01 §2E.3 同思路，此处针对权益）：

| 闸 | 载体 | 作用 |
| :-: | --- | --- |
| ① | `pay_order` 条件 UPDATE 的 `RowsAffected` | 订单迁移只发生一次 → 权益钩子只在真正迁移那次被调用 |
| ② | `vip_order.uk_order_no` + `apply_status` 条件 UPDATE | 即使钩子被重复调用（手工补单与回调竞争等），也判定为幂等并直接返回 |

第二闸不是冗余：**手工补单（`mark-paid`）与真实回调是两条独立入口**，都可能在「订单尚未支付」的判定窗口内先后通过，第一闸无法区分。同时 `vip_coin_grant.uk_user_period` 是第三层（月度粒度，与订单粒度彼此独立）。

### 2F.4 权益发放挂在支付入账事务内（本卡最关键的结构决策）

**决策：把会员域的两个钩子注入支付域，而不是「vip 调 pay，成功后再补记权益」。**

```go
// pay/repo.go markPaidOnce（事务内）
if hook != nil {            // hook = vip.ApplyPaidHook
    if e := hook(tx, orderNo, o.UserID); e != nil { return e }  // → 整体回滚
}
```

理由：**「钱与权益要么都有、要么都没有」**。若权益发放放在事务外，一旦发放失败（会员表锁冲突、赠币唯一键意外冲突等）就会留下「已收钱但没开会员」的账实不符，而渠道**不会重投**回调，该状态无法自愈，只能人工处理。挂进事务后，权益失败即订单不迁移、不入账，渠道会按重试策略重投，系统自动收敛。

钩子接口留在 pay 侧（`pay.PaidHook`、`pay.SkuHook`），使 **pay 不 import vip**——支付域不认识会员，只认识「支付成功后要调一个函数」与「下单前要校验一个商品」。

`sku` 设计的两个细节：

- **`pay_order.sku` 必须落库**：回调侧只拿得到订单号与金额，而金额会随运营改价变化；「该开哪档会员」必须来自下单时的快照。
- **`vip_order.months` 是下单时的时长快照**：套餐可能被改下架甚至删除，已付款订单不得因「套餐没了」而无法生效。

### 2F.5 连续包月：CAS 抢权 + mock 扣款换入点

**签约表 `vip_subscription`（`user_id` 唯一）**，`next_charge_at` 是续期扫描的驱动字段。

**抢权用 CAS，不用分布式锁**：

```sql
UPDATE vip_subscription SET next_charge_at = <新值>
WHERE user_id = ? AND status = 1 AND next_charge_at = <读取值>;  -- RowsAffected=1 者得
```

多实例并发扫描时只有一个实例能推进该行，其余拿到 0 直接跳过。把「谁来做这次扣款」的仲裁交给数据库，与 PAY-01 的条件 UPDATE 口径一致，且不引入 Redis 锁的持有时序问题。

**先推进时间、再扣款**：若扣款过程崩溃，本轮不再被重复扣——**宁可少扣一次，不可重复扣一次**（对用户资金的错误方向不可接受；少扣可用人工补偿，重复扣是资金事故）。

**失败处置**：失败计数 +1，下次扣款改到**次日**（而非下个整月）——给用户留出自助修复账号的时间窗；连续失败达阈值（默认 3）自动解约，避免无限重试与骚扰。**通知简化为日志**（spec 允许），真实触达（站内信/短信）为换入点。

**mock 扣款（显式换入点）**：`mockCharge` = 「建订单 → mock 渠道回调」，**与用户手动支付复用完全相同的处理链**（验签 → 幂等 → 入账 → 顺延 → 赠币），不是旁路实现。接入真实代扣时只替换这一个函数（换成「渠道代扣 API + 代扣结果回调」），**顺延/签约推进/失败计数逻辑零改动**。

### 2F.6 权益落地与跨模块最小接口

落地两处**身份标识**（服务端判定、前端只渲染）：

| 落点 | 字段 | 说明 |
| --- | --- | --- |
| 视频评论 | `user.is_vip` / `user.nickname_color` | 评论读模型一次批量查询，避免 N+1 |
| 直播弹幕 | `is_vip` / `nickname_color` | **实时帧与历史回放口径一致** |

跨模块接法：**定义最小只读接口（`VipColors([]int64) (map[int64]string, error)`）由消费方各自声明，router 注入实现**。interaction / live **不 import 会员域**——读模型只需知道「谁是会员、昵称什么颜色」，不应因此与会员域互相依赖；这也让测试可以注入替身。

**降级口径**：会员标识查询失败 → 降级为无徽章并**继续返回内容**。徽章是展示增强，不是数据完整性的一部分；若查询失败就报错，会员服务的一次抖动会放大成「全站评论/弹幕不可用」。

**清晰度与专属内容**：本卡**只下发权益位**（`rights.max_resolution` / `hdr` / `exclusive`），实际鉴权归 VIP-02（清晰度）与 video 模块（专属稿件的稿件级权限，**只留钩子**）。

### 2F.7 月度赠币幂等

**幂等键：`vip_coin_grant.uk_user_period (user_id, period_key)`**，`period_key = 'YYYY-MM'`（本地时区自然月，按运营日历）。

流程：先 `INSERT` 台账行 → 撞唯一键即视为「本月已发」，返回 `(false, nil)` **而非错误**——定时补发与运维手工补发会反复调用，把它当错误会让正常路径天天报错。插入成功后才 `UPDATE user.coin` 并写 `coin_log`，三者同一事务。

**为什么还要月度扫描**：即时发放只在开通/续期那一刻触发；「月中开通 + 月月连续续费」的用户在跨月时不会有任何开通事件，单靠即时发放会漏发。月赠币是**订阅权益**而非首购礼品，故补一条「生效中 ∩ 本月未发」的扫描（`LEFT JOIN` 一次 SQL，带 `LIMIT` 上限避免全表拉进内存）。

**硬币 ≠ 金瓜子**：赠币写 `user.coin` / `coin_log`；**绝不触碰** `melon_seed_account` / `melon_seed_log`（金瓜子是直播礼物资产）。实测以「三表零写入」为断言（见 `.dev-logs/vip01/db/`）。

### 2F.8 表结构（迁移 0039）

五表 + `pay_order` 一列（全 InnoDB / utf8mb4）：

- `pay_order.sku`（VARCHAR(64)，新增列）：商品标识，会员卡为 `vip_plan.code`。**PAY-01 的订单表不含商品标识**，而回调侧必须能判定「该开哪档权益」，故本卡补列。
- `vip_plan`：`code`（唯一）、`name`、`months`、`price`（分）、`original_price`、`first_month_price`、`auto_renew`、`sort`、`status`（0 下架/1 上架）。索引 `uk_code`、`idx_status_sort`。
- `membership`：`user_id`（唯一）、`level`、`expire_at`、`auto_renew`、`started_at`、`total_months`、`last_order_no`。索引 `uk_user`、`idx_expire`。
- `vip_order`：`order_no`（**唯一**，权益幂等第二闸）、`user_id`、`plan_code`、`months`（快照）、`apply_status`、`expire_before`、`expire_after`、`applied_at`。索引 `uk_order_no`、`idx_user`。
- `vip_subscription`：`user_id`（唯一）、`plan_code`、`channel`、`status`、`sign_at`、`cancel_at`、`next_charge_at`、`last_charge_at`、`fail_count`。索引 `uk_user`、`idx_status_next`（续期扫描）。
- `vip_coin_grant`：`user_id` + `period_key`（**唯一**）、`amount`、`coin_log_id`、`source`。索引 `uk_user_period`、`idx_period`。

### 2F.9 API 布局

C 端（`/api/v1/vip`）：
- `GET /plans` 套餐列表（**匿名可访问**：价格是公开信息）
- `GET /status` 会员状态与权益位（**匿名可访问**：游客必须能拿到「不是会员」而不是 401，否则首页无法决定是否显示会员入口）
- `POST /orders` 会员下单（登录态；**金额由服务端计算**）
- `GET /subscribe` 我的签约（登录态）
- `POST /subscribe/cancel` 解约（登录态）
- `POST /dev/renew-once` 手动触发一次续期扫描（**与定时 Worker 调用同一函数**，spec §7.4）
- `POST /dev/grant-monthly` 手动触发月度补发扫描（同上）

管理端（`/api/v1/admin/vip`，`AdminAuth` + `RequirePerm`）：
- `GET /plans` 套餐列表（**含下架项**，便于运营看到全貌）
- `POST /plans` 套餐配置（新建 / 改价 / 上下架）
- `GET /members` 会员查询（按用户 / 只看生效中 / 分页）
- `POST /members/:uid/grant-coins` 手工补发当月硬币（**幂等**，重复点不会多发）

权限点：`vip:view`（菜单）、`vip:plan`（改价/上下架）、`vip:grant`（补发硬币）——落 `builtinPermissions`，默认只授予 super（super 短路放行），其余角色不默认授予财务相关权限。

### 2F.10 错误码（92101+）

会员子段自 92101 起（支付段 92001-92012 已用）：

| 码 | 语义 |
| --- | --- |
| 92101 | 会员套餐不存在或已下架 |
| 92102 | 该套餐不支持连续包月 |
| 92103 | 订单不是会员商品 |
| 92104 | 该订单的会员权益已生效（幂等） |
| 92105 | 无有效的连续包月签约 |
| 92106 | 已有生效中的连续包月签约 |
| 92107 | 本月会员硬币已发放（幂等） |
| 92108 | 自动续期扣款失败 |
| 92109 | 会员时长不合法 |

### 2F.11 未覆盖项（交付时同步登记 tasks.md）

**装扮/皮肤**（专属头像框/空间皮肤：无前端设计资源，权益位恒 `false`）、**HDR 与 4K 清晰度的实际鉴权**（本卡只下发权益位，消费方为 VIP-02）、**专属内容的稿件级鉴权**（涉及 video 模块权限，只留 `exclusive` 权益位与钩子）、**真实免密代扣**（资质不具备，`mockCharge` 为换入点）、**到期提醒的真实触达**（只落地 `expiring_soon` 判定位，站内信/短信/推送为换入点）、**自动续期 Worker 的定时化部署验证**（dev 默认关闭；进程内 ticker 已在测试与手动入口验证，多实例下的生产验证归运维配置）、**会员缓存的 Redis 加速**（§2F.2 决策为不缓存）。

## 2G. M4-VIP-02 技术决策（2026-10-15）

### 2G.1 档位编号与配置数据驱动（迁移 0040）

**决策：编号沿用 `video_stream.quality` 既有约定（0 原画 / 360 / 720 / 1080 / 2160），档位参数与开关落配置表 `video_quality_tier`。**

编号不改的理由：`quality` 是**已有数据的一部分**（存量 `video_stream` 行、`transcode_job` 行、前端 `StreamItem.quality`、鸿蒙端换源逻辑都按它取值），重新编号等于一次数据迁移 + 全端改造，收益为零。`0001_init` 的列注释本来就写着 `360/720/1080/2160`，本卡只是把注释兑现。

初始五档的**参数逐项取自原 `transcoder.go` 的硬编码 map**（360/720 一字不差），故既有转码行为不变；1080 为 V1 档位（免费上限），2160 为 4K：

| 档位 | 标签 | 目标高度 | 码率参数 | 会员限定 | 默认排程 |
| :-: | --- | :-: | --- | :-: | :-: |
| 0 | 原画 | — | — | 否 | **否**（原画直出，不转码） |
| 360 | 360P | 360 | 500k/600k/1000k/96k | 否 | 是 |
| 720 | 720P | 720 | 2000k/2400k/4000k/128k | 否 | 是 |
| 1080 | 1080P | 1080 | 4000k/4800k/8000k/192k | 否 | 是 |
| 2160 | 4K | 2160 | 16000k/19200k/32000k/256k | **是** | **否** |

`transcode_enabled` 与 `member_only` **刻意分开**：前者是「投稿时是否排这个档位的转码任务」，后者是「谁能播这个档位」。4K 默认 `transcode_enabled=0` 而 `member_only=1`——档位**存在且可鉴权**，但不默认给每篇投稿都跑一遍（成本），由运营开启。

**配置读侧用原子快照**（`qualitytier.Store` + `atomic.Pointer`）：下发过滤与签名校验**每个播放请求**都要查档位，加锁读不划算；配置改动是低频运营动作，整体重建快照成本可忽略。video 域与 `/static` 门禁**共享同一个 Store**，故管理端改配置后两侧同时生效（实测：改 `member_only` 后免费用**同一份**请求立即看到/看不到 4K，无需重启）。

### 2G.2 鉴权位置：下发时过滤（服务端）

**决策：在视频详情/分P列表下发流列表时按身份过滤——不是「下发全部再由前端挑」，也不是「播放时才拦」。**

结构上这是一个 `Service.signedPlayURL` + 过滤的收口点，两处调用（详情、分P列表）都过同一个判定函数。关键设计是**观看者身份走 ctx**（`video.WithViewer`）而不是给 `Parts`/`PublicDetail` 加参数：这两个读模型被 banner/collection/admin 等多处复用，改公开签名会波及一批无关调用点，而「谁在看」本来就是请求作用域的事实。handler 从 `optionalAuth` 中间件注入的 `CtxUserID` 取 uid 放进 ctx；**未登录 = uid 0 = 按免费**。

### 2G.3 签名绑定档位与用户（本卡的安全核心）

**决策：转码档位产物的播放签名覆盖「路径 + 过期时间 + 档位 + 用户」，静态入口再加会员复核。**

历史 `playsign` 只签路径，故「改 URL 就完成越权」：把请求里的档位参数从 360 改成 2160，路径与签名都没变。因此门禁签名必须覆盖档位号；**同时把用户 ID 也签进去**——否则免费用户可以把会员拿到的 4K 签名串直接贴到自己的 URL 上（签名与档位都对，但它不是发给你的）。绑定 uid 后转发即失效。

档位取自**路径**（`videos/hls/<视频>/<分P>/<档位>/...`），不是取自请求参数：这样「参数档位」与「签名覆盖的档位」天然是同一个值，参数侧无从伪造。请求里的 `q` 参数只是回带值，必须与路径一致才继续。

**第二道闸（会员复核）**：签名只证明「这份 URL 是签发给你的某个档位」，不证明「你现在仍是会员」。故静态入口拿到合法门禁签名后，若该档位在**当前配置**里是 `member_only`，再按当前会员状态判一次——会员到期后旧签名立即失效（实测：档位改为不限会员后**同一签名**立即放行，改回后立即拒绝）。

两处 fail-closed：会员判定**出错**按非会员处理；门禁依赖**未装配**时退化为升级前的仅签名口径（此时档位配置根本未加载，也就没有「会员限定档位」这个概念，不会把静态服务打挂）。

### 2G.4 原画路径：只拦 2160 档是不够的

**决策：新增 `video_stream.height` 列记录流的实际画面高度，原画行按源文件真实高度判定。**

只拦「2160 转码档」会留一条通路：**原画（quality=0）直出的是上传的源文件，4K 源的原画行本身就是 4K**。故 `height` 必填：
- 原画行 = 源文件真实高度（转码时 ffprobe 探测后回填；`0` 表示未探测）；
- 转码行 = `min(源高度, 档位目标高度)`（不做无收益的放大）。

**探测时机选在转码时**（而非投稿请求里）：投稿是同步请求，探测会加一次 ffprobe 往返；而转码本来就持有本地路径与 ffprobe 调用，零额外依赖。边界如实写明：**转码总开关关闭时没有任务运行，原画行高度保持 0**（此时也不存在任何档位产物，行为与升级前一致）。

**`height=0` 的历史行只按档位配置判定、不套用分辨率上限**——这是兼容性的关键：升级前落库的所有行都是 0，若把 0 当「未知=放行」以外的任何含义（比如当 0 高度 = 低于任何上限而放行，或当异常而拒绝），既有 0/360/720 数据的可播性都会变。取「只按 member_only 判定」即等价于升级前的行为。

### 2G.5 4K 转码成本与运维开关

**决策：4K 档位默认不排程（`transcode_enabled=0`），由运营在管理端开启；转码单任务超时从 30min 放宽到 2h。**

4K 单机 CPU 转码远慢于 360/720，30min 的超时对整片 4K 不够；放宽到 2h 仍是**有界**超时（不会永久占位），且 panic 自隔离与重试链不变。

**实测到的成本口径（本机，3 秒 3840×2160 源，`libx264` ultrafast 源 + 默认 preset 转码）**：1080 档 0.9s、2160 档 4.1s。按比例外推整片 4K 是**小时级**，只能由运维决定开哪些档——这正是把 `transcode_enabled` 做成运营项而不是代码常量的原因。**本卡不以整片 4K 转码为验收口径**（spec §8.7 已登记）。

顺带修正一处既有实现风险：原来用 `scale=-2:<h>` 让 ffmpeg 自算宽度，**部分低码率/奇数宽高比测试源在取整后会得到奇数宽度，libx264 直接报 -22**（与「不指定 profile」同一个踩坑来源）。改为**按源宽高比显式换算宽度并置偶数**，已知宽高比时用 `scale=w:h,setsar=1`（`setsar=1` 防源带非方形像素时被拉伸）；宽度未知时回落到既有的 `scale=-2:` 写法。

**同档位不同画幅的落盘隔离**：`w` 既可能是 1920（1080 源）也可能是 3840（4K 源降采样），故输出目录名由 `videos/hls/<视频>/<分P>/<档位>` 变为 `.../<档位>/<宽度>`，避免「先转 4K、换源再转 4K」互相覆盖（同一 `(video, part, quality)` 唯一键下是同一行但内容不同）。探测失败时保留旧目录名，向后兼容。

### 2G.6 表结构（迁移 0040）

两处变更，全 InnoDB / utf8mb4：

- `video_stream` 补列 `height INT NOT NULL DEFAULT 0`（实际画面高度，0=未探测；**不改任何既有列**）。
- `video_quality_tier` 新表：`quality`（唯一）、`label`、`height`、`v_bits`/`max_rate`/`buf_size`/`a_bits`、`member_only`、`transcode_enabled`、`sort`、时间戳。索引 `uk_quality`、`idx_sort`。
  **不建外键、不改 `video_stream.quality` 的类型**（`SMALLINT` 已够放 2160，且 `0001_init` 的列注释早写明这四个档位）。

回滚（`0040_*.down.sql`）只删表与列，**不删已转出的 1080/2160 产物行与磁盘目录**——它们是合法产物，重新升级即恢复鉴权。

### 2G.7 升级兼容口径（历史签名）

**决策：非会员限定档位接受历史口径签名（仅路径），会员限定档位一律要求门禁签名。**

若一律要求门禁签名，部署瞬间所有在播的非限档位 URL 会集体 403，且 **6h 签名 TTL 内无法自愈**——新地址要等前端重新拉详情才有，已在播放器里的会中断。对一次权益功能上线而言这是不可接受的可用性代价。

不削弱安全性的三点（spec §8.5 逐条写明）：① 免费上限的第一道闸是**下发过滤**，签名只是 URL 泄漏时的兜底；② 非限档位本就没有「改档位即越权」的可图之处；③ **档位被运营改为会员限定后，该兼容口当场关闭**——历史签名立即失效（第二道闸按当前配置判定，实测覆盖）。

反向也一样成立：`playsign.Verify`（旧函数）**语义一字不变**，故线上既有 URL 与既有单测零影响；扩展只体现在新增的 `QueryGated`/`VerifyGated` 上。

### 2G.8 接口与权限点

C 端（**沿用既有路径，仅新增可选鉴权**）：
- `GET /videos/{bvid}` 与 `GET /videos/{bvid}/parts` 挂 `optionalAuth`：不拦游客，但登录时把 uid 注入 ctx，供档位过滤使用（未登录按免费）。

管理端（`/api/v1/admin/videos`，`AdminAuth` + `RequirePerm`）：
- `GET /quality-tiers` 全档位配置（含未启用排程的档位，便于运营看到全貌）—— 权限点 `video:view`
- `PUT /quality-tiers/:quality` 改开关/码率/会员限定/排序 —— 权限点 `video:tier`

权限点落 `builtinPermissions`（`video:tier` 为 `video:view` 下的 button 型），默认只授予 super。**查询与改配置分离**：能看到档位不等于能改「谁能看多清晰」。

管理端放在**独立方法** `RegisterAdminRoutes` 而不是塞进 C 端的 `RegisterRoutes`：权限检查器（`RequirePerm(adminSvc.HasPerm, ...)`）在 router 里的构建时机**晚于**稿件域装配（video 在 `adminSvc` 之前创建），硬塞进去要么改装配顺序、要么在闭包里捕获尚为 nil 的依赖。分开注册即在依赖就绪后调（与 pay/vip 的 `RegisterAdminRoutes` 同一口径）。

### 2G.9 未覆盖项（交付时同步登记 tasks.md）

**HDR**（无源与播放链路）、**真实 4K 转码量级**（本卡以 3 秒样片实测链路，整片耗时/存储/带宽属部署核算）、**CDN 档位分发**（鉴权在应用层，CDN 回源隔离属部署配置）、**前端档位标签的浏览器实测**（本会话构建被沙箱拦，见 tasks 证据）、**既有历史素材的真实播放回归**（以既有真实 HLS 产物 + 历史签名口径断言，未做浏览器播放走查）。

## 2H. M4-PAY-02 技术决策（2026-10-15）

### 2H.1 交付形态：全量机制 + mock 出款通道

真实出款（打款到支付宝/银行卡）与真实渠道收款一样需要支付通道资质，当前不具备。
故与 PAY-01 同口径：**观众侧充电复用 mock 渠道跑通全链路**，**创作者侧打款走 mock 出款通道**。
状态机、冻结/扣减、打款流水、幂等闸全部按真实契约实现，通道调用点是**单一换入点**
（`Repo.PayoutWithdrawTx`）。

### 2H.2 金额与账户：现金，不是虚拟币

- 观众支付走 PAY-01 的 `pay_order` + `cash_account`（现金口径，spec §6.1）。
- 创作者收益是**平台对创作者的现金负债**：`creator_income`（明细）+ `creator_balance`（余额）
  + `creator_balance_log`（流水）。
- **为什么不复用 `cash_account`**：那是**消费者现金钱包**（用户自己充进来的钱），
  创作者收益是**平台欠创作者的钱**，两者法律与对账含义完全不同；混在一张表里会让
  「用户退款」与「平台应付」互相污染。也**不写硬币/金瓜子**（那些是站内虚拟币，见过硬约束实测验证）。

### 2H.3 时序：先落充电记录，再返回支付凭据

「已支付但无充电记录」必须结构上不可达。实现上不引入跨域分布式事务，而是**排定顺序**：

1. 服务端算价 → 校验 → 调 `pay.CreateOrder` 拿订单与支付凭据；
2. **落 `charge_log`（status=0 待支付）**；
3. 只有第 2 步成功，才把 `pay_params` 返回给客户端。

客户端在拿到凭据前既不知道订单号也无法完成支付，mock 渠道也不存在「未收款先回调」，
故「钱到了但没有充电记录」不可达。**代价**：第 2 步失败会留下一条待支付订单，
由支付域既有的惰性关单（15 分钟）回收——无资金占用、无收益错记，可接受的残留
（这比引入分布式事务或跨域「两阶段提交」简单得多，也符合 YAGNI）。

### 2H.4 分成发放的接法：**组合 PaidHook**（本卡最关键的结构决策）

pay 域只有**单个** `PaidHook` 槽位（VIP-01 已占用）。若直接把槽位换给充电，
会员权益会静默失效；若让充电在事务外补记，则失去「钱与分成原子」。
故 router 装配时注入**组合钩子**：会员与充电各自按订单号**认领自己的订单**、
互不干扰，且都在同一入账事务内：

```go
paySvc.SetPaidHook(func(tx *gorm.DB, orderNo string, userID int64) error {
    if err := vipSvc.ApplyPaidHook(tx, orderNo, userID); err != nil { return err }
    return chargeSvc.ApplyPaidHook(tx, orderNo, userID)
})
```

**为什么充电不复用 `sku` 台账**（VIP-01 用 `vip_order` 记「下单时长快照」）：
`pay_order.sku` 是 `VARCHAR(64)`，而充电的非资金字段（`anonymous` / `message` / 目标稿件）
需要独立结构；且 `skuHook` 同样只有单槽且已被会员占用。充电的「下单快照」直接落在
`charge_log`（含 `share_rate` 比例快照）本身就是台账——回调按 `order_no` 反查即可识别，
比再插一层 sku 映射更少间接。故 `charge` **不参与 sku 体系**，`CreateCharge` 传 `sku=""`。

### 2H.5 月度结算：按支付时间归属 + 可重放

- 归属：`paid_at` 落在 `[月初, 次月初)` 即归该月（跨月订单自然落在真实发生的月份）。
- 幂等：`creator_charge_settlement.uk_user_month` + **按整月重算并覆盖**。
  注意**不能只聚合「待结算」的明细**：那样重跑会返回 0/0，与已生成的结算单不一致，
  使人无法判断「重跑是否算出同一个数」（本卡第一版即如此，已修正）。
- 复用 `creator_settlement` 承载「按稿聚合」（`source='charge'`），与创作激励共用结算通道
  （plan §23 的既定决策），不另起一套结算体系。

### 2H.6 表结构（迁移 0041）

新增 8 张表 + 扩展 1 张表：

| 表 | 作用 | 关键唯一键 |
| --- | --- | --- |
| `charge_tier` | 档位配置（金额分；出厂 5/10/30/50 + 自定义） | `uk_code` |
| `charge_log` | 充电记录（下单落行、支付置 1；比例快照） | `uk_order_no`、`uk_charge_no` |
| `creator_income` | 分成明细（现金负债，幂等键 `source_type+source_id`） | `uk_source` |
| `creator_balance` | 创作者可用/冻结余额 | `uk_user` |
| `creator_balance_log` | 余额流水（幂等 + 守恒取证） | `uk_biz_no` |
| `withdraw_order` | 提现单（状态机 0/2/3/4/5） | `uk_withdraw_no` |
| `withdraw_payout_log` | 打款流水（防重复出款闸 ②） | `uk_withdraw_no` |
| `creator_charge_settlement` | 月结结算单 | `uk_user_month` |
| `creator_settlement`（扩展） | 加 `source`/`charge_count`/`settlement_id` | **改** `uk_date_video` → `uk_source_date_user_video` |

**必须改 `creator_settlement` 唯一键（本卡实测发现的缺陷）**：原
`uk_date_video(settle_date, video_id)` 缺 `user_id`。空间页充电的 `video_id=0`，
于是两个创作者在同一月的行会**互相覆盖**——第二个创作者的 `ON DUPLICATE KEY UPDATE`
命中第一个创作者的行，金额被改写而 `user_id` 仍是前者的，造成**静默错账**
（实测：92710003 的行被 92710004 的 21000 分覆盖，且行数少一条）。
改为 `(settle_date, user_id, video_id, source)` 后：对创作激励语义完全不变
（一个稿件只属于一个创作者，`(date,video)` 已隐含 `user_id`），对充电分成则正确区分创作者。
**up/down 均已验证可往返**（v41 ⇄ v40，`dirty=0`）。

### 2H.7 提现：防超提、防重复出款、频次必须在锁内

- **防超提**：`ApplyWithdrawTx` 在事务内 `SELECT ... FOR UPDATE` 行锁余额 →
  校验 → 可用转冻结 → 写流水 → 落单。与 LIV-04 `SendGiftTx` 同写法（spec 明确要求同口径）。
  实测：余额 100 分、20 并发各提 10 → **恰 10 笔成功、余额归零、无超提**。
- **频次必须在同一事务内判定（本卡实测修正的设计缺陷）**：第一版把「24h 内已提几次」
  放在事务外预检，并发请求会同时读到旧计数而**全部通过**，日限形同虚设。
  移入持锁事务后，20 并发在日限 3 下**恰成功 3 笔**（新增用例 `TestConcurrentWithdrawRespectsDailyLimit` 固化）。
  事务外仍保留一次快速拒绝，只为省掉无谓事务；**判定权威在锁内**。
- **防重复出款双闸**：① 条件 UPDATE `status 2→4` + `RowsAffected`；
  ② `withdraw_payout_log.uk_withdraw_no` 唯一键。重复打款 → 92303，打款流水恒为 1 条。
- **重复审核不会二次解冻**：同样靠条件 UPDATE；否则余额会被凭空增加（最危险的资金缺陷）。
- 收款账户**脱敏**后存储与返回（只留首尾各 2 位）。

### 2H.8 API 布局

C 端（`/api/v1/charge`，`optionalAuth`；写接口叠加登录+限流）：

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| GET | `/tiers` | 档位列表（匿名可读） |
| GET | `/thanks` | 鸣谢名单（匿名可读；匿名条目不含身份） |
| POST | `/orders` | 发起充电（复用 PAY-01 下单） |
| GET | `/mine` | 我的充电记录 |
| GET | `/balance` | 创作者收益账户（现金负债口径） |
| POST | `/withdraws` | 发起提现申请 |
| GET | `/withdraws` | 我的提现记录 |
| GET | `/settlements` | 我的结算单 |

管理端（`/api/v1/admin/charge`，`AdminAuth` + `RequirePerm`）：
`GET /charges`（`charge:view`）· `GET /settlements`（`charge:view`）·
`POST /settlements/run`（`charge:settle`）· `GET /withdraws`（`charge:view`）·
`POST /withdraws/:withdrawNo/audit`（`charge:audit`）·
`POST /withdraws/:withdrawNo/payout`（`charge:payout`）。

**手动补单复用 PAY-01 的 `POST /admin/pay/orders/:orderNo/mark-paid`**（`pay:order`）：
补单与真实回调走同一入账链，故补单同样落充电记录状态与分成（已用集成用例固化）。

**ID 传输口径**：请求体的 64 位 ID 同时接受 JSON 数字与字符串
（`handler.idArg`）。**真实前端必须传字符串**——雪花 ID 超过 2^53，
JS 端 `JSON.parse` 会丢精度（本卡 E2E 实测：驱动器按字符串传 `anchor_id`，
第一版只接受数字导致全部下单落到 10002「参数不合法」）。

### 2H.9 错误码（92201+ / 92301+ / 92401+）

分段依据：91001-91030 直播、92001-92012 支付、92101-92109 会员均已占用。
各子段留足余量。

| 码 | 语义 |
| --- | --- |
| 92201 | 充电档位不存在或已下架 |
| 92202 | 充电金额不合法（低于下限/超上限） |
| 92203 | 不能给自己充电 |
| 92204 | 充电对象（UP 主/稿件）不存在 |
| 92205 | 充电记录不存在 |
| 92206 | 充电订单不合法（预留） |
| 92207 | 充电留言过长（>200 字） |
| 92301 | 提现单不存在 |
| 92302 | 提现金额不合法（低于起提额/超可用余额） |
| 92303 | 提现单状态不允许该操作（含重复审核/重复打款） |
| 92304 | 提现过于频繁（超当日次数上限） |
| 92305 | 出款通道未接入（真实出款换入点，预留） |
| 92306 | 创作者余额不足 |
| 92307 | 收款账户信息不合法 |
| 92401 | 结算月份不合法 |
| 92402 | 结算单不存在（预留） |

### 2H.10 未覆盖项（交付时同步登记 tasks.md）

**真实出款通道**（mock 打款为换入点）、**真实渠道充电联调与凭据**（无资质）、
**税务合规与个税代扣**（无税务资质与规则输入，不得宣称达标）、**自动打款/出款限额与重试**（人工触发）、
**提现单主动撤回**、**直播礼物分成的月结接入**（`creator_income.source_type=2` 已预留字段，
但 LIV-04 的礼物分成目前只落 `gift_share_log`，本卡未改动其归属，避免跨卡未定论的口径变更）、
**多实例下的月结并发**（幂等设计已保证可重放，但未在真实多实例部署验证）、
**前端页面与端侧接入**（本卡只交付后端接口与 OpenAPI，前端充电/提现页面属后续卡）。

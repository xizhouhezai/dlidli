# plan：会员与商业化

> 对应规格：[spec](/specs/monetization/spec) ｜ 技术基线：[总体架构](/architecture/overview)
> 实施状态：V3.1（2026-10-15 VIP-01 细化）；M4-PAY-01、M4-VIP-01 已实现。其余卡片（VIP-02、PAY-02）仍为预研方向。
>
> 变更记录：
> - 2026-08-21 自 PRD 迁移为 SDD，技术预研。
> - 2026-10-15 补 PAY-01 技术决策（§2E）：渠道适配器契约与 mock 渠道、订单状态机与条件 UPDATE、回调幂等与验签、金额精度与现金/虚拟币分离、表结构（迁移 0038）、对账与异常清单、合规钩子、管理端接口、错误码 92001+。
> - 2026-10-15 补 VIP-01 技术决策（§2F）：会员状态与判定口径（不缓存）、顺延语义与订单幂等双闸、权益发放挂在支付入账事务内、套餐价格服务端计算与首月优惠资格、连续包月 CAS 抢权与 mock 扣款换入点、月度赠币幂等键、权益落地点与跨模块最小接口、表结构（迁移 0039）、接口布局、错误码 92101+。

## 1. 方案概览

商业化的技术核心是支付基础设施（统一网关 + 订单状态机 + 对账），其上挂会员权益判定（清晰度/内容/身份）与充电分成结算（月度）。账户体系上现金账户与虚拟币（硬币）严格分离。

## 2. 技术决策（预研）

| 决策点 | 方向 | 理由 | 备选方案 |
| --- | --- | --- | --- |
| 支付网关 | 适配器模式统一封装支付宝/微信/内购 | 渠道可替换；业务侧无感 | 直连某一家（锁定） |
| 权益判定 | 会员状态服务端判定，清晰度在下发播放地址时鉴权（联动 [video plan](/specs/video/plan) playsign） | 权益不可前端绕过 | 前端标记（可破解） |
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

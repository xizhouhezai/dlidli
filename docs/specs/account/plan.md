# plan：账号体系

> 对应规格：[spec](/specs/account/spec) ｜ 技术基线：[后端架构](/architecture/backend) · [数据模型](/architecture/data-model) · [前端架构](/architecture/frontend)
> 实现位置：`server/internal/module/account`（含 profile / govern / growth 子域）

## 1. 方案概览

账号模块提供"凭据认证 → 会话管理 → 资料 → 成长体系"四层能力。认证支持多种登录方式（手机验证码 / 邮箱密码 / 密码 / 微信预留），凭据与用户主表分离；会话采用双令牌（短 Access + 可吊销 Refresh）；成长体系以规则引擎 + 流水表实现经验/等级/硬币，触发点埋入登录、观看、投稿、互动各链路。

## 2. 技术决策

| 决策点 | 决策 | 理由 | 备选方案 |
| --- | --- | --- | --- |
| 会话机制 | JWT Access（2h）+ Refresh 随机串（30d，Redis 存储可吊销），刷新时轮换双令牌 | 无状态校验 + 可吊销折中；多端统一 | 服务端 session（扩展性差）；单一长令牌（无法吊销） |
| 凭据模型 | `user`（主表）与 `user_auth`（凭据）分离，按 `identity_type + identifier` 唯一 | 多登录方式可扩展（手机/邮箱/微信） | 单表多列（方式固定，难扩展） |
| 密码存储 | bcrypt 加盐 | 行业标准 | — |
| 图形验证码 | 自研 SVG（crypto/rand 随机码 + Redis 5min 一次性），密码登录强制校验 | 零外部依赖；点击刷新/失败自动换 | 第三方验证码服务 |
| 短信 | 当前 mock（dev 返回 debug_code），真实短信服务待接入 | 内测优先；接口层已隔离 | 直接接云服务商 |
| 经验/等级 | growth 规则引擎：5 类经验来源 + Redis 每日去重限量 + `exp_log` 流水 + Lv0-Lv6 阈值重算 | 防刷可审计；升级即时 | 定时批量计算（延迟高） |
| 硬币 | MySQL 事务强一致（发放/消费）+ `coin_log` 流水 | 防并发超扣；幂等 | Redis 计数异步落库（最终一致，不适合资产） |
| 处罚执行 | account govern 层统一处罚（禁言/封禁/到期懒解除），登录封号拦截、发言链路禁言拦截 | 处罚一处生效全局 | 各模块自行判断（易漏） |

## 3. 数据模型

全局见 [数据模型](/architecture/data-model)（`user` / `user_auth`）。模块私有：

```sql
-- 经验流水（0012 迁移）
exp_log    { id, user_id, source, exp, created_at }          -- source：登录/观看/投稿/弹幕/评论
-- 硬币流水
coin_log   { id, user_id, delta, reason, created_at }        -- 注册送 5 / 每日首登 +1 / 投币扣减
```

Redis：`sess:{refresh_token}`（会话/30d）、`rl:{scene}:{uid|ip}`（短信与登录限频）、growth 每日去重键。

## 4. 接口设计

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| POST | /api/v1/auth/sms-code | 发送短信验证码（60s 冷却） |
| POST | /api/v1/auth/login/{sms\|password} | 验证码 / 密码登录（注册登录一体） |
| POST | /api/v1/auth/refresh ｜ /auth/logout | 令牌轮换 / 登出吊销 |
| GET/PUT | /api/v1/users/me | 资料读取/编辑 |
| POST | /api/v1/users/me/avatar | 头像上传（裁剪前端完成，机审后生效） |
| GET | /api/v1/growth/summary ｜ /growth/exp-logs | 等级/今日任务聚合 ｜ 经验明细分页 |
| GET | /api/v1/users/me/coin-logs | 硬币明细 |
| GET/PUT | /api/v1/users/me/youth-mode | 青少年模式开关 |

> 完整契约以 swaggo 生成的 OpenAPI 为准（`server/docs/`）。

## 5. 关键流程

手机号验证码登录（对应 ACC-01）：

```
输入手机号 → 图形验证码（风控触发时）→ 发送短信（60s 冷却，10 分钟有效）
→ 校验验证码 → 已注册？登录 ：自动注册（默认昵称 dli_xxxx）→ 颁发双令牌
```

经验触发点接入：登录（+5/日）、有效观看（+5/日）、投稿发布（+10，日 2 次）、发弹幕/发评论（+1，日 20 次）——均经 Redis 每日去重/限量后写 `exp_log` 并重算等级。

## 6. 风险与待定项

- [ ] 真实短信服务接入（当前 mock）
- [ ] 邮箱注册/激活（M1-ACC-02 未实现）
- [ ] 微信登录（随 V2 小程序）
- [ ] 在线设备管理/踢出（ACC-06 P1）
- [ ] 设备指纹与异地登录识别（ACC-41 细化）

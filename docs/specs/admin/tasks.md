# tasks：内容审核与管理后台

> 对应规格：[spec](/specs/admin/spec) ｜ 方案：[plan](/specs/admin/plan)
> 任务编号：`{阶段}-{模块}-{序号}`；完成即勾选并追加完成日期；括号补注实现要点与验证结论。

## M1（W5-W12）

- [x] M1-ADM-01 后端：管理员账号/登录（独立令牌与 C 端隔离；role 字段预留，完整 RBAC 待后续；默认账号 admin/admin123 首启自建） `2026-07-29`
  - 覆盖：ADMU-01、ADMU-02、ADMU-03（基础）
- [x] M1-ADM-02 后端：稿件审核队列/通过/驳回（必填原因）+ 审计日志（认领机制、锁定操作待后续） `2026-07-29`
  - 覆盖：ADT-01、ADT-03、ADT-05
- [x] M1-ADM-03 后端：用户查询/封禁/禁言（0010 迁移 user 加 muted_until/banned_until；account govern 层处罚+到期懒解除；admin 查询/处罚接口+审计；发言链路（评论/弹幕/动态/转发/投稿）禁言拦截+登录封号拦截；E2E 全绿） `2026-07-31`
  - 覆盖：UGV-01、UGV-02
- [x] M1-ADM-04 后端：敏感词库管理（CRUD + 热加载）（sensitive_word 表 + admin CRUD 接口 + moderate 包线程安全动态词库，启动加载/增删后即时热刷无需重启；E2E 验证加词→动态拦截→删词→放行） `2026-07-30`
  - 覆盖：OPS-05
- [x] M1-ADM-05 后台前端：登录 + 审核工作台（视频预览/通过/驳回原因；已拆为独立应用 apps/admin，dev :5175，与 C 端完全分离部署） `2026-07-29`
  - 覆盖：ADT-01、ADT-02、ADT-03（前端）
- [x] M1-ADM-06 后台前端：用户管理页、分区管理页（用户管理页随 ADM-03；敏感词库管理页随 ADM-04；分区管理页：video 层分区 CRUD + RBAC category:view/edit 权限点（归运营角色）+ admin 分区页（一/二级分组展示+增删改+启停，有子分区/稿件时禁删）；E2E 全绿） `2026-07-31`
  - 覆盖：UGV-01/02（前端）、OPS-01
- [x] M1-ADM-07 后台 UI 参考 vue-vben-admin 改造（AdminLayout 深色侧边栏分组菜单 + 顶栏面包屑 + 用户下拉；路由重组为 layout 子路由；新增 Dashboard 工作台（欢迎条+4统计卡+快捷入口）；三页去 header 卡片化；MingCute 图标） `2026-07-31`
  - 覆盖：—（工程，后台信息架构优化）

### RBAC 与系统管理（扩展需求，见 [spec §5-6](/specs/admin/spec)）

- [x] M2-RBAC-01 后端：RBAC 数据模型（admin_role/permission/user_role/role_permission 迁移）+ 内置角色初始化（0011 迁移 4 表 + admin_user 加 nickname/last_login_at；seedRBAC 幂等 upsert 11 权限点 + 6 内置角色，默认 admin 绑 super） `2026-07-31`
  - 覆盖：PERM-01~04（数据基座）、预设角色矩阵（spec §5.4）
- [x] M2-RBAC-02 后端：鉴权中间件（令牌→角色→权限集合，按 code 校验）+ 当前用户权限/菜单下发接口（middleware.RequirePerm + /me/permissions，super 短路放行；E2E 验证无权限访问返 10004） `2026-07-31`
  - 覆盖：PERM-05
- [x] M2-RBAC-03 后端：管理员账号 CRUD/启停/重置密码（不能停用/删除自己，登录写 last_login_at） `2026-07-31`
  - 覆盖：ADMU-02、ADMU-03、ADMU-04
- [x] M2-RBAC-04 后端：角色 CRUD + 分配权限（内置角色禁删/权限锁定）；权限点列表 `2026-07-31`
  - 覆盖：ROLE-01、ROLE-02、ROLE-03、ROLE-04
- [x] M2-RBAC-05 后台前端：账号管理页 + 角色管理页（el-tree 权限树勾选）+ v-perm 权限指令/动态菜单过滤（permissionStore 加载 /me/permissions；E2E 验证 moderator 只见工作台/用户管理） `2026-07-31`
  - 覆盖：PERM-06、ADMU-01（前端）、ROLE-01~03（前端）
- [x] M2-RBAC-06 权限点管理（PERM-01~04）：后端权限点 CRUD 接口（repo/service/handler，code唯一、button必挂合法menu、有子节点或被角色引用禁删）+ 前端权限管理页（页面权限menu/按钮权限button 两级分组展示+增删改）；新增 permission:view/edit 权限点（遵治理规范）；GET /permissions 降为仅登录（与角色分配树共用）写操作需 permission:edit；E2E 全绿（新建menu/button、重复code拒、非法父拒、删有子/被引用拒） `2026-07-31`
  - 覆盖：PERM-01、PERM-02、PERM-03、PERM-04
- [x] M2-SYS-01 后端+前端：审计日志中心（查询/筛选/导出）（audit_log 表已有全量写入；新增列表查询（操作者/动作/对象类型/日期范围筛选 + 中文映射）+ CSV 导出（UTF-8 BOM，最多 1 万条）；权限点 audit:view/audit:export；admin 审计日志页（筛选栏 + 表格 + 导出 + 分页）；E2E 实测 110 条记录/筛选/导出 CSV 全通） `2026-08-04`
  - 覆盖：SYS-01、SYS-06（CSV 部分）
- [x] M2-SYS-02 系统配置项集中管理（键值 + 热更新）+ 数据字典维护（0015 迁移 system_config 表 + data_dict 表（种子：举报类型/处罚动作/审核动作 3 组 12 项）；配置 CRUD（键唯一、业务侧 GetConfig 按键热读取）+ 字典 CRUD（类型+值唯一）；权限点 config:view/edit、dict:view/edit；admin 系统配置页（表格 CRUD 弹窗）+ 数据字典页（分组 tag 展示 + CRUD）；E2E 实测配置增改删/字典三组展示全通） `2026-08-04`
  - 覆盖：SYS-02、SYS-03

### 机审与合规（AUD）

- [x] M2-AUD-01 图像/文本内容安全 API 接入（稿件/评论/弹幕/资料）（新建 contentmod 机审包：敏感词热加载词库 + 广告/联系方式规则正则（手机号/QQ/微信/外链），统一接入评论/弹幕/动态/转发/投稿五处调用点，命中影子屏蔽或拒发；预留外部内容安全 API 接入点；bvid 补 Decode） `2026-08-03`
  - 覆盖：ADT-06（机审列）、消费点 CMT-08 / DM-24 / DYN-02 / ACC-42
- [x] M2-AUD-02 机审分级策略（低风险抽检/高风险全审）（video 表加 risk_level：投稿时标题/简介机审命中 → 高风险、新账号（注册<7 天）→ 中风险、默认低；审核队列按风险降序排列 + 列表返回 risk_level） `2026-08-03`
  - 覆盖：ADT-06（分级策略）、ADT-01（风险排序）
- [x] M2-AUD-03 举报体系（全对象类型 + 后台处理）（0013 迁移 report 表；report 模块：C 端 POST /reports（视频/评论/弹幕/动态/用户 + 6 类举报类型 + 防重复），后台 /admin/reports 队列（状态筛选/对象摘要/举报人）与处理（忽略/删除内容/删除并处罚（禁言或封禁）/结果站内通知反馈举报人）；权限点 report:view/handle（moderator 角色分配）；web 举报入口全覆盖（播放页视频/评论区一级+楼中楼/空间页用户/动态页），admin 举报处理页；E2E 实测提交/防重/删除/通知全通） `2026-08-03`
  - 覆盖：ADT-10
- [x] M2-AUD-04 青少年模式（时长/内容池/入口弹窗）（user 表加 youth_mode；GET/PUT /users/me/youth-mode 开关持久化；设置页青少年模式卡片开关 + 每日 40 分钟本地计时提醒（localStorage 按日累计，到时弹窗提醒）） `2026-08-03`
  - 覆盖：ACC-30（跨模块，见 [account spec](/specs/account/spec)）

## M3（W25-W48）

### 运营平台（OPS）

- [x] M3-OPS-01 Banner/推荐位/热搜配置系统（Banner 部分：0018 迁移 banner 表；banner 模块（公开 GET /banners image 空回退稿件封面 + admin CRUD bvid 校验）；权限点 banner:view/banner:edit（operator 角色）；admin Banner 配置页（预览/标题/跳转/排序/启停/CRUD 弹窗）；首页轮播接入（配置优先，右侧 2×2 网格最热填充不空白，无配置回退最热前 8）；AdminLayout 菜单分组/路由映射补 banner；E2E 实测封面兜底/CRUD/启停/轮播展示跳转/菜单 icon 全通） `2026-08-04`
  - 覆盖：OPS-02
- [x] M3-OPS-02 数据大盘（DAU/播放/投稿/审核时效）（GET /admin/dashboard/stats：今日实时 4 指标（活跃/新增/投稿/有效播放）+ 近 7 日四线趋势补零对齐 + 平均审核时长（TIMESTAMPDIFF）+ 待审数；admin 工作台数据大盘区块：4 渐变卡 + echarts 四线图（legend/tooltip/smooth）+ 审核时效条，5 分钟自动轮询；E2E 实测 4 卡/图表渲染/tooltip/审核时效全通） `2026-08-05`
  - 覆盖：OPS-10
- [x] M3-OPS-03 A/B 实验分流框架（0020 迁移 experiment 表；abtest 模块：按 uid 哈希稳定分流（FNV-1a(target:uid) % 100 < ratio → B 组，无需用户表字段）；admin CRUD（experiment:view/edit 权限，operator 分配）；admin 实验管理页（列表/新建/编辑/启停/删除，ratio slider）；真实场景接入：推荐策略变体（target=recommend，variant_b=hot_only 时退化为纯热度榜、hybrid 默认混合召回）；E2E 实测：同用户两次请求分流稳定、A 组含个性化召回/B 组纯热度、停用后回默认策略、admin CRUD 全通） `2026-08-05`
  - 覆盖：OPS-06
- [x] M3-VMG-01 稿件管理页（VMG-01~04；原开发清单未编号，迁移时补号）（GET /admin/videos（全状态列表 + 状态/分区/关键词筛选，Card 补 category_id）+ PUT /admin/videos/:bvid/status（已发布↔已锁定 下架/恢复）+ DELETE /admin/videos/:bvid（软删除，均审计留痕 video_offline/video_restore/video_delete）；权限点 video:view（menu）/video:manage（button，审核主管/审核员分配）；admin 稿件管理页（筛选栏/表格/下架恢复删除/分页）；E2E 实测列表筛选/下架恢复持久化/权限按钮/分页/分区列/下拉全通） `2026-08-05`
  - 覆盖：VMG-01、VMG-02、VMG-03、VMG-04
- [x] M3-SYS-01 列表数据导出（SYS-06）（原 progress 周志记录、开发清单漏登，迁移补录）：admin 用户/稿件/举报三列表导出 CSV（复用筛选、UTF-8 BOM、上限 10000）；前端三页"导出 CSV"按钮；实测行数与页面一致、Excel 可直接打开 `2026-08-10`
  - 覆盖：SYS-06

## 进度

| 里程碑 | 任务数 | 已完成 |
| --- | :-: | :-: |
| M1 | 7 | 7 |
| M2 | 12 | 12 |
| M3 | 5 | 5 |
| **合计** | **24** | **24** |

> 勾选任务后同步更新上表与 [开发进度管理](/project/progress) 的模块矩阵。

# 数据模型设计

> 状态：草案（MVP 范围核心表）｜ 更新日期：2026-07-28

## 1. ER 总览（核心域）

```
user ──1:N── video ──1:N── video_part（分P，V2）
 │             │
 │             ├──1:N── danmaku
 │             ├──1:N── comment ──自关联── comment(回复)
 │             └──1:N── user_action(点赞/投币/收藏 明细)
 ├──M:N── user (relation 关注)
 ├──1:N── favorite_folder ──M:N── video
 ├──1:N── dynamic(动态) 
 └──1:N── notification
```

## 2. 核心表结构（MVP）

### 2.1 用户域

```sql
-- 用户主表
CREATE TABLE `user` (
  `id`            BIGINT UNSIGNED PRIMARY KEY,          -- 雪花 ID（对外 UID）
  `nickname`      VARCHAR(24) NOT NULL,
  `avatar`        VARCHAR(255) NOT NULL DEFAULT '',
  `signature`     VARCHAR(200) NOT NULL DEFAULT '',
  `gender`        TINYINT NOT NULL DEFAULT 0,           -- 0保密 1男 2女
  `level`         TINYINT NOT NULL DEFAULT 0,           -- Lv0-6
  `exp`           INT NOT NULL DEFAULT 0,
  `coin`          INT NOT NULL DEFAULT 0,               -- 硬币（分单位可选）
  `status`        TINYINT NOT NULL DEFAULT 0,           -- 0正常 1禁言 2封禁 3注销
  `created_at`    DATETIME NOT NULL,
  `updated_at`    DATETIME NOT NULL,
  UNIQUE KEY `uk_nickname` (`nickname`)
);

-- 认证凭据（与主表分离，便于多种登录方式）
CREATE TABLE `user_auth` (
  `id`            BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  `user_id`       BIGINT UNSIGNED NOT NULL,
  `identity_type` TINYINT NOT NULL,                     -- 1手机 2邮箱 3微信
  `identifier`    VARCHAR(128) NOT NULL,                -- 手机号(加密)/邮箱/openid
  `credential`    VARCHAR(128) NOT NULL DEFAULT '',     -- bcrypt 密码；三方为空
  UNIQUE KEY `uk_type_identifier` (`identity_type`, `identifier`),
  KEY `idx_user` (`user_id`)
);
```

### 2.2 视频域

```sql
CREATE TABLE `video` (
  `id`            BIGINT UNSIGNED PRIMARY KEY,          -- 雪花 ID
  `bvid`          VARCHAR(16) NOT NULL,                 -- 对外展示 ID（DV 开头）
  `user_id`       BIGINT UNSIGNED NOT NULL,             -- UP 主
  `title`         VARCHAR(80) NOT NULL,
  `cover`         VARCHAR(255) NOT NULL,
  `description`   VARCHAR(2000) NOT NULL DEFAULT '',
  `category_id`   INT NOT NULL,                         -- 分区
  `tags`          JSON,                                  -- 标签数组
  `copyright`     TINYINT NOT NULL DEFAULT 1,           -- 1自制 2转载
  `duration`      INT NOT NULL DEFAULT 0,               -- 秒
  `status`        TINYINT NOT NULL,   -- 0草稿 1上传中 2转码中 3待审 4已发布 5驳回 6锁定 7删除
  `reject_reason` VARCHAR(255) DEFAULT NULL,
  `published_at`  DATETIME DEFAULT NULL,
  `created_at`    DATETIME NOT NULL,
  `updated_at`    DATETIME NOT NULL,
  `version`       INT NOT NULL DEFAULT 0,               -- 乐观锁
  UNIQUE KEY `uk_bvid` (`bvid`),
  KEY `idx_user_status` (`user_id`, `status`),
  KEY `idx_category_pub` (`category_id`, `status`, `published_at`)
);

-- 转码产物
CREATE TABLE `video_stream` (
  `id`         BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  `video_id`   BIGINT UNSIGNED NOT NULL,
  `quality`    SMALLINT NOT NULL,                       -- 360/720/1080/2160
  `format`     VARCHAR(8) NOT NULL DEFAULT 'hls',
  `play_path`  VARCHAR(255) NOT NULL,                   -- m3u8 对象存储 key
  `file_size`  BIGINT NOT NULL DEFAULT 0,
  UNIQUE KEY `uk_video_quality` (`video_id`, `quality`)
);

-- 计数表（与主表分离，高频更新）
CREATE TABLE `video_stat` (
  `video_id`   BIGINT UNSIGNED PRIMARY KEY,
  `view_cnt`   BIGINT NOT NULL DEFAULT 0,
  `like_cnt`   BIGINT NOT NULL DEFAULT 0,
  `coin_cnt`   BIGINT NOT NULL DEFAULT 0,
  `fav_cnt`    BIGINT NOT NULL DEFAULT 0,
  `danmaku_cnt` BIGINT NOT NULL DEFAULT 0,
  `comment_cnt` BIGINT NOT NULL DEFAULT 0,
  `share_cnt`  BIGINT NOT NULL DEFAULT 0
);
```

### 2.3 互动域

```sql
-- 行为明细（点赞/投币/收藏统一模型，按 user_id 分表预留）
CREATE TABLE `user_action` (
  `id`         BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  `user_id`    BIGINT UNSIGNED NOT NULL,
  `oid`        BIGINT UNSIGNED NOT NULL,                -- 对象 ID（视频/评论）
  `obj_type`   TINYINT NOT NULL,                        -- 1视频 2评论 3弹幕 4动态
  `action`     TINYINT NOT NULL,                        -- 1赞 2币 3藏
  `extra`      INT NOT NULL DEFAULT 0,                  -- 投币数量等
  `created_at` DATETIME NOT NULL,
  UNIQUE KEY `uk_user_obj_action` (`user_id`, `oid`, `obj_type`, `action`)
);

CREATE TABLE `comment` (
  `id`         BIGINT UNSIGNED PRIMARY KEY,
  `oid`        BIGINT UNSIGNED NOT NULL,                -- 视频/动态 ID
  `obj_type`   TINYINT NOT NULL DEFAULT 1,
  `user_id`    BIGINT UNSIGNED NOT NULL,
  `root_id`    BIGINT UNSIGNED NOT NULL DEFAULT 0,      -- 一级评论 ID（楼中楼）
  `parent_id`  BIGINT UNSIGNED NOT NULL DEFAULT 0,      -- 直接回复对象
  `content`    VARCHAR(1000) NOT NULL,
  `like_cnt`   INT NOT NULL DEFAULT 0,
  `reply_cnt`  INT NOT NULL DEFAULT 0,
  `status`     TINYINT NOT NULL DEFAULT 0,              -- 0正常 1影子屏蔽 2删除
  `is_top`     TINYINT NOT NULL DEFAULT 0,
  `created_at` DATETIME NOT NULL,
  KEY `idx_oid_root` (`oid`, `obj_type`, `root_id`, `created_at`)
);

CREATE TABLE `danmaku` (
  `id`         BIGINT UNSIGNED PRIMARY KEY,
  `video_id`   BIGINT UNSIGNED NOT NULL,
  `user_id`    BIGINT UNSIGNED NOT NULL,
  `content`    VARCHAR(100) NOT NULL,
  `time_ms`    INT NOT NULL,                            -- 视频内毫秒
  `mode`       TINYINT NOT NULL DEFAULT 1,              -- 1滚动 2顶部 3底部
  `color`      INT UNSIGNED NOT NULL DEFAULT 16777215,  -- RGB
  `font_size`  TINYINT NOT NULL DEFAULT 25,
  `status`     TINYINT NOT NULL DEFAULT 0,
  `created_at` DATETIME NOT NULL,
  KEY `idx_video_time` (`video_id`, `time_ms`)
);
```

### 2.4 关系域（V1）

```sql
CREATE TABLE `relation` (
  `id`          BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  `user_id`     BIGINT UNSIGNED NOT NULL,               -- 关注者
  `target_id`   BIGINT UNSIGNED NOT NULL,               -- 被关注者
  `type`        TINYINT NOT NULL DEFAULT 1,             -- 1关注 2特别关注 3拉黑
  `created_at`  DATETIME NOT NULL,
  UNIQUE KEY `uk_user_target` (`user_id`, `target_id`),
  KEY `idx_target` (`target_id`, `type`)                -- 查粉丝列表
);
```

## 3. ID 与分片策略

- 主键：雪花 ID（时间有序，利于分页与迁移）；对外视频用 `bvid` 短编码防遍历。
- 分表预留：`user_action`、`comment`、`danmaku`、`notification` 按对象/用户 hash 分表（基因法），MVP 单表 + 中间层路由抽象。
- 冷热分离：90 天前弹幕/播放日志归档 ClickHouse/对象存储（V2）。

## 4. Redis Key 规范

| 用途 | Key | 类型 / TTL |
| --- | --- | --- |
| 会话 | `sess:{refresh_token}` | string / 30d |
| 视频计数 | `stat:video:{id}` | hash / 常驻+定时回写 |
| 点赞去重 | `like:v:{video_id}` | set（大 V 视频换 bloom）/ 常驻 |
| 弹幕段 | `dm:{video_id}:{seg}` | zset / 7d 滑动 |
| Feed 收件箱 | `feed:inbox:{uid}` | zset(1000 截断) / 30d |
| 未读数 | `unread:{uid}` | hash / 常驻 |
| 限流 | `rl:{scene}:{uid|ip}` | string+EXPIRE |

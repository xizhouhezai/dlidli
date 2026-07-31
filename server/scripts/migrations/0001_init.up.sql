-- 0001_init: MVP 核心表（对齐 docs/architecture/data-model.md）

-- ============ 用户域 ============
CREATE TABLE `user` (
  `id`         BIGINT UNSIGNED PRIMARY KEY COMMENT '雪花ID，对外UID',
  `nickname`   VARCHAR(24)  NOT NULL COMMENT '昵称，唯一',
  `avatar`     VARCHAR(255) NOT NULL DEFAULT '' COMMENT '头像URL',
  `signature`  VARCHAR(200) NOT NULL DEFAULT '' COMMENT '个性签名',
  `gender`     TINYINT      NOT NULL DEFAULT 0 COMMENT '0保密 1男 2女',
  `level`      TINYINT      NOT NULL DEFAULT 0 COMMENT '等级 Lv0-6',
  `exp`        INT          NOT NULL DEFAULT 0 COMMENT '经验值',
  `coin`       INT          NOT NULL DEFAULT 0 COMMENT '硬币',
  `status`     TINYINT      NOT NULL DEFAULT 0 COMMENT '0正常 1禁言 2封禁 3注销',
  `created_at` DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  UNIQUE KEY `uk_nickname` (`nickname`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='用户主表';

CREATE TABLE `user_auth` (
  `id`            BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  `user_id`       BIGINT UNSIGNED NOT NULL,
  `identity_type` TINYINT      NOT NULL COMMENT '1手机 2邮箱 3微信',
  `identifier`    VARCHAR(128) NOT NULL COMMENT '手机号(加密)/邮箱/openid',
  `credential`    VARCHAR(128) NOT NULL DEFAULT '' COMMENT 'bcrypt密码，三方为空',
  `created_at`    DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
  UNIQUE KEY `uk_type_identifier` (`identity_type`, `identifier`),
  KEY `idx_user` (`user_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='认证凭据';

-- ============ 分区 ============
CREATE TABLE `category` (
  `id`         INT AUTO_INCREMENT PRIMARY KEY,
  `parent_id`  INT          NOT NULL DEFAULT 0 COMMENT '0为一级分区',
  `name`       VARCHAR(32)  NOT NULL,
  `sort`       INT          NOT NULL DEFAULT 0,
  `status`     TINYINT      NOT NULL DEFAULT 0 COMMENT '0启用 1停用',
  UNIQUE KEY `uk_parent_name` (`parent_id`, `name`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='内容分区';

INSERT INTO `category` (`parent_id`, `name`, `sort`) VALUES
(0, '动画', 1), (0, '游戏', 2), (0, '科技数码', 3), (0, '知识', 4),
(0, '生活', 5), (0, '美食', 6), (0, '音乐', 7), (0, '舞蹈', 8),
(0, '影视', 9), (0, '体育', 10), (0, '时尚', 11), (0, '娱乐', 12);

-- ============ 视频域 ============
CREATE TABLE `video` (
  `id`            BIGINT UNSIGNED PRIMARY KEY COMMENT '雪花ID',
  `bvid`          VARCHAR(16)   NOT NULL COMMENT '对外展示ID',
  `user_id`       BIGINT UNSIGNED NOT NULL COMMENT 'UP主',
  `title`         VARCHAR(80)   NOT NULL,
  `cover`         VARCHAR(255)  NOT NULL DEFAULT '',
  `description`   VARCHAR(2000) NOT NULL DEFAULT '',
  `category_id`   INT           NOT NULL,
  `tags`          JSON          NULL COMMENT '标签数组',
  `copyright`     TINYINT       NOT NULL DEFAULT 1 COMMENT '1自制 2转载',
  `duration`      INT           NOT NULL DEFAULT 0 COMMENT '时长(秒)',
  `status`        TINYINT       NOT NULL DEFAULT 0 COMMENT '0草稿 1上传中 2转码中 3待审 4已发布 5驳回 6锁定 7删除',
  `reject_reason` VARCHAR(255)  NULL,
  `published_at`  DATETIME      NULL,
  `created_at`    DATETIME      NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at`    DATETIME      NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  `version`       INT           NOT NULL DEFAULT 0 COMMENT '乐观锁',
  UNIQUE KEY `uk_bvid` (`bvid`),
  KEY `idx_user_status` (`user_id`, `status`),
  KEY `idx_category_pub` (`category_id`, `status`, `published_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='稿件';

CREATE TABLE `video_stream` (
  `id`        BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  `video_id`  BIGINT UNSIGNED NOT NULL,
  `quality`   SMALLINT     NOT NULL COMMENT '360/720/1080/2160',
  `format`    VARCHAR(8)   NOT NULL DEFAULT 'hls',
  `play_path` VARCHAR(255) NOT NULL COMMENT 'm3u8对象存储key',
  `file_size` BIGINT       NOT NULL DEFAULT 0,
  UNIQUE KEY `uk_video_quality` (`video_id`, `quality`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='转码产物';

CREATE TABLE `video_stat` (
  `video_id`    BIGINT UNSIGNED PRIMARY KEY,
  `view_cnt`    BIGINT NOT NULL DEFAULT 0,
  `like_cnt`    BIGINT NOT NULL DEFAULT 0,
  `coin_cnt`    BIGINT NOT NULL DEFAULT 0,
  `fav_cnt`     BIGINT NOT NULL DEFAULT 0,
  `danmaku_cnt` BIGINT NOT NULL DEFAULT 0,
  `comment_cnt` BIGINT NOT NULL DEFAULT 0,
  `share_cnt`   BIGINT NOT NULL DEFAULT 0
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='稿件计数';

CREATE TABLE `transcode_job` (
  `id`         BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  `video_id`   BIGINT UNSIGNED NOT NULL,
  `quality`    SMALLINT NOT NULL,
  `status`     TINYINT  NOT NULL DEFAULT 0 COMMENT '0待处理 1处理中 2成功 3失败',
  `retry_cnt`  TINYINT  NOT NULL DEFAULT 0,
  `error_msg`  VARCHAR(500) NOT NULL DEFAULT '',
  `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  KEY `idx_video` (`video_id`),
  KEY `idx_status` (`status`, `updated_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='转码任务';

-- ============ 互动域 ============
CREATE TABLE `user_action` (
  `id`         BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  `user_id`    BIGINT UNSIGNED NOT NULL,
  `oid`        BIGINT UNSIGNED NOT NULL COMMENT '对象ID',
  `obj_type`   TINYINT  NOT NULL COMMENT '1视频 2评论 3弹幕 4动态',
  `action`     TINYINT  NOT NULL COMMENT '1赞 2币 3藏',
  `extra`      INT      NOT NULL DEFAULT 0 COMMENT '投币数量等',
  `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  UNIQUE KEY `uk_user_obj_action` (`user_id`, `oid`, `obj_type`, `action`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='行为明细';

CREATE TABLE `comment` (
  `id`         BIGINT UNSIGNED PRIMARY KEY COMMENT '雪花ID',
  `oid`        BIGINT UNSIGNED NOT NULL COMMENT '视频/动态ID',
  `obj_type`   TINYINT       NOT NULL DEFAULT 1,
  `user_id`    BIGINT UNSIGNED NOT NULL,
  `root_id`    BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '一级评论ID',
  `parent_id`  BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '直接回复对象',
  `content`    VARCHAR(1000) NOT NULL,
  `like_cnt`   INT           NOT NULL DEFAULT 0,
  `reply_cnt`  INT           NOT NULL DEFAULT 0,
  `status`     TINYINT       NOT NULL DEFAULT 0 COMMENT '0正常 1影子屏蔽 2删除',
  `is_top`     TINYINT       NOT NULL DEFAULT 0,
  `created_at` DATETIME      NOT NULL DEFAULT CURRENT_TIMESTAMP,
  KEY `idx_oid_root` (`oid`, `obj_type`, `root_id`, `created_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='评论';

CREATE TABLE `danmaku` (
  `id`         BIGINT UNSIGNED PRIMARY KEY COMMENT '雪花ID',
  `video_id`   BIGINT UNSIGNED NOT NULL,
  `user_id`    BIGINT UNSIGNED NOT NULL,
  `content`    VARCHAR(100) NOT NULL,
  `time_ms`    INT          NOT NULL COMMENT '视频内毫秒',
  `mode`       TINYINT      NOT NULL DEFAULT 1 COMMENT '1滚动 2顶部 3底部',
  `color`      INT UNSIGNED NOT NULL DEFAULT 16777215 COMMENT 'RGB',
  `font_size`  TINYINT      NOT NULL DEFAULT 25,
  `status`     TINYINT      NOT NULL DEFAULT 0,
  `created_at` DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
  KEY `idx_video_time` (`video_id`, `time_ms`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='弹幕';

-- ============ 关系域 ============
CREATE TABLE `relation` (
  `id`         BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  `user_id`    BIGINT UNSIGNED NOT NULL COMMENT '关注者',
  `target_id`  BIGINT UNSIGNED NOT NULL COMMENT '被关注者',
  `type`       TINYINT  NOT NULL DEFAULT 1 COMMENT '1关注 2特别关注 3拉黑',
  `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  UNIQUE KEY `uk_user_target` (`user_id`, `target_id`),
  KEY `idx_target` (`target_id`, `type`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='用户关系';

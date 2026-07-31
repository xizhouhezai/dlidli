-- 0003_admin: 后台管理员账号（RBAC 完整角色体系随 M1-ADM 演进）
CREATE TABLE `admin_user` (
  `id`         BIGINT UNSIGNED PRIMARY KEY COMMENT '雪花ID',
  `username`   VARCHAR(32)  NOT NULL,
  `password`   VARCHAR(128) NOT NULL COMMENT 'bcrypt',
  `role`       VARCHAR(16)  NOT NULL DEFAULT 'reviewer' COMMENT 'super/reviewer/operator',
  `status`     TINYINT      NOT NULL DEFAULT 0 COMMENT '0启用 1停用',
  `created_at` DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
  UNIQUE KEY `uk_username` (`username`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='后台管理员';

-- 审核操作日志（ADT-05 审计留痕）
CREATE TABLE `audit_log` (
  `id`         BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  `admin_id`   BIGINT UNSIGNED NOT NULL,
  `action`     VARCHAR(32)  NOT NULL COMMENT 'approve/reject/lock...',
  `obj_type`   VARCHAR(16)  NOT NULL COMMENT 'video/user/comment',
  `oid`        BIGINT UNSIGNED NOT NULL,
  `detail`     VARCHAR(500) NOT NULL DEFAULT '',
  `created_at` DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
  KEY `idx_admin` (`admin_id`, `created_at`),
  KEY `idx_obj` (`obj_type`, `oid`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='后台操作审计';

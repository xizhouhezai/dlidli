-- 0011_rbac: RBAC 权限体系（角色/权限点/用户-角色/角色-权限）
CREATE TABLE admin_role (
  id         BIGINT UNSIGNED PRIMARY KEY COMMENT '雪花ID',
  code       VARCHAR(32)  NOT NULL COMMENT '角色编码',
  name       VARCHAR(32)  NOT NULL COMMENT '角色名',
  remark     VARCHAR(128) NOT NULL DEFAULT '',
  is_builtin TINYINT      NOT NULL DEFAULT 0 COMMENT '1=内置角色不可删',
  created_at DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
  UNIQUE KEY uk_code (code)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='后台角色';

CREATE TABLE admin_permission (
  id        BIGINT UNSIGNED PRIMARY KEY COMMENT '雪花ID',
  code      VARCHAR(48) NOT NULL COMMENT '权限码 模块:操作',
  name      VARCHAR(32) NOT NULL COMMENT '权限名',
  type      VARCHAR(8)  NOT NULL DEFAULT 'menu' COMMENT 'menu 菜单 / button 按钮',
  parent    VARCHAR(48) NOT NULL DEFAULT '' COMMENT '父权限码（按钮归属菜单）',
  path      VARCHAR(64) NOT NULL DEFAULT '' COMMENT '菜单路由',
  icon      VARCHAR(64) NOT NULL DEFAULT '' COMMENT '菜单图标',
  sort      INT         NOT NULL DEFAULT 0,
  UNIQUE KEY uk_code (code)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='后台权限点';

CREATE TABLE admin_user_role (
  admin_user_id BIGINT UNSIGNED NOT NULL,
  role_id       BIGINT UNSIGNED NOT NULL,
  PRIMARY KEY (admin_user_id, role_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='账号-角色关联';

CREATE TABLE admin_role_permission (
  role_id       BIGINT UNSIGNED NOT NULL,
  permission_id BIGINT UNSIGNED NOT NULL,
  PRIMARY KEY (role_id, permission_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='角色-权限关联';

-- 兼容旧字段：admin_user 加昵称与最近登录
ALTER TABLE admin_user
  ADD COLUMN nickname VARCHAR(32) NOT NULL DEFAULT '' AFTER password,
  ADD COLUMN last_login_at DATETIME NULL AFTER status;

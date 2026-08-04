-- 0015_system: 系统管理（M2-SYS-02 配置与数据字典）
CREATE TABLE system_config (
  id         BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  config_key VARCHAR(64)   NOT NULL COMMENT '配置键（模块:名称）',
  name       VARCHAR(64)   NOT NULL DEFAULT '' COMMENT '配置名',
  value      VARCHAR(500)  NOT NULL DEFAULT '' COMMENT '配置值',
  remark     VARCHAR(200)  NOT NULL DEFAULT '' COMMENT '说明',
  updated_at DATETIME      NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  created_at DATETIME      NOT NULL DEFAULT CURRENT_TIMESTAMP,
  UNIQUE KEY uk_key (config_key)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='系统配置（键值，热更新）';

CREATE TABLE data_dict (
  id         BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  dict_type  VARCHAR(32) NOT NULL COMMENT '字典类型（report_reason/punish_action/review_action 等）',
  label      VARCHAR(64) NOT NULL COMMENT '展示名',
  value      VARCHAR(64) NOT NULL COMMENT '值',
  sort       INT         NOT NULL DEFAULT 0,
  remark     VARCHAR(200) NOT NULL DEFAULT '',
  created_at DATETIME    NOT NULL DEFAULT CURRENT_TIMESTAMP,
  UNIQUE KEY uk_type_value (dict_type, value),
  KEY idx_type (dict_type)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='数据字典';

-- 种子：举报类型字典（与 report 模块 ReasonNames 对齐）
INSERT INTO data_dict (dict_type, label, value, sort) VALUES
('report_reason', '违法违规', '1', 1),
('report_reason', '色情低俗', '2', 2),
('report_reason', '人身攻击', '3', 3),
('report_reason', '垃圾广告', '4', 4),
('report_reason', '剧透', '5', 5),
('report_reason', '其他', '6', 6),
('punish_action', '禁言', 'mute', 1),
('punish_action', '解除禁言', 'unmute', 2),
('punish_action', '封禁', 'ban', 3),
('punish_action', '解除封禁', 'unban', 4),
('review_action', '通过', 'approve', 1),
('review_action', '驳回', 'reject', 2);

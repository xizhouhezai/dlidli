-- 0013_audit: 机审与合规（M2-AUD 举报/风险分级/青少年模式）
-- 稿件风险等级（AUD-02：低风险抽检/高风险全人审）
ALTER TABLE video ADD COLUMN risk_level TINYINT NOT NULL DEFAULT 0 COMMENT '0低 1中 2高（投稿机审计算）';

-- 青少年模式（AUD-04）
ALTER TABLE user ADD COLUMN youth_mode TINYINT NOT NULL DEFAULT 0 COMMENT '0关闭 1开启';

-- 举报（AUD-03）
CREATE TABLE report (
  id            BIGINT UNSIGNED PRIMARY KEY COMMENT '雪花ID',
  reporter_id   BIGINT UNSIGNED NOT NULL COMMENT '举报人',
  target_type   TINYINT       NOT NULL COMMENT '1视频 2评论 3弹幕 4动态 5用户',
  target_id     BIGINT UNSIGNED NOT NULL COMMENT '对象ID',
  reason_type   TINYINT       NOT NULL COMMENT '举报类型（字典：1违法违规 2色情低俗 3人身攻击 4垃圾广告 5剧透 6其他）',
  reason        VARCHAR(500)  NOT NULL DEFAULT '' COMMENT '补充说明',
  status        TINYINT       NOT NULL DEFAULT 0 COMMENT '0待处理 1已处理 2已忽略',
  handler_id    BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '处理管理员ID',
  handle_result VARCHAR(255)  NOT NULL DEFAULT '' COMMENT '处理结果',
  handled_at    DATETIME      NULL,
  created_at    DATETIME      NOT NULL DEFAULT CURRENT_TIMESTAMP,
  KEY idx_target (target_type, target_id),
  KEY idx_status (status, created_at),
  KEY idx_reporter (reporter_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='举报';

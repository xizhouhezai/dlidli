-- 0013_audit: 回滚
DROP TABLE IF EXISTS report;
ALTER TABLE user DROP COLUMN youth_mode;
ALTER TABLE video DROP COLUMN risk_level;

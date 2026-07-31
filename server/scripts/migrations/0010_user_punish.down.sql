-- 0010_user_punish 回滚
ALTER TABLE user
  DROP COLUMN muted_until,
  DROP COLUMN banned_until;

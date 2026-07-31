-- 0010_user_punish: 用户处罚（ADM-03，禁言/封禁到期时间）
ALTER TABLE user
  ADD COLUMN muted_until DATETIME NULL COMMENT '禁言到期时间（status=1 时有效）' AFTER status,
  ADD COLUMN banned_until DATETIME NULL COMMENT '封禁到期时间（status=2 时有效，NULL=永久）' AFTER muted_until;

-- 0029_user_auth_activate: 回滚
ALTER TABLE `user_auth` DROP COLUMN `activated`;

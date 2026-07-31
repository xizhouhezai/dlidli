-- 0011_rbac 回滚
ALTER TABLE admin_user
  DROP COLUMN nickname,
  DROP COLUMN last_login_at;
DROP TABLE IF EXISTS admin_role_permission;
DROP TABLE IF EXISTS admin_user_role;
DROP TABLE IF EXISTS admin_permission;
DROP TABLE IF EXISTS admin_role;

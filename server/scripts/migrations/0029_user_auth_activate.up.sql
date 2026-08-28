-- 0029_user_auth_activate: 邮箱注册激活状态（ACC-02）
-- 邮箱注册创建"待激活"账号（activated=0），未激活拒绝登录；激活后置 1。
-- 手机/微信登录凭据默认激活（DEFAULT 1），存量行自动为已激活，无需回填。

ALTER TABLE `user_auth`
  ADD COLUMN `activated` TINYINT NOT NULL DEFAULT 1 COMMENT '0待激活（邮箱注册） 1已激活（手机/微信默认1）' AFTER `credential`;

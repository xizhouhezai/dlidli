-- 0028_user_auth_encrypt: 手机号/认证标识加密存储（ACC-43）
-- 两阶段迁移第 1 步：新增确定性哈希列用于查重/查询。
-- 明文 identifier 保留待存量逐条惰性回填；旧索引 uk_type_identifier 暂留（明文仍存在，容量有限，不破坏查询）。
-- 注意：本步骤不建立 hash 唯一索引——存量行 hash 默认为 '' 会互相碰撞（(phone,'') 全量重复），
-- 唯一性由应用层 FindAuth 先查后插（事务）保证；待全量回填（第 2 阶段）hash 非空后再补唯一索引 + 删明文列。

ALTER TABLE `user_auth`
  ADD COLUMN `identifier_hash` VARCHAR(64) NOT NULL DEFAULT '' COMMENT '标识确定性哈希 SHA-256(identity_type:identifier)，用于查重/查询' AFTER `identifier`;

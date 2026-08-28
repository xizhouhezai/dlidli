-- 0028_user_auth_encrypt: 回滚（本步骤仅新增了 identifier_hash 列，未加索引；明文 identifier 仍在）
ALTER TABLE `user_auth` DROP COLUMN `identifier_hash`;

-- 0033 回滚：唯一键还原为全局 file_hash
--
-- 注意：若已存在「同一文件被多个用户上传」的记录（修复后新增的数据），
-- 回滚会因 file_hash 重复而失败。此时需先手工清理重复行：
--   DELETE uf FROM upload_file uf
--   JOIN (SELECT file_hash, MIN(id) keep_id FROM upload_file GROUP BY file_hash HAVING COUNT(*) > 1) d
--     ON d.file_hash = uf.file_hash AND uf.id <> d.keep_id;
ALTER TABLE `upload_file` DROP INDEX `uk_user_hash`;
ALTER TABLE `upload_file` ADD UNIQUE KEY `uk_hash` (`file_hash`);

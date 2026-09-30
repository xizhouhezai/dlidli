-- 0033_upload_file_unique_per_user: upload_file 唯一键由「全局 file_hash」改为「(user_id, file_hash)」
--
-- 背景（缺陷修复，2026-09-30）：
--   VID-24（M3-VID-01）引入「文件必须属于投稿人」的归属校验，但 upload_file 的
--   uk_hash 仍是**全局唯一**，且 Repo.Create 在唯一键冲突时回退 FindByHash 返回
--   【他人】记录 —— 于是第二个用户上传同一文件时，complete 会返回别人的 file_id，
--   直到投稿阶段才以 10004「该文件不属于当前用户」暴露。
--
--   语义上 upload_file 承载两种职责：① 上传登记（属主维度）② 秒传索引（内容维度）。
--   归属校验需要前者，故唯一键必须带上 user_id；同一文件被多个用户上传时，
--   各自登记一条记录（store_key 仍相同，物理层依旧按 hash 内容寻址、不重复落盘）。
--
-- 兼容：老数据 file_hash 全局唯一 → 新唯一键天然满足（(user_id, file_hash) 更宽松），
--       无需回填，仅换索引。
ALTER TABLE `upload_file` DROP INDEX `uk_hash`;
ALTER TABLE `upload_file` ADD UNIQUE KEY `uk_user_hash` (`user_id`, `file_hash`);

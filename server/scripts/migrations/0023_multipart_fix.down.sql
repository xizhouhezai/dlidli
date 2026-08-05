-- 0023_multipart_fix: 回滚
ALTER TABLE transcode_job DROP COLUMN part_index;

-- 0021_multipart: 回滚
ALTER TABLE video_stream DROP COLUMN part_index;
DROP TABLE IF EXISTS video_part;

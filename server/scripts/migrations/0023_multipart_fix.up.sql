-- 0023_multipart_fix: 转码任务补充分P列（M3-CRT-05 多P配套）
ALTER TABLE transcode_job ADD COLUMN part_index TINYINT NOT NULL DEFAULT 0 COMMENT '分P（0单P）' AFTER video_id;

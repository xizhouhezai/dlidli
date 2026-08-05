-- 0024_multipart_fix: 回滚
ALTER TABLE video_stream DROP INDEX uk_video_part_quality;
ALTER TABLE video_stream ADD UNIQUE KEY uk_video_quality (video_id, quality);

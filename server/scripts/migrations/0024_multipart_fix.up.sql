-- 0024_multipart_fix: video_stream 唯一键纳入分P（多P各分P同档位不冲突）
ALTER TABLE video_stream DROP INDEX uk_video_quality;
ALTER TABLE video_stream ADD UNIQUE KEY uk_video_part_quality (video_id, part_index, quality);

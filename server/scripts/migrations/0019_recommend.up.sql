-- 0019_recommend: ItemCF 相似视频表（M3-REC-03 离线计算产物）
CREATE TABLE video_similar (
  id               BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  video_id         BIGINT UNSIGNED NOT NULL COMMENT '稿件',
  similar_video_id BIGINT UNSIGNED NOT NULL COMMENT '相似稿件',
  score            DECIMAL(6,4)    NOT NULL DEFAULT 0 COMMENT '相似度（余弦 0-1）',
  created_at       DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  UNIQUE KEY uk_video_sim (video_id, similar_video_id),
  KEY idx_video (video_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='ItemCF 相似视频（离线计算，推荐召回用）';

-- 0025_collection_fix: 历史修复记录（0022 已修正为直接建 video_collection，本迁移保留 no-op）
-- 清理可能存在的 0022 中断遗留孤儿表
DROP TABLE IF EXISTS collection_item;

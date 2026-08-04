-- 0016_rec: 回滚
ALTER TABLE user DROP COLUMN recommend_on;
DROP TABLE IF EXISTS user_dislike;
DROP TABLE IF EXISTS user_behavior;

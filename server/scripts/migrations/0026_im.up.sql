-- 0026_im: 私信 IM（M3-IM-01，PRD MSG-10~13）
CREATE TABLE conversation (
  id            BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  user_a        BIGINT UNSIGNED NOT NULL COMMENT '会话双方较小 uid',
  user_b        BIGINT UNSIGNED NOT NULL COMMENT '会话双方较大 uid',
  last_content  VARCHAR(500) NOT NULL DEFAULT '' COMMENT '最后一条消息预览',
  last_at       DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '最后消息时间（会话排序）',
  unread_a      INT          NOT NULL DEFAULT 0 COMMENT 'a 方未读数',
  unread_b      INT          NOT NULL DEFAULT 0 COMMENT 'b 方未读数',
  UNIQUE KEY uk_users (user_a, user_b)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='一对一私信会话';

CREATE TABLE private_message (
  id              BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  conversation_id BIGINT UNSIGNED NOT NULL COMMENT '会话',
  sender_id       BIGINT UNSIGNED NOT NULL COMMENT '发送者',
  content         VARCHAR(500) NOT NULL COMMENT '内容（机审通过后入库）',
  content_type    TINYINT      NOT NULL DEFAULT 1 COMMENT '1文字 2图片',
  created_at      DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
  read_at         DATETIME     NULL COMMENT '接收方已读时间',
  KEY idx_conv (conversation_id, id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='私信消息';

-- 0040_video_quality: 清晰度档位配置 + 源分辨率（M4-VIP-02，monetization spec §2.1 / §8，video spec VID-11 / PLY-02）
-- 硬约束：
--   1. **不改动既有语义**：`quality=0` 仍是原画直出源文件，`360/720` 仍是既有转码档位，
--      本迁移只**新增** 1080/2160（4K）两档与一张配置表，不触碰既有 `video_stream` 行。
--   2. 档位编号沿用 0001_init 的列注释（`360/720/1080/2160`）与 `video_stream.quality` 的
--      SMALLINT 类型，无需改列类型（`transcode_job.quality` 同）。
--   3. 权益判定在**服务端**、在下发播放地址时生效；配置表是判定输入之一，
--      故必须可运营（管理端可开关/改码率/改会员限定）。

-- video_stream 补列：流的实际画面高度。
--   为什么需要它：权益口径是「免费最高 1080P」，而**原画（quality=0）的物理分辨率是不确定的**
--   —— 上传一个 4K 源文件时，quality=0 那行本身就是 4K。没有这一列，
--   「免费不下发 4K」就只拦住了 2160 转码档，原画那条路照样把 4K 直出给免费用户。
--   取值口径：转码档 = min(源高度, 档位目标高度)；原画行 = 源文件真实高度（探测失败留 0）。
--   0 表示**未探测**（既有历史行都是 0）——判定侧对「未知高度」只按档位配置（member_only）判定，
--   不套用分辨率上限，从而保证既有 0/360/720 数据的行为与升级前完全一致。
ALTER TABLE `video_stream`
  ADD COLUMN `height` INT NOT NULL DEFAULT 0 COMMENT '实际画面高度（0=未探测；原画行=源文件真实高度）' AFTER `quality`;

-- video_quality_tier：清晰度档位配置（管理端可运营：开关 / 码率 / 是否会员限定）。
--   quality          档位编号（0=原画直出，不是转码目标；360/720/1080/2160=转码档）
--   height           转码目标高度；0 档位为 0（原画不改分辨率）
--   v_bits/max_rate/buf_size/a_bits  转码参数（数据驱动，替代原 presets 硬编码 map）
--   member_only      1=仅会员可播放：**下发过滤与签名校验两侧都拦**（spec §8.3）
--   transcode_enabled 1=投稿时为该档位排转码任务。
--     与 `member_only` 分开是刻意的：**「档位存在」与「默认就为每个投稿跑这个档位」是两回事**
--     ——4K 单机 CPU 转码成本极高，故 2160 默认 transcode_enabled=0，
--     由运营/运维在管理端开启（本卡验收时同样经管理端开启后实测）。
--   sort             展示排序（小在前；下发列表按 quality 升序）
CREATE TABLE `video_quality_tier` (
  `id`               BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  `quality`          SMALLINT     NOT NULL COMMENT '档位编号（0=原画直出）',
  `label`            VARCHAR(32)  NOT NULL DEFAULT '' COMMENT '展示名（如 1080P / 4K）',
  `height`           INT          NOT NULL DEFAULT 0 COMMENT '转码目标高度（0=不改分辨率）',
  `v_bits`           VARCHAR(16)  NOT NULL DEFAULT '' COMMENT '视频码率（-b:v）',
  `max_rate`         VARCHAR(16)  NOT NULL DEFAULT '' COMMENT '最大码率（-maxrate）',
  `buf_size`         VARCHAR(16)  NOT NULL DEFAULT '' COMMENT '码率缓冲（-bufsize）',
  `a_bits`           VARCHAR(16)  NOT NULL DEFAULT '' COMMENT '音频码率（-b:a）',
  `member_only`      TINYINT      NOT NULL DEFAULT 0 COMMENT '1=仅大会员可播放（下发与签名校验双拦）',
  `transcode_enabled` TINYINT     NOT NULL DEFAULT 1 COMMENT '1=投稿时排转码任务（0 档位恒为 0）',
  `sort`             INT          NOT NULL DEFAULT 0 COMMENT '展示排序（小在前）',
  `created_at`       DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at`       DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  UNIQUE KEY `uk_quality` (`quality`),
  KEY `idx_sort` (`sort`, `quality`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='清晰度档位配置（M4-VIP-02）';

-- 初始档位：0/360/720 与升级前**逐参数一致**（v_bits 等取自原 transcoder.go 的 presets 硬编码），
-- 故既有转码产物与排队行为不变；1080 为 V1 档位（免费上限），2160 为 4K（会员限定）。
INSERT INTO `video_quality_tier`
  (`quality`, `label`, `height`, `v_bits`, `max_rate`, `buf_size`, `a_bits`, `member_only`, `transcode_enabled`, `sort`)
VALUES
  (0,    '原画',   0,    '',       '',       '',       '',     0, 0, 0),
  (360,  '360P',   360,  '500k',   '600k',   '1000k',  '96k',  0, 1, 10),
  (720,  '720P',   720,  '2000k',  '2400k',  '4000k',  '128k', 0, 1, 20),
  (1080, '1080P',  1080, '4000k',  '4800k',  '8000k',  '192k', 0, 1, 30),
  (2160, '4K',     2160, '16000k', '19200k', '32000k', '256k', 1, 0, 40);

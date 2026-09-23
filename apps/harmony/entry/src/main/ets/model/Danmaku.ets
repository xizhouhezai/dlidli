/**
 * 弹幕域端侧模型（字段以 server/internal/module/danmaku/model.go 的 Item 为准）。
 * 注意两点契约差异，端侧消费方必须知道：
 * ① `id` 是 snowflake int64，服务端按**字符串**下发（防 JS 精度丢失），故端侧一律按 string 处理；
 * ② `sender_hash` / `is_self` 是 omitempty，缺失即「游客发送者 / 非本人」，不可当空串以外的语义用。
 */

/** 弹幕模式：1 滚动（默认） 2 顶部固定 3 底部固定；顶/底需 Lv3 */
export type DanmakuMode = 1 | 2 | 3;

export interface DanmakuItem {
  id: string;
  content: string;
  /** 视频内时间点（毫秒），非墙钟 */
  time_ms: number;
  mode: DanmakuMode;
  /** RGB 整数（如 0xFFFFFF） */
  color: number;
  /** 服务端默认 25；端侧只区分「小档（≤18）/ 标准档」 */
  font_size: number;
  /** 发送者 UID 哈希（16 字符），供屏蔽用户用；游客发送的弹幕为空 */
  sender_hash?: string;
  /** 仅发送响应里为 true（服务端不向接收方标记他人「自己的弹幕」） */
  is_self?: boolean;
}

/** GET /videos/:bvid/danmaku 的响应包裹（分段拉取） */
export interface DanmakuSegmentResult {
  segment: number;
  /** 分段长度（毫秒），服务端为 6 分钟；端侧以响应值而非硬编码为准 */
  segment_ms: number;
  list: DanmakuItem[];
}

/** GET /videos/:bvid/danmaku/list 的响应包裹（列表面板，分页） */
export interface DanmakuListResult {
  list: DanmakuItem[];
  total: number;
}

/** 屏蔽项类型：1 关键词 2 用户（按 UID 哈希） */
export type DanmakuBlockType = 1 | 2;

export interface DanmakuBlockItem {
  id: string;
  block_type: DanmakuBlockType;
  keyword?: string;
  block_hash?: string;
  created_at: string;
}

/** GET /videos/:bvid/danmaku/blocks 的响应包裹 */
export interface DanmakuBlockListResult {
  list: DanmakuBlockItem[];
}

/** 发送请求（POST /videos/:bvid/danmaku）；font_size 端侧不提供选择，交由服务端取默认 */
export interface SendDanmakuReq {
  content: string;
  time_ms: number;
  mode: DanmakuMode;
  color: number;
}

/** 新增屏蔽请求：type=1 传 keyword；type=2 传 block_hash（端侧无真实 UID） */
export interface AddDanmakuBlockReq {
  block_type: DanmakuBlockType;
  keyword?: string;
  block_hash?: string;
}

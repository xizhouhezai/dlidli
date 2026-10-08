/**
 * 投稿上传域端侧模型（字段名以 server/internal/module/upload 的 model.go 与 handler.go 为准，见 plan §2.1）。
 *
 * 上传会话由后端持有（Redis `up:sess:*`，TTL 24h），端侧**不自建分片账本**：
 * `uploaded`/`chunk_size`/`chunk_count` 每次 init 都由后端下发，断点续传据此只补缺失分片。
 * 因此这里的模型是「无会话状态」的——除下列响应结构外，端侧只额外保存稿件草稿（HMY-52 承接）。
 */

/**
 * `POST /api/v1/upload/init` 响应。
 * `fast=true` 时 `upload_id`/`chunk_size`/`chunk_count`/`uploaded` 均不下发，直接拿 `file_id` 进下一步。
 * 非秒传时 `file_id`/`store_key` 不下发，须走完分片后由 complete 收口。
 */
export interface UploadInitResult {
  fast: boolean;
  file_id?: string;
  store_key?: string;
  upload_id?: string;
  /** 后端固定 5MB（upload.Service 的 ChunkSize），以响应为准而非端侧硬编码 */
  chunk_size?: number;
  chunk_count?: number;
  /** 已上传分片下标；新建会话为空数组（断点续传依据） */
  uploaded?: number[];
}

/** `GET /api/v1/upload/{id}` 响应（与 init 的非秒传分支同形态，无 fast 字段） */
export interface UploadProgressResult {
  upload_id: string;
  chunk_size: number;
  chunk_count: number;
  uploaded: number[];
}

/** `POST /api/v1/upload/{id}/complete` 响应；合并后由后端登记并返回可投稿的 file_id */
export interface UploadCompleteResult {
  file_id: string;
  store_key: string;
}

/** `/videos` 投稿请求（本卡只负责产出 file_id，表单字段由 HMY-52 承接） */
export interface SubmitReq {
  file_id: string;
  title: string;
  description: string;
  category_id: number;
  tags: string[];
  /** 1 自制 2 转载 */
  copyright: number;
  cover: string;
  parts: SubmitPart[];
}

export interface SubmitPart {
  file_id: string;
  title: string;
}

/** 选片结果：系统选择器给的媒体库 uri 与端侧补出的展示信息 */
export interface PickedVideo {
  /** 选择器返回的原始 uri（媒体库，如 file://media/Photo/…）——不能直接投递给 request.uploadFile */
  uri: string;
  fileName: string;
  /** 小写扩展名，不含点 */
  ext: string;
  size: number;
}

/** 一次上传的最终产物（投稿接口消费 file_id） */
export interface UploadResult {
  fast: boolean;
  fileId: string;
  storeKey: string;
}

/**
 * 本地前置校验结果（HMY-54：能在端侧拦下的失败不发请求）。
 * `reason` 供调用方分支（如移动网络要弹二次确认），`message` 已是可展示文案。
 */
export interface UploadCheckResult {
  ok: boolean;
  reason: UploadRejectReason;
  message: string;
}

export type UploadRejectReason = 'none' | 'ext' | 'size' | 'space' | 'network';

/** 上传阶段：算哈希 → 上传分片 → 收口 */
export type UploadPhase = 'idle' | 'hashing' | 'uploading' | 'completing' | 'done' | 'failed';

/**
 * 进度视图（HMY-50 要求「百分比 + 已传/总量」）。
 * 字节口径比分数口径更贴近 HMY-50 原文，故主口径用字节，分片数仅作辅助展示。
 */
export interface UploadProgressView {
  phase: UploadPhase;
  /** 0~100 整数百分比 */
  percent: number;
  uploadedBytes: number;
  totalBytes: number;
  uploadedParts: number;
  totalParts: number;
  /** 已传分片是否由后端 `uploaded` 命中（断点续传生效） */
  resumed: boolean;
  fast: boolean;
}

/** 进度回调：控制器在每次字节/阶段推进时调用 */
export type UploadProgressListener = (view: UploadProgressView) => void;

/** 空进度视图的构造入口，避免各处手写字段 */
export function idleProgress(): UploadProgressView {
  return {
    phase: 'idle', percent: 0, uploadedBytes: 0, totalBytes: 0,
    uploadedParts: 0, totalParts: 0, resumed: false, fast: false
  };
}

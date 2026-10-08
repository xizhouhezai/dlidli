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

// ---------- 稿件信息与封面（HMY-51） ----------

/** 标题上限：对齐 Web `maxlength="80"` 与后端 `SubmitReq.title` 的 `max=80` */
export const TITLE_MAX: number = 80;
/** 简介上限：对齐后端 `SubmitReq.description` 的 `max=2000` */
export const DESC_MAX: number = 2000;
/** 标签数量上限：对齐 Web `addTag` 与后端 `SubmitReq.tags` 的 `min=1,max=10` */
export const TAGS_MAX: number = 10;

/** 稿件信息草稿：字段与 `SubmitReq` 一一对应，校验与提交共用同一份状态 */
export interface SubmitDraft {
  title: string;
  description: string;
  /** 0 表示未选（分区 ID 从 1 起，0 不会是合法分区） */
  categoryId: number;
  tags: string[];
  /** 1 自制 2 转载 */
  copyright: number;
}

export function emptyDraft(): SubmitDraft {
  return { title: '', description: '', categoryId: 0, tags: [], copyright: 1 };
}

/**
 * 稿件信息校验：返回空串表示通过，否则返回**可直接展示**的原因。
 *
 * 口径对齐 Web `useUploadForm.submit` 与后端 `SubmitReq` binding——端侧先拦、失败不发请求。
 * 分区不仅要「选了」还要「在已加载的一级分区里」：`category_id` 后端只校验非零，
 * 传一个不存在的 ID 会在服务端 500/入库脏数据，端侧按存在性拦下更准。
 */
export function validateDraft(draft: SubmitDraft, categoryIds: number[]): string {
  const title: string = draft.title.trim();
  if (title.length === 0) {
    return '请填写标题';
  }
  if (title.length > TITLE_MAX) {
    return `标题不能超过 ${TITLE_MAX} 个字（当前 ${title.length} 个）`;
  }
  if (draft.description.length > DESC_MAX) {
    return `简介不能超过 ${DESC_MAX} 个字（当前 ${draft.description.length} 个）`;
  }
  if (draft.categoryId <= 0) {
    return '请选择分区';
  }
  if (!categoryIds.includes(draft.categoryId)) {
    return '所选分区不存在，请重新选择';
  }
  if (draft.tags.length === 0) {
    return '请至少添加 1 个标签';
  }
  if (draft.tags.length > TAGS_MAX) {
    return `最多 ${TAGS_MAX} 个标签（当前 ${draft.tags.length} 个）`;
  }
  if (draft.copyright !== 1 && draft.copyright !== 2) {
    return '请选择稿件类型';
  }
  return '';
}

/** 标签规整：去首尾空白、丢空串、去重且保持输入顺序（添加与提交前都走这里） */
export function normalizeTags(raw: string[]): string[] {
  const out: string[] = [];
  for (const tag of raw) {
    const text: string = tag.trim();
    if (text.length > 0 && !out.includes(text)) {
      out.push(text);
    }
  }
  return out;
}

/** 封面来源：视频截帧 / 相册选图 */
export type CoverOrigin = 'frame' | 'photo';

/**
 * 本地封面草稿：**已按 16:9 裁切**并落盘 cache。
 *
 * 端侧先裁切再上传是 HMY-51 的端侧差异条款：相册原图常见 4:3 或竖图，
 * 原样上传会被消费端按 16:9 的 `object-fit: cover` 裁掉主体，端侧按同一几何裁切后所见即所得。
 */
export interface CoverDraft {
  origin: CoverOrigin;
  /** `internal://cache/…`——`request.uploadFile` 唯一接受的 uri 形态 */
  cacheUri: string;
  /** 沙箱真实路径（清理用） */
  sandboxPath: string;
  /** 裁切后的实际像素（校验 16:9 与展示用） */
  width: number;
  height: number;
  /** 上传成功后的封面 URL；未上传时为空串（提交时才上传，与 Web `resolveCover` 同时机） */
  url: string;
}

/** `POST /api/v1/videos/cover` 响应体 */
export interface CoverUploadResult {
  cover: string;
}

/** `POST /api/v1/videos` 响应（本卡只消费 bvid/status；状态回看属 HMY-53） */
export interface SubmitResult {
  bvid: string;
  /** 0 草稿 / 1 上传中 / 2 转码中 / 3 待审核 / 4 已发布 / 5 已驳回 */
  status: number;
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

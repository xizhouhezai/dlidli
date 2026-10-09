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
/** 分P 标题上限：对齐 Web `PartDraft` 的 `slice(0,80)`、后端 `PartSubmit.title` 的 `max=80` 与 `video_part.title VARCHAR(80)` */
export const PART_TITLE_MAX: number = 80;

/**
 * 分P 数量上限 = 10，**对齐 Web `useUploadParts.addPart` 的硬编码 10**（「最多 10 个分P」）。
 *
 * 这是**产品自律值，不是后端强制边界**——2026-10-09 实测（见 docs/specs/harmony/tasks.md M4-HMY-53）：
 * ① `SubmitReq.parts` **没有任何数量 binding**，传 100 / 200 个分P 均返回 200 且 `video_part`
 *    逐条落库；`tasks.md` 原写「上限 10 对齐 Web 与 `SubmitReq.parts`」，后半句不成立。
 * ② 真正的硬边界在**存储层**：`video_part.part_index` 是 `int8`（TINYINT）且 `Submit` 按 `int8(i+1)`
 *    写入，第 **129** 个分P 起下标溢出回绕为**负数**（n=130 时 `part_index` 为 `-128,-127,-126,1..127`），
 *    第 **257** 个起整单投稿 **HTTP 500**。
 * ③ 故端侧取 10 与 Web 同一口径（两端 UI 一致优先于逼近一个会静默写坏数据的 127），
 *    并有真实验证：后端只保证「不报错」，不保证下标正确，端侧**不该**把这个值放大到 10 以上。
 */
export const PART_MAX: number = 10;

/**
 * 一个分P（HMY-52）。
 *
 * `uploadId` 是**上传会话 ID**（后端 Redis `up:sess:*`，TTL 24h），只在分P上传中途被中断时才有意义：
 * 上传成功收口后后端会**主动删除该会话**（`upload.Service.Complete` 末尾 `Del(sessKey…)`），
 * 故「有 file_id 的分P」拿 uploadId 去查会话必然查不到——校验逻辑必须区分这两种状态，
 * 否则会把「已完成」误判成「已失效」（`store/UploadDraftStore` 与 `UploadPage.checkSessions` 的口径）。
 */
export interface PartDraft {
  /** 分P 标题；空则由提交时回落到文件名 */
  title: string;
  /** 上传完成后拿到的 file_id；空串表示未上传完成 */
  fileId: string;
  /** 原始文件名（展示、标题回落、占位文案） */
  fileName: string;
  /** 上传会话 ID；秒传（fast）没有会话，为空串 */
  uploadId: string;
}

/** 空白分P */
export function emptyPart(): PartDraft {
  return { title: '', fileId: '', fileName: '', uploadId: '' };
}

/**
 * 稿件信息草稿：字段与 `SubmitReq` 一一对应，**并包含 HMY-52 的分P 与封面**，校验、持久化、提交共用同一份状态。
 *
 * 与 `SubmitReq` 的差异：`parts` 只装**额外的分P**（主视频单独占 `fileId`），提交时由 `resolveSubmitParts`
 * 合成——这样单P投稿（`parts` 为空）仍走后端既有的单P分支，与 HMY-51 的行为逐字节一致。
 */
export interface SubmitDraft {
  title: string;
  description: string;
  /** 0 表示未选（分区 ID 从 1 起，0 不会是合法分区） */
  categoryId: number;
  tags: string[];
  /** 1 自制 2 转载 */
  copyright: number;
  /** 额外的分P（分P2 起）；主视频即分P1，标题取稿件标题 */
  parts: PartDraft[];
  /** 主视频 file_id；空串表示未上传完成 */
  fileId: string;
  /** 主视频的上传会话 ID（中断续传用；收口后为空） */
  uploadId: string;
  /** 封面已按 16:9 裁切的沙箱路径；空串表示未设置封面 */
  coverPath: string;
  /** 封面的 `internal://cache/…` 形态（`request.uploadFile` 专用；恢复预览与重传都要它） */
  coverCacheUri: string;
  /** 封面来源 `frame`/`photo`；空串表示无封面 */
  coverOrigin: string;
  /** 裁切后的实际像素（校验 16:9 与展示用；恢复时原样带回） */
  coverWidth: number;
  coverHeight: number;
  /** 封面已上传的 URL；空串表示尚未上传 */
  coverUrl: string;
}

export function emptyDraft(): SubmitDraft {
  return {
    title: '', description: '', categoryId: 0, tags: [], copyright: 1,
    parts: [], fileId: '', uploadId: '',
    coverPath: '', coverCacheUri: '', coverOrigin: '', coverWidth: 0, coverHeight: 0, coverUrl: ''
  };
}

/** 文件名去扩展名（分P 标题的默认值，与 Web `p.title || 文件名` 同口径） */
export function baseNameOf(fileName: string): string {
  const dot: number = fileName.lastIndexOf('.');
  return dot > 0 ? fileName.substring(0, dot) : fileName;
}

/**
 * 分P 的最终标题：显式标题 > 文件名 > 「分P{n}」。
 * 后端对空标题会落空串（`video_part.title NOT NULL DEFAULT ''`），端侧此处保证非空。
 */
export function partTitleOf(part: PartDraft, index: number): string {
  const explicit: string = part.title.trim();
  if (explicit.length > 0) {
    return explicit;
  }
  const fromName: string = baseNameOf(part.fileName).trim();
  if (fromName.length > 0) {
    return fromName.substring(0, PART_TITLE_MAX);
  }
  return `分P${index + 2}`;
}

/** 额外分P 在界面与提示里的编号：主视频占分P1，故数组下标 0 对应分P2 */
export function extraPartLabel(index: number): string {
  return `分P${index + 2}`;
}

/** 有 file_id 的分P（未传完的留在草稿里但不参与提交） */
export function completedParts(draft: SubmitDraft): PartDraft[] {
  return draft.parts.filter((part: PartDraft) => part.fileId.length > 0);
}

/**
 * 提交用的 `parts`。
 *
 * **单P投稿必须回空数组**：`parts` 非空会把后端推进多P分支（`service.Submit` 的 `len(req.Parts)>0`），
 * 该分支只用 `parts` 建 `video_part` 与视频流，`file_id` 仅用于归属校验、**其流不入库**——
 * 即「有 parts 时主视频被整体丢掉」。故只有真的存在额外分P 时才把主视频一起并进 parts（作为分P1）。
 */
export function resolveSubmitParts(draft: SubmitDraft): SubmitPart[] {
  const extra: PartDraft[] = completedParts(draft);
  if (extra.length === 0) {
    return [];
  }
  const out: SubmitPart[] = [];
  if (draft.fileId.length > 0) {
    // 主视频 = 分P1，标题取稿件标题（Web 的多P表单没有主视频的独立分P标题，端侧沿用同一口径）
    out.push({ file_id: draft.fileId, title: draft.title.trim().substring(0, PART_TITLE_MAX) });
  }
  for (let i = 0; i < extra.length; i++) {
    out.push({ file_id: extra[i].fileId, title: partTitleOf(extra[i], i) });
  }
  return out;
}

/**
 * 提交用的 `file_id`（后端 `required`，不可为空）。
 * 多P时后端只拿它做归属校验，故主视频缺失时回落到分P1，避免整单被 binding 打回。
 */
export function resolveSubmitFileId(draft: SubmitDraft): string {
  if (draft.fileId.length > 0) {
    return draft.fileId;
  }
  const parts: SubmitPart[] = resolveSubmitParts(draft);
  return parts.length > 0 ? parts[0].file_id : '';
}

/** 草稿是否有值得持久化的内容（全空则不写盘，避免重进时弹无意义的恢复询问） */
export function draftHasContent(draft: SubmitDraft): boolean {
  return draft.title.trim().length > 0
    || draft.description.trim().length > 0
    || draft.categoryId > 0
    || draft.tags.length > 0
    || draft.parts.length > 0
    || draft.fileId.length > 0
    || draft.coverPath.length > 0;
}

/**
 * 稿件信息校验：返回空串表示通过，否则返回**可直接展示**的原因。
 *
 * 口径对齐 Web `useUploadForm.submit` 与后端 `SubmitReq` binding——端侧先拦、失败不发请求。
 * 分区不仅要「选了」还要「在已加载的一级分区里」：`category_id` 后端只校验非零，
 * 传一个不存在的 ID 会在服务端 500/入库脏数据，端侧按存在性拦下更准。
 *
 * 分P 标题必须端侧先拦：后端 `PartSubmit.title` 的 `max=80` 是 **`binding` 对该字段无效**的写法
 * （实测传 81 字不报 10002 而是落成 **空标题**、HTTP 500 是后续写入异常），静默丢标题比报错更难查。
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
  if (draft.parts.length > PART_MAX - 1) {
    return `最多 ${PART_MAX} 个分P（当前 ${draft.parts.length + 1} 个）`;
  }
  for (let i = 0; i < draft.parts.length; i++) {
    const part: PartDraft = draft.parts[i];
    if (part.fileId.length === 0) {
      return `${extraPartLabel(i)} 还没有上传完成的文件，请先上传或删除该分P`;
    }
    const partTitle: string = partTitleOf(part, i);
    if (partTitle.length > PART_TITLE_MAX) {
      return `${extraPartLabel(i)} 标题不能超过 ${PART_TITLE_MAX} 个字（当前 ${partTitle.length} 个）`;
    }
  }
  if (resolveSubmitFileId(draft).length === 0) {
    return '请先上传视频文件';
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

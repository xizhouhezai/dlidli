import { request } from '@kit.BasicServicesKit';
import { BusinessError } from '@kit.BasicServicesKit';
import { common } from '@kit.AbilityKit';
import { connection } from '@kit.NetworkKit';
import { statfs } from '@kit.CoreFileKit';
import { ApiConfig } from '../common/constants/ApiConfig';
import { ApiError, networkError } from '../model/Api';
import { ErrCode } from '../common/constants/ErrorMessages';
import {
  PickedVideo, UploadCheckResult, UploadInitResult, UploadProgressListener, UploadPhase,
  UploadProgressView, UploadRejectReason, UploadResult, idleProgress
} from '../model/Upload';
import { UploadApi } from './UploadApi';
import { AppContext } from '../store/AppContext';
import { TokenStore } from '../store/TokenStore';
import { MediaLib } from '../common/utils/MediaLib';
import { Logger } from '../common/utils/Logger';

const TAG: string = 'UploadController';

/** 与后端 upload.Service 的 allowedExts / maxFileSize 保持一致（spec §2.6 HMY-54） */
const ALLOWED_EXTS: string[] = ['mp4', 'mov', 'mkv', 'flv', 'avi'];
const MAX_FILE_SIZE: number = 8 * 1024 * 1024 * 1024;
/** 分片任务限并发（HMY-50 实测：循环里密集发起 uploadFile 会触发 appfreeze THREAD_BLOCK_6S） */
const MAX_CONCURRENCY: number = 2;
/** 单分片超时：5MB 在弱网下的宽裕上限，超时即判失败以便整轮重试 */
const PART_TIMEOUT_MS: number = 120000;
/** 分片整轮重试次数（网络抖动自愈，不打扰用户） */
const MAX_ROUNDS: number = 3;

/**
 * 上传会话建立回调（HMY-52 草稿恢复用）。
 *
 * 会话 ID 只在 `init` 之后的**上传中途**有意义：一收口后端就 `Del` 掉它（见 `UploadApi.progress` 的说明）。
 * 上传失败/用户中途退出时 `start()` 会抛错，没有返回值可带出该 ID，故必须在 `init` 返回的那一刻
 * 就把它交给调用方——否则「退出后重进能否续传」这条链路在端侧根本没有可持久化的凭据。
 */
export type UploadSessionListener = (uploadId: string) => void;

/** 一次上传的运行态（端侧不持有跨会话账本，仅在本次上传内维护） */
interface UploadContext {
  picked: PickedVideo;
  /** `internal://cache/…`，投递给 request.uploadFile */
  cacheUri: string;
  session: UploadInitResult;
  /** 已成功上传的分片下标（含后端下发的 uploaded，续传时**不重传**） */
  done: Set<number>;
  chunkSize: number;
  chunkCount: number;
  totalBytes: number;
  /** 已传字节（按整片累加；片内进度由 on('progress') 折算） */
  partBytes: number;
}

/**
 * 投稿上传链路（HMY-50 / HMY-54）。
 *
 * 通道走 HMY-50 的定论 A 路线：`request.uploadFile` + `method:'PUT'` + 显式 `begins`/`ends` 字节区间，
 * 后端零改动。三条硬约束来自该预研的实测，改动前务必先读 plan §6.1：
 * ① 分片**必须显式给区间**——不给时 PUT 发的是整个源文件，大文件会撑爆连接并卡住客户端；
 * ② `method` 必须是 `PUT`——`POST` 会发 multipart 信封，后端按裸 body 落盘会多出边界字节并报「分片大小不合法」；
 * ③ 任务必须**限并发**（≤2）——密集齐发会阻塞主线程触发 appfreeze。
 *
 * 上传会话由后端持有（Redis，TTL 24h），本类不落任何分片账本：断点续传完全依据 `init` 返回的 `uploaded`，
 * 因此「退出应用再进入」与「同一次会话内重试」走的是同一条代码路径。
 */
export class UploadController {
  private onProgress: UploadProgressListener;
  private onSession: UploadSessionListener;
  private view: UploadProgressView = idleProgress();
  private cancelled: boolean = false;
  /** 在飞的分片任务，取消时统一回收 */
  private inFlight: request.UploadTask[] = [];

  constructor(onProgress?: UploadProgressListener, onSession?: UploadSessionListener) {
    this.onProgress = onProgress ?? ((_: UploadProgressView) => {});
    this.onSession = onSession ?? ((_: string) => {});
  }

  // ---------- 前置校验（HMY-54） ----------

  /**
   * 上传前校验：白名单、单文件上限、剩余空间、网络类型。
   * 网络返回 `network` 而非直接失败——移动网络下由调用方弹确认框（HMY-54 要求「提示并让用户确认」，
   * 不是禁止上传），其余原因为硬拒绝。
   */
  static async check(context: common.UIAbilityContext, picked: PickedVideo): Promise<UploadCheckResult> {
    if (!ALLOWED_EXTS.includes(picked.ext)) {
      return UploadController.reject('ext', `不支持的视频格式，仅支持 ${ALLOWED_EXTS.join(' / ')}`);
    }
    if (picked.size <= 0) {
      return UploadController.reject('size', '文件为空或无法读取，请重新选择');
    }
    if (picked.size > MAX_FILE_SIZE) {
      return UploadController.reject('size',
        `单文件不能超过 8GB，当前 ${UploadController.humanSize(picked.size)}`);
    }
    // 分片源是 cache 里的整份副本，剩余空间必须容得下它（无法边拷边传）
    const free: number = await UploadController.freeSpace(context);
    if (free > 0 && free < picked.size) {
      return UploadController.reject('space',
        `剩余空间不足，需要 ${UploadController.humanSize(picked.size)}，可用 ${UploadController.humanSize(free)}`);
    }
    if (await UploadController.isCellular()) {
      return UploadController.reject('network',
        `当前为移动网络，将上传 ${UploadController.humanSize(picked.size)}，可能消耗较多流量`);
    }
    return { ok: true, reason: 'none', message: '' };
  }

  private static reject(reason: UploadRejectReason, message: string): UploadCheckResult {
    return { ok: false, reason: reason, message: message };
  }

  /** cacheDir 所在卷的可用空间；取不到（异常）返回 0，按「无法判定」放行而不误拦 */
  private static async freeSpace(context: common.UIAbilityContext): Promise<number> {
    try {
      return await statfs.getFreeSize(context.cacheDir);
    } catch (err) {
      Logger.warn(TAG, `读取剩余空间失败，跳过空间校验：${(err as BusinessError).message}`);
      return 0;
    }
  }

  /** 当前默认网络是否为蜂窝（HMY-54 的触发条件）；判定失败按「非移动网络」放行，不阻塞上传 */
  private static async isCellular(): Promise<boolean> {
    try {
      const caps: connection.NetCapabilities =
        await connection.getNetCapabilities(await connection.getDefaultNet());
      return caps.bearerTypes.includes(connection.NetBearType.BEARER_CELLULAR);
    } catch (err) {
      Logger.warn(TAG, `读取网络类型失败，按非移动网络处理：${(err as BusinessError).message}`);
      return false;
    }
  }

  // ---------- 主流程 ----------

  /**
   * 跑完整条链路：暂存副本 → 算 SHA-256 → init（秒传直接收口）→ 补齐缺失分片 → complete。
   * 任何一步抛出都表示本次上传未完成；用户重试时按后端 `uploaded` 自动续传。
   */
  async start(context: common.UIAbilityContext, picked: PickedVideo): Promise<UploadResult> {
    this.cancelled = false;
    this.view = idleProgress();
    this.view.totalBytes = picked.size;
    this.emit('hashing');

    const cacheUri: string = await MediaLib.stageToCache(context, picked);

    // 哈希阶段也报进度：8GB 文件在端侧可达数十秒，无反馈会被当成卡死
    const fileHash: string = await MediaLib.sha256Chunked(MediaLib.stagedPath(context, picked),
      (read: number, total: number) => {
        this.view.phase = 'hashing';
        this.view.uploadedBytes = read;
        this.view.totalBytes = total;
        // 哈希占总进度的一成、上传占九成，避免进度条在哈希阶段就走满
        this.view.percent = total > 0 ? Math.floor((read / total) * 10) : 0;
        this.publish();
      });
    Logger.info(TAG, `端侧 SHA-256 完成：${fileHash}`);

    const init: UploadInitResult = await UploadApi.init(picked.fileName, picked.size, fileHash);
    if (init.fast) {
      // 秒传命中：不上传任何分片，直接进入下一步（HMY-50 ③）
      Logger.info(TAG, `秒传命中，跳过上传：file_id=${init.file_id}`);
      this.view.phase = 'done';
      this.view.fast = true;
      this.view.uploadedBytes = picked.size;
      this.view.percent = 100;
      this.publish();
      return { fast: true, fileId: init.file_id ?? '', storeKey: init.store_key ?? '' };
    }

    const uploadId: string = init.upload_id ?? '';
    const chunkSize: number = init.chunk_size ?? 0;
    const chunkCount: number = init.chunk_count ?? 0;
    if (uploadId.length === 0 || chunkSize <= 0 || chunkCount <= 0) {
      throw new ApiError(ErrCode.BAD_RESPONSE, '上传会话信息不完整，请重试', '', true);
    }
    // 会话一经建立就上报：此时失败/退出才在端侧留有可续传的凭据（HMY-52 草稿恢复）
    this.onSession(uploadId);

    const done: Set<number> = new Set<number>();
    for (const index of init.uploaded ?? []) {
      done.add(index);
    }
    const ctx: UploadContext = {
      picked: picked,
      cacheUri: cacheUri,
      session: init,
      done: done,
      chunkSize: chunkSize,
      chunkCount: chunkCount,
      totalBytes: picked.size,
      // uploaded 非空即代表续传：把已传分片的**实际字节**先计入，进度条从真实起点继续
      partBytes: UploadController.completedBytes(done, chunkSize, picked.size)
    };
    this.view.resumed = done.size > 0;
    this.view.totalParts = chunkCount;
    this.view.uploadedParts = done.size;
    Logger.info(TAG, `会话就绪：upload_id=${uploadId} 已传 ${done.size}/${chunkCount} 片，` +
      `chunk_size=${chunkSize} 续传=${this.view.resumed}`);

    await this.uploadMissing(ctx);

    this.emitUploading(ctx);
    this.emit('completing');
    const complete = await UploadApi.complete(uploadId);
    Logger.info(TAG, `上传收口：file_id=${complete.file_id}`);

    this.view.phase = 'done';
    this.view.percent = 100;
    this.view.uploadedBytes = ctx.totalBytes;
    this.view.uploadedParts = ctx.chunkCount;
    this.publish();
    return { fast: false, fileId: complete.file_id, storeKey: complete.store_key };
  }

  /**
   * 只传缺失分片。分多轮：每轮把当前缺口按 ≤2 并发跑完，整轮结束后**重新按缺口过滤**再下一轮——
   * 失败的分片留在缺口里自然进入下一轮，无需单独的错误收集。
   * 轮尽仍有缺口才抛错，此时已传分片都在后端，用户重试即续传（HMY-50 ②）。
   */
  private async uploadMissing(ctx: UploadContext): Promise<void> {
    for (let round: number = 1; round <= MAX_ROUNDS; round++) {
      const missing: number[] = [];
      for (let i: number = 0; i < ctx.chunkCount; i++) {
        if (!ctx.done.has(i)) {
          missing.push(i);
        }
      }
      if (missing.length === 0) {
        return;
      }
      Logger.info(TAG, `第 ${round} 轮上传：待传 ${missing.length} 片`);
      await this.runBatch(ctx, missing);
      if (this.cancelled) {
        throw new ApiError(ErrCode.LOCAL_REJECT, '上传已取消', '', false);
      }
    }
    const left: number = ctx.chunkCount - ctx.done.size;
    throw networkError(ErrCode.NET_OFFLINE, `还有 ${left} 个分片未上传成功，请重试`);
  }

  /** 把一批分片按 MAX_CONCURRENCY 分组，组内并发、组间串行（限并发的落点） */
  private async runBatch(ctx: UploadContext, indexes: number[]): Promise<void> {
    for (let i: number = 0; i < indexes.length; i += MAX_CONCURRENCY) {
      if (this.cancelled) {
        return;
      }
      const group: number[] = indexes.slice(i, i + MAX_CONCURRENCY);
      const tasks: Promise<void>[] = group.map((index: number) => this.uploadPart(ctx, index));
      // 单片失败不中止整组：allSettled 让同组其它片照常推进，失败片留给下一轮
      await Promise.allSettled(tasks);
    }
  }

  /**
   * 传一片。三条硬约束都在这里：`PUT` + 显式 `begins`/`ends` + 单文件 `files`。
   * `begins/ends` 是**字节闭区间**，末尾片长度 = 总长 - begins。
   */
  private async uploadPart(ctx: UploadContext, index: number): Promise<void> {
    const begins: number = index * ctx.chunkSize;
    const ends: number = Math.min(begins + ctx.chunkSize, ctx.totalBytes) - 1;
    const uploadId: string = ctx.session.upload_id ?? '';
    const url: string = `${ApiConfig.BASE_URL}/api/v1/upload/${encodeURIComponent(uploadId)}/parts/${index}`;

    const config: request.UploadConfig = {
      url: url,
      header: UploadController.authHeader(),
      method: 'PUT',
      // config.index 是 files 数组下标（单文件必须 0），**不是分片序号**（HMY-50 实测坑）
      index: 0,
      begins: begins,
      ends: ends,
      files: [{
        filename: ctx.picked.fileName,
        name: 'file',
        uri: ctx.cacheUri,
        type: 'application/octet-stream'
      }],
      data: []
    };

    let task: request.UploadTask | null = null;
    try {
      task = await request.uploadFile(AppContext.require(), config);
      this.inFlight.push(task);
      const state: request.TaskState = await this.awaitPart(task, ctx);
      if (!UploadController.isPartOk(state.responseCode)) {
        throw new Error(`分片 ${index} 返回 HTTP ${state.responseCode} ${state.message}`);
      }
      ctx.done.add(index);
      ctx.partBytes = Math.min(ctx.partBytes + (ends - begins + 1), ctx.totalBytes);
      this.emitUploading(ctx);
    } catch (err) {
      const be: BusinessError = err as BusinessError;
      Logger.warn(TAG, `分片 ${index} 上传失败：code=${be.code} message=${be.message}`);
      throw new Error(`分片上传失败：code=${be.code} ${be.message}`);
    } finally {
      if (task !== null) {
        UploadController.release(task);
        const at: number = this.inFlight.indexOf(task);
        if (at >= 0) {
          this.inFlight.splice(at, 1);
        }
      }
    }
  }

  /**
   * 等一片收口，同时把 `on('progress')` 的片内字节折算进总进度。
   * 进度统一按**字节**口径（HMY-50 要求「已传/总量」）：总已传 = 本轮之前已完成的片 + 本片已发字节。
   */
  private awaitPart(task: request.UploadTask, ctx: UploadContext): Promise<request.TaskState> {
    return new Promise<request.TaskState>((resolve: (state: request.TaskState) => void,
      reject: (err: Error) => void) => {
      const base: number = ctx.partBytes;
      let settled: boolean = false;
      const timer: number = setTimeout(() => {
        finish(() => reject(new Error(`分片超时（${PART_TIMEOUT_MS}ms）`)));
      }, PART_TIMEOUT_MS);

      const finish = (fn: () => void): void => {
        if (settled) {
          return;
        }
        settled = true;
        clearTimeout(timer);
        fn();
      };

      task.on('progress', (uploadedSize: number, totalSize: number) => {
        if (settled) {
          return;
        }
        this.view.uploadedBytes = Math.min(base + uploadedSize, ctx.totalBytes);
        this.view.phase = 'uploading';
        this.view.percent = ctx.totalBytes > 0
          ? Math.floor((this.view.uploadedBytes / ctx.totalBytes) * 100) : 0;
        this.publish();
      });
      task.on('complete', (states: request.TaskState[]) => {
        finish(() => {
          if (states.length === 0) {
            reject(new Error('上传任务未返回状态'));
            return;
          }
          resolve(states[0]);
        });
      });
      task.on('fail', (states: request.TaskState[]) => {
        finish(() => reject(new Error(UploadController.failReason(states))));
      });
    });
  }

  /** 失败态只给了 TaskState[]，取首个的 message/responseCode 拼出可读原因 */
  private static failReason(states: request.TaskState[]): string {
    if (states.length === 0) {
      return '上传任务失败';
    }
    const state: request.TaskState = states[0];
    return state.message.length > 0
      ? `上传任务失败（${state.responseCode}）：${state.message}`
      : `上传任务失败（${state.responseCode}）`;
  }

  /**
   * 分片是否成功。
   * **`responseCode` 在成功时是 0**（SDK：「0 means that the task is successful, and other values means that the task fails」），
   * 不是 HTTP 语义的 200——HMY-50 的探针记录（「responseCode=0」）与此一致，本卡实测再次确认。
   * 兼容 200 是防御性写法：若某版本改回 HTTP 语义也不会把成功判成失败。
   */
  private static isPartOk(responseCode: number): boolean {
    return responseCode === 0 || responseCode === 200;
  }

  private emitUploading(ctx: UploadContext): void {
    this.view.phase = 'uploading';
    this.view.totalParts = ctx.chunkCount;
    this.view.uploadedParts = ctx.done.size;
    this.view.totalBytes = ctx.totalBytes;
    this.view.uploadedBytes = Math.max(this.view.uploadedBytes, ctx.partBytes);
    this.view.percent = ctx.totalBytes > 0
      ? Math.floor((this.view.uploadedBytes / ctx.totalBytes) * 100) : 0;
    this.publish();
  }

  private emit(phase: UploadPhase): void {
    this.view.phase = phase;
    this.publish();
  }

  /** 当前进度快照（未订阅回调的调用方，或收口后取终值） */
  snapshot(): UploadProgressView {
    return this.view;
  }

  /**
   * 把当前进度**发布为一份新对象**。
   * 必须每次新建：`@Local` 按引用比对，若反复回传同一个 `this.view`，
   * UI 只会渲染第一次的值——实测表现为「进度条停在 10%、阶段一直显示正在校验文件」，
   * 而日志里上传其实已经完成。这是 HMY-50 ③「进度可见」的直接约束。
   */
  private publish(): void {
    const next: UploadProgressView = {
      phase: this.view.phase,
      percent: this.view.percent,
      uploadedBytes: this.view.uploadedBytes,
      totalBytes: this.view.totalBytes,
      uploadedParts: this.view.uploadedParts,
      totalParts: this.view.totalParts,
      resumed: this.view.resumed,
      fast: this.view.fast
    };
    this.onProgress(next);
  }

  /** 取消：回收在飞任务并标记，主流程会在下一批检查点退出；已传分片保留在后端可续传 */
  cancel(): void {    this.cancelled = true;
    for (const task of this.inFlight) {
      UploadController.release(task);
    }
    this.inFlight = [];
    Logger.info(TAG, '上传已取消，已传分片保留在后端可续传');
  }

  /** 回收上传任务（delete 是异步的，失败不影响业务，仅记日志） */
  private static release(task: request.UploadTask): void {
    task.delete().catch((err: BusinessError) => {
      Logger.warn(TAG, `回收上传任务失败：${err.message}`);
    });
  }

  /** 上传任务不走 HttpClient，令牌需自行注入（后端 upload 路由全部要求登录） */
  private static authHeader(): Record<string, string> {
    const header: Record<string, string> = { 'Content-Type': 'application/octet-stream' };
    const token: string = TokenStore.getAccessToken();
    if (token.length > 0) {
      header['Authorization'] = `Bearer ${token}`;
    }
    return header;
  }

  /**
   * 一组分片的**实际字节数**。
   * 末尾片通常不满 `chunk_size`，若按 `片数 × chunk_size` 估算，7MB 文件会算出「已传 10MB / 总 7MB」
   * （本卡实测出过 142% 的进度），故逐片按真实区间长度累加。
   */
  private static completedBytes(indexes: Set<number>, chunkSize: number, totalBytes: number): number {
    let sum: number = 0;
    indexes.forEach((index: number) => {
      const begins: number = index * chunkSize;
      sum += Math.max(0, Math.min(begins + chunkSize, totalBytes) - begins);
    });
    return Math.min(sum, totalBytes);
  }

  /** 人读体积文案（校验提示用） */
  static humanSize(bytes: number): string {
    if (bytes < 1024) {
      return `${bytes} B`;
    }
    if (bytes < 1024 * 1024) {
      return `${(bytes / 1024).toFixed(1)} KB`;
    }
    if (bytes < 1024 * 1024 * 1024) {
      return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
    }
    return `${(bytes / (1024 * 1024 * 1024)).toFixed(2)} GB`;
  }
}

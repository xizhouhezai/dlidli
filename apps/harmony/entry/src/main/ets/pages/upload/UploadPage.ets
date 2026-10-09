import { common } from '@kit.AbilityKit';
import { DliFont, DliRadius, DliSize, DliSpace } from '../../common/constants/Theme';
import { DliMaterial } from '../../common/constants/MaterialTokens';
import { AppRouter } from '../../common/utils/AppRouter';
import { Logger } from '../../common/utils/Logger';
import { showToast } from '../../common/utils/Toast';
import { MediaLib } from '../../common/utils/MediaLib';
import { ApiError } from '../../model/Api';
import {
  CoverDraft, DESC_MAX, PART_MAX, PART_TITLE_MAX, PartDraft, PickedVideo, SubmitDraft, SubmitReq,
  SubmitResult, TAGS_MAX, TITLE_MAX, UploadCheckResult, UploadProgressView, UploadResult, baseNameOf,
  draftHasContent, emptyDraft, emptyPart, extraPartLabel, idleProgress, normalizeTags,
  resolveSubmitFileId, resolveSubmitParts, validateDraft
} from '../../model/Upload';
import { Category } from '../../model/Video';
import { UploadController } from '../../service/UploadController';
import { CoverService } from '../../service/CoverService';
import { VideoApi } from '../../service/VideoApi';
import { BackgroundTransfer } from '../../service/BackgroundTransfer';
import { SessionReport, UploadDraftStore } from '../../store/UploadDraftStore';
import { AppContext } from '../../store/AppContext';
import { session } from '../../store/Session';

const TAG: string = 'UploadPage';

/**
 * 投稿页：选片 → 分片上传拿 file_id（HMY-50）→ 稿件信息与封面（HMY-51）→ 多P 与草稿（HMY-52）→ 提交拿 bvid。
 *
 * 表单与 Web `UploadView.vue` 同口径：标题 ≤80、简介 ≤2000、标签 1~10、分区必选、类型 1 自制 / 2 转载、
 * **分P 最多 10**（`PART_MAX`，与 Web `useUploadParts` 同一数字；后端实测无数量 binding，见 model/Upload.ets）。
 * 与 Web 的一处刻意差异：Web 用 `maxlength` 硬截断，端侧**不截断而是拦下并说明原因**
 * （验收要求「标题超长」等分支给出明确提示且不发请求），故输入框保留超长输入、提交前统一校验。
 *
 * **主视频即分P1**：`parts` 只装额外分P，提交时由 `resolveSubmitParts` 合成。这是必需的——
 * 后端 `Submit` 只要 `parts` 非空就走多P分支，该分支**只按 parts 建 `video_part` 与视频流，
 * `file_id` 仅用于归属校验、其流不入库**（实测 n=3 时主视频被整体丢弃）。故单P投稿必须让 `parts` 为空数组，
 * 否则会把主视频投丢。
 *
 * 草稿（HMY-52）：表单字段 + 各分P 的 `file_id`/上传会话 ID + 封面路径存 `preferences`，
 * 退出重进可恢复；恢复时用 `GET /upload/{id}` 校验会话有效性，失效的分P 明确提示重选文件。
 */
@ComponentV2
export struct UploadPage {
  // ---- 主视频（分P1）----
  @Local picked: PickedVideo | null = null;
  @Local fileId: string = '';
  @Local progress: UploadProgressView = idleProgress();
  @Local running: boolean = false;
  @Local errorText: string = '';

  // ---- 稿件信息（HMY-51）----
  @Local title: string = '';
  @Local description: string = '';
  @Local categoryId: number = 0;
  @Local copyright: number = 1;
  @Local tags: string[] = [];
  @Local tagInput: string = '';
  @Local categories: Category[] = [];
  @Local categoriesFailed: boolean = false;
  @Local cover: CoverDraft | null = null;
  @Local coverBusy: boolean = false;
  @Local submitting: boolean = false;
  @Local submitted: SubmitResult | null = null;

  // ---- 多P（HMY-52）----
  @Local parts: PartDraft[] = [];
  /** 正在上传的分P下标；-1 表示没有分P在上传（与主视频的 `running` 分开，两者可各自独立） */
  @Local partUploading: number = -1;
  @Local partProgress: UploadProgressView = idleProgress();
  /** 会话已失效、须重选文件的分P 编号（校验后置入，重选文件后清除） */
  @Local expiredParts: string[] = [];
  /** 会话仍有效、可续传的分P 编号 */
  @Local resumableParts: string[] = [];
  /** 已恢复草稿（用于页面顶部提示条） */
  @Local draftRestored: boolean = false;

  /** 主视频的上传会话 ID（收口后清空；草稿持久化用，不参与渲染） */
  private mainUploadId: string = '';
  /** 页面是否已销毁：销毁后在飞的异步回调不再弹提示（否则会在别的页面上闪一条错误） */
  private disposed: boolean = false;

  private controller: UploadController = new UploadController(
    (view: UploadProgressView) => {
      this.progress = view;
    },
    (uploadId: string) => {
      // 会话一经建立就落盘：此刻失败/退出才留有可续传的凭据
      this.mainUploadId = uploadId;
      this.persist();
    });
  private backgroundTask: BackgroundTransfer = new BackgroundTransfer();

  aboutToAppear(): void {
    this.loadCategories();
    this.restoreDraft();
  }

  aboutToDisappear(): void {
    this.disposed = true;
    // 退出即落盘：这是「投稿中退出可恢复」的最后一道保险
    this.persist();
    // 页面销毁必须释放长时任务与在飞分片，否则上传任务会泄漏到下一次投稿
    this.controller.cancel();
    this.backgroundTask.stop(AppContext.require());
  }

  private context(): common.UIAbilityContext {
    return AppContext.require();
  }

  // ---------- 草稿：恢复 / 持久化（HMY-52） ----------

  /**
   * 重进投稿页时读草稿并询问是否恢复（spec：**WHEN** 重新进入 **THE SYSTEM SHALL** 提供恢复或丢弃的选择）。
   * 无草稿或草稿为空则静默跳过，不打扰只想投新稿的用户。
   */
  private async restoreDraft(): Promise<void> {
    let saved: SubmitDraft | null = null;
    try {
      saved = await UploadDraftStore.load();
    } catch (err) {
      Logger.warn(TAG, `读取草稿失败：${(err as Error).message}`);
      return;
    }
    if (saved === null || !draftHasContent(saved)) {
      return;
    }
    const draft: SubmitDraft = saved;
    this.getUIContext().showAlertDialog({
      title: $r('app.string.upload_draft_title'),
      message: $r('app.string.upload_draft_body'),
      autoCancel: false,
      primaryButton: {
        value: $r('app.string.upload_draft_discard'),
        action: () => {
          UploadDraftStore.clear();
          Logger.info(TAG, '用户选择丢弃草稿');
        }
      },
      secondaryButton: {
        value: $r('app.string.upload_draft_restore'),
        action: () => {
          this.applyDraft(draft);
        }
      }
    });
  }

  /** 把草稿灌回界面状态；盘上已失效的封面文件不复活（改由用户重选） */
  private applyDraft(draft: SubmitDraft): void {
    this.title = draft.title;
    this.description = draft.description;
    this.categoryId = draft.categoryId;
    this.copyright = draft.copyright === 2 ? 2 : 1;
    this.tags = draft.tags;
    this.fileId = draft.fileId;
    this.mainUploadId = draft.uploadId;
    this.parts = draft.parts;
    this.cover = restoreCover(draft);
    this.draftRestored = true;
    Logger.info(TAG, `草稿已恢复：标题 ${draft.title.length} 字、额外分P ${draft.parts.length} 个`);
    this.checkSessions(draft);
  }

  /**
   * 校验草稿里各分P 的上传会话（HMY-52 硬性要求：**不要让用户提交后才失败**）。
   * 判定口径全在 `UploadDraftStore.checkSessions`（只查「有会话无 file_id」的条目；30001 才算失效）。
   */
  private async checkSessions(draft: SubmitDraft): Promise<void> {
    const report: SessionReport = await UploadDraftStore.checkSessions(draft);
    if (this.disposed) {
      return;
    }
    this.expiredParts = report.expired;
    this.resumableParts = report.resumable;
    if (report.networkFailed) {
      this.failWith($r('app.string.upload_draft_network'));
      return;
    }
    if (report.expired.length > 0) {
      // 走 toast 而非 errorText：errorText 是 `string`，塞 Resource 就得强转成 `[object Object]`；
      // 而「哪个分P 失效」的可操作信息由每行的 `已失效，请重选文件` 标签承载，比一段总述更准。
      showToast(this.getUIContext(), $r('app.string.upload_draft_expired', report.expired.join('、')));
      return;
    }
    if (report.resumable.length > 0) {
      showToast(this.getUIContext(), $r('app.string.upload_draft_resumable', report.resumable.join('、')));
    }
  }

  /** 把当前界面状态收成草稿并落盘（全空则不写，避免重进时弹无意义的恢复询问） */
  private persist(): void {
    const draft: SubmitDraft = this.currentDraft();
    if (!draftHasContent(draft)) {
      return;
    }
    UploadDraftStore.save(draft).catch((err: Object) => {
      Logger.warn(TAG, `草稿保存失败：${JSON.stringify(err)}`);
    });
  }

  private currentDraft(): SubmitDraft {
    const draft: SubmitDraft = emptyDraft();
    draft.title = this.title;
    draft.description = this.description;
    draft.categoryId = this.categoryId;
    draft.tags = this.tags;
    draft.copyright = this.copyright;
    draft.parts = this.parts;
    draft.fileId = this.fileId;
    draft.uploadId = this.mainUploadId;
    draft.coverPath = this.cover === null ? '' : this.cover.sandboxPath;
    draft.coverCacheUri = this.cover === null ? '' : this.cover.cacheUri;
    draft.coverOrigin = this.cover === null ? '' : this.cover.origin;
    draft.coverWidth = this.cover === null ? 0 : this.cover.width;
    draft.coverHeight = this.cover === null ? 0 : this.cover.height;
    draft.coverUrl = this.cover === null ? '' : this.cover.url;
    return draft;
  }

  // ---------- 主视频分片上传（HMY-50 链路，本卡不改流程） ----------

  private async pick(): Promise<void> {
    try {
      const picked: PickedVideo | null = await MediaLib.pickVideo(this.context());
      if (picked === null) {
        return;
      }
      const check: UploadCheckResult = await UploadController.check(this.context(), picked);
      if (!check.ok) {
        if (check.reason === 'network') {
          // HMY-54：移动网络不静默上传大文件，须用户确认后才开始
          this.confirmCellular(picked, check.message);
          return;
        }
        this.fail(check.message);
        return;
      }
      this.adoptMain(picked);
    } catch (err) {
      this.fail((err as Error).message);
      Logger.error(TAG, `选片失败：${(err as Error).message}`);
    }
  }

  /** 选定主视频：换片即作废上一次的 file_id 与会话（旧会话留在后端自然过期，不误用） */
  private adoptMain(picked: PickedVideo): void {
    this.picked = picked;
    this.fileId = '';
    this.mainUploadId = '';
    this.errorText = '';
    // 与 Web 一致：标题默认取文件名（去扩展名），用户可改
    if (this.title.trim().length === 0) {
      this.title = baseNameOf(picked.fileName).substring(0, TITLE_MAX);
    }
    Logger.info(TAG, `已选片：${picked.fileName} (${UploadController.humanSize(picked.size)})`);
    this.persist();
  }

  /** 移动网络二次确认：确认后照常上传（HMY-54 要求提示并可继续，不是禁止） */
  private confirmCellular(picked: PickedVideo, message: string): void {
    this.getUIContext().showAlertDialog({
      title: $r('app.string.upload_cellular_title'),
      message: message,
      autoCancel: true,
      primaryButton: {
        value: $r('app.string.action_cancel'),
        action: () => {}
      },
      secondaryButton: {
        value: $r('app.string.upload_cellular_confirm'),
        action: () => {
          this.adoptMain(picked);
          this.start();
        }
      }
    });
  }

  private async start(): Promise<void> {
    const picked: PickedVideo | null = this.picked;
    if (picked === null || this.running) {
      return;
    }
    if (!session.loggedIn) {
      this.fail('请先登录后再投稿');
      return;
    }
    this.running = true;
    this.errorText = '';
    const context: common.UIAbilityContext = this.context();
    await this.backgroundTask.start(context);
    try {
      const done: UploadResult = await this.controller.start(context, picked);
      this.fileId = done.fileId;
      // 收口后后端已删除该会话，留着它会让草稿恢复时把「已完成」误判成「已失效」
      this.mainUploadId = '';
      this.persist();
      // 默认封面候选：视频第 1 秒画面（与 Web 的 capturePoster 同位；失败静默，不挡投稿）
      if (this.cover === null) {
        await this.captureFrame(picked, false);
      }
      // 收口后清掉缓存副本，避免大文件长期占 cache；已传内容都在后端，不影响续传语义
      MediaLib.clearStaged(context, picked);
      Logger.info(TAG, `上传完成：file_id=${done.fileId} 秒传=${done.fast}`);
      showToast(this.getUIContext(), done.fast
        ? $r('app.string.upload_fast_done') : $r('app.string.upload_done'));
    } catch (err) {
      if (!this.disposed) {
        if (err instanceof ApiError) {
          this.fail((err as ApiError).message);
          Logger.error(TAG, `上传失败：${(err as ApiError).describe()}`);
        } else {
          this.fail((err as Error).message);
          Logger.error(TAG, `上传失败：${(err as Error).message}`);
        }
      }
      // 失败时把会话写进草稿：重进可校验有效性并按同一文件续传
      this.persist();
    } finally {
      this.running = false;
      await this.backgroundTask.stop(context);
    }
  }

  // ---------- 分P 管理（HMY-52） ----------

  /** 添加一个空分P；已达上限给出明确提示（`PART_MAX` 与 Web 同一口径） */
  private addPart(): void {
    if (this.parts.length + 1 >= PART_MAX) {
      this.failWith(`最多 ${PART_MAX} 个分P`);
      return;
    }
    this.clearError();
    this.parts = this.parts.concat([emptyPart()]);
    this.persist();
  }

  private removePart(index: number): void {
    if (index < 0 || index >= this.parts.length) {
      return;
    }
    this.clearError();
    // 删掉的分P 若已上传，其 file_id 就此弃用（后端 upload_file 记录保留，不作清理）
    this.parts = this.parts.filter((_: PartDraft, at: number) => at !== index);
    this.dropPartMarks(index);
    this.persist();
  }

  /**
   * 分P 被删除后，其「已失效/可续传」编号标记要一并摘掉。
   *
   * 注意其余行的标记**不重排**：标记是「校验时那一轮」的结论，删除一行并不改变别的行
   * 曾用哪个会话被校验过；重排反而会把结论挂到没校验过的行上。被删行之后的标记随
   * 下标前移而暂时不对应，但重新校验（重进页面）即整体刷新，不做本地猜测。
   */
  private dropPartMarks(removedIndex: number): void {
    const removedLabel: string = extraPartLabel(removedIndex);
    this.expiredParts = this.expiredParts.filter((label: string) => label !== removedLabel);
    this.resumableParts = this.resumableParts.filter((label: string) => label !== removedLabel);
  }

  private setPartTitle(index: number, value: string): void {
    if (index < 0 || index >= this.parts.length) {
      return;
    }
    const next: PartDraft[] = this.parts.slice();
    next[index] = {
      title: value,
      fileId: next[index].fileId,
      fileName: next[index].fileName,
      uploadId: next[index].uploadId
    };
    this.parts = next;
  }

  /**
   * 为某个分P 选片并**立即上传**（与 Web `uploadPart` 同时机：选完就传，不额外给一次「开始」）。
   *
   * 并发口径：**同一时刻只允许一个分P 在上传**（`partUploading` 门闩），且主视频上传中（`running`）
   * 也不允许起分P——这既避开了「密集齐发 uploadFile 触发 appfreeze」的既有坑（HMY-50 实测），
   * 也让进度只有一个来源。每个分P 用**自己的** `UploadController`：分P 的会话 ID 必须与它所属的
   * 那一行绑定，共用一个实例会让会话回调分不清是谁的会话。
   */
  private async pickPart(index: number): Promise<void> {
    if (this.partUploading >= 0) {
      this.failWith('有分P 正在上传，请稍候');
      return;
    }
    let picked: PickedVideo | null = null;
    try {
      picked = await MediaLib.pickVideo(this.context());
    } catch (err) {
      this.fail((err as Error).message);
      return;
    }
    if (picked === null) {
      return;
    }
    const check: UploadCheckResult = await UploadController.check(this.context(), picked);
    if (!check.ok && check.reason !== 'network') {
      this.fail(check.message);
      return;
    }
    if (check.reason === 'network') {
      // 与主视频同口径（HMY-54）：移动网络下先确认再传，不静默消耗流量
      this.confirmPartCellular(index, picked, check.message);
      return;
    }
    this.runPartUpload(index, picked);
  }

  /** 分P 的移动网络二次确认 */
  private confirmPartCellular(index: number, picked: PickedVideo, message: string): void {
    this.getUIContext().showAlertDialog({
      title: $r('app.string.upload_cellular_title'),
      message: message,
      autoCancel: true,
      primaryButton: {
        value: $r('app.string.action_cancel'),
        action: () => {}
      },
      secondaryButton: {
        value: $r('app.string.upload_cellular_confirm'),
        action: () => {
          this.runPartUpload(index, picked);
        }
      }
    });
  }

  /** 分P 的实际上传（校验与二次确认都通过后进入） */
  private async runPartUpload(index: number, picked: PickedVideo): Promise<void> {
    this.clearError();
    this.partUploading = index;
    this.partProgress = idleProgress();
    const label: string = extraPartLabel(index);
    const context: common.UIAbilityContext = this.context();
    try {
      const running: UploadController = new UploadController(
        (view: UploadProgressView) => {
          this.partProgress = view;
        },
        (uploadId: string) => {
          // 分P 的会话 ID 写进草稿，供失败后重进校验（下标与闭包绑定的这一行一致）
          this.markPartSession(index, uploadId);
        });
      const done: UploadResult = await running.start(context, picked);
      this.markPartDone(index, picked, done.fileId);
      MediaLib.clearStaged(context, picked);
      Logger.info(TAG, `${label} 上传完成：file_id=${done.fileId}`);
      showToast(this.getUIContext(), done.fast
        ? $r('app.string.upload_fast_done') : $r('app.string.upload_done'));
    } catch (err) {
      if (!this.disposed) {
        this.fail(err instanceof ApiError
          ? `${label}：${(err as ApiError).message}` : `${label}：${(err as Error).message}`);
      }
      // 失败也留下文件名与会话：重进时能校验该会话是否还能续传
      this.markPartPicked(index, picked);
    } finally {
      this.partUploading = -1;
      this.persist();
    }
  }

  /** 分P 选好文件（上传开始前/失败后）：记住文件名，清掉该行的失效标记 */
  private markPartPicked(index: number, picked: PickedVideo): void {
    const next: PartDraft[] = this.parts.slice();
    if (index >= next.length) {
      return;
    }
    next[index] = {
      title: next[index].title.length > 0
        ? next[index].title : baseNameOf(picked.fileName).substring(0, PART_TITLE_MAX),
      fileId: next[index].fileId,
      fileName: picked.fileName,
      uploadId: next[index].uploadId
    };
    this.parts = next;
    this.clearPartMark(index, 'expired');
  }

  private markPartSession(index: number, uploadId: string): void {
    const next: PartDraft[] = this.parts.slice();
    if (index >= next.length) {
      return;
    }
    next[index] = {
      title: next[index].title,
      fileId: next[index].fileId,
      fileName: next[index].fileName,
      uploadId: uploadId
    };
    this.parts = next;
    this.persist();
  }

  private markPartDone(index: number, picked: PickedVideo, fileId: string): void {
    const next: PartDraft[] = this.parts.slice();
    if (index >= next.length) {
      return;
    }
    next[index] = {
      title: next[index].title.length > 0
        ? next[index].title : baseNameOf(picked.fileName).substring(0, PART_TITLE_MAX),
      fileId: fileId,
      fileName: picked.fileName,
      // 收口后会话已被后端删除，留着会被下次校验判成已失效
      uploadId: ''
    };
    this.parts = next;
    this.clearPartMark(index, 'expired');
    this.clearPartMark(index, 'resumable');
  }

  /** 清掉某分P 的编号型标记（重选文件后不再提示失效/续传） */
  private clearPartMark(index: number, which: string): void {
    const label: string = extraPartLabel(index);
    if (which === 'expired') {
      this.expiredParts = this.expiredParts.filter((item: string) => item !== label);
    } else {
      this.resumableParts = this.resumableParts.filter((item: string) => item !== label);
    }
  }

  private partBusy(index: number): boolean {
    return this.partUploading === index;
  }

  // ---------- 稿件信息（HMY-51） ----------

  private async loadCategories(): Promise<void> {
    try {
      const list: Category[] = await VideoApi.listCategories();
      // 投稿只落一级分区（与 Web `loadCategories` 的 `parent_id === 0` 过滤一致）
      this.categories = list.filter((item: Category) => item.parent_id === 0);
      this.categoriesFailed = false;
    } catch (err) {
      this.categoriesFailed = true;
      Logger.warn(TAG, `分区加载失败：${(err as Error).message}`);
    }
  }

  /** 标签入列：空输入与重复项直接挡下，超上限给明确提示（对齐 Web `addTag`） */
  private addTag(): void {
    const text: string = this.tagInput.trim();
    if (text.length === 0) {
      this.fail('标签不能为空，请先输入内容');
      return;
    }
    if (this.tags.includes(text)) {
      this.tagInput = '';
      return;
    }
    if (this.tags.length >= TAGS_MAX) {
      this.fail(`最多 ${TAGS_MAX} 个标签（当前 ${this.tags.length} 个）`);
      return;
    }
    this.clearError();
    this.tags = this.tags.concat([text]);
    this.tagInput = '';
    this.persist();
  }

  /** 截帧按钮入口：闭包里取 `this.picked` 拿不到收窄后的类型，故收窄在方法里做 */
  private onCaptureFrameTap(): void {
    const picked: PickedVideo | null = this.picked;
    if (picked !== null) {
      this.captureFrame(picked, true);
    }
  }

  private removeTag(tag: string): void {
    this.clearError();
    this.tags = this.tags.filter((item: string) => item !== tag);
    this.persist();
  }

  /** 相册选图：取消保持原封面；失败给提示 */
  private async pickPhotoCover(): Promise<void> {
    if (this.coverBusy) {
      return;
    }
    this.coverBusy = true;
    try {
      const draft: CoverDraft | null = await CoverService.fromPhoto(this.context());
      if (draft === null) {
        return;
      }
      CoverService.discard(this.cover);
      this.cover = draft;
      this.errorText = '';
      Logger.info(TAG, `封面来源=相册，裁切后 ${draft.width}×${draft.height}`);
      this.persist();
    } catch (err) {
      this.fail('选图失败，请重试');
      Logger.warn(TAG, `选图失败：${(err as Error).message}`);
    } finally {
      this.coverBusy = false;
    }
  }

  /** 视频截帧：`hintOnFail` 为真时（用户主动点击）失败要出提示，自动截帧失败静默 */
  private async captureFrame(picked: PickedVideo, hintOnFail: boolean): Promise<void> {
    if (this.coverBusy) {
      return;
    }
    this.coverBusy = true;
    try {
      const draft: CoverDraft | null = await CoverService.fromVideo(this.context(), picked);
      if (draft === null) {
        if (hintOnFail) {
          this.fail('该视频无法截帧，请改用相册选图');
        }
        return;
      }
      CoverService.discard(this.cover);
      this.cover = draft;
      Logger.info(TAG, `封面来源=截帧，裁切后 ${draft.width}×${draft.height}`);
      this.persist();
    } catch (err) {
      if (hintOnFail) {
        this.fail('截帧失败，请改用相册选图');
      }
      Logger.warn(TAG, `截帧失败：${(err as Error).message}`);
    } finally {
      this.coverBusy = false;
    }
  }

  /**
   * 提交：端侧先校验（不过不发请求）→ 需要时上传封面 → `POST /videos` 拿 bvid。
   * 成功即清掉本地封面、暂存视频与草稿（都已落到服务端），并切到成功态。
   *
   * 校验排在 file_id 之前：表单问题与文件是否传完是两回事，先报用户能改的那个，
   * 也让「校验分支」在没有完成上传时同样可见（本卡验收要求这几条提示能被实测到）。
   */
  private async submit(): Promise<void> {
    if (this.submitting) {
      return;
    }
    if (!session.loggedIn) {
      this.fail('请先登录后再投稿');
      return;
    }
    const draft: SubmitDraft = this.currentDraft();
    draft.tags = normalizeTags(draft.tags);
    const violation: string = validateDraft(draft, this.categoryIds());
    if (violation.length > 0) {
      // 校验分支：给出明确原因且**不发任何请求**
      this.fail(violation);
      Logger.info(TAG, `表单校验未通过，未发请求：${violation}`);
      return;
    }

    this.submitting = true;
    this.errorText = '';
    const context: common.UIAbilityContext = this.context();
    try {
      const coverUrl: string = await this.resolveCover(context);
      const parts = resolveSubmitParts(draft);
      const req: SubmitReq = {
        file_id: resolveSubmitFileId(draft),
        title: draft.title,
        description: draft.description,
        category_id: draft.categoryId,
        tags: draft.tags,
        copyright: draft.copyright,
        cover: coverUrl,
        parts: parts
      };
      const done: SubmitResult = await VideoApi.submit(req);
      this.submitted = done;
      CoverService.discard(this.cover);
      this.cover = null;
      if (this.picked !== null) {
        MediaLib.clearStaged(context, this.picked);
      }
      // 投稿成功：草稿使命已尽，必须清掉，否则下次进投稿页会提示恢复一份已提交的稿
      await UploadDraftStore.clear();
      Logger.info(TAG, `投稿成功：bvid=${done.bvid} status=${done.status} ` +
        `parts=${parts.length} cover=${coverUrl.length > 0 ? 'yes' : 'none'}`);
    } catch (err) {
      if (err instanceof ApiError) {
        this.fail((err as ApiError).message);
        Logger.error(TAG, `投稿失败：${(err as ApiError).describe()}`);
      } else {
        this.fail((err as Error).message);
        Logger.error(TAG, `投稿失败：${(err as Error).message}`);
      }
    } finally {
      this.submitting = false;
    }
  }

  /** 封面 URL：已传过则复用，否则此刻上传（本地预览在前、上传在后——HMY-51 的「先裁切预览再上传」） */
  private async resolveCover(context: common.UIAbilityContext): Promise<string> {
    const draft: CoverDraft | null = this.cover;
    if (draft === null) {
      return '';
    }
    if (draft.url.length > 0) {
      return draft.url;
    }
    const url: string = await CoverService.upload(context, draft);
    const uploaded: CoverDraft = {
      origin: draft.origin,
      cacheUri: draft.cacheUri,
      sandboxPath: draft.sandboxPath,
      width: draft.width,
      height: draft.height,
      url: url
    };
    this.cover = uploaded;
    return url;
  }

  private resetDraft(): void {
    CoverService.discard(this.cover);
    this.cover = null;
    this.picked = null;
    this.fileId = '';
    this.mainUploadId = '';
    this.progress = idleProgress();
    this.title = '';
    this.description = '';
    this.categoryId = 0;
    this.copyright = 1;
    this.tags = [];
    this.tagInput = '';
    this.errorText = '';
    this.submitted = null;
    this.parts = [];
    this.partUploading = -1;
    this.partProgress = idleProgress();
    this.expiredParts = [];
    this.resumableParts = [];
    this.draftRestored = false;
    UploadDraftStore.clear();
  }

  private fail(message: string): void {
    this.errorText = message;
    showToast(this.getUIContext(), message);
  }

  /**
   * 提示但**不进错误区**：用于「已恢复/可续传」这类正向或非阻塞信息。
   * 收 `ResourceStr` 而非 `string`——传资源对象时若调用点先 `as string` 强转，
   * 拿到的是对象在渲染层的 `[object Object]`（本项目在收藏夹弹层已踩过一次）。
   */
  private failWith(message: ResourceStr): void {
    showToast(this.getUIContext(), message);
  }

  /** 用户一动手就把上一条错误撤掉：留着旧提示会让人以为改完仍然不行 */
  private clearError(): void {
    if (this.errorText.length > 0) {
      this.errorText = '';
    }
  }

  private categoryIds(): number[] {
    return this.categories.map((item: Category) => item.id);
  }

  private coverUri(): string {
    return this.cover === null ? '' : `file://${this.cover.sandboxPath}`;
  }

  private coverWidth(): number {
    return this.cover === null ? 0 : this.cover.width;
  }

  private coverHeight(): number {
    return this.cover === null ? 0 : this.cover.height;
  }

  private originLabel(): ResourceStr {
    return this.cover !== null && this.cover.origin === 'photo'
      ? $r('app.string.upload_cover_origin_photo')
      : $r('app.string.upload_cover_origin_frame');
  }

  /**
   * 分P 行状态：上传中 > 已上传 > 会话失效 > 可续传 > 未选择文件。
   *
   * 刻意返回 `ResourceStr` 而不是拼字符串——失效/未选文件两种状态用字符串资源，
   * 若函数签名是 `string` 就得对 `$r()` 做 `as string` 强转，那是**错误**的：
   * `$r()` 返回 `Resource` 对象，强转后 `Text()` 会渲染成 `[object Object]`
   * （本项目在收藏夹弹层已踩过一次 `$r()` 拼字符串的问题）。
   */
  private partStatus(index: number): ResourceStr {
    const part: PartDraft = this.parts[index];
    const label: string = extraPartLabel(index);
    if (part.fileId.length > 0) {
      return `已上传 · ${part.fileName}`;
    }
    if (this.expiredParts.includes(label)) {
      return $r('app.string.upload_parts_expired_tag');
    }
    if (part.uploadId.length > 0 && this.resumableParts.includes(label)) {
      return `上次未传完（会话有效）· ${part.fileName}`;
    }
    if (part.fileName.length > 0) {
      return part.fileName;
    }
    return $r('app.string.upload_parts_no_file');
  }

  /** 分P 行是否处于「需要（重新）选文件」的状态 */
  private partNeedsFile(index: number): boolean {
    const part: PartDraft = this.parts[index];
    return part.fileId.length === 0 && part.uploadId.length === 0;
  }

  private submitLabel(): ResourceStr {
    if (this.submitting) {
      return $r('app.string.upload_submitting');
    }
    return resolveSubmitFileId(this.currentDraft()).length > 0
      ? $r('app.string.upload_submit') : $r('app.string.upload_wait_upload');
  }

  private canSubmit(): boolean {
    return resolveSubmitFileId(this.currentDraft()).length > 0 && !this.submitting;
  }

  private statusText(): ResourceStr {
    const status: number = this.submitted === null ? 0 : this.submitted.status;
    return status === 2 ? $r('app.string.upload_status_transcoding') : $r('app.string.upload_status_review');
  }

  private categoryName(): string {
    for (const item of this.categories) {
      if (item.id === this.categoryId) {
        return item.name;
      }
    }
    return '';
  }

  /** 阶段文案：上传/收口/秒传各有各的说法，统一在标题行展示 */
  private phaseText(): string {
    if (this.progress.phase === 'hashing') {
      return '正在校验文件';
    }
    if (this.progress.phase === 'completing') {
      return '正在合并文件';
    }
    if (this.progress.phase === 'done') {
      return this.progress.fast ? '秒传完成' : '上传完成';
    }
    if (this.running) {
      return '正在上传';
    }
    return this.picked === null ? '选择要投稿的视频' : '准备上传';
  }

  /**
   * 已传/总量（含分片数）。
   * 字节口径取整展示——上传中每 5MB 才跳一次，小数位没有信息量。
   */
  private bytesText(): string {
    const uploaded: string = UploadController.humanSize(this.progress.uploadedBytes);
    const total: string = UploadController.humanSize(this.progress.totalBytes);
    if (this.progress.totalParts > 1) {
      return `${uploaded} / ${total}（${this.progress.uploadedParts}/${this.progress.totalParts} 片）`;
    }
    return `${uploaded} / ${total}`;
  }

  // ---------- 视图 ----------

  @Builder
  pickCard() {
    Column({ space: DliSpace.SM }) {
      SymbolGlyph($r('sys.symbol.upload'))
        .fontSize(40)
        .fontColor([$r('app.color.brand_primary')])
      Text($r('app.string.upload_pick_hint'))
        .fontSize(DliFont.BODY)
        .fontColor($r('app.color.text_primary'))
      Text($r('app.string.upload_pick_rule'))
        .fontSize(DliFont.CAPTION)
        .fontColor($r('app.color.text_tertiary'))
        .textAlign(TextAlign.Center)
    }
    .width('100%')
    .padding(DliSpace.XL)
    .borderRadius(DliRadius.LG)
    .backgroundColor(DliMaterial.surface())
    .systemMaterial(DliMaterial.card())
    .onClick(() => {
      if (!this.running && this.partUploading < 0) {
        this.pick();
      }
    })
  }

  /** 恢复提示条：仅在本次确实恢复过草稿时出现，可手动关掉 */
  @Builder
  restoredBanner() {
    Row({ space: DliSpace.SM }) {
      Text('已恢复上次未提交的投稿草稿')
        .fontSize(DliFont.CAPTION)
        .fontColor($r('app.color.brand_primary'))
        .layoutWeight(1)
      Text('×')
        .fontSize(DliFont.BODY_SM)
        .fontColor($r('app.color.brand_primary'))
        .onClick(() => {
          this.draftRestored = false;
        })
    }
    .width('100%')
    .padding(DliSpace.MD)
    .borderRadius(DliRadius.MD)
    .backgroundColor($r('app.color.brand_primary_light'))
  }

  /** 进度卡：百分比 + 已传/总量（HMY-50 ③），续传/秒传另有标记 */
  @Builder
  progressCard() {
    Column({ space: DliSpace.SM }) {
      Row({ space: DliSpace.SM }) {
        Text(this.phaseText())
          .fontSize(DliFont.TITLE_SM)
          .fontWeight(FontWeight.Medium)
          .fontColor($r('app.color.text_primary'))
          .layoutWeight(1)
        if (this.progress.resumed) {
          Text($r('app.string.upload_resumed_tag'))
            .fontSize(DliFont.CAPTION)
            .fontColor($r('app.color.brand_primary'))
            .padding({ left: 6, right: 6, top: 1, bottom: 1 })
            .borderRadius(DliRadius.SM)
            .backgroundColor($r('app.color.brand_primary_light'))
        }
        if (this.progress.fast) {
          Text($r('app.string.upload_fast_tag'))
            .fontSize(DliFont.CAPTION)
            .fontColor($r('app.color.brand_primary'))
            .padding({ left: 6, right: 6, top: 1, bottom: 1 })
            .borderRadius(DliRadius.SM)
            .backgroundColor($r('app.color.brand_primary_light'))
        }
        Text(`${this.progress.percent}%`)
          .fontSize(DliFont.TITLE_SM)
          .fontColor($r('app.color.brand_primary'))
      }
      .width('100%')

      // 自绘进度条而不用 Progress：本 SDK 的 Progress 无 showDefaultPercentage，
      // 未设 style 时会在轨道上叠出内置的 `10.000000` 文案（实测），自绘可控且与主题一致
      Row() {
        Row()
          .width(`${this.progress.percent}%`)
          .height('100%')
          .borderRadius(DliRadius.FULL)
          .backgroundColor($r('app.color.brand_primary'))
      }
      .width('100%')
      .height(6)
      .borderRadius(DliRadius.FULL)
      .backgroundColor($r('app.color.surface_secondary'))

      Row() {
        Text(this.picked === null ? '' : this.picked.fileName)
          .fontSize(DliFont.CAPTION)
          .fontColor($r('app.color.text_tertiary'))
          .layoutWeight(1)
          .maxLines(1)
          .textOverflow({ overflow: TextOverflow.Ellipsis })
        Text(this.bytesText())
          .fontSize(DliFont.CAPTION)
          .fontColor($r('app.color.text_tertiary'))
      }
      .width('100%')
    }
    .width('100%')
    .padding(DliSpace.LG)
    .borderRadius(DliRadius.LG)
    .backgroundColor(DliMaterial.surface())
    .systemMaterial(DliMaterial.card())
  }

  /**
   * 字段名（左侧灰字）。刻意不带参数：`@Builder` 的**值类型参数不参与刷新**——
   * 早先版本把「计数/已选分区/封面来源」当参数传给带参 @Builder，10 个标签加完计数仍显示 `0/10`（实测），
   * 故所有这些动态文案都改到**无参 @Builder** 里直接读 `@Local`（无参 Builder 随组件重渲染重跑）。
   */
  @Builder
  fieldName(label: ResourceStr) {
    Text(label)
      .fontSize(DliFont.BODY_SM)
      .fontColor($r('app.color.text_secondary'))
  }

  @Builder
  titleRow() {
    Row() {
      this.fieldName($r('app.string.upload_form_title'))
      Blank()
      Text(`${this.title.length}/${TITLE_MAX}`)
        .fontSize(DliFont.CAPTION)
        .fontColor(this.title.length > TITLE_MAX ? $r('app.color.state_danger') : $r('app.color.text_tertiary'))
    }
    .width('100%')
  }

  /** 分区行：右侧回显已选分区名 */
  @Builder
  categoryRow() {
    Row() {
      this.fieldName($r('app.string.upload_form_category'))
      Blank()
      Text(this.categoryId > 0 ? this.categoryName() : '')
        .fontSize(DliFont.CAPTION)
        .fontColor($r('app.color.text_tertiary'))
    }
    .width('100%')
  }

  @Builder
  tagsRow() {
    Row() {
      this.fieldName($r('app.string.upload_form_tags'))
      Blank()
      Text(`${this.tags.length}/${TAGS_MAX}`)
        .fontSize(DliFont.CAPTION)
        .fontColor(this.tags.length > TAGS_MAX ? $r('app.color.state_danger') : $r('app.color.text_tertiary'))
    }
    .width('100%')
  }

  @Builder
  descRow() {
    Row() {
      this.fieldName($r('app.string.upload_form_desc'))
      Blank()
      Text(`${this.description.length}/${DESC_MAX}`)
        .fontSize(DliFont.CAPTION)
        .fontColor(this.description.length > DESC_MAX ? $r('app.color.state_danger') : $r('app.color.text_tertiary'))
    }
    .width('100%')
  }

  /** 封面行：右侧回显来源（截帧/相册） */
  @Builder
  coverRow() {
    Row() {
      this.fieldName($r('app.string.upload_form_cover'))
      Blank()
      if (this.cover !== null) {
        Text(this.originLabel())
          .fontSize(DliFont.CAPTION)
          .fontColor($r('app.color.text_tertiary'))
      }
    }
    .width('100%')
  }

  /**
   * 分P 管理区（HMY-52）。
   * 主视频即分P1（标题取稿件标题，不可在此单独命名），额外分P 逐个增删、各自选文件与上传。
   * 封面只有一张（后端 `SubmitReq` 只有一个 `cover`，分P 的封面沿用主视频封面）——
   * 这是接口的现实，不在端侧假装每P 都能配封面。
   */
  @Builder
  partsSection() {
    Column({ space: DliSpace.MD }) {
      Row() {
        Text($r('app.string.upload_parts_section'))
          .fontSize(DliFont.TITLE_SM)
          .fontWeight(FontWeight.Medium)
          .fontColor($r('app.color.text_primary'))
        Blank()
        Text(`${this.parts.length + 1}/${PART_MAX}`)
          .fontSize(DliFont.CAPTION)
          .fontColor($r('app.color.text_tertiary'))
      }
      .width('100%')

      Text($r('app.string.upload_parts_tip'))
        .fontSize(DliFont.CAPTION)
        .fontColor($r('app.color.text_tertiary'))
        .width('100%')

      // 分P1 = 主视频（只读回显，选片与上传仍在页面顶部）
      Row({ space: DliSpace.SM }) {
        Text($r('app.string.upload_parts_label_main'))
          .fontSize(DliFont.BODY_SM)
          .fontColor($r('app.color.text_primary'))
          .layoutWeight(1)
          .maxLines(1)
          .textOverflow({ overflow: TextOverflow.Ellipsis })
        Text(this.fileId.length > 0
          ? (this.picked === null ? '已上传' : `已上传 · ${this.picked.fileName}`)
          : '未上传')
          .fontSize(DliFont.CAPTION)
          .fontColor(this.fileId.length > 0 ? $r('app.color.brand_primary') : $r('app.color.text_tertiary'))
      }
      .width('100%')
      .padding(DliSpace.MD)
      .borderRadius(DliRadius.MD)
      .backgroundColor($r('app.color.surface_secondary'))

      // 额外分P 列表
      ForEach(this.parts, (part: PartDraft, index: number) => {
        Column({ space: DliSpace.SM }) {
          Row({ space: DliSpace.SM }) {
            Text(extraPartLabel(index))
              .fontSize(DliFont.BODY_SM)
              .fontColor($r('app.color.text_primary'))
            Text(this.partStatus(index))
              .fontSize(DliFont.CAPTION)
              .fontColor(part.fileId.length > 0
                ? $r('app.color.brand_primary') : $r('app.color.text_tertiary'))
              .layoutWeight(1)
              .maxLines(1)
              .textOverflow({ overflow: TextOverflow.Ellipsis })
            Text($r('app.string.upload_parts_remove'))
              .fontSize(DliFont.CAPTION)
              .fontColor($r('app.color.state_danger'))
              .onClick(() => {
                if (!this.partBusy(index)) {
                  this.removePart(index);
                }
              })
          }
          .width('100%')

          TextInput({ text: part.title, placeholder: $r('app.string.upload_parts_title_hint') })
            .fontSize(DliFont.BODY_SM)
            .maxLines(1)
            .onChange((value: string) => {
              this.setPartTitle(index, value);
            })

          if (this.partBusy(index)) {
            Row() {
              Row() {
                Row()
                  .width(`${this.partProgress.percent}%`)
                  .height('100%')
                  .borderRadius(DliRadius.FULL)
                  .backgroundColor($r('app.color.brand_primary'))
              }
              .width('100%')
              .height(6)
              .borderRadius(DliRadius.FULL)
              .backgroundColor($r('app.color.surface_secondary'))
            }
            .width('100%')
            Text(`${this.partProgress.percent}% · ${UploadController.humanSize(this.partProgress.uploadedBytes)} / ${UploadController.humanSize(this.partProgress.totalBytes)}`)
              .fontSize(DliFont.CAPTION)
              .fontColor($r('app.color.text_tertiary'))
              .width('100%')
          } else {
            Row({ space: DliSpace.SM }) {
              Text(part.fileId.length > 0
                ? '重新选择视频（重新上传）' : $r('app.string.upload_parts_choose'))
                .fontSize(DliFont.BODY_SM)
                .fontColor($r('app.color.text_primary'))
                .padding({ left: DliSpace.LG, right: DliSpace.LG, top: DliSpace.SM, bottom: DliSpace.SM })
                .borderRadius(DliRadius.FULL)
                .backgroundColor($r('app.color.surface_secondary'))
                .onClick(() => {
                  if (this.partUploading < 0 && !this.running) {
                    this.pickPart(index);
                  }
                })
            }
            .width('100%')
          }
        }
        .width('100%')
        .padding(DliSpace.MD)
        .borderRadius(DliRadius.MD)
        .backgroundColor(DliMaterial.surface())
        .systemMaterial(DliMaterial.card())
        // key 必须含 title：TextInput 的 `text` 只在组件**首次构造**时取值，键不变时 ArkUI 复用旧组件，
        // 恢复回来的分P 标题就显示不出来（`fileId`/`fileName` 都为空、只有 `title` 变了）。
        // 同类缺陷本项目已踩过两次（HMY-54 投稿卡 key 需含 `status`、HMY-08 评论 key 需含 `like_cnt`）。
      }, (part: PartDraft, index: number) => `${index}:${part.fileId}:${part.fileName}:${part.title}`)

      // 加号按钮：到上限即置灰不可点
      Row() {
        Text($r('app.string.upload_parts_add'))
          .fontSize(DliFont.BODY_SM)
          .fontColor(this.parts.length + 1 >= PART_MAX
            ? $r('app.color.text_tertiary') : $r('app.color.brand_primary'))
      }
      .width('100%')
      .height(40)
      .justifyContent(FlexAlign.Center)
      .borderRadius(DliRadius.FULL)
      .backgroundColor($r('app.color.brand_primary_light'))
      .onClick(() => {
        if (this.parts.length + 1 < PART_MAX && this.partUploading < 0 && !this.running) {
          this.addPart();
        }
      })
    }
    .width('100%')
    .padding(DliSpace.LG)
    .borderRadius(DliRadius.LG)
    .backgroundColor(DliMaterial.surface())
    .systemMaterial(DliMaterial.card())
  }

  @Builder
  formSection() {
    Column({ space: DliSpace.MD }) {
      Text($r('app.string.upload_form_section'))
        .fontSize(DliFont.TITLE_SM)
        .fontWeight(FontWeight.Medium)
        .fontColor($r('app.color.text_primary'))
        .width('100%')

      // 标题：不设 maxLength，超长在提交时拦下并说明（见类注释）
      this.titleRow()
      TextInput({ text: $$this.title, placeholder: $r('app.string.upload_form_title_hint') })
        .fontSize(DliFont.BODY)
        .maxLines(1)
        .onChange(() => {
          this.clearError();
        })

      // 分区
      this.categoryRow()
      if (this.categoriesFailed) {
        Text($r('app.string.upload_form_category_fail'))
          .fontSize(DliFont.BODY_SM)
          .fontColor($r('app.color.state_danger'))
          .onClick(() => {
            this.loadCategories();
          })
      } else if (this.categories.length === 0) {
        LoadingProgress()
          .width(20)
          .height(20)
          .color($r('app.color.brand_primary'))
      } else {
        Flex({ wrap: FlexWrap.Wrap }) {
          ForEach(this.categories, (item: Category) => {
            Text(item.name)
              .fontSize(DliFont.BODY_SM)
              .fontColor(this.categoryId === item.id
                ? $r('app.color.text_on_brand') : $r('app.color.text_secondary'))
              .padding({ left: DliSpace.MD, right: DliSpace.MD, top: 6, bottom: 6 })
              .margin({ right: DliSpace.SM, bottom: DliSpace.SM })
              .borderRadius(DliRadius.FULL)
              .backgroundColor(this.categoryId === item.id
                ? $r('app.color.brand_primary') : $r('app.color.surface_secondary'))
              .onClick(() => {
                this.clearError();
                this.categoryId = item.id;
                this.persist();
              })
          }, (item: Category) => `${item.id}`)
        }
        .width('100%')
      }

      // 标签
      this.tagsRow()
      if (this.tags.length > 0) {
        Flex({ wrap: FlexWrap.Wrap }) {
          ForEach(this.tags, (tag: string) => {
            Row({ space: DliSpace.XS }) {
              Text(tag)
                .fontSize(DliFont.BODY_SM)
                .fontColor($r('app.color.brand_primary'))
              Text('×')
                .fontSize(DliFont.BODY_SM)
                .fontColor($r('app.color.brand_primary'))
            }
            .padding({ left: DliSpace.MD, right: DliSpace.MD, top: 6, bottom: 6 })
            .margin({ right: DliSpace.SM, bottom: DliSpace.SM })
            .borderRadius(DliRadius.FULL)
            .backgroundColor($r('app.color.brand_primary_light'))
            .onClick(() => {
              this.removeTag(tag);
            })
          }, (tag: string) => tag)
        }
        .width('100%')
      }
      Row({ space: DliSpace.SM }) {
        TextInput({ text: $$this.tagInput, placeholder: $r('app.string.upload_form_tag_hint') })
          .fontSize(DliFont.BODY_SM)
          .maxLines(1)
          .layoutWeight(1)
          .enterKeyType(EnterKeyType.Done)
          .onSubmit(() => {
            this.addTag();
          })
        Text($r('app.string.upload_form_tag_add'))
          .fontSize(DliFont.BODY_SM)
          .fontColor($r('app.color.text_on_brand'))
          .padding({ left: DliSpace.LG, right: DliSpace.LG, top: DliSpace.SM, bottom: DliSpace.SM })
          .borderRadius(DliRadius.FULL)
          .backgroundColor($r('app.color.brand_primary'))
          .onClick(() => {
            this.addTag();
          })
      }
      .width('100%')

      // 类型（copyright 1 自制 / 2 转载）
      this.fieldName($r('app.string.upload_form_type'))
      Row({ space: DliSpace.SM }) {
        Text($r('app.string.upload_form_copyright_self'))
          .fontSize(DliFont.BODY_SM)
          .fontColor(this.copyright === 1
            ? $r('app.color.text_on_brand') : $r('app.color.text_secondary'))
          .padding({ left: DliSpace.XL, right: DliSpace.XL, top: 6, bottom: 6 })
          .borderRadius(DliRadius.FULL)
          .backgroundColor(this.copyright === 1
            ? $r('app.color.brand_primary') : $r('app.color.surface_secondary'))
          .onClick(() => {
            this.clearError();
            this.copyright = 1;
            this.persist();
          })
        Text($r('app.string.upload_form_copyright_repost'))
          .fontSize(DliFont.BODY_SM)
          .fontColor(this.copyright === 2
            ? $r('app.color.text_on_brand') : $r('app.color.text_secondary'))
          .padding({ left: DliSpace.XL, right: DliSpace.XL, top: 6, bottom: 6 })
          .borderRadius(DliRadius.FULL)
          .backgroundColor(this.copyright === 2
            ? $r('app.color.brand_primary') : $r('app.color.surface_secondary'))
          .onClick(() => {
            this.clearError();
            this.copyright = 2;
            this.persist();
          })
      }
      .width('100%')

      // 简介
      this.descRow()
      TextArea({ text: $$this.description, placeholder: $r('app.string.upload_form_desc_hint') })
        .fontSize(DliFont.BODY_SM)
        .height(96)
        .onChange(() => {
          this.clearError();
        })

      // 封面：本地已按 16:9 裁切，此处按同一比例预览
      this.coverRow()
      if (this.cover !== null) {
        Image(this.coverUri())
          .width('100%')
          .aspectRatio(16 / 9)
          .objectFit(ImageFit.Cover)
          .borderRadius(DliRadius.MD)
      } else {
        Row() {
          Text($r('app.string.upload_cover_empty'))
            .fontSize(DliFont.BODY_SM)
            .fontColor($r('app.color.text_tertiary'))
        }
        .width('100%')
        .aspectRatio(16 / 9)
        .justifyContent(FlexAlign.Center)
        .borderRadius(DliRadius.MD)
        .backgroundColor($r('app.color.surface_secondary'))
      }
      Row({ space: DliSpace.SM }) {
        Text($r('app.string.upload_cover_from_frame'))
          .fontSize(DliFont.BODY_SM)
          .fontColor(this.picked === null ? $r('app.color.text_tertiary') : $r('app.color.text_primary'))
          .padding({ left: DliSpace.LG, right: DliSpace.LG, top: DliSpace.SM, bottom: DliSpace.SM })
          .borderRadius(DliRadius.FULL)
          .backgroundColor($r('app.color.surface_secondary'))
          .onClick(() => {
            if (this.picked !== null && !this.coverBusy) {
              this.onCaptureFrameTap();
            }
          })
        Text($r('app.string.upload_cover_from_photo'))
          .fontSize(DliFont.BODY_SM)
          .fontColor($r('app.color.text_primary'))
          .padding({ left: DliSpace.LG, right: DliSpace.LG, top: DliSpace.SM, bottom: DliSpace.SM })
          .borderRadius(DliRadius.FULL)
          .backgroundColor($r('app.color.surface_secondary'))
          .onClick(() => {
            this.pickPhotoCover();
          })
      }
      .width('100%')

      if (this.cover === null) {
        Text($r('app.string.upload_cover_tip'))
          .fontSize(DliFont.CAPTION)
          .fontColor($r('app.color.text_tertiary'))
          .width('100%')
      } else if (this.coverBusy) {
        Text('正在处理封面…')
          .fontSize(DliFont.CAPTION)
          .fontColor($r('app.color.text_tertiary'))
          .width('100%')
      } else {
        Text($r('app.string.upload_cover_cropped', this.coverWidth(), this.coverHeight()))
          .fontSize(DliFont.CAPTION)
          .fontColor($r('app.color.text_tertiary'))
          .width('100%')
      }
    }
    .width('100%')
    .padding(DliSpace.LG)
    .borderRadius(DliRadius.LG)
    .backgroundColor(DliMaterial.surface())
    .systemMaterial(DliMaterial.card())
  }

  /**
   * 提交区：错误提示紧跟按钮上方。
   * 提示不放页面顶部——用户点提交时正在页尾，放在顶部等于看不见（toast 会闪掉，不足以替代）。
   */
  @Builder
  submitButton() {
    Column({ space: DliSpace.SM }) {
      if (this.errorText.length > 0) {
        Text(this.errorText)
          .fontSize(DliFont.BODY_SM)
          .fontColor($r('app.color.state_danger'))
          .width('100%')
      }
      Row() {
        Text(this.submitLabel())
          .fontSize(DliFont.BODY)
          .fontColor(this.canSubmit() ? $r('app.color.text_on_brand') : $r('app.color.text_tertiary'))
      }
      .width('100%')
      .height(44)
      .justifyContent(FlexAlign.Center)
      .borderRadius(DliRadius.FULL)
      .backgroundColor(this.canSubmit() ? $r('app.color.brand_primary') : $r('app.color.surface_secondary'))
      .onClick(() => {
        this.submit();
      })
    }
    .width('100%')
  }

  @Builder
  successCard() {
    Column({ space: DliSpace.MD }) {
      Text($r('app.string.upload_submit_ok_title'))
        .fontSize(DliFont.TITLE)
        .fontWeight(FontWeight.Medium)
        .fontColor($r('app.color.text_primary'))
      Text($r('app.string.upload_submit_ok_sub',
        this.submitted === null ? '' : this.submitted.bvid, this.statusText()))
        .fontSize(DliFont.BODY_SM)
        .fontColor($r('app.color.text_secondary'))
        .textAlign(TextAlign.Center)
      Row({ space: DliSpace.SM }) {
        Text($r('app.string.upload_submit_again'))
          .fontSize(DliFont.BODY_SM)
          .fontColor($r('app.color.brand_primary'))
          .padding({ left: DliSpace.XL, right: DliSpace.XL, top: DliSpace.SM, bottom: DliSpace.SM })
          .borderRadius(DliRadius.FULL)
          .backgroundColor($r('app.color.brand_primary_light'))
          .onClick(() => {
            this.resetDraft();
          })
        Text($r('app.string.action_back'))
          .fontSize(DliFont.BODY_SM)
          .fontColor($r('app.color.text_secondary'))
          .padding({ left: DliSpace.XL, right: DliSpace.XL, top: DliSpace.SM, bottom: DliSpace.SM })
          .borderRadius(DliRadius.FULL)
          .backgroundColor($r('app.color.surface_secondary'))
          .onClick(() => {
            AppRouter.pop();
          })
      }
    }
    .width('100%')
    .padding(DliSpace.XL)
    .borderRadius(DliRadius.LG)
    .backgroundColor(DliMaterial.surface())
    .systemMaterial(DliMaterial.card())
    .alignItems(HorizontalAlign.Center)
  }

  @Builder
  actionButton() {
    Row() {
      Text(this.running ? $r('app.string.upload_cancel') : $r('app.string.upload_start'))
        .fontSize(DliFont.BODY)
        .fontColor(this.picked === null ? $r('app.color.text_tertiary') : $r('app.color.text_on_brand'))
        .onClick(() => {
          if (this.running) {
            this.controller.cancel();
            return;
          }
          this.start();
        })
    }
    .width('100%')
    .height(44)
    .justifyContent(FlexAlign.Center)
    .borderRadius(DliRadius.FULL)
    .backgroundColor(this.picked === null ? $r('app.color.surface_secondary') : $r('app.color.brand_primary'))
  }

  build() {
    NavDestination() {
      Scroll() {
        Column({ space: DliSpace.MD }) {
          if (this.submitted !== null) {
            this.successCard()
          } else {
            if (this.draftRestored) {
              this.restoredBanner()
            }

            this.pickCard()

            if (this.picked !== null || this.progress.phase !== 'idle') {
              this.progressCard()
            }

            // 开始/取消上传紧跟进度卡：它控制的正是上面这块状态，不该压在整个表单之后
            if (this.picked !== null) {
              this.actionButton()
            }

            // 分P 管理与稿件信息：选片后即可填写（与 Web 同步机），提交按钮等有可投的 file_id 才可用
            if (this.picked !== null || this.fileId.length > 0 || this.parts.length > 0) {
              this.partsSection()
              this.formSection()
              this.submitButton()
            }
          }
        }
        .width('100%')
        .padding({ left: DliSpace.PAGE, right: DliSpace.PAGE, bottom: DliSize.TAB_RESERVED })
      }
      .scrollBar(BarState.Off)
      .width('100%')
      .height('100%')
    }
    .title($r('app.string.title_upload'))
    .backgroundColor(Color.Transparent)
  }
}

/**
 * 从草稿复活封面草稿。
 * 裁切产物在 cache 里，进程被杀或系统清 cache 后就没了——此时**不假装封面还在**
 * （页面会显示「未设置封面」并让用户重选），避免提交时上传一个不存在的文件。
 */
function restoreCover(draft: SubmitDraft): CoverDraft | null {
  if (draft.coverPath.length === 0 || draft.coverOrigin.length === 0) {
    return null;
  }
  if (!MediaLib.exists(draft.coverPath)) {
    Logger.warn(TAG, `草稿里的封面文件已不存在，按无封面处理：${draft.coverPath}`);
    return null;
  }
  return {
    origin: draft.coverOrigin === 'photo' ? 'photo' : 'frame',
    // cacheUri 与像素尺寸都随草稿落盘，恢复时原样带回——重算会与盘上那份对不上
    cacheUri: draft.coverCacheUri,
    sandboxPath: draft.coverPath,
    width: draft.coverWidth,
    height: draft.coverHeight,
    // 已上传过的 URL 直接复用，不必重传（后端返回的地址长期有效）
    url: draft.coverUrl
  };
}

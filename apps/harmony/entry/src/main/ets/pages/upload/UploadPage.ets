import { common } from '@kit.AbilityKit';
import { DliFont, DliRadius, DliSize, DliSpace } from '../../common/constants/Theme';
import { DliMaterial } from '../../common/constants/MaterialTokens';
import { AppRouter } from '../../common/utils/AppRouter';
import { Logger } from '../../common/utils/Logger';
import { showToast } from '../../common/utils/Toast';
import { MediaLib } from '../../common/utils/MediaLib';
import { ApiError } from '../../model/Api';
import {
  CoverDraft, DESC_MAX, PickedVideo, SubmitDraft, SubmitReq, SubmitResult, TAGS_MAX, TITLE_MAX,
  UploadCheckResult, UploadProgressView, UploadResult, emptyDraft, idleProgress, normalizeTags, validateDraft
} from '../../model/Upload';
import { Category } from '../../model/Video';
import { UploadController } from '../../service/UploadController';
import { CoverService } from '../../service/CoverService';
import { VideoApi } from '../../service/VideoApi';
import { BackgroundTransfer } from '../../service/BackgroundTransfer';
import { AppContext } from '../../store/AppContext';
import { session } from '../../store/Session';

const TAG: string = 'UploadPage';

/**
 * 投稿页：选片 → 分片上传拿 file_id（HMY-50）→ 稿件信息与封面（HMY-51）→ 提交拿 bvid。
 *
 * 表单与 Web `UploadView.vue` 同口径：标题 ≤80、简介 ≤2000、标签 1~10、分区必选、类型 1 自制 / 2 转载。
 * 与 Web 的一处刻意差异：Web 用 `maxlength` 硬截断，端侧**不截断而是拦下并说明原因**
 * （验收要求「标题超长」等分支给出明确提示且不发请求），故输入框保留超长输入、提交前统一校验。
 *
 * 封面走「本地先按 16:9 裁切 → 预览 → 提交时才 multipart 上传」，与 Web `resolveCover` 同时机。
 */
@ComponentV2
export struct UploadPage {
  @Local picked: PickedVideo | null = null;
  @Local progress: UploadProgressView = idleProgress();
  @Local running: boolean = false;
  @Local result: UploadResult | null = null;
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

  private controller: UploadController = new UploadController((view: UploadProgressView) => {
    this.progress = view;
  });
  private backgroundTask: BackgroundTransfer = new BackgroundTransfer();

  aboutToAppear(): void {
    this.loadCategories();
  }

  aboutToDisappear(): void {
    // 页面销毁必须释放长时任务与在飞分片，否则上传任务会泄漏到下一次投稿
    this.controller.cancel();
    this.backgroundTask.stop(AppContext.require());
  }

  private context(): common.UIAbilityContext {
    return AppContext.require();
  }

  // ---------- 分片上传（HMY-50 链路，本卡不改） ----------

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
      this.picked = picked;
      this.result = null;
      this.errorText = '';
      // 与 Web 一致：标题默认取文件名（去扩展名），用户可改
      if (this.title.trim().length === 0) {
        this.title = UploadPage.baseName(picked.fileName).substring(0, TITLE_MAX);
      }
      Logger.info(TAG, `已选片：${picked.fileName} (${UploadController.humanSize(picked.size)})`);
    } catch (err) {
      this.fail((err as Error).message);
      Logger.error(TAG, `选片失败：${(err as Error).message}`);
    }
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
          this.picked = picked;
          this.result = null;
          this.errorText = '';
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
    this.result = null;
    const context: common.UIAbilityContext = this.context();
    await this.backgroundTask.start(context);
    try {
      const done: UploadResult = await this.controller.start(context, picked);
      this.result = done;
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
      if (err instanceof ApiError) {
        this.fail((err as ApiError).message);
        Logger.error(TAG, `上传失败：${(err as ApiError).describe()}`);
      } else {
        this.fail((err as Error).message);
        Logger.error(TAG, `上传失败：${(err as Error).message}`);
      }
    } finally {
      this.running = false;
      await this.backgroundTask.stop(context);
    }
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
   * 成功即清掉本地封面与暂存视频（都已落到服务端），并切到成功态。
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
    const draft: SubmitDraft = emptyDraft();
    draft.title = this.title.trim();
    draft.description = this.description;
    draft.categoryId = this.categoryId;
    draft.tags = normalizeTags(this.tags);
    draft.copyright = this.copyright;
    const violation: string = validateDraft(draft, this.categoryIds());
    if (violation.length > 0) {
      // 校验分支：给出明确原因且**不发任何请求**
      this.fail(violation);
      Logger.info(TAG, `表单校验未通过，未发请求：${violation}`);
      return;
    }
    const fileId: string = this.result === null ? '' : this.result.fileId;
    if (fileId.length === 0) {
      this.fail('请先上传视频文件');
      return;
    }

    this.submitting = true;
    this.errorText = '';
    const context: common.UIAbilityContext = this.context();
    try {
      const coverUrl: string = await this.resolveCover(context);
      const req: SubmitReq = {
        file_id: fileId,
        title: draft.title,
        description: draft.description,
        category_id: draft.categoryId,
        tags: draft.tags,
        copyright: draft.copyright,
        cover: coverUrl,
        // 多 P 属 HMY-52（需求 HMY-52）范围，本卡固定单 P
        parts: []
      };
      const done: SubmitResult = await VideoApi.submit(req);
      this.submitted = done;
      CoverService.discard(this.cover);
      this.cover = null;
      if (this.picked !== null) {
        MediaLib.clearStaged(context, this.picked);
      }
      Logger.info(TAG, `投稿成功：bvid=${done.bvid} status=${done.status} cover=${coverUrl.length > 0 ? 'yes' : 'none'}`);
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
    this.result = null;
    this.progress = idleProgress();
    this.title = '';
    this.description = '';
    this.categoryId = 0;
    this.copyright = 1;
    this.tags = [];
    this.tagInput = '';
    this.errorText = '';
    this.submitted = null;
  }

  private fail(message: string): void {
    this.errorText = message;
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

  private fileId(): string {
    return this.result === null ? '' : this.result.fileId;
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

  /** 文件名去扩展名（默认标题） */
  private static baseName(fileName: string): string {
    const dot: number = fileName.lastIndexOf('.');
    return dot > 0 ? fileName.substring(0, dot) : fileName;
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
      if (!this.running) {
        this.pick();
      }
    })
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

  private canSubmit(): boolean {
    return this.fileId().length > 0 && !this.submitting;
  }

  private submitLabel(): ResourceStr {
    if (this.submitting) {
      return $r('app.string.upload_submitting');
    }
    return this.fileId().length > 0
      ? $r('app.string.upload_submit') : $r('app.string.upload_wait_upload');
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
            this.pickCard()

            if (this.picked !== null || this.progress.phase !== 'idle') {
              this.progressCard()
            }

            // 开始/取消上传紧跟进度卡：它控制的正是上面这块状态，不该压在整个表单之后
            if (this.picked !== null) {
              this.actionButton()
            }

            // 稿件信息与封面：选片后即可填写（与 Web 同步机），提交按钮等 file_id 到手才可用
            if (this.picked !== null) {
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

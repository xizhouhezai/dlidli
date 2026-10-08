import { common } from '@kit.AbilityKit';
import { BusinessError } from '@kit.BasicServicesKit';
import { DliFont, DliRadius, DliSize, DliSpace } from '../../common/constants/Theme';
import { DliMaterial } from '../../common/constants/MaterialTokens';
import { AppRouter } from '../../common/utils/AppRouter';
import { Logger } from '../../common/utils/Logger';
import { showToast } from '../../common/utils/Toast';
import { MediaLib } from '../../common/utils/MediaLib';
import { ApiError } from '../../model/Api';
import { ErrCode } from '../../common/constants/ErrorMessages';
import { PickedVideo, UploadCheckResult, UploadProgressView, UploadResult, idleProgress } from '../../model/Upload';
import { UploadController } from '../../service/UploadController';
import { BackgroundTransfer } from '../../service/BackgroundTransfer';
import { AppContext } from '../../store/AppContext';
import { session } from '../../store/Session';

const TAG: string = 'UploadPage';

/**
 * 投稿上传页（HMY-50）：选片 → 分片上传（进度可见）→ 拿到 file_id。
 *
 * 本卡只负责到 file_id 为止——稿件信息（标题/简介/分区/标签/封面）与提交流程属 HMY-52/53，
 * 故此处不渲染表单，收口后用 file_id 提示并交给下一步。
 */
@ComponentV2
export struct UploadPage {
  @Local picked: PickedVideo | null = null;
  @Local progress: UploadProgressView = idleProgress();
  @Local running: boolean = false;
  @Local result: UploadResult | null = null;
  @Local errorText: string = '';
  private controller: UploadController = new UploadController((view: UploadProgressView) => {
    this.progress = view;
  });
  private backgroundTask: BackgroundTransfer = new BackgroundTransfer();

  aboutToDisappear(): void {
    // 页面销毁必须释放长时任务与在飞分片，否则上传任务会泄漏到下一次投稿
    this.controller.cancel();
    this.backgroundTask.stop(AppContext.require());
  }

  private context(): common.UIAbilityContext {
    return AppContext.require();
  }

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
        this.errorText = check.message;
        showToast(this.getUIContext(), check.message);
        return;
      }
      this.picked = picked;
      this.result = null;
      this.errorText = '';
      Logger.info(TAG, `已选片：${picked.fileName} (${UploadController.humanSize(picked.size)})`);
    } catch (err) {
      const cause: Error = err as Error;
      this.errorText = cause.message;
      Logger.error(TAG, `选片失败：${cause.message}`);
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
      this.errorText = '请先登录后再投稿';
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
      // 收口后清掉缓存副本，避免大文件长期占 cache；已传内容都在后端，不影响续传语义
      MediaLib.clearStaged(context, picked);
      Logger.info(TAG, `上传完成：file_id=${done.fileId} 秒传=${done.fast}`);
      showToast(this.getUIContext(), done.fast
        ? $r('app.string.upload_fast_done') : $r('app.string.upload_done'));
    } catch (err) {
      if (err instanceof ApiError) {
        this.errorText = (err as ApiError).message;
        Logger.error(TAG, `上传失败：${(err as ApiError).describe()}`);
      } else {
        const cause: Error = err as Error;
        this.errorText = cause.message;
        Logger.error(TAG, `上传失败：${cause.message}`);
      }
    } finally {
      this.running = false;
      await this.backgroundTask.stop(context);
    }
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
          this.pickCard()

          if (this.picked !== null || this.progress.phase !== 'idle') {
            this.progressCard()
          }

          if (this.errorText.length > 0) {
            Text(this.errorText)
              .fontSize(DliFont.BODY_SM)
              .fontColor($r('app.color.state_danger'))
              .width('100%')
          }

          if (this.result !== null) {
            Text(`file_id：${this.result.fileId}`)
              .fontSize(DliFont.BODY_SM)
              .fontColor($r('app.color.text_secondary'))
              .width('100%')
          }

          if (this.picked !== null) {
            this.actionButton()
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

import { VideoCard } from '../model/Video';
import { resolveMediaUrl } from '../common/utils/MediaUrl';
import { formatDuration, formatPubdate } from '../common/utils/Format';
import { DliFont, DliRadius, DliSpace } from '../common/constants/Theme';
import { DliMaterial } from '../common/constants/MaterialTokens';

/**
 * 「我的投稿」专用卡片（HMY-53 状态回看）。
 *
 * 与信息流卡片（VideoCardItem）分开的原因：投稿档要展示稿件状态与驳回原因，
 * 而信息流/搜索的卡片契约里没有「状态」这一维度（公开列表只出已发布），
 * 把两者塞进同一个组件会让 info 流的渲染分支背上永不触发的状态逻辑。
 *
 * 状态取值对齐 server/internal/module/video/model.go 的 StatusXxx：
 * 0 草稿 / 1 上传中 / 2 转码中 / 3 待审核 / 4 已发布 / 5 已驳回。
 * status=6 已锁定不在本任务范围，走通用兜底文案。
 */
@ComponentV2
export struct MineVideoCardItem {
  @Require @Param video: VideoCard;
  @Event onTap: (bvid: string) => void = () => {};

  private statusText(): ResourceStr {
    switch (this.video.status) {
      case 0:
        return $r('app.string.video_status_draft');
      case 1:
        return $r('app.string.video_status_uploading');
      case 2:
        return $r('app.string.video_status_transcoding');
      case 3:
        return $r('app.string.video_status_reviewing');
      case 4:
        return $r('app.string.video_status_published');
      case 5:
        return $r('app.string.video_status_rejected');
      default:
        return $r('app.string.video_status_unknown');
    }
  }

  /** 状态胶囊的文字色（深色，保证在浅色胶囊底上可读） */
  private statusColor(): ResourceColor {
    switch (this.video.status) {
      case 2:
        return $r('app.color.brand_primary');
      case 3:
        return $r('app.color.state_warning');
      case 4:
        return $r('app.color.state_success');
      case 5:
        return $r('app.color.state_danger');
      default:
        return $r('app.color.text_secondary');
    }
  }

  /** 状态胶囊的底色：中性态用次级底色，其余用对应语义的浅色底 */
  private statusBackground(): ResourceColor {
    switch (this.video.status) {
      case 2:
        return $r('app.color.brand_primary_light');
      case 3:
        return $r('app.color.state_warning_light');
      case 4:
        return $r('app.color.state_success_light');
      case 5:
        return $r('app.color.state_danger_light');
      default:
        return $r('app.color.surface_secondary');
    }
  }

  /**
   * 驳回原因：接口只在 `status == 5` 时返回该字段（见 plan §3 接口表），
   * 故这里按「字段可能不存在」处理，缺失时给通用文案，绝不渲染空白区块。
   */
  private rejectReason(): ResourceStr {
    const reason: string | undefined = this.video.reject_reason;
    if (reason !== undefined && reason.length > 0) {
      return reason;
    }
    return $r('app.string.mine_reject_reason_fallback');
  }

  /** 封面底图 + 右下时长角标（与信息流卡片同位，便于两档切换时视觉连续） */
  @Builder
  coverImage() {
    Stack({ alignContent: Alignment.BottomEnd }) {
      if (this.video.cover.length > 0) {
        Image(resolveMediaUrl(this.video.cover))
          .width('100%')
          .aspectRatio(16 / 9)
          .objectFit(ImageFit.Cover)
          .borderRadius(DliRadius.MD)
      } else {
        // 未设封面时用品牌默认封面兜底（与 Web/H5/Admin 同一份派生产物）
        Image($r('app.media.default_cover'))
          .width('100%')
          .aspectRatio(16 / 9)
          .objectFit(ImageFit.Cover)
          .borderRadius(DliRadius.MD)
      }
      if (this.video.duration > 0) {
        Text(formatDuration(this.video.duration))
          .fontSize(DliFont.CAPTION)
          .fontColor($r('app.color.text_on_brand'))
          .padding({ left: 6, right: 6, top: 1, bottom: 1 })
          .borderRadius(4)
          .backgroundColor('#99000000')
          .margin({ right: 6, bottom: 6 })
      }
    }
    .width('100%')
  }

  @Builder
  coverBlock() {
    Stack({ alignContent: Alignment.TopStart }) {
      this.coverImage()
      // 状态胶囊压在封面左上：投稿档最先要读到的是「这稿到哪一步了」
      Text(this.statusText())
        .fontSize(DliFont.CAPTION)
        .fontColor(this.statusColor())
        .padding({ left: 6, right: 6, top: 2, bottom: 2 })
        .borderRadius(4)
        .backgroundColor(this.statusBackground())
        .margin({ left: 6, top: 6 })
    }
    .width('100%')
  }

  /** 流转中的稿件给一句下一步说明（与 Web 端 MineVideosView 同口径） */
  @Builder
  statusHint() {
    if (this.video.status === 2) {
      Text($r('app.string.upload_status_transcoding'))
        .fontSize(DliFont.CAPTION)
        .fontColor($r('app.color.text_secondary'))
        .maxLines(2)
        .textOverflow({ overflow: TextOverflow.Ellipsis })
        .width('100%')
    } else if (this.video.status === 3) {
      Text($r('app.string.upload_status_review'))
        .fontSize(DliFont.CAPTION)
        .fontColor($r('app.color.text_secondary'))
        .maxLines(2)
        .textOverflow({ overflow: TextOverflow.Ellipsis })
        .width('100%')
    }
  }

  /**
   * 驳回原因提示区：带「驳回原因」标签的浅红圆角块。
   * 做成区块而非裸文本，是为了与下方的稿件元信息分隔开——驳回原因是需要作者
   * 据此改稿的内容，不能混在播放量/时间那一行里被扫过去。
   */
  @Builder
  rejectBlock() {
    Column({ space: DliSpace.XS }) {
      Text($r('app.string.mine_reject_reason_label'))
        .fontSize(DliFont.CAPTION)
        .fontColor($r('app.color.state_danger'))
        .fontWeight(FontWeight.Medium)
        .width('100%')
      Text(this.rejectReason())
        .fontSize(DliFont.CAPTION)
        .fontColor($r('app.color.text_primary'))
        .maxLines(4)
        .textOverflow({ overflow: TextOverflow.Ellipsis })
        .width('100%')
    }
    .width('100%')
    .padding(DliSpace.SM)
    .borderRadius(DliRadius.SM)
    .backgroundColor($r('app.color.state_danger_light'))
    .alignItems(HorizontalAlign.Start)
  }

  build() {
    Column({ space: DliSpace.XS }) {
      this.coverBlock()
      Text(this.video.title)
        .fontSize(DliFont.BODY_SM)
        .fontColor($r('app.color.text_primary'))
        .maxLines(2)
        .textOverflow({ overflow: TextOverflow.Ellipsis })
        .width('100%')
      // 投稿档看的是「什么时候投的」，故固定用 created_at（信息流卡片用 published_at）
      Text(formatPubdate(this.video.created_at))
        .fontSize(DliFont.CAPTION)
        .fontColor($r('app.color.text_tertiary'))
        .maxLines(1)
        .textOverflow({ overflow: TextOverflow.Ellipsis })
        .width('100%')
      this.statusHint()
      if (this.video.status === 5) {
        this.rejectBlock()
      }
    }
    .width('100%')
    .padding(DliSpace.SM)
    .borderRadius(DliRadius.LG)
    .backgroundColor(DliMaterial.surface())
    .systemMaterial(DliMaterial.card())
    .alignItems(HorizontalAlign.Start)
    .onClick(() => {
      this.onTap(this.video.bvid);
    })
  }
}

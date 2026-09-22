import { VideoCard } from '../model/Video';
import { resolveMediaUrl } from '../common/utils/MediaUrl';
import { formatCount, formatDuration, formatPubdate } from '../common/utils/Format';
import { DliFont, DliRadius, DliSpace } from '../common/constants/Theme';
import { DliMaterial } from '../common/constants/MaterialTokens';

/**
 * 信息流与搜索结果共用的视频卡片，版式对齐 Web 端 components/VideoCard.vue：
 * 封面 16:9 + 右下时长角标 + 两行标题 + meta（播放量 · UP 主 · 发布日期）。
 * 卡片本身走沉浸光感材质（plan §2）：玻璃底浮在品牌氛围底之上。
 */
@ComponentV2
export struct VideoCardItem {
  @Require @Param video: VideoCard;
  /** 是否在 meta 行追加发布日期 */
  @Param showDate: boolean = false;
  @Event onTap: (bvid: string) => void = () => {};

  private metaText(): string {
    const parts: string[] = [formatCount(this.video.stat.view)];
    if (this.video.owner.nickname.length > 0) {
      parts.push(this.video.owner.nickname);
    }
    if (this.showDate) {
      const date: string = formatPubdate(this.video.published_at ?? this.video.created_at);
      if (date.length > 0) {
        parts.push(date);
      }
    }
    return parts.join(' · ');
  }

  @Builder
  coverBlock() {
    Stack({ alignContent: Alignment.BottomEnd }) {
      if (this.video.cover.length > 0) {
        Image(resolveMediaUrl(this.video.cover))
          .width('100%')
          .aspectRatio(16 / 9)
          .objectFit(ImageFit.Cover)
          .borderRadius(DliRadius.MD)
      } else {
        // 未设封面时不留白洞，用底色块占位（对齐 Web 端 default-cover 的角色）
        Column()
          .width('100%')
          .aspectRatio(16 / 9)
          .borderRadius(DliRadius.MD)
          .backgroundColor($r('app.color.border_default'))
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

  build() {
    Column({ space: DliSpace.XS }) {
      this.coverBlock()
      Text(this.video.title)
        .fontSize(DliFont.BODY_SM)
        .fontColor($r('app.color.text_primary'))
        .maxLines(2)
        .textOverflow({ overflow: TextOverflow.Ellipsis })
        .width('100%')
      Text(this.metaText())
        .fontSize(DliFont.CAPTION)
        .fontColor($r('app.color.text_tertiary'))
        .maxLines(1)
        .textOverflow({ overflow: TextOverflow.Ellipsis })
        .width('100%')
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

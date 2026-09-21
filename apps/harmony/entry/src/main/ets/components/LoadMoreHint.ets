import { DliFont, DliSpace } from '../common/constants/Theme';

/**
 * 分页列表尾部的加载态：加载中 / 加载失败可重试 / 没有更多。
 * 三个标志都为否时不渲染任何内容，尾部自然塌陷为 0 高度。
 */
@ComponentV2
export struct LoadMoreHint {
  @Param loading: boolean = false;
  @Param failed: boolean = false;
  @Param done: boolean = false;
  @Event onRetry: () => void = () => {};

  build() {
    Row({ space: DliSpace.SM }) {
      if (this.failed) {
        Text($r('app.string.feed_load_more_failed'))
          .fontSize(DliFont.CAPTION)
          .fontColor($r('app.color.brand_primary'))
      } else if (this.loading) {
        LoadingProgress()
          .width(16)
          .height(16)
          .color($r('app.color.brand_primary'))
        Text($r('app.string.feed_loading_more'))
          .fontSize(DliFont.CAPTION)
          .fontColor($r('app.color.text_tertiary'))
      } else if (this.done) {
        Text($r('app.string.feed_no_more'))
          .fontSize(DliFont.CAPTION)
          .fontColor($r('app.color.text_tertiary'))
      }
    }
    .width('100%')
    .height(44)
    .justifyContent(FlexAlign.Center)
    .onClick(() => {
      if (this.failed) {
        this.onRetry();
      }
    })
  }
}

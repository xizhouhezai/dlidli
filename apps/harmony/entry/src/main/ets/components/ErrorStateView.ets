import { DliFont, DliRadius, DliSpace } from '../common/constants/Theme';

/**
 * 可重试失败态：网络不可用/超时/服务瞬时故障时替代白屏。
 * 只有 ApiError.retryable 为真才给重试入口，业务错误（如参数不合法）重试无意义。
 */
@ComponentV2
export struct ErrorStateView {
  @Param message: ResourceStr = '';
  @Param canRetry: boolean = true;
  @Event onRetry: () => void = () => {};

  build() {
    Column({ space: DliSpace.LG }) {
      Column({ space: DliSpace.SM }) {
        Text($r('app.string.state_error_title'))
          .fontSize(DliFont.BODY)
          .fontWeight(FontWeight.Medium)
          .fontColor($r('app.color.text_primary'))
        Text(this.message)
          .fontSize(DliFont.BODY_SM)
          .fontColor($r('app.color.text_secondary'))
          .textAlign(TextAlign.Center)
      }
      .width('100%')

      if (this.canRetry) {
        Button($r('app.string.action_retry'))
          .fontSize(DliFont.BODY_SM)
          .fontColor($r('app.color.text_on_brand'))
          .backgroundColor($r('app.color.brand_primary'))
          .borderRadius(DliRadius.FULL)
          .padding({ left: DliSpace.XL, right: DliSpace.XL })
          .onClick(() => {
            this.onRetry();
          })
      }
    }
    .width('100%')
    .height('100%')
    .padding({ left: DliSpace.XL, right: DliSpace.XL })
    .justifyContent(FlexAlign.Center)
  }
}

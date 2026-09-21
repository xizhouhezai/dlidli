@ComponentV2
export struct PlaceholderView {
  @Param text: ResourceStr = '';

  build() {
    Column() {
      Text(this.text)
        .fontSize(15)
        .fontColor($r('app.color.text_secondary'))
    }
    .width('100%')
    .height('100%')
    .justifyContent(FlexAlign.Center)
    .backgroundColor($r('app.color.bg_page'))
  }
}

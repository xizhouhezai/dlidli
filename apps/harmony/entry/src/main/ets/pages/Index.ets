import { uiMaterial } from '@kit.ArkUI';
import { DliFont, DliRadius, DliSize, DliSpace } from '../common/constants/Theme';
import { DliMaterial } from '../common/constants/MaterialTokens';
import { HomePage } from './home/HomePage';
import { SearchPage } from './search/SearchPage';
import { ProfilePage } from './profile/ProfilePage';

/**
 * 应用壳层：Navigation（标题栏）+ Tabs（底部标签栏）。
 *
 * 该结构不是视觉偏好而是硬约束——沉浸光感系统材质的作用面仅限 Navigation/NavDestination
 * 标题栏与 barPosition: BarPosition.End 的底部 TabBar，自绘顶栏/底栏将无处挂载材质。
 */
@Entry
@ComponentV2
struct Index {
  @Local currentIndex: number = 0;
  private titleBarMaterial: uiMaterial.ImmersiveMaterial | undefined = DliMaterial.titleBar();
  private tabBarMaterial: uiMaterial.ImmersiveMaterial | undefined = DliMaterial.tabBar();

  @Builder
  titleBar() {
    Row() {
      if (this.currentIndex === 0) {
        Row({ space: DliSpace.XS }) {
          SymbolGlyph($r('sys.symbol.magnifyingglass'))
            .fontSize(16)
            .fontColor([$r('app.color.text_tertiary')])
          Text($r('app.string.search_placeholder'))
            .fontSize(DliFont.BODY_SM)
            .fontColor($r('app.color.text_tertiary'))
        }
        .layoutWeight(1)
        .height(36)
        .padding({ left: DliSpace.MD, right: DliSpace.MD })
        .borderRadius(DliRadius.FULL)
        .backgroundColor($r('app.color.bg_surface'))
      } else {
        Text(this.currentIndex === 1 ? $r('app.string.title_search') : $r('app.string.title_profile'))
          .fontSize(DliFont.TITLE_SM)
          .fontWeight(FontWeight.Medium)
          .fontColor($r('app.color.text_primary'))
      }
    }
    .width('100%')
    .height(DliSize.TITLE_BAR_HEIGHT)
    .padding({ left: DliSpace.LG, right: DliSpace.LG })
    .alignItems(VerticalAlign.Center)
  }

  @Builder
  tabBarItem(index: number, label: Resource, symbol: Resource) {
    Column({ space: DliSpace.XS }) {
      SymbolGlyph(symbol)
        .fontSize(DliSize.TAB_ICON)
        .fontColor([this.currentIndex === index ? $r('app.color.brand_primary') : $r('app.color.text_tertiary')])
      Text(label)
        .fontSize(DliFont.TAB_LABEL)
        .fontColor(this.currentIndex === index ? $r('app.color.brand_primary') : $r('app.color.text_tertiary'))
    }
    .width('100%')
    .height('100%')
    .justifyContent(FlexAlign.Center)
  }

  build() {
    Navigation() {
      Tabs({ barPosition: BarPosition.End, index: this.currentIndex }) {
        TabContent() {
          HomePage()
        }
        .tabBar(this.tabBarItem(0, $r('app.string.tab_home'), $r('sys.symbol.house')))

        TabContent() {
          SearchPage()
        }
        .tabBar(this.tabBarItem(1, $r('app.string.tab_search'), $r('sys.symbol.magnifyingglass')))

        TabContent() {
          ProfilePage()
        }
        .tabBar(this.tabBarItem(2, $r('app.string.tab_profile'), $r('sys.symbol.person')))
      }
      .scrollable(false)
      .barFloatingStyle({
        maskColor: $r('app.color.tabbar_mask'),
        maskHeight: DliSize.TAB_MASK_HEIGHT,
        systemMaterial: this.tabBarMaterial
      })
      .onChange((index: number) => {
        this.currentIndex = index;
      })
      .width('100%')
      .height('100%')
    }
    .mode(NavigationMode.Stack)
    .title(this.titleBar(), { systemMaterial: this.titleBarMaterial })
    .titleMode(NavigationTitleMode.Mini)
    // 壳层是根内容，标题栏不应出现返回键（返回键由后续压栈的 NavDestination 自带）
    .hideBackButton(true)
    .backgroundColor($r('app.color.bg_page'))
    .width('100%')
    .height('100%')
  }
}

import { uiMaterial } from '@kit.ArkUI';
import { DividerMode, HdsTabs, hdsMaterial } from '@kit.UIDesignKit';
import { DliFont, DliRadius, DliSize, DliSpace } from '../common/constants/Theme';
import { DliMaterial } from '../common/constants/MaterialTokens';
import { AmbientBackdrop } from '../components/AmbientBackdrop';
import { AppRouter, RouteName } from '../common/utils/AppRouter';
import { Logger } from '../common/utils/Logger';
import { HomePage } from './home/HomePage';
import { SearchPage } from './search/SearchPage';
import { ProfilePage } from './profile/ProfilePage';
import { LoginPage } from './auth/LoginPage';

const TAG: string = 'Index';

/**
 * 应用壳层：氛围底（Stack 最底层）+ Navigation（标题栏）+ HdsTabs（悬浮玻璃胶囊底栏）。
 *
 * 该结构不是视觉偏好而是硬约束——沉浸光感系统材质在 Navigation/NavDestination 标题栏上是一等作用面，
 * 自绘顶栏将无处挂载材质。底栏同理交给 HDS：`HdsTabs.barFloatingStyle` 用窄 `barWidth` 把标签栏做成
 * 悬浮胶囊，材质与投影由 HDS 统一出（比自绘 Tabs + 手动 maskColor 更接近官方观感）。
 */
@Entry
@ComponentV2
struct Index {
  @Local currentIndex: number = 0;
  @Local pathStack: NavPathStack = new NavPathStack();
  /** 底栏材质档：能力不足时降级为 NONE + SMOOTH，避免 HDS 在不支持的设备上退化成异常观感 */
  @Local tabBarMaterialType: hdsMaterial.MaterialType = hdsMaterial.MaterialType.ADAPTIVE;
  @Local tabBarMaterialLevel: hdsMaterial.MaterialLevel = hdsMaterial.MaterialLevel.ADAPTIVE;
  private titleBarMaterial: uiMaterial.ImmersiveMaterial | undefined = DliMaterial.titleBar();

  aboutToAppear(): void {
    // 子页经 AppRouter 跳转，避免把路由栈逐层传递
    AppRouter.bind(this.pathStack);
    Logger.info(TAG, `沉浸光感：${DliMaterial.describe()}`);
    this.initTabBarMaterial();
  }

  /** 底栏材质只有 IMMERSIVE 一种实现；设备不支持时退回无材质，交给 HDS 的常规胶囊外观 */
  private initTabBarMaterial(): void {
    try {
      if (!hdsMaterial.getSystemMaterialTypes().includes(hdsMaterial.MaterialType.IMMERSIVE)) {
        this.tabBarMaterialType = hdsMaterial.MaterialType.NONE;
        this.tabBarMaterialLevel = hdsMaterial.MaterialLevel.SMOOTH;
      }
    } catch (err) {
      Logger.warn(TAG, `查询底栏材质能力失败：${(err as Error).message}`);
    }
  }

  /** 压栈页登记处：新增页面在此加一条分支 */
  @Builder
  destination(name: string) {
    if (name === RouteName.LOGIN) {
      LoginPage()
    }
  }

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
        .backgroundColor(DliMaterial.surface())
        .systemMaterial(DliMaterial.chip())
        .onClick(() => {
          // 标题栏搜索框是入口而非输入框：点击切到搜索页签（官方「一镜到底」不切层级的做法在端侧简化为切页签）
          this.currentIndex = 1;
        })
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
        .fontColor([this.currentIndex === index ? $r('app.color.brand_primary') : $r('app.color.text_secondary')])
      Text(label)
        .fontSize(DliFont.TAB_LABEL)
        .fontColor(this.currentIndex === index ? $r('app.color.brand_primary') : $r('app.color.text_secondary'))
    }
  }

  build() {
    Stack() {
      // 氛围底在最底层：标题栏与胶囊底栏的材质、卡片的磨砂都靠它才有可折射的内容
      AmbientBackdrop()

      Navigation(this.pathStack) {
        HdsTabs({ index: this.currentIndex }) {
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
        .barPosition(BarPosition.End)
        // 内容从胶囊下穿过，玻璃才有可折射的东西；页面据此留出 TAB_RESERVED 的底部空白
        .barOverlap(true)
        .barMode(BarMode.Fixed)
        .scrollable(false)
        .divider({ mode: DividerMode.NONE })
        .barFloatingStyle({
          barBottomMargin: DliSize.TAB_BOTTOM_MARGIN,
          adaptToHandedness: true,
          // 窄于屏宽是"胶囊"的来源：铺满整宽的标签栏永远是"一条底栏"
          barWidth: {
            smallWidth: DliSize.TAB_WIDTH_SMALL,
            mediumWidth: DliSize.TAB_WIDTH,
            largeWidth: DliSize.TAB_WIDTH
          },
          systemMaterialEffect: {
            materialType: this.tabBarMaterialType,
            materialLevel: this.tabBarMaterialLevel
          }
        })
        .onChange((index: number) => {
          this.currentIndex = index;
        })
        .width('100%')
        .height('100%')
      }
      .mode(NavigationMode.Stack)
      .navDestination(this.destination)
      .title(this.titleBar(), { systemMaterial: this.titleBarMaterial })
      .titleMode(NavigationTitleMode.Mini)
      // 壳层是根内容，标题栏不应出现返回键（返回键由后续压栈的 NavDestination 自带）
      .hideBackButton(true)
      // 透明交给底下的氛围底，避免不透明底色把材质与光斑一起盖住
      .backgroundColor(Color.Transparent)
      .width('100%')
      .height('100%')
    }
    .width('100%')
    .height('100%')
    .backgroundColor($r('app.color.bg_page'))
  }
}

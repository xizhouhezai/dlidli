import { AppRouter, RouteName } from '../../common/utils/AppRouter';
import { session } from '../../store/Session';
import { DliFont, DliRadius, DliSpace } from '../../common/constants/Theme';

/**
 * 「我的」页：本任务只落会话相关部分（未登录入口 / 已登录身份 / 退出登录），
 * 资料编辑、我的投稿、观看历史、收藏随 M4-HMY-09 补齐。
 */
@ComponentV2
export struct ProfilePage {
  aboutToAppear(): void {
    // 凭证在则校正一次（续期失败会被 HttpClient 清掉，此处同步回归未登录态）
    session.hydrate();
  }

  private confirmLogout(): void {
    // AlertDialog.show 已废弃，走 UIContext 的对话框接口
    this.getUIContext().showAlertDialog({
      title: $r('app.string.profile_logout'),
      message: $r('app.string.profile_logout_confirm'),
      autoCancel: true,
      primaryButton: {
        value: $r('app.string.action_cancel'),
        action: () => {}
      },
      secondaryButton: {
        value: $r('app.string.profile_logout'),
        fontColor: $r('app.color.state_danger'),
        action: () => {
          session.logout();
        }
      }
    });
  }

  @Builder
  guestView() {
    Column({ space: DliSpace.LG }) {
      Column({ space: DliSpace.SM }) {
        Text($r('app.string.profile_guest'))
          .fontSize(DliFont.TITLE)
          .fontWeight(FontWeight.Medium)
          .fontColor($r('app.color.text_primary'))
        Text($r('app.string.profile_login_hint'))
          .fontSize(DliFont.BODY_SM)
          .fontColor($r('app.color.text_tertiary'))
          .textAlign(TextAlign.Center)
      }
      .width('100%')

      Button($r('app.string.profile_login_entry'))
        .width('100%')
        .height(44)
        .fontSize(DliFont.BODY)
        .fontColor($r('app.color.text_on_brand'))
        .backgroundColor($r('app.color.brand_primary'))
        .borderRadius(DliRadius.FULL)
        .onClick(() => {
          AppRouter.push(RouteName.LOGIN);
        })
    }
    .width('100%')
    .padding(DliSpace.PAGE)
    .alignItems(HorizontalAlign.Center)
  }

  @Builder
  userView() {
    Column({ space: DliSpace.LG }) {
      Row({ space: DliSpace.MD }) {
        if (session.avatar.length > 0) {
          Image(session.avatar)
            .width(56)
            .height(56)
            .borderRadius(DliRadius.FULL)
            .backgroundColor($r('app.color.bg_surface'))
        } else {
          Row() {
            SymbolGlyph($r('sys.symbol.person'))
              .fontSize(28)
              .fontColor([$r('app.color.text_tertiary')])
          }
          .width(56)
          .height(56)
          .justifyContent(FlexAlign.Center)
          .borderRadius(DliRadius.FULL)
          .backgroundColor($r('app.color.bg_surface'))
        }

        Column({ space: DliSpace.XS }) {
          Text(session.nickname)
            .fontSize(DliFont.TITLE_SM)
            .fontWeight(FontWeight.Medium)
            .fontColor($r('app.color.text_primary'))
          Text(`Lv${session.level}`)
            .fontSize(DliFont.CAPTION)
            .fontColor($r('app.color.text_tertiary'))
        }
        .alignItems(HorizontalAlign.Start)
        .layoutWeight(1)
      }
      .width('100%')
      .padding(DliSpace.LG)
      .borderRadius(DliRadius.LG)
      .backgroundColor($r('app.color.bg_surface'))

      Button($r('app.string.profile_logout'))
        .width('100%')
        .height(44)
        .fontSize(DliFont.BODY_SM)
        .fontColor($r('app.color.text_secondary'))
        .backgroundColor($r('app.color.bg_surface'))
        .borderRadius(DliRadius.FULL)
        .onClick(() => {
          this.confirmLogout();
        })
    }
    .width('100%')
    .padding(DliSpace.PAGE)
  }

  build() {
    Column() {
      if (session.loggedIn) {
        this.userView()
      } else {
        this.guestView()
      }
    }
    .width('100%')
    .height('100%')
    .justifyContent(FlexAlign.Start)
    .backgroundColor($r('app.color.bg_page'))
  }
}

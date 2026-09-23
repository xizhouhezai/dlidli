import { common } from '@kit.AbilityKit';
import { ApiError } from '../model/Api';
import { VideoDetail } from '../model/Video';
import { InteractionController } from '../service/InteractionController';
import { ShareUtil } from '../service/ShareUtil';
import { TokenStore } from '../store/TokenStore';
import { AppRouter, RouteName } from '../common/utils/AppRouter';
import { DliFont, DliRadius, DliSpace } from '../common/constants/Theme';
import { showToast } from '../common/utils/Toast';
import { Logger } from '../common/utils/Logger';

const TAG: string = 'ActionBar';

/**
 * 长按三连后抑制单击的窗口（ms）。
 * Tap 与 LongPress 同挂 Parallel 手势组，长按松手时两者都会命中；三连已含点赞，
 * 若再走一次单击会把刚点上的赞取消掉，故留一个短窗口丢弃紧随的单击。
 */
const TAP_AFTER_LONG_PRESS_GUARD_MS: number = 300;

/**
 * 播放页互动栏（M4-HMY-08 / ITR-01/10/20/30 + SHR-02）：
 * 点赞（单击）/ 长按三连（1.5s，幂等 + 原生动画）/ 投币 / 收藏（弹层）/ 分享（系统面板）。
 *
 * 三连动画（HMY-30「原生动画反馈」）：长按点赞触发三连后，点赞图标做一次品牌色放大脉冲，
 * 并用三段递进延迟让 赞→币→藏 依次点亮——原生 ArkUI 属性动画，不依赖 CSS/DOM。
 *
 * 与 Web 端 useVideoActions 的交互对齐，差异：长按时长取 spec 的 1.5s（Web 800ms）；
 * 分享走系统分享面板（鸿蒙端无小程序卡片，承 spec HMY-32 端侧差异）。
 */
@ComponentV2
export struct ActionBar {
  @Require @Param bvid: string;
  @Require @Param detail: VideoDetail;

  private acts: InteractionController = new InteractionController();

  /** 三连动画：点赞图标的缩放（脉冲），由 tripleAnimTick 触发 */
  @Local likeScale: number = 1;
  /** 三连三段点亮延迟用的「当前点亮到第几项」：0=未播 1=赞 2=币 3=藏 */
  @Local lightUp: number = 0;
  /** 长按三连后抑制单击的截止时刻（见 TAP_AFTER_LONG_PRESS_GUARD_MS） */
  private suppressTapUntil: number = 0;

  aboutToAppear(): void {
    this.acts.load(this.bvid, this.detail).catch((err: Object) => {
      Logger.warn(TAG, `互动状态装载失败：${JSON.stringify(err)}`);
    });
  }

  private loggedIn(): boolean {
    return TokenStore.isLoggedIn();
  }

  private requireLogin(): boolean {
    if (!this.loggedIn()) {
      showToast(this.getUIContext(), $r('app.string.act_need_login'));
      AppRouter.push(RouteName.LOGIN);
      return false;
    }
    return true;
  }

  // ---- 点赞（单击）/ 三连（长按 1.5s） ----

  /**
   * 长按命中：直接触发三连，并**屏蔽紧随其后的单击**。
   *
   * 为什么需要屏蔽：点赞按钮上 Tap 与 LongPress 挂在同一个 Parallel 手势组里，
   * 长按松手时两个手势都会命中（与播放页双击/单击并存是同一现象）。三连本身已包含点赞，
   * 若再走一次 toggleLike 会把刚点上的赞又取消掉——所以长按后给单击留一个抑制窗口。
   */
  private onLikeLongPress(): void {
    if (!this.requireLogin()) {
      return;
    }
    this.suppressTapUntil = Date.now() + TAP_AFTER_LONG_PRESS_GUARD_MS;
    this.playTripleAnim();
    this.acts.doTriple()
      .then(() => {
        showToast(this.getUIContext(), $r('app.string.act_triple_done'));
      })
      .catch((err: ApiError) => {
        showToast(this.getUIContext(), err.message);
      });
  }

  /** 单击：抑制窗口内直接丢弃（长按刚走完三连） */
  private onLikeTap(): void {
    if (Date.now() < this.suppressTapUntil) {
      return;
    }
    if (!this.requireLogin()) {
      return;
    }
    this.acts.toggleLike().catch((err: ApiError) => {
      showToast(this.getUIContext(), err.message);
    });
  }

  /** 三连原生动画：点赞图标放大脉冲 + 赞/币/藏依次点亮（递进延迟） */
  private playTripleAnim(): void {
    // 脉冲：1 → 1.4，iterations:2 + Alternate 即一来一回。getUIContext().animateTo 是非废弃替代。
    const uiContext: UIContext = this.getUIContext();
    uiContext.animateTo(
      {
        duration: 180,
        curve: Curve.EaseOut,
        iterations: 2,
        playMode: PlayMode.Alternate
      },
      () => {
        this.likeScale = 1.4;
      }
    );
    // 依次点亮：赞 0ms → 币 240ms → 藏 480ms（视觉上的「三连走完」）
    this.lightUp = 1;
    setTimeout(() => {
      this.lightUp = 2;
    }, 240);
    setTimeout(() => {
      this.lightUp = 3;
    }, 480);
    setTimeout(() => {
      this.lightUp = 0;
    }, 1200);
  }

  // ---- 投币（ITR-10） ----

  private openCoinPop(): void {
    if (!this.requireLogin()) {
      return;
    }
    if (this.acts.coinCount > 0) {
      showToast(this.getUIContext(), $r('app.string.act_coined_hint'));
      return;
    }
    this.acts.coinPopVisible = !this.acts.coinPopVisible;
  }

  private async doCoin(count: number): Promise<void> {
    this.acts.coinPopVisible = false;
    try {
      await this.acts.coin(count);
      showToast(this.getUIContext(), $r('app.string.act_coin_done', count));
    } catch (err) {
      showToast(this.getUIContext(), (err as ApiError).message);
    }
  }

  // ---- 收藏（ITR-20/21） ----

  private openFavPop(): void {
    if (!this.requireLogin()) {
      return;
    }
    if (this.acts.faved) {
      // 已收藏 → 直接取消（默认夹）
      this.acts.toggleFavorite('').then(() => {
        showToast(this.getUIContext(), $r('app.string.act_fav_off'));
      }).catch((err: ApiError) => {
        showToast(this.getUIContext(), err.message);
      });
      return;
    }
    this.acts.favPopVisible = !this.acts.favPopVisible;
    if (this.acts.favPopVisible) {
      this.acts.loadCollections().catch(() => {});
    }
  }

  private async doFav(collectionId: string): Promise<void> {
    this.acts.favPopVisible = false;
    try {
      await this.acts.toggleFavorite(collectionId);
      showToast(this.getUIContext(), this.acts.faved ? $r('app.string.act_fav_on') : $r('app.string.act_fav_off'));
    } catch (err) {
      showToast(this.getUIContext(), (err as ApiError).message);
    }
  }

  // ---- 分享（SHR-02，系统面板） ----

  private async openShare(): Promise<void> {
    const ctx = this.getUIContext().getHostContext();
    if (ctx === undefined) {
      return;
    }
    try {
      const url: string = ShareUtil.videoShareUrl(this.bvid);
      await ShareUtil.shareVideo(ctx as common.UIAbilityContext, this.detail.title, url);
    } catch (err) {
      // 面板调起失败兜底为复制链接（SHR-02 站外分享的最简形态）
      Logger.warn(TAG, `分享面板失败，改复制链接：${JSON.stringify(err)}`);
      const url: string = ShareUtil.videoShareUrl(this.bvid);
      ShareUtil.copyLink(url).then(() => {
        showToast(this.getUIContext(), $r('app.string.act_link_copied'));
      }).catch((e: Object) => {
        Logger.warn(TAG, `复制链接失败：${JSON.stringify(e)}`);
      });
    }
  }

  private fmtCount(n: number): string {
    if (n >= 10000) {
      return `${(n / 10000).toFixed(1)}万`;
    }
    return `${n}`;
  }

  private coinLimit(): number {
    // 转载 1 枚、自制 2 枚（服务端 copyright：1 自制 2 转载）
    return this.detail.copyright === 1 ? 2 : 1;
  }

  build() {
    Column({ space: DliSpace.SM }) {
      Row() {
        // 点赞：单击切赞、长按 1.5s 三连（原生动画 + 幂等）。
        // 手势与拖动层同款 Parallel：Tap 与 LongPress 并存，LongPress 命中即不再发单击。
        Column({ space: 2 }) {
          SymbolGlyph(this.acts.liked || this.lightUp >= 1 ? $r('sys.symbol.heart_fill') : $r('sys.symbol.heart'))
            .fontSize(24)
            .fontColor([
              this.acts.liked || this.lightUp >= 1 ? $r('app.color.brand_primary') : $r('app.color.text_secondary')
            ])
            .scale({ x: this.likeScale, y: this.likeScale })
          if (this.acts.likeCnt > 0) {
            Text(this.fmtCount(this.acts.likeCnt))
              .fontSize(DliFont.CAPTION)
              .fontColor(this.acts.liked ? $r('app.color.brand_primary') : $r('app.color.text_tertiary'))
          }
        }
        .padding({ left: DliSpace.MD, right: DliSpace.MD, top: DliSpace.XS, bottom: DliSpace.XS })
        .gesture(
          GestureGroup(GestureMode.Parallel,
            TapGesture({ count: 1 }).onAction(() => {
              this.onLikeTap();
            }),
            LongPressGesture({ duration: 1500 }).onAction(() => {
              this.onLikeLongPress();
            })
          )
        )

        // 投币
        Column({ space: 2 }) {
          SymbolGlyph(this.acts.coinCount > 0 || this.lightUp >= 2 ? $r('sys.symbol.star_fill') : $r('sys.symbol.star'))
            .fontSize(24)
            .fontColor([
              this.acts.coinCount > 0 || this.lightUp >= 2 ? $r('app.color.brand_primary') : $r('app.color.text_secondary')
            ])
          if (this.acts.coinCnt > 0) {
            Text(this.fmtCount(this.acts.coinCnt))
              .fontSize(DliFont.CAPTION)
              .fontColor(this.acts.coinCount > 0 ? $r('app.color.brand_primary') : $r('app.color.text_tertiary'))
          }
        }
        .padding({ left: DliSpace.MD, right: DliSpace.MD, top: DliSpace.XS, bottom: DliSpace.XS })
        .onClick(() => {
          this.openCoinPop();
        })

        // 收藏
        Column({ space: 2 }) {
          SymbolGlyph(this.acts.faved || this.lightUp >= 3 ? $r('sys.symbol.star_fill') : $r('sys.symbol.star'))
            .fontSize(24)
            .fontColor([
              this.acts.faved || this.lightUp >= 3 ? $r('app.color.brand_primary') : $r('app.color.text_secondary')
            ])
          if (this.acts.favCnt > 0) {
            Text(this.fmtCount(this.acts.favCnt))
              .fontSize(DliFont.CAPTION)
              .fontColor(this.acts.faved ? $r('app.color.brand_primary') : $r('app.color.text_tertiary'))
          }
        }
        .padding({ left: DliSpace.MD, right: DliSpace.MD, top: DliSpace.XS, bottom: DliSpace.XS })
        .onClick(() => {
          this.openFavPop();
        })

        // 分享（系统面板）
        Column({ space: 2 }) {
          SymbolGlyph($r('sys.symbol.share'))
            .fontSize(24)
            .fontColor([$r('app.color.text_secondary')])
          if (this.detail.stat.share > 0) {
            Text(this.fmtCount(this.detail.stat.share))
              .fontSize(DliFont.CAPTION)
              .fontColor($r('app.color.text_tertiary'))
          }
        }
        .padding({ left: DliSpace.MD, right: DliSpace.MD, top: DliSpace.XS, bottom: DliSpace.XS })
        .onClick(() => {
          this.openShare();
        })
      }
      .width('100%')
      .justifyContent(FlexAlign.SpaceBetween)

      // 投币弹层（选 1/2 枚，上限由服务端 copyright 决定）
      if (this.acts.coinPopVisible) {
        Row({ space: DliSpace.SM }) {
          ForEach(this.coinLimit() === 1 ? [1] : [1, 2], (n: number) => {
            Text($r('app.string.act_coin_opt', n))
              .fontSize(DliFont.BODY_SM)
              .fontColor($r('app.color.text_on_brand'))
              .padding({ left: DliSpace.MD, right: DliSpace.MD, top: DliSpace.XS, bottom: DliSpace.XS })
              .borderRadius(DliRadius.FULL)
              .backgroundColor($r('app.color.brand_primary'))
              .onClick(() => {
                this.doCoin(n);
              })
          }, (n: number) => `${n}`)
          Text($r('app.string.action_cancel'))
            .fontSize(DliFont.BODY_SM)
            .fontColor($r('app.color.text_secondary'))
            .padding({ left: DliSpace.MD, right: DliSpace.MD, top: DliSpace.XS, bottom: DliSpace.XS })
            .borderRadius(DliRadius.FULL)
            .backgroundColor($r('app.color.surface_secondary'))
            .onClick(() => {
              this.acts.coinPopVisible = false;
            })
        }
        .width('100%')
        .justifyContent(FlexAlign.Center)
      }

      // 收藏弹层（默认夹置顶 + 新建）
      if (this.acts.favPopVisible) {
        Column({ space: DliSpace.XS }) {
          ForEach(this.acts.collections, (col: import('../model/Interaction').CollectionItem) => {
            Text(col.name + (col.is_default === 1 ? $r('app.string.act_fav_default_suffix') : ''))
              .fontSize(DliFont.BODY_SM)
              .fontColor($r('app.color.text_primary'))
              .padding({ left: DliSpace.MD, right: DliSpace.MD, top: DliSpace.SM, bottom: DliSpace.SM })
              .borderRadius(DliRadius.SM)
              .backgroundColor($r('app.color.surface_secondary'))
              .onClick(() => {
                this.doFav(col.id);
              })
          }, (col: import('../model/Interaction').CollectionItem) => col.id)
        }
        .width('100%')
        .alignItems(HorizontalAlign.Start)
      }
    }
    .width('100%')
  }
}

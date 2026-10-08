import { ApiError } from '../../model/Api';
import { VideoCard, MineResult } from '../../model/Video';
import { FavoritesResult } from '../../model/Interaction';
import { VideoApi } from '../../service/VideoApi';
import { InteractionApi } from '../../service/InteractionApi';
import { VideoCardItem } from '../../components/VideoCardItem';
import { LoadMoreHint } from '../../components/LoadMoreHint';
import { ErrorStateView } from '../../components/ErrorStateView';
import { LazyDataSource } from '../../common/utils/LazyDataSource';
import { AppRouter, PlayRouteParam, RouteName } from '../../common/utils/AppRouter';
import { DliFont, DliRadius, DliSize, DliSpace } from '../../common/constants/Theme';
import { DliMaterial } from '../../common/constants/MaterialTokens';
import { ErrCode, ErrorMessages } from '../../common/constants/ErrorMessages';
import { Logger } from '../../common/utils/Logger';
import { showToast } from '../../common/utils/Toast';
import { session } from '../../store/Session';

const TAG: string = 'ProfilePage';
const PAGE_SIZE: number = 20;
const TAB_MINE: string = 'mine';
const TAB_HISTORY: string = 'history';
const TAB_FAVORITES: string = 'favorites';

/**
 * 一页列表结果：total 为 -1 表示该接口不返回总数（观看历史），此时按「满页即还有下一页」推断。
 */
interface ProfilePageResult {
  list: VideoCard[];
  total: number;
}

/**
 * 「我的」页（HMY-42）：登录态由 Session 承担，列表在登录后才创建 ——
 * 列表状态放在 ProfileContent 里（而非本组件），使登录成功重新渲染到已登录分支时
 * 子组件全新创建、aboutToAppear 自然触发首屏拉取；退出登录则随分支销毁一并清空。
 */
@ComponentV2
export struct ProfilePage {
  aboutToAppear(): void {
    // 凭证在则校正一次（续期失败会被 HttpClient 清掉，此处同步回归未登录态）
    session.hydrate();
  }

  build() {
    Column() {
      if (session.loggedIn) {
        ProfileContent()
      } else {
        ProfileGuest()
      }
    }
    .width('100%')
    .height('100%')
    .justifyContent(FlexAlign.Start)
  }
}

/** 未登录态：登录入口（HMY-42 的「未登录展示登录入口」） */
@ComponentV2
struct ProfileGuest {
  build() {
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
}

/** 已登录态：资料卡 + 投稿/历史/收藏 三档 + 列表 */
@ComponentV2
struct ProfileContent {
  @Local tab: string = TAB_MINE;
  @Local firstLoading: boolean = true;
  @Local refreshing: boolean = false;
  @Local loadingMore: boolean = false;
  @Local loadMoreFailed: boolean = false;
  @Local errorText: string = '';
  @Local canRetry: boolean = true;
  @Local hasMore: boolean = true;
  @Local empty: boolean = false;
  private page: number = 1;
  private dataSource: LazyDataSource<VideoCard> = new LazyDataSource<VideoCard>();
  private scroller: Scroller = new Scroller();

  aboutToAppear(): void {
    this.loadFirstPage();
  }

  /**
   * 按当前档位取一页。三个接口的分页参数同名（page/page_size），但返回形态不同：
   * 投稿与收藏返回 total，观看历史不返回。
   */
  private async fetchPage(page: number): Promise<ProfilePageResult> {
    if (this.tab === TAB_HISTORY) {
      const list: VideoCard[] = await VideoApi.history(page, PAGE_SIZE);
      return { list: list, total: -1 };
    }
    if (this.tab === TAB_FAVORITES) {
      const res: FavoritesResult = await InteractionApi.favorites(page, PAGE_SIZE);
      return { list: res.list, total: res.total };
    }
    const res: MineResult = await VideoApi.mine(page, PAGE_SIZE);
    return { list: res.list, total: res.total };
  }

  private async loadFirstPage(): Promise<void> {
    const isRefresh: boolean = this.refreshing;
    this.page = 1;
    this.errorText = '';
    this.loadMoreFailed = false;
    this.empty = false;
    if (!isRefresh) {
      this.firstLoading = true;
    }
    try {
      const res: ProfilePageResult = await this.fetchPage(1);
      this.dataSource.reset(res.list);
      this.hasMore = res.total >= 0 ? res.list.length < res.total : res.list.length === PAGE_SIZE;
      this.empty = res.list.length === 0;
      this.scroller.scrollEdge(Edge.Top);
      Logger.info(TAG, `列表加载完成：tab=${this.tab} 共 ${res.list.length} 项 total=${res.total}`);
    } catch (err) {
      await this.onLoadFailed(err as ApiError, isRefresh);
    }
    this.firstLoading = false;
    this.refreshing = false;
  }

  /** 触底加载下一页；失败保留 page 不变，重试即重取同一页 */
  private async loadMore(): Promise<void> {
    if (!this.hasMore || this.firstLoading || this.loadingMore || this.errorText.length > 0) {
      return;
    }
    this.loadingMore = true;
    this.loadMoreFailed = false;
    const next: number = this.page + 1;
    try {
      const res: ProfilePageResult = await this.fetchPage(next);
      this.dataSource.append(res.list);
      // 有 total 的档位按「已加载数 < total」判定；观看历史无 total，退回满页推断
      this.hasMore = res.total >= 0 ? this.dataSource.count() < res.total : res.list.length === PAGE_SIZE;
      this.page = next;
      Logger.info(TAG, `加载第 ${next} 页完成：${res.list.length} 项（累计 ${this.dataSource.count()} hasMore=${this.hasMore}）`);
    } catch (err) {
      this.loadMoreFailed = true;
      Logger.warn(TAG, `加载下一页失败：${(err as ApiError).describe()}`);
    }
    this.loadingMore = false;
  }

  /**
   * 失败处理：401 交给 Session 校正登录态（校正后本页会切到未登录分支，
   * 故不在此处落错误态）；已有内容时刷新失败不砸掉列表，只给一次性提示。
   */
  private async onLoadFailed(apiErr: ApiError, isRefresh: boolean): Promise<void> {
    if (apiErr.code === ErrCode.UNAUTHORIZED) {
      Logger.warn(TAG, `列表拉取返回未授权，校正会话：${apiErr.describe()}`);
      await session.hydrate();
      return;
    }
    if (isRefresh && this.dataSource.count() > 0) {
      showToast(this.getUIContext(), $r('app.string.feed_refresh_failed'));
    } else {
      this.errorText = apiErr.message.length > 0 ? apiErr.message : ErrorMessages.text(ErrCode.INTERNAL, '');
      this.canRetry = apiErr.retryable === true;
    }
    Logger.error(TAG, `列表加载失败：${apiErr.describe()}`);
  }

  private switchTab(next: string): void {
    if (this.tab === next) {
      return;
    }
    this.tab = next;
    this.dataSource.reset([]);
    this.loadFirstPage();
  }

  private openVideo(bvid: string): void {
    const param: PlayRouteParam = { 'bvid': bvid };
    AppRouter.push(RouteName.PLAY, param);
  }

  /** 投稿入口（HMY-50）：上传链路是压栈页，与三档列表同层不冲突 */
  private openUpload(): void {
    AppRouter.push(RouteName.UPLOAD);
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

  private emptyText(): ResourceStr {
    if (this.tab === TAB_HISTORY) {
      return $r('app.string.profile_empty_history');
    }
    if (this.tab === TAB_FAVORITES) {
      return $r('app.string.profile_empty_favorites');
    }
    return $r('app.string.profile_empty_mine');
  }

  @Builder
  avatar() {
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
  }

  @Builder
  profileCard() {
    Row({ space: DliSpace.MD }) {
      this.avatar()

      Column({ space: DliSpace.XS }) {
        Row({ space: DliSpace.XS }) {
          Text(session.nickname)
            .fontSize(DliFont.TITLE_SM)
            .fontWeight(FontWeight.Medium)
            .fontColor($r('app.color.text_primary'))
          Text(`Lv${session.level}`)
            .fontSize(DliFont.CAPTION)
            .fontColor($r('app.color.brand_primary'))
            .padding({ left: 6, right: 6, top: 1, bottom: 1 })
            .borderRadius(DliRadius.SM)
            .backgroundColor($r('app.color.brand_primary_light'))
        }

        Text(session.signature.length > 0 ? session.signature : $r('app.string.profile_signature_empty'))
          .fontSize(DliFont.BODY_SM)
          .fontColor($r('app.color.text_tertiary'))
          .maxLines(1)
          .textOverflow({ overflow: TextOverflow.Ellipsis })
      }
      .alignItems(HorizontalAlign.Start)
      .layoutWeight(1)

      // 小控件用中性次级底色实填、不挂材质（材质在均匀浅底上出不了轮廓，控件会失去可点暗示）
      Text($r('app.string.profile_action_upload'))
        .fontSize(DliFont.CAPTION)
        .fontColor($r('app.color.text_on_brand'))
        .padding({ left: DliSpace.MD, right: DliSpace.MD, top: 6, bottom: 6 })
        .borderRadius(DliRadius.FULL)
        .backgroundColor($r('app.color.brand_primary'))
        .onClick(() => {
          this.openUpload();
        })

      Text($r('app.string.profile_logout'))
        .fontSize(DliFont.CAPTION)
        .fontColor($r('app.color.text_secondary'))
        .padding({ left: DliSpace.MD, right: DliSpace.MD, top: 6, bottom: 6 })
        .borderRadius(DliRadius.FULL)
        .backgroundColor($r('app.color.surface_secondary'))
        .onClick(() => {
          this.confirmLogout();
        })
    }
    .width('100%')
    .padding(DliSpace.LG)
    .borderRadius(DliRadius.LG)
    .backgroundColor(DliMaterial.surface())
    .systemMaterial(DliMaterial.card())
  }

  @Builder
  tabItem(label: Resource, value: string) {
    Text(label)
      .fontSize(DliFont.BODY_SM)
      .fontWeight(this.tab === value ? FontWeight.Medium : FontWeight.Normal)
      .fontColor(this.tab === value ? $r('app.color.brand_primary') : $r('app.color.text_secondary'))
      .onClick(() => {
        this.switchTab(value);
      })
  }

  @Builder
  header() {
    Column({ space: DliSpace.MD }) {
      this.profileCard()

      Row({ space: DliSpace.LG }) {
        this.tabItem($r('app.string.profile_tab_mine'), TAB_MINE)
        this.tabItem($r('app.string.profile_tab_history'), TAB_HISTORY)
        this.tabItem($r('app.string.profile_tab_favorites'), TAB_FAVORITES)
      }
      .width('100%')
    }
    .width('100%')
    .padding({ left: DliSpace.PAGE, right: DliSpace.PAGE, top: DliSpace.SM })
  }

  @Builder
  feed() {
    Refresh({ refreshing: $$this.refreshing }) {
      Grid(this.scroller) {
        LazyForEach(this.dataSource, (item: VideoCard) => {
          GridItem() {
            VideoCardItem({
              video: item,
              showDate: true,
              onTap: (bvid: string) => {
                this.openVideo(bvid);
              }
            })
          }
        }, (item: VideoCard) => item.bvid)

        GridItem() {
          LoadMoreHint({
            loading: this.loadingMore,
            failed: this.loadMoreFailed,
            done: !this.hasMore && this.dataSource.count() > 0,
            onRetry: () => {
              this.loadMore();
            }
          })
        }
        .columnStart(0)
        .columnEnd(1)
      }
      .columnsTemplate('1fr 1fr')
      .columnsGap(DliSpace.MD)
      .rowsGap(DliSpace.LG)
      .padding({ left: DliSpace.PAGE, right: DliSpace.PAGE, top: DliSpace.SM, bottom: DliSize.TAB_RESERVED })
      .scrollBar(BarState.Off)
      .onReachEnd(() => {
        this.loadMore();
      })
      .width('100%')
      .height('100%')
    }
    .layoutWeight(1)
    .onRefreshing(() => {
      this.loadFirstPage();
    })
  }

  @Builder
  emptyView() {
    Column() {
      Text(this.emptyText())
        .fontSize(DliFont.BODY_SM)
        .fontColor($r('app.color.text_tertiary'))
    }
    .width('100%')
    .layoutWeight(1)
    .justifyContent(FlexAlign.Center)
  }

  build() {
    Column() {
      this.header()

      if (this.firstLoading) {
        Column() {
          LoadingProgress()
            .width(40)
            .height(40)
            .color($r('app.color.brand_primary'))
        }
        .width('100%')
        .layoutWeight(1)
        .justifyContent(FlexAlign.Center)
      } else if (this.errorText.length > 0) {
        Column() {
          ErrorStateView({
            message: this.errorText,
            canRetry: this.canRetry,
            onRetry: () => {
              this.loadFirstPage();
            }
          })
        }
        .width('100%')
        .layoutWeight(1)
      } else if (this.empty) {
        this.emptyView()
      } else {
        this.feed()
      }
    }
    .width('100%')
    .height('100%')
  }
}

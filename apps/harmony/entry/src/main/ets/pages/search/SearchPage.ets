import { ApiError } from '../../model/Api';
import { Profile } from '../../model/Auth';
import { PagedResult, VideoCard } from '../../model/Video';
import { SearchApi } from '../../service/SearchApi';
import { SearchHistoryStore } from '../../store/SearchHistoryStore';
import { VideoCardItem } from '../../components/VideoCardItem';
import { LoadMoreHint } from '../../components/LoadMoreHint';
import { ErrorStateView } from '../../components/ErrorStateView';
import { LazyDataSource } from '../../common/utils/LazyDataSource';
import { resolveMediaUrl } from '../../common/utils/MediaUrl';
import { DliFont, DliRadius, DliSize, DliSpace } from '../../common/constants/Theme';
import { DliMaterial } from '../../common/constants/MaterialTokens';
import { ErrCode, ErrorMessages } from '../../common/constants/ErrorMessages';
import { Logger } from '../../common/utils/Logger';
import { showToast } from '../../common/utils/Toast';

const TAG: string = 'SearchPage';
const PAGE_SIZE: number = 20;
const TAB_VIDEO: number = 0;
const TAB_USER: number = 1;

/**
 * 搜索页（HMY-41）：关键词搜索视频 / UP 主 + 端侧搜索历史。
 *
 * 上游缺口（本期零后端改动）：
 * - SRH-02 排序与筛选：/search 无排序与筛选参数，端侧无法自造（跨页客户端排序语义不成立）；
 * - SRH-04 云端同步：全仓无搜索历史接口，故历史只落本地副本（SearchHistoryStore）。
 */
@ComponentV2
export struct SearchPage {
  @Local keyword: string = '';
  /** 已提交的关键词；空串表示尚未搜索，此时展示搜索历史 */
  @Local submitted: string = '';
  @Local tab: number = TAB_VIDEO;
  @Local history: string[] = [];
  @Local firstLoading: boolean = false;
  @Local loadingMore: boolean = false;
  @Local loadMoreFailed: boolean = false;
  @Local errorText: string = '';
  @Local canRetry: boolean = true;
  @Local hasMore: boolean = false;
  @Local total: number = 0;
  private page: number = 1;
  private videoSource: LazyDataSource<VideoCard> = new LazyDataSource<VideoCard>();
  private userSource: LazyDataSource<Profile> = new LazyDataSource<Profile>();
  private gridScroller: Scroller = new Scroller();
  private listScroller: Scroller = new Scroller();

  aboutToAppear(): void {
    this.loadHistory();
  }

  private async loadHistory(): Promise<void> {
    this.history = await SearchHistoryStore.list();
  }

  private async submit(): Promise<void> {
    const kw: string = this.keyword.trim();
    if (kw.length === 0) {
      showToast(this.getUIContext(), $r('app.string.search_need_keyword'));
      return;
    }
    this.keyword = kw;
    await SearchHistoryStore.add(kw);
    this.history = await SearchHistoryStore.list();
    this.submitted = kw;
    this.searchFirstPage();
  }

  private switchTab(next: number): void {
    if (this.tab === next) {
      return;
    }
    this.tab = next;
    this.total = 0;
    this.hasMore = false;
    this.searchFirstPage();
  }

  private async searchFirstPage(): Promise<void> {
    if (this.submitted.length === 0) {
      return;
    }
    this.page = 1;
    this.errorText = '';
    this.loadMoreFailed = false;
    this.firstLoading = true;
    try {
      if (this.tab === TAB_VIDEO) {
        const res: PagedResult<VideoCard> = await SearchApi.searchVideos(this.submitted, 1, PAGE_SIZE);
        this.videoSource.reset(res.list);
        this.total = res.total;
        this.hasMore = res.list.length < res.total;
      } else {
        const res: PagedResult<Profile> = await SearchApi.searchUsers(this.submitted, 1, PAGE_SIZE);
        this.userSource.reset(res.list);
        this.total = res.total;
        this.hasMore = res.list.length < res.total;
      }
      this.activeScroller().scrollEdge(Edge.Top);
      Logger.info(TAG, `搜索完成：kw=${this.submitted} type=${this.tab} total=${this.total}`);
    } catch (err) {
      const apiErr: ApiError = err as ApiError;
      this.errorText = apiErr.message.length > 0 ? apiErr.message : ErrorMessages.text(ErrCode.INTERNAL, '');
      this.canRetry = apiErr.retryable === true;
      Logger.error(TAG, `搜索失败：${apiErr.describe()}`);
    }
    this.firstLoading = false;
  }

  private async loadMore(): Promise<void> {
    if (!this.hasMore || this.firstLoading || this.loadingMore || this.errorText.length > 0) {
      return;
    }
    this.loadingMore = true;
    this.loadMoreFailed = false;
    const next: number = this.page + 1;
    try {
      if (this.tab === TAB_VIDEO) {
        const res: PagedResult<VideoCard> = await SearchApi.searchVideos(this.submitted, next, PAGE_SIZE);
        this.videoSource.append(res.list);
        this.total = res.total;
        this.hasMore = this.videoSource.count() < res.total;
      } else {
        const res: PagedResult<Profile> = await SearchApi.searchUsers(this.submitted, next, PAGE_SIZE);
        this.userSource.append(res.list);
        this.total = res.total;
        this.hasMore = this.userSource.count() < res.total;
      }
      this.page = next;
    } catch (err) {
      this.loadMoreFailed = true;
      Logger.warn(TAG, `加载下一页失败：${(err as ApiError).describe()}`);
    }
    this.loadingMore = false;
  }

  private activeScroller(): Scroller {
    return this.tab === TAB_VIDEO ? this.gridScroller : this.listScroller;
  }

  private async useHistory(kw: string): Promise<void> {
    this.keyword = kw;
    await SearchHistoryStore.add(kw);
    this.history = await SearchHistoryStore.list();
    this.submitted = kw;
    this.searchFirstPage();
  }

  private async removeHistory(kw: string): Promise<void> {
    await SearchHistoryStore.remove(kw);
    this.history = await SearchHistoryStore.list();
  }

  private async clearHistory(): Promise<void> {
    await SearchHistoryStore.clear();
    this.history = await SearchHistoryStore.list();
  }

  /** 播放页与个人空间分别归 M4-HMY-06 / M4-HMY-09，先给可感知反馈，避免点击无响应 */
  private openVideo(bvid: string): void {
    Logger.info(TAG, `点击稿件 ${bvid}（播放页待 M4-HMY-06 接入）`);
    showToast(this.getUIContext(), $r('app.string.video_pending'));
  }

  @Builder
  searchBar() {
    Row({ space: DliSpace.MD }) {
      Row({ space: DliSpace.XS }) {
        SymbolGlyph($r('sys.symbol.magnifyingglass'))
          .fontSize(16)
          .fontColor([$r('app.color.text_tertiary')])
        TextInput({ text: $$this.keyword, placeholder: $r('app.string.search_placeholder') })
          .maxLength(50)
          .height(36)
          .fontSize(DliFont.BODY_SM)
          .placeholderColor($r('app.color.text_tertiary'))
          .backgroundColor(Color.Transparent)
          .padding(0)
          .layoutWeight(1)
          .enterKeyType(EnterKeyType.Search)
          .onSubmit(() => {
            this.submit();
          })
        if (this.keyword.length > 0) {
          SymbolGlyph($r('sys.symbol.xmark_circle'))
            .fontSize(16)
            .fontColor([$r('app.color.text_tertiary')])
            .onClick(() => {
              // 清空即回历史面板（与 Web 端一致），无需再点搜索
              this.keyword = '';
            })
        }
      }
      .layoutWeight(1)
      .height(36)
      .padding({ left: DliSpace.MD, right: DliSpace.MD })
      .borderRadius(DliRadius.FULL)
      .backgroundColor(DliMaterial.surface())
      .systemMaterial(DliMaterial.chip())
      .alignItems(VerticalAlign.Center)

      Text($r('app.string.search_action'))
        .fontSize(DliFont.BODY_SM)
        .fontColor($r('app.color.brand_primary'))
        .onClick(() => {
          this.submit();
        })
    }
    .width('100%')
    .padding({ left: DliSpace.PAGE, right: DliSpace.PAGE, top: DliSpace.SM, bottom: DliSpace.SM })
    .alignItems(VerticalAlign.Center)
  }

  @Builder
  historyPanel() {
    Column({ space: DliSpace.MD }) {
      if (this.history.length > 0) {
        Row() {
          Text($r('app.string.search_history_title'))
            .fontSize(DliFont.BODY_SM)
            .fontColor($r('app.color.text_secondary'))
          Blank()
          Text($r('app.string.search_history_clear'))
            .fontSize(DliFont.CAPTION)
            .fontColor($r('app.color.text_tertiary'))
            .onClick(() => {
              this.clearHistory();
            })
        }
        .width('100%')
        .alignItems(VerticalAlign.Center)

        Flex({ wrap: FlexWrap.Wrap }) {
          ForEach(this.history, (kw: string) => {
            Row({ space: DliSpace.SM }) {
              Text(kw)
                .fontSize(DliFont.BODY_SM)
                .fontColor($r('app.color.text_secondary'))
                .onClick(() => {
                  this.useHistory(kw);
                })
              SymbolGlyph($r('sys.symbol.xmark'))
                .fontSize(12)
                .fontColor([$r('app.color.text_tertiary')])
                .onClick(() => {
                  this.removeHistory(kw);
                })
            }
            .padding({ left: DliSpace.MD, right: DliSpace.MD, top: 6, bottom: 6 })
            .borderRadius(DliRadius.FULL)
            // 小标签不挂材质：材质会压过 backgroundColor，而它在均匀浅底上又出不了轮廓，二者相抵会变成"没有底"
            .backgroundColor($r('app.color.surface_secondary'))
            .margin({ right: DliSpace.SM, bottom: DliSpace.SM })
          }, (kw: string) => kw)
        }
        .width('100%')
      } else {
        Column() {
          Text($r('app.string.search_idle'))
            .fontSize(DliFont.BODY_SM)
            .fontColor($r('app.color.text_tertiary'))
        }
        .width('100%')
        .layoutWeight(1)
        .justifyContent(FlexAlign.Center)
      }
    }
    .width('100%')
    .layoutWeight(1)
    .padding({ left: DliSpace.PAGE, right: DliSpace.PAGE })
    .alignItems(HorizontalAlign.Start)
  }

  @Builder
  tabItem(index: number, label: ResourceStr) {
    Text(label)
      .fontSize(DliFont.BODY_SM)
      .fontWeight(this.tab === index ? FontWeight.Medium : FontWeight.Normal)
      .fontColor(this.tab === index ? $r('app.color.brand_primary') : $r('app.color.text_secondary'))
      .padding({ top: 6, bottom: 6 })
      .onClick(() => {
        this.switchTab(index);
      })
  }

  @Builder
  resultHeader() {
    Row({ space: DliSpace.XL }) {
      this.tabItem(TAB_VIDEO, $r('app.string.search_tab_video'))
      this.tabItem(TAB_USER, $r('app.string.search_tab_user'))
      Blank()
      if (this.total > 0) {
        Text($r('app.string.search_total', this.total))
          .fontSize(DliFont.CAPTION)
          .fontColor($r('app.color.text_tertiary'))
      }
    }
    .width('100%')
    .padding({ left: DliSpace.PAGE, right: DliSpace.PAGE, top: DliSpace.XS, bottom: DliSpace.SM })
    .alignItems(VerticalAlign.Center)
  }

  @Builder
  videoGrid() {
    Grid(this.gridScroller) {
      LazyForEach(this.videoSource, (item: VideoCard) => {
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
          done: !this.hasMore,
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
    .layoutWeight(1)
  }

  @Builder
  userRow(user: Profile) {
    Row({ space: DliSpace.MD }) {
      if (user.avatar.length > 0) {
        Image(resolveMediaUrl(user.avatar))
          .width(48)
          .height(48)
          .borderRadius(DliRadius.FULL)
          .objectFit(ImageFit.Cover)
      } else {
        Column()
          .width(48)
          .height(48)
          .borderRadius(DliRadius.FULL)
          .backgroundColor($r('app.color.brand_primary_light'))
      }
      Column({ space: DliSpace.XS }) {
        Text(user.nickname)
          .fontSize(DliFont.BODY)
          .fontWeight(FontWeight.Medium)
          .fontColor($r('app.color.text_primary'))
          .maxLines(1)
          .textOverflow({ overflow: TextOverflow.Ellipsis })
          .width('100%')
        Text(user.signature.length > 0 ? user.signature : $r('app.string.user_no_signature'))
          .fontSize(DliFont.CAPTION)
          .fontColor($r('app.color.text_tertiary'))
          .maxLines(1)
          .textOverflow({ overflow: TextOverflow.Ellipsis })
          .width('100%')
      }
      .layoutWeight(1)
      .alignItems(HorizontalAlign.Start)

      Text(`Lv${user.level}`)
        .fontSize(DliFont.CAPTION)
        .fontColor($r('app.color.brand_primary'))
        .padding({ left: 6, right: 6, top: 2, bottom: 2 })
        .borderRadius(4)
        .backgroundColor($r('app.color.brand_primary_light'))
    }
    .width('100%')
    .padding(DliSpace.MD)
    .borderRadius(DliRadius.MD)
    .backgroundColor(DliMaterial.surface())
    .systemMaterial(DliMaterial.card())
    .alignItems(VerticalAlign.Center)
    .onClick(() => {
      Logger.info(TAG, `点击 UP 主 ${user.id}（个人空间待 M4-HMY-09 接入）`);
      showToast(this.getUIContext(), $r('app.string.space_pending'));
    })
  }

  @Builder
  userList() {
    List({ space: DliSpace.MD, scroller: this.listScroller }) {
      LazyForEach(this.userSource, (item: Profile) => {
        ListItem() {
          this.userRow(item)
        }
      }, (item: Profile) => item.id)

      ListItem() {
        LoadMoreHint({
          loading: this.loadingMore,
          failed: this.loadMoreFailed,
          done: !this.hasMore,
          onRetry: () => {
            this.loadMore();
          }
        })
      }
    }
    .width('100%')
    .layoutWeight(1)
    .padding({ left: DliSpace.PAGE, right: DliSpace.PAGE, bottom: DliSize.TAB_RESERVED })
    .scrollBar(BarState.Off)
    .onReachEnd(() => {
      this.loadMore();
    })
  }

  @Builder
  resultContent() {
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
            this.searchFirstPage();
          }
        })
      }
      .width('100%')
      .layoutWeight(1)
    } else if (this.total === 0) {
      Column() {
        Text(this.tab === TAB_VIDEO ? $r('app.string.search_empty_video') : $r('app.string.search_empty_user'))
          .fontSize(DliFont.BODY_SM)
          .fontColor($r('app.color.text_tertiary'))
      }
      .width('100%')
      .layoutWeight(1)
      .justifyContent(FlexAlign.Center)
    } else {
      if (this.tab === TAB_VIDEO) {
        this.videoGrid()
      } else {
        this.userList()
      }
    }
  }

  build() {
    Column() {
      this.searchBar()
      // 输入框清空即回到历史；已提交且仍有输入时展示结果
      if (this.submitted.length === 0 || this.keyword.trim().length === 0) {
        this.historyPanel()
      } else {
        // tab 行独立于结果态，空结果/失败时仍可切 tab
        this.resultHeader()
        this.resultContent()
      }
    }
    .width('100%')
    .height('100%')
  }
}

import { ApiError } from '../../model/Api';
import { Category, VideoCard } from '../../model/Video';
import { VideoApi } from '../../service/VideoApi';
import { VideoCardItem } from '../../components/VideoCardItem';
import { LoadMoreHint } from '../../components/LoadMoreHint';
import { ErrorStateView } from '../../components/ErrorStateView';
import { LazyDataSource } from '../../common/utils/LazyDataSource';
import { DliFont, DliRadius, DliSpace } from '../../common/constants/Theme';
import { ErrCode, ErrorMessages } from '../../common/constants/ErrorMessages';
import { Logger } from '../../common/utils/Logger';
import { showToast } from '../../common/utils/Toast';

const TAG: string = 'HomePage';
const PAGE_SIZE: number = 20;
const SORT_RECOMMEND: string = 'recommend';
const SORT_NEW: string = 'new';
const SORT_HOT: string = 'hot';

/**
 * 首页：分区导航 + 推荐/最新/最热 + 信息流（HMY-40）。
 *
 * 推荐流走 /recommend/videos（REC-01，与 Web 端首页「推荐」同一接口），**不支持分区过滤**，
 * 故选定分区时自动切到「最新」；最新/最热走 /videos?sort=。两者都不返回 total，
 * 以「满页即还有下一页」推断 hasMore（与 Web 端 useHomeFeed 同口径）。
 */
@ComponentV2
export struct HomePage {
  @Local categories: Category[] = [];
  /** 0 = 首页（全站） */
  @Local categoryId: number = 0;
  @Local sort: string = SORT_RECOMMEND;
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
    this.loadCategories();
    this.loadFirstPage();
  }

  /** 分区栏失败不阻断信息流，仅记日志（信息流自身的失败另有三态处理） */
  private async loadCategories(): Promise<void> {
    try {
      const all: Category[] = await VideoApi.listCategories();
      this.categories = all.filter((item: Category) => item.parent_id === 0);
    } catch (err) {
      Logger.warn(TAG, `分区列表加载失败：${(err as ApiError).describe()}`);
    }
  }

  private fetchPage(page: number): Promise<VideoCard[]> {
    return this.sort === SORT_RECOMMEND
      ? VideoApi.listRecommended(page, PAGE_SIZE)
      : VideoApi.listVideos(this.categoryId, this.sort, page, PAGE_SIZE);
  }

  private async loadFirstPage(): Promise<void> {
    this.page = 1;
    this.errorText = '';
    this.loadMoreFailed = false;
    this.empty = false;
    if (!this.refreshing) {
      this.firstLoading = true;
    }
    try {
      const list: VideoCard[] = await this.fetchPage(1);
      this.dataSource.reset(list);
      this.hasMore = list.length === PAGE_SIZE;
      this.empty = list.length === 0;
      this.scroller.scrollEdge(Edge.Top);
      Logger.info(TAG, `信息流加载完成：sort=${this.sort} category=${this.categoryId} 共 ${list.length} 项`);
    } catch (err) {
      const apiErr: ApiError = err as ApiError;
      if (this.dataSource.count() > 0) {
        // 已有内容时刷新失败不砸掉列表，只给一次性提示
        showToast(this.getUIContext(), $r('app.string.feed_refresh_failed'));
      } else {
        this.errorText = apiErr.message.length > 0 ? apiErr.message : ErrorMessages.text(ErrCode.INTERNAL, '');
        this.canRetry = apiErr.retryable === true;
      }
      Logger.error(TAG, `信息流加载失败：${apiErr.describe()}`);
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
      const list: VideoCard[] = await this.fetchPage(next);
      this.dataSource.append(list);
      this.hasMore = list.length === PAGE_SIZE;
      this.page = next;
    } catch (err) {
      this.loadMoreFailed = true;
      Logger.warn(TAG, `加载下一页失败：${(err as ApiError).describe()}`);
    }
    this.loadingMore = false;
  }

  private selectCategory(id: number): void {
    if (this.categoryId === id) {
      return;
    }
    this.categoryId = id;
    if (id > 0 && this.sort === SORT_RECOMMEND) {
      this.sort = SORT_NEW;
    }
    this.loadFirstPage();
  }

  private switchSort(next: string): void {
    if (this.sort === next) {
      return;
    }
    this.sort = next;
    this.loadFirstPage();
  }

  /** 播放页归 M4-HMY-06，此处先给可感知反馈，避免点击无响应 */
  private openVideo(bvid: string): void {
    Logger.info(TAG, `点击稿件 ${bvid}（播放页待 M4-HMY-06 接入）`);
    showToast(this.getUIContext(), $r('app.string.video_pending'));
  }

  @Builder
  categoryChip(id: number, label: ResourceStr) {
    Text(label)
      .fontSize(DliFont.BODY_SM)
      .fontColor(this.categoryId === id ? $r('app.color.text_on_brand') : $r('app.color.text_secondary'))
      .padding({ left: DliSpace.MD, right: DliSpace.MD, top: 6, bottom: 6 })
      .borderRadius(DliRadius.FULL)
      .backgroundColor(this.categoryId === id ? $r('app.color.brand_primary') : $r('app.color.bg_surface'))
      .onClick(() => {
        this.selectCategory(id);
      })
  }

  @Builder
  sortTab(label: ResourceStr, value: string) {
    Text(label)
      .fontSize(DliFont.BODY_SM)
      .fontWeight(this.sort === value ? FontWeight.Medium : FontWeight.Normal)
      .fontColor(this.sort === value ? $r('app.color.brand_primary') : $r('app.color.text_secondary'))
      .onClick(() => {
        this.switchSort(value);
      })
  }

  @Builder
  header() {
    Column({ space: DliSpace.SM }) {
      Scroll() {
        Row({ space: DliSpace.SM }) {
          this.categoryChip(0, $r('app.string.home_category_all'))
          ForEach(this.categories, (item: Category) => {
            this.categoryChip(item.id, item.name)
          }, (item: Category) => `${item.id}`)
        }
      }
      .scrollable(ScrollDirection.Horizontal)
      .scrollBar(BarState.Off)
      .width('100%')

      Row({ space: DliSpace.LG }) {
        if (this.categoryId === 0) {
          this.sortTab($r('app.string.home_sort_recommend'), SORT_RECOMMEND)
        }
        this.sortTab($r('app.string.home_sort_new'), SORT_NEW)
        this.sortTab($r('app.string.home_sort_hot'), SORT_HOT)
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
      .padding({ left: DliSpace.PAGE, right: DliSpace.PAGE, top: DliSpace.SM, bottom: DliSpace.XL })
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
      Text($r('app.string.feed_empty'))
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
    .backgroundColor($r('app.color.bg_page'))
  }
}

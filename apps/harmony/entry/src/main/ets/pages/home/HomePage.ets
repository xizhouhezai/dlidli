import { ApiError } from '../../model/Api';
import { Category } from '../../model/Video';
import { VideoApi } from '../../service/VideoApi';
import { ErrorStateView } from '../../components/ErrorStateView';
import { DliFont, DliRadius, DliSpace } from '../../common/constants/Theme';
import { ErrCode, ErrorMessages } from '../../common/constants/ErrorMessages';
import { Logger } from '../../common/utils/Logger';

const TAG: string = 'HomePage';

/**
 * 首页：分区导航 + 信息流。
 * 本任务先接通分区列表以验证网络层（加载中/成功/失败可重试三态，HMY-03），
 * 信息流列表随 M4-HMY-05 在此页补齐。
 */
@ComponentV2
export struct HomePage {
  @Local loading: boolean = true;
  @Local errorText: string = '';
  @Local canRetry: boolean = true;
  @Local categories: Category[] = [];

  aboutToAppear(): void {
    this.load();
  }

  private async load(): Promise<void> {
    this.loading = true;
    this.errorText = '';
    try {
      this.categories = await VideoApi.listCategories();
      Logger.info(TAG, `分区列表加载完成，共 ${this.categories.length} 项`);
    } catch (err) {
      const apiErr: ApiError = err as ApiError;
      const message: string = apiErr.message ?? '';
      this.errorText = message.length > 0 ? message : ErrorMessages.text(ErrCode.INTERNAL, '');
      this.canRetry = apiErr.retryable === true;
      Logger.error(TAG, `分区列表加载失败：${apiErr.describe()}`);
    }
    this.loading = false;
  }

  @Builder
  categoryBar() {
    Column({ space: DliSpace.SM }) {
      Text($r('app.string.home_section_category'))
        .fontSize(DliFont.TITLE_SM)
        .fontWeight(FontWeight.Medium)
        .fontColor($r('app.color.text_primary'))
        .width('100%')

      Scroll() {
        Row({ space: DliSpace.SM }) {
          ForEach(this.categories, (item: Category) => {
            Text(item.name)
              .fontSize(DliFont.BODY_SM)
              .fontColor($r('app.color.text_secondary'))
              .padding({ left: DliSpace.MD, right: DliSpace.MD, top: DliSpace.XS, bottom: DliSpace.XS })
              .borderRadius(DliRadius.FULL)
              .backgroundColor($r('app.color.bg_surface'))
          }, (item: Category) => `${item.id}`)
        }
      }
      .scrollable(ScrollDirection.Horizontal)
      .scrollBar(BarState.Off)
      .width('100%')
    }
    .width('100%')
    .padding({ left: DliSpace.PAGE, right: DliSpace.PAGE, top: DliSpace.MD })
  }

  build() {
    Column() {
      if (this.loading) {
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
        ErrorStateView({
          message: this.errorText,
          canRetry: this.canRetry,
          onRetry: () => {
            this.load();
          }
        })
      } else {
        this.categoryBar()
        Column() {
          Text($r('app.string.page_pending_home'))
            .fontSize(DliFont.BODY_SM)
            .fontColor($r('app.color.text_tertiary'))
        }
        .width('100%')
        .layoutWeight(1)
        .justifyContent(FlexAlign.Center)
      }
    }
    .width('100%')
    .height('100%')
    .backgroundColor($r('app.color.bg_page'))
  }
}

import { DliSize } from '../common/constants/Theme';

const PRIMARY_BLOB_SIZE: number = 320;
/** 光斑重心落在标题栏后方：上移一个标题栏高度再抬高 60 */
const PRIMARY_BLOB_Y: number = 0 - DliSize.TITLE_BAR_HEIGHT - 60;
const SECONDARY_BLOB_SIZE: number = 240;
const SECONDARY_BLOB_Y: number = -90;
const SECONDARY_BLOB_X: string = '58%';
/** 远景柔光：落在下半屏，让底部标签栏与空态页也"有光可透"，避免下半屏塌成纯色 */
const FAR_BLOB_SIZE: number = 480;
const FAR_BLOB_Y: string = '58%';

/**
 * 品牌氛围底：一层极淡的品牌洗色 + 三处柔和光斑。
 *
 * 沉浸光感是"透"出来的——纯色底上做磨砂与纯色底在观感上无异。故壳层统一铺这一层，
 * 给标题栏、胶囊底栏与卡片提供可折射的内容，玻璃质感才立得住。
 * 全应用只挂一次（壳层 Stack 的最底层），各页因此不再设不透明页面底色。
 *
 * **克制是这一层的全部要点**：品牌粉只作为"洗色"出现（顶部 ≈3% 亮度差），光斑最高 12% 不透明度。
 * 早先版本把品牌粉铺到 30%，界面到处是半透明的粉，玻璃与品牌色互相稀释，反倒读不出主次。
 *
 * 光斑用渐隐到透明的矩形而非 `Circle`：形状组件自带默认填充色，会盖掉作为背景层的渐变。
 */
@ComponentV2
export struct AmbientBackdrop {
  @Builder
  blob(size: number, color: Resource, x: Length, y: Length) {
    Column()
      .width(size)
      .height(size)
      .radialGradient({
        center: ['50%', '50%'],
        radius: size / 2,
        colors: [[color, 0.0], [Color.Transparent, 1.0]]
      })
      .position({ x: x, y: y })
  }

  build() {
    Stack() {
      // 基底：顶部品牌粉 → 页面底色，约在屏高 62% 处收敛为纯页面底色
      Column()
        .width('100%')
        .height('100%')
        .linearGradient({
          angle: 180,
          colors: [
            [$r('app.color.ambient_top'), 0.0],
            [$r('app.color.ambient_mid'), 0.2],
            [$r('app.color.bg_page'), 0.62]
          ]
        })

      // 远景柔光（最淡，铺在下半屏，压在标签栏后方）
      this.blob(FAR_BLOB_SIZE, $r('app.color.ambient_blob_soft'), -160, FAR_BLOB_Y)
      // 左上主光斑（品牌粉）
      this.blob(PRIMARY_BLOB_SIZE, $r('app.color.ambient_blob'), -110, PRIMARY_BLOB_Y)
      // 右上副光斑（品牌浅粉，偏暖，避免同色双斑发闷）
      this.blob(SECONDARY_BLOB_SIZE, $r('app.color.ambient_blob_alt'), SECONDARY_BLOB_X, SECONDARY_BLOB_Y)
    }
    .width('100%')
    .height('100%')
  }
}

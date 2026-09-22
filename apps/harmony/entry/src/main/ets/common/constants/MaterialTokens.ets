import { uiMaterial } from '@kit.ArkUI';

/**
 * 沉浸光感（API 26 系统材质）token 工厂。
 *
 * `systemMaterial()` 是 CommonMethod 上的通用属性（@since 26.0.0），配合 module.json5 的
 * `ohos.arkui.UIMaterial.state = enable`，除 Navigation 标题栏与底部 TabBar 外**任意组件**都可挂载。
 * 层级按官方《沉浸光感》规范取值：顶部悬浮 ULTRA_THIN、卡片/胶囊 REGULAR~THIN、弹出 THICK。
 *
 * 三条必须守住的边界：
 * ① **赋色取中性、不取品牌色**——`materialColor` 是全应用铺开的底色，一律染品牌粉会让玻璃与品牌色
 *    互相稀释，界面上到处是"半透明的粉"，反倒读不出主次（HDS 官方样张同样只在**选中态**用强调色）。
 *    品牌粉收敛到选中态与主按钮，玻璃本身取次级底色（灰）——页面底是白的，材质若也用白，
 *    在浅底上会退化成"没有底"（HDS 的口径：页面 primary 白、次级 secondary 灰）。
 * ② 材质只作增强、不作视觉底线——能力不支持时 `systemMaterial` 静默无效，低算力设备上
 *    `style`/`materialColor` 也不生效，故同一组件仍需经 `surface()` 给出磨砂兜底色，否则卡片会变透明。
 * ③ 材质对象在模块加载时构造一次并复用，逐项 `new` 会在 LazyForEach 里产生无谓开销。
 *
 * 底栏不在此处——它已改为 HdsTabs 悬浮胶囊，材质经 `barFloatingStyle.systemMaterialEffect` 交给 HDS 出。
 */
const SUPPORTED: boolean = uiMaterial.isImmersiveMaterialSupported();

function materialOf(options: uiMaterial.ImmersiveOptions): uiMaterial.ImmersiveMaterial | undefined {
  return SUPPORTED ? new uiMaterial.ImmersiveMaterial(options) : undefined;
}

const TITLE_BAR: uiMaterial.ImmersiveMaterial | undefined = materialOf({
  style: uiMaterial.ImmersiveStyle.ULTRA_THIN,
  interactive: true,
  lightEffect: { color: $r('app.color.material_edge') }
});

const CARD: uiMaterial.ImmersiveMaterial | undefined = materialOf({
  style: uiMaterial.ImmersiveStyle.REGULAR,
  materialColor: $r('app.color.surface_secondary'),
  applyShadow: true
});

const CHIP: uiMaterial.ImmersiveMaterial | undefined = materialOf({
  style: uiMaterial.ImmersiveStyle.THIN,
  materialColor: $r('app.color.surface_secondary'),
  interactive: true,
  applyShadow: true,
  lightEffect: { color: $r('app.color.material_edge') }
});

export class DliMaterial {
  static isSupported(): boolean {
    return SUPPORTED;
  }

  /** Navigation 标题栏：顶部悬浮，最薄一层 + 内容可从其下滚过 */
  static titleBar(): uiMaterial.ImmersiveMaterial | undefined {
    return TITLE_BAR;
  }

  /** 信息流卡片、列表行、资料卡：页面底色 + 投影，浮在氛围底之上 */
  static card(): uiMaterial.ImmersiveMaterial | undefined {
    return CARD;
  }

  /** 可点的胶囊/按钮：薄层 + 按压形变 + 感光反馈 */
  static chip(): uiMaterial.ImmersiveMaterial | undefined {
    return CHIP;
  }

  /**
   * 材质层的地基色：始终给次级底色，不返回透明。
   *
   * 实测（模拟器 API 26）`systemMaterial` **不会吃掉 `backgroundColor`**——二者同挂时底色照旧透出来，
   * 材质只在上面叠一层光感。反过来，若把地基色设成透明，材质在浅色页底上几乎不可见（卡片会"消失"）。
   * 故这里把兜底色与材质自带的赋色取同一档中性色：能力可用时是"中性玻璃 + 光感"，
   * 不可用时退化为平铺的次级底色，两种情况下组件都不会丢轮廓。
   */
  static surface(): ResourceColor {
    return $r('app.color.surface_secondary');
  }

  /** 关掉某个组件的系统材质；注意传 undefined 是「恢复为无材质」而非本方法语义 */
  static off(): uiMaterial.Material {
    return uiMaterial.Material.empty;
  }

  /** 应用级材质开关状态（module.json5 的 ohos.arkui.UIMaterial.state 生效结果） */
  static appState(): uiMaterial.MaterialState {
    return uiMaterial.getMaterialInfo().state;
  }

  /** 全局材质档位；低算力设备上 style/materialColor/colorInvert 可能不生效 */
  static globalLevel(): uiMaterial.MaterialLevel {
    return uiMaterial.getGlobalMaterialLevel();
  }

  /** 能力探测留痕：支持性 / 应用级开关状态 / 全局材质档，启动时打一条从日志即可核对 */
  static describe(): string {
    return `supported=${SUPPORTED} state=${DliMaterial.appState()} level=${DliMaterial.globalLevel()}`;
  }
}

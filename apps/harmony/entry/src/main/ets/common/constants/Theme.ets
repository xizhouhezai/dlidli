/**
 * 品牌 token：与 Web 端 apps/web/src/styles/_variables.scss 同源，ArkTS 侧以常量形式落地。
 * Canvas 绘制（弹幕、图表）只能消费字面量，故此处不依赖 $r() 资源。
 */

export class DliColor {
  static readonly PRIMARY: string = '#FB7299';
  static readonly PRIMARY_LIGHT: string = '#FFD9E4';
  static readonly PRIMARY_HOVER: string = '#FC8BAB';
  static readonly ON_PRIMARY: string = '#FFFFFF';
  static readonly TEXT_1: string = '#18191C';
  static readonly TEXT_2: string = '#61666D';
  static readonly TEXT_3: string = '#9499A0';
  static readonly BORDER: string = '#E3E5E7';
  static readonly BG_PAGE: string = '#F6F7F8';
  static readonly BG_SURFACE: string = '#FFFFFF';
  static readonly MASK: string = '#CC000000';
}

/** 对应 Web 端 $radius-sm/md/lg，单位 vp */
export class DliRadius {
  static readonly SM: number = 6;
  static readonly MD: number = 8;
  static readonly LG: number = 12;
  static readonly FULL: number = 999;
}

export class DliSpace {
  static readonly XS: number = 4;
  static readonly SM: number = 8;
  static readonly MD: number = 12;
  static readonly LG: number = 16;
  static readonly XL: number = 24;
  static readonly PAGE: number = 12;
}

export class DliFont {
  static readonly CAPTION: number = 11;
  static readonly BODY_SM: number = 13;
  static readonly BODY: number = 15;
  static readonly TITLE_SM: number = 17;
  static readonly TITLE: number = 20;
  static readonly TAB_LABEL: number = 10;
}

export class DliSize {
  static readonly TAB_ICON: number = 24;
  static readonly TAB_BAR_HEIGHT: number = 56;
  static readonly TAB_MASK_HEIGHT: number = 88;
  static readonly TITLE_BAR_HEIGHT: number = 56;
}

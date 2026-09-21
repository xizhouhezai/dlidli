import { DliRadius } from '../common/constants/Theme';

// 与后端 SVG 的 120x40 同为 3:1，保证等比放大、不变形
const BOX_W: number = 150;
const BOX_H: number = 50;
const DEFAULT_BG: string = '#f5f5f5';
const DEFAULT_FONT_SIZE: number = 22;

interface SvgLine {
  x1: number;
  y1: number;
  x2: number;
  y2: number;
  stroke: string;
}

interface SvgChar {
  x: number;
  y: number;
  size: number;
  fill: string;
  angle: number;
  text: string;
}

/**
 * 图形验证码渲染：后端 /auth/captcha 返回内联 SVG，字符以 `<text transform="rotate(...)">` 绘制。
 *
 * 两条 ArkUI 能力路线实测均不可用，故改为端侧解析 SVG 后用 Canvas 重绘：
 * 1) Image 组件对 SVG 的支持**不含 `<text>`**（同一份 SVG 里 `<rect>`/`<line>` 正常、`<text>` 不渲染）；
 * 2) ArkWeb 能加载 data URI（onControllerAttached / onPageEnd 均触发）但**画面不合成**，
 *    连纯色 div 都不上屏，模拟器上整块空白。
 * SVG 由本仓库自己的服务端生成，元素形态固定（rect + 4 line + 4 text），解析成本可控；
 * 本期零后端改动，接口形态不变。
 */
@ComponentV2
export struct CaptchaImage {
  /** 待渲染的 SVG 原文，空串时不绘制 */
  @Param svg: string = '';
  @Event onRefresh: () => void = () => {};
  private settings: RenderingContextSettings = new RenderingContextSettings(true);
  private ctx: CanvasRenderingContext2D = new CanvasRenderingContext2D(this.settings);
  private ready: boolean = false;

  @Monitor('svg')
  onSvgChange(): void {
    this.draw();
  }

  private draw(): void {
    if (!this.ready || this.svg.length === 0) {
      return;
    }
    const ctx: CanvasRenderingContext2D = this.ctx;
    ctx.clearRect(0, 0, BOX_W, BOX_H);

    const rootAttrs: string = matchAttr(this.svg, /<svg\b([^>]*)>/, 1);
    const svgW: number = attrNum(rootAttrs, 'width', BOX_W);
    const svgH: number = attrNum(rootAttrs, 'height', BOX_H);
    const scale: number = Math.min(BOX_W / svgW, BOX_H / svgH);
    const offsetX: number = (BOX_W - svgW * scale) / 2;
    const offsetY: number = (BOX_H - svgH * scale) / 2;

    const bgMatched: RegExpMatchArray | null = this.svg.match(/<rect\b[^>]*fill="([^"]*)"/);
    ctx.fillStyle = bgMatched !== null ? bgMatched[1] : DEFAULT_BG;
    ctx.fillRect(0, 0, BOX_W, BOX_H);

    const lineRe: RegExp = /<line\b([^>]*)\/>/g;
    let lineMatch: RegExpExecArray | null = lineRe.exec(this.svg);
    while (lineMatch !== null) {
      const attrs: string = lineMatch[1];
      ctx.beginPath();
      ctx.strokeStyle = attr(attrs, 'stroke');
      ctx.lineWidth = attrNum(attrs, 'stroke-width', 1) * scale;
      ctx.moveTo(offsetX + attrNum(attrs, 'x1', 0) * scale, offsetY + attrNum(attrs, 'y1', 0) * scale);
      ctx.lineTo(offsetX + attrNum(attrs, 'x2', 0) * scale, offsetY + attrNum(attrs, 'y2', 0) * scale);
      ctx.stroke();
      lineMatch = lineRe.exec(this.svg);
    }

    const textRe: RegExp = /<text\b([^>]*)>([^<]*)<\/text>/g;
    let textMatch: RegExpExecArray | null = textRe.exec(this.svg);
    while (textMatch !== null) {
      const attrs: string = textMatch[1];
      const rotateMatched: RegExpMatchArray | null = attrs.match(/rotate\(\s*(-?[\d.]+)/);
      const angle: number = rotateMatched !== null ? Number.parseFloat(rotateMatched[1]) : 0;
      // SVG 的 text x/y 是基线起点，Canvas 的 textBaseline 默认也是 alphabetic，
      // 故平移到 (x,y) 后直接 fillText 即可对齐；rotate(a x y) 等价于绕该点旋转。
      ctx.save();
      ctx.translate(offsetX + attrNum(attrs, 'x', 0) * scale, offsetY + attrNum(attrs, 'y', 0) * scale);
      ctx.rotate(angle * Math.PI / 180);
      ctx.font = `${Math.round(attrNum(attrs, 'font-size', DEFAULT_FONT_SIZE) * scale)}px monospace`;
      ctx.fillStyle = attr(attrs, 'fill');
      ctx.fillText(textMatch[2], 0, 0);
      ctx.restore();
      textMatch = textRe.exec(this.svg);
    }
  }

  build() {
    Stack() {
      Canvas(this.ctx)
        .width(BOX_W)
        .height(BOX_H)
        .onReady(() => {
          this.ready = true;
          this.draw();
        })
    }
    .width(BOX_W)
    .height(BOX_H)
    .clip(true)
    .borderRadius(DliRadius.MD)
    .backgroundColor($r('app.color.bg_surface'))
    .onClick(() => {
      this.onRefresh();
    })
  }
}

function attr(attrs: string, name: string): string {
  const matched: RegExpMatchArray | null = attrs.match(new RegExp(`${name}="([^"]*)"`));
  return matched !== null ? matched[1] : '';
}

function attrNum(attrs: string, name: string, fallback: number): number {
  const parsed: number = Number.parseFloat(attr(attrs, name));
  return Number.isNaN(parsed) ? fallback : parsed;
}

function matchAttr(source: string, pattern: RegExp, group: number): string {
  const matched: RegExpMatchArray | null = source.match(pattern);
  return matched !== null ? matched[group] : '';
}

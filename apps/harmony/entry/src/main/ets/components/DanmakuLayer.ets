import { DanmakuController } from '../danmaku/DanmakuController';
import { DanmakuFrame, dmLetterboxBar } from '../danmaku/DanmakuEngine';
import { dmSettings } from '../danmaku/DanmakuSettings';
import { Logger } from '../common/utils/Logger';

const TAG: string = 'DanmakuLayer';

/** 渲染帧间隔（毫秒，≈60fps） */
const FRAME_INTERVAL_MS: number = 16;
/**
 * 全屏且**无上下黑边**时的同屏密度系数：官方影音娱乐规范要求全屏无黑边时弹幕不宜过多，
 * 这里按比例压低轨道数（0.6）。有黑边时不用它——那时弹幕已被限制在上方黑边区域内。
 */
const FULLSCREEN_DENSITY_SCALE: number = 0.6;

/**
 * 弹幕渲染层（Canvas）。
 *
 * 分工：页面给尺寸与播放态，控制器给数据与引擎，本组件只做「逐帧驱动」——
 * 每帧取当前位置（含按倍速外推）→ 补分段/上屏 → 绘制。
 *
 * 触摸：整层 `HitTestMode.None`（自身与子节点都不参与命中，也不阻塞兄弟节点），
 * 否则这层 Canvas 会把播放页的手势层（单击/双击/拖动）全吃掉。
 */
@ComponentV2
export struct DanmakuLayer {
  @Require @Param controller: DanmakuController;
  /** 舞台尺寸（vp）：弹幕区域与轨道分配都按它算 */
  @Param stageWidth: number = 0;
  @Param stageHeight: number = 0;
  /** 视频原始画幅（用于判断上下黑边） */
  @Param videoWidth: number = 0;
  @Param videoHeight: number = 0;
  @Param fullscreen: boolean = false;

  private ctxSettings: RenderingContextSettings = new RenderingContextSettings(true);
  private ctx: CanvasRenderingContext2D = new CanvasRenderingContext2D(this.ctxSettings);
  private timerId: number = -1;
  private lastLayoutSig: string = '';
  private lastStyleSig: string = '';

  build() {
    Canvas(this.ctx)
      .width('100%')
      .height('100%')
      .hitTestBehavior(HitTestMode.None)
      .onReady(() => {
        this.controller.attachCanvas(this.ctx);
        this.startLoop();
      })
  }

  aboutToDisappear(): void {
    this.stopLoop();
  }

  /** 画幅/全屏变化：区域变了，旧的轨道号在新区间可能越界，交由控制器清屏重来 */
  @Monitor('stageWidth', 'stageHeight', 'fullscreen', 'videoWidth', 'videoHeight')
  onStageChange(): void {
    // 画幅是弹幕区域的唯一输入，出问题时（尤其真机上）先看这行日志即可定位
    const bar: number = dmLetterboxBar(this.stageWidth, this.stageHeight, this.videoWidth, this.videoHeight);
    Logger.info(TAG,
      `弹幕区域：舞台 ${this.stageWidth}x${this.stageHeight}vp 视频 ${this.videoWidth}x${this.videoHeight} 全屏=${this.fullscreen} 黑边=${bar}vp`);
    this.controller.onFrameChanged();
  }

  private startLoop(): void {
    if (this.timerId !== -1) {
      return;
    }
    this.timerId = setInterval(() => {
      this.onFrame();
    }, FRAME_INTERVAL_MS);
  }

  private stopLoop(): void {
    if (this.timerId !== -1) {
      clearInterval(this.timerId);
      this.timerId = -1;
    }
  }

  private onFrame(): void {
    // 影响轨道数的档位（区域/密度）改了要清屏重排；只影响画笔的档位（开关/透明度/字号/速度）重画即可。
    // 二者都必须在这里比较：暂停时没有新弹幕入队，光靠引擎的 dirty 标记不会重绘。
    const layoutSig: string = this.layoutSig();
    if (layoutSig !== this.lastLayoutSig) {
      this.lastLayoutSig = layoutSig;
      this.controller.onFrameChanged();
    }
    const styleSig: string = this.styleSig();
    if (styleSig !== this.lastStyleSig) {
      this.lastStyleSig = styleSig;
      this.controller.markDirty();
    }
    const videoMs: number = this.controller.videoMs(Date.now());
    this.controller.tick(videoMs);
    this.controller.draw(videoMs, this.frame(), dmSettings);
  }

  /** 影响轨道分配与弹幕区域上限的设置（改了要清屏重排） */
  private layoutSig(): string {
    return `${dmSettings.area}|${dmSettings.density}`;
  }

  /** 只影响单条弹幕绘制参数的设置（改了重画即可） */
  private styleSig(): string {
    const s = dmSettings;
    return `${s.enabled}|${s.opacity}|${s.fontScale}|${s.speed}`;
  }

  /**
   * 弹幕绘制区域：上下有黑边时**只在上方黑边内**显示（官方影音娱乐规范）；
   * 无黑边时铺满舞台，全屏态再压低同屏密度。
   */
  private frame(): DanmakuFrame {
    const bar: number = dmLetterboxBar(this.stageWidth, this.stageHeight, this.videoWidth, this.videoHeight);
    if (bar > 0) {
      return { left: 0, top: 0, width: this.stageWidth, height: bar, densityScale: 1 };
    }
    return {
      left: 0,
      top: 0,
      width: this.stageWidth,
      height: this.stageHeight,
      densityScale: this.fullscreen ? FULLSCREEN_DENSITY_SCALE : 1
    };
  }
}

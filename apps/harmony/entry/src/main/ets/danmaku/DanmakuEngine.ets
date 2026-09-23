import { DanmakuItem } from '../model/Danmaku';
import { DanmakuSettings } from './DanmakuSettings';

/** 单条弹幕占用的轨道高度（vp） */
const LINE_HEIGHT: number = 32;
/** 首轨道距区域顶部/底部的留白，避免贴边 */
const TRACK_PAD: number = 4;
/** 顶部/底部固定弹幕的驻留时长（毫秒，视频时间口径） */
const FIXED_DURATION_MS: number = 4000;
/** 轨道数下限：区域再矮也要有几条能过 */
const MIN_TRACKS: number = 3;
/** 同帧绘制上限，兜底防止异常数据把一帧拖爆 */
const MAX_ACTIVE: number = 240;
/** 黑边窄于一整行时放不下任何弹幕，按「无有效黑边」处理 */
const MIN_BAR_HEIGHT: number = LINE_HEIGHT + TRACK_PAD * 2;
/** 字体降部占字号的比例：用于把基线从行框底部上提（无字体度量接口，按经验取 1/4） */
const DESCENT_RATIO: number = 0.25;

/**
 * 弹幕绘制区域（vp，相对弹幕层左上角）。
 *
 * `densityScale` 只在**全屏且无上下黑边**时小于 1：官方影音娱乐规范要求全屏无黑边时
 * 「同屏弹幕不宜过多」，故按比例压低轨道数（见 DanmakuLayer.frame）。
 */
export interface DanmakuFrame {
  left: number;
  top: number;
  width: number;
  height: number;
  densityScale: number;
}

/** 上屏中的弹幕（位置是 videoMs 的纯函数，故不存 x/进度，只存静态要素） */
interface ActiveItem {
  id: string;
  content: string;
  /** 1 滚动 2 顶部 3 底部 */
  kind: number;
  color: string;
  fontSize: number;
  textWidth: number;
  /** 命中轨道在**区域内**的序号，绘制时结合 frame 换算绝对位置 */
  track: number;
  /** 起算时间 = 弹幕自身的 time_ms（视频时间），暂停/倍速/seek 全部自动跟随 */
  startMs: number;
  durationMs: number;
  isSelf: boolean;
}

/**
 * 弹幕渲染引擎：轨道分配 + Canvas 逐帧绘制。
 *
 * 核心取舍——**以视频时间（而非墙钟）为相位基准**：一条弹幕的位置是
 * `(videoMs - startMs) / durationMs` 的纯函数。由此免费得到三个正确行为：
 * ① 暂停时弹幕冻住（与画面同步，Web 端用 DOM 动画做不到，暂停后弹幕仍在飘）；
 * ② seek 后按新位置落到正确轨迹，而不是从右侧重新飘一遍；
 * ③ 分段/实时弹幕**迟到**时直接出现在它该在的位置，回退轮询也能对齐画面。
 * 代价是没有「动画完成回调」——回收改为每帧按 `videoMs` 判定，见 `draw`。
 */
export class DanmakuEngine {
  private ctx: CanvasRenderingContext2D | null = null;
  /** 待上屏（已拿到数据但未到时间点） */
  private pending: DanmakuItem[] = [];
  private active: ActiveItem[] = [];
  /** 轨道占用截止时间（视频时间毫秒），滚动/顶部/底部各自独立 */
  private busyScroll: number[] = [];
  private busyTop: number[] = [];
  private busyBottom: number[] = [];
  /** 画面自上次绘制后是否已变化；配合 lastDrawnMs 跳过「暂停且无变化」时的空转重绘 */
  private dirty: boolean = true;
  private lastDrawnMs: number = -1;

  attach(ctx: CanvasRenderingContext2D): void {
    this.ctx = ctx;
    this.dirty = true;
  }

  detach(): void {
    this.ctx = null;
  }

  /** 入队一条弹幕（去重由调用方负责，见 DanmakuController 的 shown 集合） */
  push(item: DanmakuItem): void {
    this.pending.push(item);
    this.dirty = true;
  }

  /** seek / 关弹幕 / 画幅变化时清屏：活动列表与轨道占用一并作废，避免新旧叠在一起 */
  clear(): void {
    this.pending = [];
    this.active = [];
    this.busyScroll = [];
    this.busyTop = [];
    this.busyBottom = [];
    this.dirty = true;
  }

  /** 展示设置变化后由渲染方调用（字号/不透明度/区域改了必须重画） */
  markDirty(): void {
    this.dirty = true;
  }

  /** 活动弹幕条数（仅用于日志与实测核对） */
  activeCount(): number {
    return this.active.length;
  }

  /**
   * 把已到时间点的弹幕放入轨道。需在 `draw` 前调用（同一帧内先分配后绘制，
   * 否则新弹幕要等下一帧才出现，表现为「慢半拍」）。
   */
  spawnDue(videoMs: number, frame: DanmakuFrame, settings: DanmakuSettings): void {
    const ctx: CanvasRenderingContext2D | null = this.ctx;
    if (ctx === null || this.pending.length === 0) {
      return;
    }
    const remain: DanmakuItem[] = [];
    for (const item of this.pending) {
      // 已过期的直接丢弃（分段拉取回来时可能整段都在过去）
      if (item.time_ms > videoMs) {
        remain.push(item);
        continue;
      }
      this.enter(item, frame, settings, ctx);
    }
    this.pending = remain;
  }

  /** 绘制一帧并回收已结束的弹幕 */
  draw(videoMs: number, frame: DanmakuFrame, settings: DanmakuSettings): void {
    const ctx: CanvasRenderingContext2D | null = this.ctx;
    if (ctx === null) {
      return;
    }
    // 位置未变（暂停/缓冲）且无新增弹幕时，画面与上一帧完全一致，省掉一次全画布重绘
    if (!this.dirty && videoMs === this.lastDrawnMs) {
      return;
    }
    this.dirty = false;
    this.lastDrawnMs = videoMs;
    ctx.clearRect(0, 0, ctx.width, ctx.height);
    if (!settings.enabled) {
      this.active = [];
      return;
    }

    const alive: ActiveItem[] = [];
    let currentFont: string = '';
    for (const item of this.active) {
      const elapsed: number = videoMs - item.startMs;
      if (elapsed < 0) {
        alive.push(item);
        continue;
      }
      const progress: number = elapsed / item.durationMs;
      if (progress >= 1) {
        continue; // 结束：本帧开始不再绘制
      }
      const font: string = `600 ${item.fontSize}vp sans-serif`;
      if (font !== currentFont) {
        ctx.font = font;
        currentFont = font;
      }
      const x: number = item.kind === 1
        ? this.scrollX(item, frame, progress)
        : frame.left + (frame.width - item.textWidth) / 2;
      const y: number = this.itemY(item, frame);
      ctx.globalAlpha = settings.opacity * this.fadeFactor(item, progress);
      // 先描边后填充：描边提供暗底轮廓，填充盖住内半边，比投影更省
      ctx.lineWidth = 2;
      ctx.strokeStyle = 'rgba(0, 0, 0, 0.75)';
      ctx.strokeText(item.content, x, y);
      ctx.fillStyle = item.color;
      ctx.fillText(item.content, x, y);
      if (item.isSelf) {
        ctx.lineWidth = 1;
        ctx.strokeStyle = 'rgba(255, 255, 255, 0.8)';
        ctx.strokeRect(x - 2, this.lineTop(item, frame) + 1, item.textWidth + 4, LINE_HEIGHT - 2);
      }
      alive.push(item);
    }
    ctx.globalAlpha = 1;
    this.active = alive;
  }

  private enter(
    item: DanmakuItem,
    frame: DanmakuFrame,
    settings: DanmakuSettings,
    ctx: CanvasRenderingContext2D
  ): void {
    if (this.active.length >= MAX_ACTIVE) {
      return;
    }
    const kind: number = item.mode === 2 ? 2 : item.mode === 3 ? 3 : 1;
    const fontSize: number = settings.baseFontSize(item.font_size) * settings.fontScale;
    ctx.font = `600 ${fontSize}vp sans-serif`;
    const textWidth: number = ctx.measureText(item.content).width;
    const tracks: number = this.trackCount(frame, settings);
    if (tracks === 0) {
      return; // 区域放不下整行：宁可不显示，也不画到画面里
    }
    const track: number = this.allocTrack(kind, tracks, item.time_ms, settings);
    this.active.push({
      id: item.id,
      content: item.content,
      kind: kind,
      color: colorHex(item.color),
      fontSize: fontSize,
      textWidth: textWidth,
      track: track,
      startMs: item.time_ms,
      durationMs: kind === 1 ? settings.scrollDurationMs() : FIXED_DURATION_MS,
      isSelf: item.is_self === true
    });
  }

  /** 可用轨道数：区域高度 × 显示区域比例 × 密度档 × 全屏密度系数，再按轨道高换算 */
  private trackCount(frame: DanmakuFrame, settings: DanmakuSettings): number {
    // 物理容量：区域内真正放得下的行数。密度档与 MIN_TRACKS 都不得突破它——
    // 上下黑边场景下突破就意味着弹幕画到画面里（官方规范要求只在上方黑边内显示）。
    // 例如 2.39:1 宽银幕片横屏全屏时黑边只有约 64vp，放得下 1 行，MIN_TRACKS 的 3 行会溢出到画面。
    const capacity: number = Math.floor((frame.height - TRACK_PAD * 2) / LINE_HEIGHT);
    if (capacity <= 0) {
      return 0; // 连一行都放不下（黑边窄于 MIN_BAR_HEIGHT 时不会走到这里）
    }
    const usable: number = frame.height * settings.areaRatio() * settings.densityRatio() * frame.densityScale;
    const count: number = Math.floor(usable / LINE_HEIGHT);
    return Math.min(Math.max(MIN_TRACKS, count), capacity);
  }

  /**
   * 轨道分配：取第一条空闲轨道；全满时随机叠放（密度已由轨道数与区域控制）。
   * 占用时长取滚动总时长的 1/3（同速弹幕在前车走完 1/3 时入场，后续保持固定间距不追尾）。
   */
  private allocTrack(kind: number, tracks: number, startMs: number, settings: DanmakuSettings): number {
    const busy: number[] = kind === 2 ? this.busyTop : kind === 3 ? this.busyBottom : this.busyScroll;
    const hold: number = kind === 1 ? settings.scrollDurationMs() / 3 : FIXED_DURATION_MS;
    for (let i = 0; i < tracks; i++) {
      if (busy[i] === undefined || busy[i] <= startMs) {
        busy[i] = startMs + hold;
        return i;
      }
    }
    return Math.floor(Math.random() * tracks);
  }

  /** 滚动弹幕横向位置：区域右缘外 → 左缘外 */
  private scrollX(item: ActiveItem, frame: DanmakuFrame, progress: number): number {
    const from: number = frame.left + frame.width;
    const to: number = frame.left - item.textWidth;
    return from + (to - from) * progress;
  }

  /** 顶/底固定弹幕的纵向位置：自区域上缘（顶部）/下缘（底部）按轨道序号排布 */
  private itemY(item: ActiveItem, frame: DanmakuFrame): number {
    // fillText 的 y 是**基线**，不是行框上缘：基线要落在行框底部再上提一个降部。
    // 少了这一提，整行文字会比行框高出约一个升部——首行压到区域外被裁掉（实测）。
    return this.lineTop(item, frame) + LINE_HEIGHT - item.fontSize * DESCENT_RATIO;
  }

  /** 轨道行框上缘（vp）：顶/滚动自区域上缘往下排，底部自区域下缘往上排 */
  private lineTop(item: ActiveItem, frame: DanmakuFrame): number {
    if (item.kind === 3) {
      return frame.top + frame.height - TRACK_PAD - (item.track + 1) * LINE_HEIGHT;
    }
    return frame.top + TRACK_PAD + item.track * LINE_HEIGHT;
  }

  /** 顶/底弹幕的淡入淡出（首尾各 10%） */
  private fadeFactor(item: ActiveItem, progress: number): number {
    if (item.kind === 1) {
      return 1;
    }
    if (progress < 0.1) {
      return progress / 0.1;
    }
    if (progress > 0.9) {
      return (1 - progress) / 0.1;
    }
    return 1;
  }
}

/** RGB 整数 → #rrggbb（服务端 color 默认 0xFFFFFF） */
export function colorHex(color: number): string {
  const value: number = color > 0 ? color : 0xffffff;
  return `#${value.toString(16).padStart(6, '0')}`;
}

/** 弹幕区域：把「上下黑边」规则算成可视区域，供引擎与页面共用（见 DanmakuLayer.frame） */
export function dmLetterboxBar(stageWidth: number, stageHeight: number, videoWidth: number, videoHeight: number): number {
  if (stageWidth <= 0 || stageHeight <= 0 || videoWidth <= 0 || videoHeight <= 0) {
    return 0;
  }
  const videoAspect: number = videoWidth / videoHeight;
  const stageAspect: number = stageWidth / stageHeight;
  if (videoAspect <= stageAspect) {
    return 0; // 视频更「方」：左右出现黑边，上下没有
  }
  const bar: number = (stageHeight - stageWidth / videoAspect) / 2;
  return bar >= MIN_BAR_HEIGHT ? bar : 0;
}

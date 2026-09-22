import { display, window } from '@kit.ArkUI';
import { emitter } from '@kit.BasicServicesKit';
import { ApiError } from '../../model/Api';
import { StreamItem, VideoDetail } from '../../model/Video';
import { VideoApi } from '../../service/VideoApi';
import { ErrorStateView } from '../../components/ErrorStateView';
import { AppRouter } from '../../common/utils/AppRouter';
import { AppEvent } from '../../common/constants/AppEvents';
import { resolveMediaUrl } from '../../common/utils/MediaUrl';
import { formatCount, formatDuration, formatPubdate } from '../../common/utils/Format';
import { signExpiryMs } from '../../media/PlaySign';
import { VideoPlayer } from '../../media/VideoPlayer';
import { PlaybackProgressStore } from '../../store/PlaybackProgressStore';
import { TokenStore } from '../../store/TokenStore';
import { DliFont, DliRadius, DliSpace } from '../../common/constants/Theme';
import { ErrCode, ErrorMessages } from '../../common/constants/ErrorMessages';
import { showToast } from '../../common/utils/Toast';
import { Logger } from '../../common/utils/Logger';

const TAG: string = 'PlayPage';

/**
 * 采样间隔：以 positionSec 之差累计观看时长（对齐 Web 端 usePlaybackReport 的「取真实增量」口径）。
 * 取 500ms 而非 1s，是因为 2x 倍速下 1s 采样的增量恰好到 2.0，会被 SEEK_JUMP_SEC 判成跳转而不计数。
 */
const SAMPLE_INTERVAL_MS: number = 500;
/** 进度落盘节流（对齐 Web 端 10s） */
const SAVE_INTERVAL_MS: number = 10000;
/** 有效播放门槛（PLY-05 / HMY-13） */
const VIEW_THRESHOLD_SEC: number = 5;
/** 单次采样位移超过它即认为是跳转，不计入观看时长 */
const SEEK_JUMP_SEC: number = 2;
/** 签名剩余不足此时长就换新地址（后端 TTL 6h，留足重取详情与重连缓冲） */
const RENEW_MARGIN_MS: number = 300000;
/** 片头/片尾这两段内不续播 */
const RESUME_MIN_SEC: number = 3;
const RESUME_TAIL_SEC: number = 3;
/** 手势层里判定拖动的最小位移（vp） */
const DRAG_DISTANCE: number = 12;
/** 拖动收尾后继续屏蔽单击的时长，见 onStageTap */
const DRAG_TAP_GUARD_MS: number = 200;
/** 竖拖改亮度/音量时，手指划过整屏高度对应的变化量 */
const DRAG_FULL_RANGE: number = 1;
/** 亮度下限：允许压暗但不能到 0，否则用户看不到恢复的控件 */
const BRIGHTNESS_MIN: number = 0.05;

/**
 * 控制层上下蒙层：控件是白字 + 半透明白轨，压在亮画面上会整体消失
 * （实测白色画面下进度条与时间戳几乎看不见）。用渐变蒙层保底，越靠近边缘越实。
 */
const SCRIM_TOP: string = '#B3000000';
const SCRIM_BOTTOM: string = '#CC000000';
const SCRIM_NONE: string = '#00000000';

const RATE_OPTIONS: number[] = [0.5, 0.75, 1, 1.25, 1.5, 2];
/** 未知画幅时的兜底（16:9） */
const DEFAULT_RATIO: number = 16 / 9;

/** 设置面板里的单个选项（清晰度/倍速共用）；单对象入参是为了让 @Builder 走按引用传递 */
interface ChipOption {
  label: ResourceStr;
  active: boolean;
  onTap: () => void;
}

/** 一次拖动的主轴：起手方向定死，途中不再切换 */
enum DragAxis {
  NONE = 0,
  SEEK = 1,
  VOLUME = 2,
  BRIGHTNESS = 3
}

/**
 * 播放页（NavDestination，承 HMY-10~15）。
 *
 * 分工：`media/VideoPlayer` 管播放器状态机与事件，本页管业务——详情装载、跨端续播、
 * 进度与有效播放上报、签名续签、手势、横屏全屏、切后台。
 *
 * 三个必须守住的时序：
 * ① **surface 先于起播**：AVPlayer 的 surfaceId 首次只能赋在 initialized 态，故等 XComponent 的
 *    onLoad（surfaceReady）与详情（detailReady）都就绪才 open，二者先到先等（见 tryStart）。
 * ② **换源即 open**：切清晰度与签名续签都是「重取地址 + 保留进度与播放态换源」，
 *    在 VideoPlayer 内是同一条 reset→url→prepare→seek→play 路径。
 * ③ **离开即落盘**：进度不可只依赖节流定时器，返回、切后台、销毁三处都必须主动 flush。
 *
 * 本期未接：弹幕（M4-HMY-07）、互动与评论（M4-HMY-08），页内以一行提示标注。
 */
@ComponentV2
export struct PlayPage {
  @Require @Param bvid: string;

  @Local loading: boolean = true;
  @Local errorText: string = '';
  @Local canRetry: boolean = true;
  @Local noStream: boolean = false;
  @Local title: string = '';
  @Local metaText: string = '';
  @Local ownerName: string = '';
  @Local ownerAvatar: string = '';
  @Local description: string = '';
  @Local tags: string[] = [];
  @Local streams: StreamItem[] = [];
  @Local qualityIndex: number = 0;

  @Local controlsVisible: boolean = true;
  @Local panelOpen: boolean = false;
  @Local fullscreen: boolean = false;
  @Local sliderSec: number = 0;
  @Local sliderDragging: boolean = false;
  @Local dragHint: string = '';

  /** 播放器是业务无关的状态容器，页面只读它的 @Trace 字段驱动 UI */
  private player: VideoPlayer = new VideoPlayer();
  private surfaceController: XComponentController = new XComponentController();
  private screenWidthVp: number = 0;
  private playUrl: string = '';
  private resumeMs: number = 0;
  private surfaceReady: boolean = false;
  private detailReady: boolean = false;
  /** 换源进行中：屏蔽进度条的程序化 onChange 被误判为用户拖动（见 reopen） */
  private switching: boolean = false;
  private started: boolean = false;

  private tickerId: number = -1;
  private lastSampleSec: number = 0;
  private watchedSec: number = 0;
  private viewReported: boolean = false;
  private lastSaveMs: number = 0;
  private renewing: boolean = false;

  /** 窗口亮度：端侧无读接口，从满亮起算；用户真调过则在离开时还原为跟随系统 */
  private windowBrightness: number = 1;
  private brightnessTouched: boolean = false;
  private dragStartSec: number = 0;
  private dragStartValue: number = 1;
  private dragAxis: DragAxis = DragAxis.NONE;
  private dragPivotX: number = 0;
  private dragging: boolean = false;
  private dragEndMs: number = 0;
  private lastDurationSec: number = 0;

  private bgListener: (data: emitter.EventData) => void = (data: emitter.EventData) => {
    this.onBackground();
  };

  aboutToAppear(): void {
    this.player.onEnded = () => {
      // 自然播完也要落盘：否则下次进来会从 0 重看
      this.flushProgress();
    };
    this.updateScreenMetrics();
    // 压栈页拿不到 @Entry 页的生命周期，切后台只能靠 Ability 广播（见 AppEvents）
    emitter.on({ eventId: AppEvent.ID_BACKGROUND }, this.bgListener);
    this.loadDetail();
  }

  aboutToDisappear(): void {
    this.stopTicker();
    emitter.off(AppEvent.ID_BACKGROUND, this.bgListener);
    this.flushProgress();
    this.player.release();
    // 全屏与横屏是窗口级状态，离开必须还原，否则返回列表仍是横屏；亮度同理，别把 App 留在暗屏
    if (this.fullscreen) {
      this.applyWindowMode(false);
    }
    if (this.brightnessTouched) {
      this.restoreSystemBrightness();
    }
  }

  /** 详情装载：只落页面需要的字段，避免 build 里到处判空 */
  private async loadDetail(): Promise<void> {
    this.loading = true;
    this.errorText = '';
    this.noStream = false;
    try {
      const detail: VideoDetail = await VideoApi.detail(this.bvid);
      this.applyDetail(detail);
      const list: StreamItem[] = detail.streams ?? [];
      if (list.length === 0) {
        this.noStream = true;
        this.loading = false;
        Logger.warn(TAG, `稿件 ${this.bvid} 无可播放流`);
        return;
      }
      this.streams = list;
      // 服务端按 quality 降序下发，取首档即最高画质（与 Web 端 pickDefaultSource 同口径）
      this.qualityIndex = 0;
      this.playUrl = resolveMediaUrl(list[0].url);
      this.resumeMs = (await this.resolveResume(detail.duration)) * 1000;
      this.detailReady = true;
      this.loading = false;
      this.tryStart();
    } catch (err) {
      const apiErr: ApiError = err as ApiError;
      this.errorText = apiErr.message.length > 0 ? apiErr.message : ErrorMessages.text(ErrCode.INTERNAL, '');
      this.canRetry = apiErr.retryable === true;
      this.loading = false;
      Logger.error(TAG, `详情加载失败：${apiErr.describe()}`);
    }
  }

  private applyDetail(detail: VideoDetail): void {
    this.title = detail.title;
    const parts: string[] = [formatCount(detail.stat.view)];
    const date: string = formatPubdate(detail.published_at ?? detail.created_at);
    if (date.length > 0) {
      parts.push(date);
    }
    parts.push(detail.bvid);
    this.metaText = parts.join(' · ');
    this.ownerName = detail.owner.nickname;
    this.ownerAvatar = resolveMediaUrl(detail.owner.avatar);
    this.description = detail.description;
    this.tags = detail.tags ?? [];
  }

  /**
   * 续播位置：登录取服务端（跨端续播），未登录或读失败回落端侧缓存。
   * 片头几秒与片尾几秒内不续播——那两处续播对用户是打扰。
   */
  private async resolveResume(durationSec: number): Promise<number> {
    let pos: number = 0;
    if (TokenStore.isLoggedIn()) {
      try {
        pos = await VideoApi.getProgress(this.bvid);
      } catch (err) {
        Logger.warn(TAG, `服务端进度不可用，回落端侧缓存：${(err as ApiError).describe()}`);
      }
    }
    if (pos <= 0) {
      pos = await PlaybackProgressStore.get(this.bvid);
    }
    if (pos <= RESUME_MIN_SEC) {
      return 0;
    }
    if (durationSec > 0 && pos >= durationSec - RESUME_TAIL_SEC) {
      return 0;
    }
    showToast(this.getUIContext(), $r('app.string.play_resumed'));
    return pos;
  }

  /** 详情与 surface 都就绪才起播（两者顺序不定，先到先等） */
  private tryStart(): void {
    if (this.started || !this.detailReady || !this.surfaceReady) {
      return;
    }
    this.started = true;
    this.lastSampleSec = this.resumeMs / 1000;
    this.startTicker();
    this.reopen(this.playUrl, this.resumeMs, true);
  }

  private startTicker(): void {
    if (this.tickerId !== -1) {
      return;
    }
    this.tickerId = setInterval(() => {
      this.onTick();
    }, SAMPLE_INTERVAL_MS);
  }

  private stopTicker(): void {
    if (this.tickerId !== -1) {
      clearInterval(this.tickerId);
      this.tickerId = -1;
    }
  }

  /** 心跳：观看时长累计 + 有效播放 + 签名续签 + 进度落盘，全部收在这一处 */
  private onTick(): void {
    const pos: number = this.player.positionSec();
    const delta: number = pos - this.lastSampleSec;
    this.lastSampleSec = pos;

    if (this.player.phase === 'playing' && delta > 0 && delta < SEEK_JUMP_SEC) {
      this.watchedSec += delta;
      if (!this.viewReported && this.watchedSec >= VIEW_THRESHOLD_SEC) {
        this.viewReported = true;
        VideoApi.reportView(this.bvid).catch(() => {
          // 上报失败允许重试：服务端按 uid/IP 去重，重发不会重复计数
          this.viewReported = false;
        });
      }
    }

    // 换源（续签/切清晰度）会让播放器 reset 到 preparing，此时 position/duration 都是 0：
    // 让它照写会把进度条打回 0 并把总时长显示成 00:01。换源本就丢失了当前帧（同实例 reset），
    // 但进度条不该跟着跳——跳了就不叫「用户无感」了。
    if (!this.sliderDragging && !this.switching) {
      this.sliderSec = pos;
    }
    this.maybeRenew();
    this.maybeSave(pos);
  }

  /** 进度落盘：端侧缓存给未登录/读服务端失败兜底，服务端进度才是跨端续播的依据 */
  private maybeSave(pos: number): void {
    const now: number = Date.now();
    if (now - this.lastSaveMs < SAVE_INTERVAL_MS) {
      return;
    }
    this.lastSaveMs = now;
    PlaybackProgressStore.save(this.bvid, pos).catch((err: Object) => {
      Logger.warn(TAG, `本地进度写入失败：${JSON.stringify(err)}`);
    });
    if (!TokenStore.isLoggedIn() || pos <= 2) {
      return;
    }
    VideoApi.saveProgress(this.bvid, pos).catch((err: Object) => {
      Logger.warn(TAG, `服务端进度上报失败：${JSON.stringify(err)}`);
    });
  }

  private flushProgress(): void {
    const pos: number = this.player.positionSec();
    if (pos <= 0) {
      return;
    }
    PlaybackProgressStore.save(this.bvid, pos).catch(() => {});
    if (!TokenStore.isLoggedIn() || pos <= 2) {
      return;
    }
    VideoApi.saveProgress(this.bvid, pos).catch(() => {});
  }

  /** 签名临期则重取详情、换源续播（HMY-14：「用户无感」即不打断播放态与进度） */
  private async maybeRenew(): Promise<void> {
    if (this.renewing || this.switching || this.playUrl.length === 0) {
      return;
    }
    // 只在播放器稳住时换签。心跳先于首次 open 就起来了，若签名已临期（用户隔了几小时回来点开），
    // 会在起播途中再插一次 reopen：两次 open 交错，先落地的那个把状态打回 idle，
    // 后一个的 seek 撞 5400102「Operate Not Permit」，表现为起播失败。
    const phase: string = this.player.phase;
    if (phase !== 'playing' && phase !== 'paused') {
      return;
    }
    const exp: number = signExpiryMs(this.playUrl);
    if (exp === 0 || Date.now() < exp - RENEW_MARGIN_MS) {
      return;
    }
    this.renewing = true;
    const posSec: number = this.player.positionSec();
    const wasPlaying: boolean = this.player.phase === 'playing';
    Logger.info(TAG, `播放签名临期，静默换源（pos=${posSec}s playing=${wasPlaying}）`);
    try {
      const detail: VideoDetail = await VideoApi.detail(this.bvid);
      const list: StreamItem[] = detail.streams ?? [];
      if (list.length === 0) {
        Logger.warn(TAG, '换签后无可用流，保持原地址');
      } else {
        // 按画质值而非下标对齐，避免服务端档位增减后切到别的清晰度
        const quality: number = this.currentQuality();
        const matched: number = list.findIndex((item: StreamItem) => item.quality === quality);
        this.streams = list;
        this.qualityIndex = matched >= 0 ? matched : 0;
        this.playUrl = resolveMediaUrl(list[this.qualityIndex].url);
        this.lastSampleSec = posSec;
        await this.reopen(this.playUrl, posSec * 1000, wasPlaying);
      }
    } catch (err) {
      Logger.warn(TAG, `换签失败，下个心跳重试：${(err as ApiError).describe()}`);
    }
    this.renewing = false;
  }

  private currentQuality(): number {
    if (this.qualityIndex < 0 || this.qualityIndex >= this.streams.length) {
      return 0;
    }
    return this.streams[this.qualityIndex].quality;
  }

  /** 切清晰度：HMY-11 要求保留当前进度与播放态，故复用换源路径 */
  private async switchQuality(index: number): Promise<void> {
    if (index < 0 || index >= this.streams.length || index === this.qualityIndex) {
      return;
    }
    const posSec: number = this.player.positionSec();
    const wasPlaying: boolean = this.player.phase === 'playing';
    this.qualityIndex = index;
    this.playUrl = resolveMediaUrl(this.streams[index].url);
    this.lastSampleSec = posSec;
    Logger.info(TAG, `切换清晰度：quality=${this.streams[index].quality} pos=${posSec}s`);
    await this.reopen(this.playUrl, posSec * 1000, wasPlaying);
  }

  /**
   * 换源统一入口。换源期间 durationMs 归零，进度条的 max 会塌缩到 durationSec() 的下限 1，
   * 于是非零的当前进度被 Slider 夹到 1 并触发一次 onChange；若不屏蔽，这次「程序化夹取」会被
   * 当成用户拖动，反过来 seekTo(1000) 覆盖掉换源要恢复的进度。故在换源期间关闭该入口。
   */
  private async reopen(url: string, startMs: number, autoPlay: boolean): Promise<void> {
    this.switching = true;
    try {
      await this.player.open(url, startMs, autoPlay);
    } finally {
      this.switching = false;
    }
  }

  private retryPlay(): void {
    this.reopen(this.playUrl, this.player.positionSec() * 1000, true);
  }

  private replay(): void {
    this.player.seekTo(0);
    this.player.play();
  }

  /** 切后台：停播并落盘；回前台不自动续播（用户可能只是切走看了眼别的） */
  private onBackground(): void {
    Logger.info(TAG, '应用切后台，暂停播放并落盘进度');
    this.player.pause();
    this.flushProgress();
  }

  private updateScreenMetrics(): void {
    try {
      const d: display.Display = display.getDefaultDisplaySync();
      // px2vp 全局函数已废弃，改走 UIContext
      this.screenWidthVp = this.getUIContext().px2vp(d.width);
    } catch (err) {
      Logger.warn(TAG, `读取屏幕尺寸失败：${JSON.stringify(err)}`);
    }
  }

  private async hostWindow(): Promise<window.Window | null> {
    const ctx = this.getUIContext().getHostContext();
    if (ctx === undefined) {
      return null;
    }
    try {
      return await window.getLastWindow(ctx);
    } catch (err) {
      Logger.warn(TAG, `获取窗口失败：${JSON.stringify(err)}`);
      return null;
    }
  }

  /** 全屏 = 横屏 + 铺满窗口；退出时还原竖屏与非全屏布局 */
  private async applyWindowMode(on: boolean): Promise<void> {
    const win: window.Window | null = await this.hostWindow();
    if (win === null) {
      return;
    }
    try {
      if (on) {
        await win.setWindowLayoutFullScreen(true);
        await win.setPreferredOrientation(window.Orientation.LANDSCAPE);
      } else {
        await win.setPreferredOrientation(window.Orientation.PORTRAIT);
        await win.setWindowLayoutFullScreen(false);
      }
      this.updateScreenMetrics();
    } catch (err) {
      Logger.warn(TAG, `切换窗口模式失败：${JSON.stringify(err)}`);
    }
  }

  private async setFullscreen(on: boolean): Promise<void> {
    this.fullscreen = on;
    this.panelOpen = false;
    Logger.info(TAG, `全屏：${on}`);
    await this.applyWindowMode(on);
  }

  /** 手势改窗口亮度（只影响本应用窗口，不动系统亮度）；-1 为跟随系统 */
  private async setBrightness(value: number): Promise<void> {
    const clamped: number = Math.max(BRIGHTNESS_MIN, Math.min(1, value));
    this.windowBrightness = clamped;
    this.brightnessTouched = true;
    await this.applyBrightness(clamped);
  }

  private async restoreSystemBrightness(): Promise<void> {
    this.windowBrightness = 1;
    await this.applyBrightness(-1);
  }

  private async applyBrightness(value: number): Promise<void> {
    const win: window.Window | null = await this.hostWindow();
    if (win === null) {
      return;
    }
    try {
      await win.setWindowBrightness(value);
    } catch (err) {
      Logger.warn(TAG, `设置窗口亮度失败：${JSON.stringify(err)}`);
    }
  }

  /** 倍速：走 setPlaybackRate（连续取值），与 Web 端 [0.5, 0.75, 1, 1.25, 1.5, 2] 档位一致 */
  private setRate(rate: number): void {
    this.player.setRate(rate);
  }

  /** 画面高度：竖屏按视频画幅等比（未知画幅按 16:9），全屏铺满窗口 */
  private stageHeight(): Length {
    if (this.fullscreen) {
      return '100%';
    }
    const h: number = this.stageHeightPx();
    return h > 0 ? h : '60%';
  }

  private stageHeightPx(): number {
    const w: number = this.player.videoWidth;
    const h: number = this.player.videoHeight;
    const ratio: number = w > 0 && h > 0 ? h / w : 1 / DEFAULT_RATIO;
    return this.screenWidthVp * ratio;
  }

  /** 总时长：换源途中 player.durationMs 会归零，沿用上一次读到的值，避免进度条量程缩到 00:01 */
  private durationSec(): number {
    const fromPlayer: number = this.player.durationMs / 1000;
    if (fromPlayer > 0) {
      this.lastDurationSec = fromPlayer;
      return fromPlayer;
    }
    return this.lastDurationSec > 0 ? this.lastDurationSec : 1;
  }

  private qualityLabel(quality: number): ResourceStr {
    return quality === 0 ? $r('app.string.play_quality_original') : `${quality}P`;
  }

  private rateLabel(rate: number): ResourceStr {
    return rate === 1 ? $r('app.string.play_speed_normal') : `${rate}x`;
  }

  /**
   * 单击切控件、双击播放/暂停。
   *
   * 两个 `TapGesture` 挂在 `GestureMode.Parallel` 里而不是自己按时间差判定：Parallel 下双击会
   * 让 count:1 与 count:2 各命中一次，净效果正好是「控件切两次回到原状 + 暂停」，无需延迟单击，
   * 单击也不用为双击窗口让路（手写计时版单击要等 300ms 才响应）。
   *
   * 代价是 Parallel 里 Tap 不会因为位移自行取消，拖进度/亮度时这根手指会顺带命中一次单击，
   * 把控件栏闪掉（拖动过程中反而看不到进度反馈）。故拖动期间与刚结束的 200ms 内丢弃单击。
   */
  private onStageTap(): void {
    if (this.dragging || Date.now() - this.dragEndMs < DRAG_TAP_GUARD_MS) {
      return;
    }
    this.controlsVisible = !this.controlsVisible;
    Logger.info(TAG, `舞台单击：控件${this.controlsVisible ? '显示' : '隐藏'}`);
    if (!this.controlsVisible) {
      this.panelOpen = false;
    }
  }

  private onStageDoubleTap(): void {
    Logger.info(TAG, '舞台双击：切播放/暂停');
    this.player.toggle();
  }

  private handleDragStart(e: GestureEvent): void {
    this.dragging = true;
    this.dragAxis = DragAxis.NONE;
    this.dragStartSec = this.player.positionSec();
    this.dragStartValue = 1;
    if (e.fingerList.length > 0) {
      this.dragPivotX = e.fingerList[0].localX;
    }
  }

  private handleDragUpdate(e: GestureEvent): void {
    const offsetX: number = e.offsetX;
    const offsetY: number = e.offsetY;
    if (this.dragAxis === DragAxis.NONE) {
      if (Math.abs(offsetX) < 12 && Math.abs(offsetY) < 12) {
        return;
      }
      if (Math.abs(offsetX) >= Math.abs(offsetY)) {
        this.dragAxis = DragAxis.SEEK;
        this.dragStartSec = this.player.positionSec();
      } else {
        // 竖拖分左右半屏：左亮度、右音量（Web 端靠键鼠的操作在端侧只能落到手势上）
        this.dragAxis = this.dragPivotX < this.screenWidthVp / 2 ? DragAxis.BRIGHTNESS : DragAxis.VOLUME;
        this.dragStartValue = this.dragAxis === DragAxis.BRIGHTNESS ? this.windowBrightness : this.player.volume;
      }
    }

    if (this.dragAxis === DragAxis.SEEK) {
      const span: number = this.screenWidthVp > 0 ? this.screenWidthVp : 1;
      const ratio: number = offsetX / span;
      const total: number = this.durationSec();
      const target: number = Math.max(0, Math.min(this.dragStartSec + ratio * total, total));
      this.sliderSec = target;
      this.dragHint = `${formatDuration(target)} / ${formatDuration(total)}`;
      return;
    }

    const height: number = Math.max(1, this.stageHeightPx());
    const value: number = this.dragStartValue + (-offsetY / height) * DRAG_FULL_RANGE;
    if (this.dragAxis === DragAxis.BRIGHTNESS) {
      this.setBrightness(value);
      this.dragHint = `亮度 ${Math.round(this.windowBrightness * 100)}%`;
    } else if (this.dragAxis === DragAxis.VOLUME) {
      this.player.setVolume(value);
      this.dragHint = `音量 ${Math.round(this.player.volume * 100)}%`;
    }
  }

  private handleDragEnd(): void {
    if (this.dragAxis === DragAxis.SEEK) {
      this.player.seekTo(this.sliderSec * 1000);
    }
    this.dragAxis = DragAxis.NONE;
    this.dragging = false;
    this.dragEndMs = Date.now();
    this.dragHint = '';
  }

  /**
   * 多参数的 @Builder 走「按值传递」，参数变化不会引起内部 UI 刷新——档位高亮会停在旧值
   * （实测：切到 360P 后仍高亮 720P）。改为单对象参数走「按引用传递」，高亮才跟随状态。
   */
  @Builder
  optionChip($$: ChipOption) {
    Text($$.label)
      .fontSize(DliFont.BODY_SM)
      .fontColor($$.active ? $r('app.color.text_on_brand') : $r('app.color.text_primary'))
      .padding({ left: DliSpace.MD, right: DliSpace.MD, top: 4, bottom: 4 })
      .borderRadius(DliRadius.FULL)
      .backgroundColor($$.active ? $r('app.color.brand_primary') : $r('app.color.surface_secondary'))
      .onClick(() => {
        $$.onTap();
      })
  }

  @Builder
  settingsPanel() {
    Column({ space: DliSpace.SM }) {
      Row({ space: DliSpace.SM }) {
        Text($r('app.string.play_quality'))
          .fontSize(DliFont.BODY_SM)
          .fontColor($r('app.color.text_secondary'))
        ForEach(this.streams, (item: StreamItem, index: number) => {
          this.optionChip({
            label: this.qualityLabel(item.quality),
            active: index === this.qualityIndex,
            onTap: () => {
              this.switchQuality(index);
            }
          });
        }, (item: StreamItem) => `${item.quality}`)
      }
      .alignItems(VerticalAlign.Center)

      Row({ space: DliSpace.SM }) {
        Text($r('app.string.play_speed'))
          .fontSize(DliFont.BODY_SM)
          .fontColor($r('app.color.text_secondary'))
        ForEach(RATE_OPTIONS, (item: number) => {
          this.optionChip({
            label: this.rateLabel(item),
            active: item === this.player.rate,
            onTap: () => {
              this.setRate(item);
            }
          });
        }, (item: number) => `${item}`)
      }
      .alignItems(VerticalAlign.Center)
    }
    .width('100%')
    .alignItems(HorizontalAlign.Start)
    .padding(DliSpace.MD)
    .borderRadius(DliRadius.MD)
    .backgroundColor($r('app.color.bg_glass'))
    .margin({ bottom: DliSpace.SM })
  }

  @Builder
  topBar() {
    Row({ space: DliSpace.SM }) {
      SymbolGlyph($r('sys.symbol.chevron_left'))
        .fontSize(22)
        .fontColor([$r('app.color.text_on_brand')])
        .onClick(() => {
          AppRouter.pop();
        })
      Text(this.title)
        .fontSize(DliFont.BODY_SM)
        .fontColor($r('app.color.text_on_brand'))
        .maxLines(1)
        .textOverflow({ overflow: TextOverflow.Ellipsis })
        .layoutWeight(1)
    }
    .width('100%')
    .padding({ left: DliSpace.SM, right: DliSpace.MD, top: DliSpace.SM, bottom: DliSpace.SM })
    .alignItems(VerticalAlign.Center)
    .linearGradient({
      angle: 180,
      colors: [[SCRIM_TOP, 0.0], [SCRIM_NONE, 1.0]]
    })
  }

  @Builder
  bottomBar() {
    Column() {
      if (this.panelOpen) {
        this.settingsPanel()
      }
      Row({ space: DliSpace.SM }) {
        SymbolGlyph(this.player.phase === 'playing' ? $r('sys.symbol.pause_fill') : $r('sys.symbol.play_fill'))
          .fontSize(24)
          .fontColor([$r('app.color.text_on_brand')])
          .onClick(() => {
            this.player.toggle();
          })
        Text(`${formatDuration(this.sliderSec)} / ${formatDuration(this.durationSec())}`)
          .fontSize(DliFont.CAPTION)
          .fontColor($r('app.color.text_on_brand'))
        Slider({ value: this.sliderSec, min: 0, max: this.durationSec(), step: 1 })
          .layoutWeight(1)
          .trackColor('#66FFFFFF')
          .selectedColor($r('app.color.brand_primary'))
          .blockColor($r('app.color.text_on_brand'))
          .onChange((value: number, mode: SliderChangeMode) => {
            if (mode === SliderChangeMode.Begin) {
              this.sliderDragging = true;
              return;
            }
            if (mode === SliderChangeMode.Moving) {
              this.sliderSec = value;
              return;
            }
            if (mode === SliderChangeMode.End || mode === SliderChangeMode.Click) {
              const dragged: boolean = this.sliderDragging;
              this.sliderDragging = false;
              this.sliderSec = value;
              // 只有真实拖动/点击才产生 seek；且时长未就绪时时刻校不准，一律不发
              if (dragged || mode === SliderChangeMode.Click) {
                if (!this.switching && this.player.durationMs > 0) {
                  this.player.seekTo(value * 1000);
                }
              }
            }
          })
        Text($r('app.string.play_speed'))
          .fontSize(DliFont.CAPTION)
          .fontColor($r('app.color.text_on_brand'))
          .padding({ left: DliSpace.SM, right: DliSpace.SM, top: 4, bottom: 4 })
          .borderRadius(DliRadius.SM)
          .backgroundColor(this.panelOpen ? $r('app.color.brand_primary') : '#4DFFFFFF')
          .onClick(() => {
            this.panelOpen = !this.panelOpen;
          })
        Text(this.fullscreen ? '退出全屏' : '全屏')
          .fontSize(DliFont.CAPTION)
          .fontColor($r('app.color.text_on_brand'))
          .padding({ left: DliSpace.SM, right: DliSpace.SM, top: 4, bottom: 4 })
          .borderRadius(DliRadius.SM)
          .backgroundColor('#4DFFFFFF')
          .onClick(() => {
            this.setFullscreen(!this.fullscreen);
          })
      }
      .width('100%')
      .alignItems(VerticalAlign.Center)
    }
    .width('100%')
    .padding({ left: DliSpace.MD, right: DliSpace.MD, bottom: DliSpace.SM })
    .linearGradient({
      angle: 180,
      colors: [[SCRIM_NONE, 0.0], [SCRIM_BOTTOM, 1.0]]
    })
  }

  @Builder
  stageOverlay() {
    Column() {
      this.topBar()
      Blank()
      if (this.dragHint.length > 0) {
        Text(this.dragHint)
          .fontSize(DliFont.BODY)
          .fontColor($r('app.color.text_on_brand'))
          .padding({ left: DliSpace.MD, right: DliSpace.MD, top: DliSpace.SM, bottom: DliSpace.SM })
          .borderRadius(DliRadius.MD)
          .backgroundColor('#99000000')
      }
      Blank()
      this.bottomBar()
    }
    .width('100%')
    .height('100%')
    // 覆盖层铺满舞台，必须让空白处的触摸穿透到下层手势层，否则控件一显示手势就全失效
    // （实测：控件可见时单击/双击/拖动都不触发，控件隐藏时才正常）
    .hitTestBehavior(HitTestMode.Transparent)
  }

  /** 中央状态位：缓冲/起播中、自然结束、播放出错，三者互斥 */
  @Builder
  stageCenterState() {
    if (this.player.phase === 'error') {
      Column({ space: DliSpace.MD }) {
        Text(this.player.errorMessage)
          .fontSize(DliFont.BODY_SM)
          .fontColor($r('app.color.text_on_brand'))
          .textAlign(TextAlign.Center)
        Button($r('app.string.action_retry'))
          .fontSize(DliFont.BODY_SM)
          .fontColor($r('app.color.text_on_brand'))
          .backgroundColor($r('app.color.brand_primary'))
          .borderRadius(DliRadius.FULL)
          .onClick(() => {
            this.retryPlay();
          })
      }
      .padding(DliSpace.LG)
    } else if (this.player.phase === 'ended') {
      Button($r('app.string.play_ended_replay'))
        .fontSize(DliFont.BODY_SM)
        .fontColor($r('app.color.text_on_brand'))
        .backgroundColor($r('app.color.brand_primary'))
        .borderRadius(DliRadius.FULL)
        .onClick(() => {
          this.replay();
        })
    } else if (this.player.buffering || this.player.phase === 'preparing' || this.player.phase === 'idle') {
      LoadingProgress()
        .width(36)
        .height(36)
        .color($r('app.color.text_on_brand'))
    }
  }

  @Builder
  stage() {
    Stack({ alignContent: Alignment.Center }) {
      XComponent({ type: XComponentType.SURFACE, controller: this.surfaceController })
        .onLoad(() => {
          // surface 早于/晚于详情都可能，两处都试着开播（见 tryStart）
          this.player.setSurface(this.surfaceController.getXComponentSurfaceId());
          this.surfaceReady = true;
          this.tryStart();
        })
        .onDestroy(() => {
          this.surfaceReady = false;
        })
        .width('100%')
        .height('100%')

      // 手势层：单击切控件、双击播放/暂停、横向拖动进度、竖向拖动亮度/音量
      // 空容器默认不参与触摸测试（实测点击与拖动都不触发），故显式声明背景与 Block：
      // 自身响应、并拦住下方 XComponent 的原生 surface，否则手势会被 surface 吃掉。
      Column()
        .width('100%')
        .height('100%')
        .backgroundColor(Color.Transparent)
        .hitTestBehavior(HitTestMode.Block)
        .gesture(
          GestureGroup(GestureMode.Parallel,
            TapGesture({ count: 2 }).onAction(() => {
              this.onStageDoubleTap();
            }),
            TapGesture({ count: 1 }).onAction(() => {
              this.onStageTap();
            }),
            PanGesture({ direction: PanDirection.All, distance: DRAG_DISTANCE })
              .onActionStart((e: GestureEvent) => {
                this.handleDragStart(e);
              })
              .onActionUpdate((e: GestureEvent) => {
                this.handleDragUpdate(e);
              })
              .onActionEnd(() => {
                this.handleDragEnd();
              })
              .onActionCancel(() => {
                this.handleDragEnd();
              })
          )
        )

      // 控制层：Transparent 让空白处的触摸仍能落到底下的手势层，控件自身照常响应
      if (this.controlsVisible) {
        this.stageOverlay()
      }

      this.stageCenterState()
    }
    .width('100%')
    .height(this.stageHeight())
    .backgroundColor(Color.Black)
    .clip(true)
  }

  @Builder
  infoBlock() {
    Scroll() {
      Column({ space: DliSpace.MD }) {
        Text(this.title)
          .fontSize(DliFont.TITLE_SM)
          .fontWeight(FontWeight.Medium)
          .fontColor($r('app.color.text_primary'))
          .width('100%')
        Text(this.metaText)
          .fontSize(DliFont.CAPTION)
          .fontColor($r('app.color.text_tertiary'))
          .width('100%')

        Row({ space: DliSpace.SM }) {
          if (this.ownerAvatar.length > 0) {
            Image(this.ownerAvatar)
              .width(36)
              .height(36)
              .borderRadius(DliRadius.FULL)
              .objectFit(ImageFit.Cover)
          }
          Text(this.ownerName)
            .fontSize(DliFont.BODY_SM)
            .fontColor($r('app.color.text_primary'))
        }
        .width('100%')
        .alignItems(VerticalAlign.Center)

        if (this.tags.length > 0) {
          Flex({ wrap: FlexWrap.Wrap }) {
            ForEach(this.tags, (item: string) => {
              Text(item)
                .fontSize(DliFont.CAPTION)
                .fontColor($r('app.color.text_secondary'))
                .padding({ left: DliSpace.SM, right: DliSpace.SM, top: 2, bottom: 2 })
                .borderRadius(DliRadius.SM)
                .backgroundColor($r('app.color.surface_secondary'))
                .margin({ right: DliSpace.SM, bottom: DliSpace.XS })
            }, (item: string) => item)
          }
          .width('100%')
        }

        if (this.description.length > 0) {
          Text(this.description)
            .fontSize(DliFont.BODY_SM)
            .fontColor($r('app.color.text_secondary'))
            .width('100%')
        }

        Text($r('app.string.play_pending'))
          .fontSize(DliFont.CAPTION)
          .fontColor($r('app.color.text_tertiary'))
          .width('100%')
      }
      .width('100%')
      .padding({ left: DliSpace.PAGE, right: DliSpace.PAGE, top: DliSpace.MD, bottom: DliSpace.XL })
      .alignItems(HorizontalAlign.Start)
    }
    .scrollBar(BarState.Off)
    .align(Alignment.Top)
    .width('100%')
    .layoutWeight(1)
  }

  @Builder
  loadingView() {
    Column() {
      LoadingProgress()
        .width(40)
        .height(40)
        .color($r('app.color.brand_primary'))
    }
    .width('100%')
    .layoutWeight(1)
    .justifyContent(FlexAlign.Center)
  }

  @Builder
  failureView() {
    Column() {
      ErrorStateView({
        message: this.errorText,
        canRetry: this.canRetry,
        onRetry: () => {
          this.loadDetail();
        }
      })
    }
    .width('100%')
    .layoutWeight(1)
  }

  @Builder
  noticeView() {
    Column() {
      Text($r('app.string.play_no_stream'))
        .fontSize(DliFont.BODY_SM)
        .fontColor($r('app.color.text_tertiary'))
    }
    .width('100%')
    .layoutWeight(1)
    .justifyContent(FlexAlign.Center)
  }

  build() {
    NavDestination() {
      Column() {
        if (this.loading) {
          this.loadingView()
        } else if (this.errorText.length > 0) {
          this.failureView()
        } else if (this.noStream) {
          this.noticeView()
        } else {
          this.stage()
          if (!this.fullscreen) {
            this.infoBlock()
          }
        }
      }
      .width('100%')
      .height('100%')
    }
    // 沉浸播放：自绘顶栏承担返回与标题，视频画面顶到状态栏下
    .hideTitleBar(true)
    .backgroundColor($r('app.color.bg_page'))
    .onBackPressed(() => {
      if (this.fullscreen) {
        // 全屏态下返回键先退全屏，再按一次才离开播放页
        this.setFullscreen(false);
        return true;
      }
      return false;
    })
  }
}
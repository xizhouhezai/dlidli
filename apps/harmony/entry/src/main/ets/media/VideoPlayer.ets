import media from '@ohos.multimedia.media';
import { BusinessError } from '@kit.BasicServicesKit';
import { Logger } from '../common/utils/Logger';

const TAG: string = 'VideoPlayer';

/**
 * media 命名空间的**值引用**（枚举成员、命名空间函数）会触发 DevEco 的 syscap 提示
 * 「The system capacity of this api 'media' is not supported on all devices」——类型引用不触发。
 * 提到模块级常量只是把这几条提示从调用点收拢到文件头，去不掉（AVPlayer 是本端的核心能力，
 * 该提示由 SDK 的 syscap 表产生，不影响编译产物与运行）。故本任务起，端侧不再宣称「零告警」。
 */
const SEEK_CLOSEST: media.SeekMode = media.SeekMode.SEEK_CLOSEST;
const BUFFERING_START: media.BufferingInfoType = media.BufferingInfoType.BUFFERING_START;
const BUFFERING_END: media.BufferingInfoType = media.BufferingInfoType.BUFFERING_END;

/** 等待状态迁移的上限（起播/换源）；超时即报错，避免页面永久停在 loading */
const STATE_TIMEOUT_MS: number = 15000;

/** 播放相位：对 media.AVPlayerState 的窄化，页面只关心这几档 */
export type PlayerPhase = 'idle' | 'preparing' | 'ready' | 'playing' | 'paused' | 'ended' | 'error';

/** 只在这些状态下才能 seek / setPlaybackRate（AVPlayer 的状态机约束） */
function canControl(state: media.AVPlayerState): boolean {
  return state === 'prepared' || state === 'playing' || state === 'paused' || state === 'completed';
}

/** 一次性状态等待器：把「等 stateChange 落到某档」包成 Promise，供 open() 串行驱动状态机 */
interface StateWaiter {
  target: media.AVPlayerState;
  /** 状态达成（ok=true）或以失败收场（ok=false），二选一必调一次 */
  settle: (ok: boolean) => void;
  /** 放弃等待：只清定时器与槽位、不结算 Promise，用于 prepare 已抛出、无人再 await 的场景 */
  dispose: () => void;
}

/**
 * AVPlayer 封装：把系统播放器的状态机、事件，以及「换源但保留进度与播放态」的流程收在一处。
 *
 * 分工：本类只管播放本身（起播/暂停/seek/倍速/音量/释放）；跨端续播位置、进度节流上报、
 * 签名续签由页面层负责——它们依赖网络与业务语义，不属于播放器。
 */
@ObservedV2
export class VideoPlayer {
  @Trace phase: PlayerPhase = 'idle';
  /** UI 进度（毫秒，秒级节流后）：timeUpdate 每 100ms 一次，进度条无需这个频率 */
  @Trace positionMs: number = 0;
  @Trace durationMs: number = 0;
  @Trace buffering: boolean = false;
  @Trace videoWidth: number = 0;
  @Trace videoHeight: number = 0;
  @Trace errorMessage: string = '';
  /** 倍速：1 为正常。走 setPlaybackRate，取值连续（[0.125, 8.0]），与 Web 端的倍速档一致 */
  @Trace rate: number = 1;
  /** 播放器内音量 0~1（竖向手势调音量用） */
  @Trace volume: number = 1;

  /** 自然播完（非错误），页面据此决定是否给重播入口 */
  onEnded: () => void = () => {};
  /** 播放出错，页面据此决定是否给重试/换源入口 */
  onError: (message: string) => void = () => {};

  private player: media.AVPlayer | null = null;
  private surfaceId: string = '';
  /** 换源后要恢复到的位置（毫秒），prepare 完成后消费 */
  private pendingSeekMs: number = 0;
  /** 最新位置（秒，不节流）——上报与落盘读它，避免受 UI 节流影响 */
  private latestSec: number = 0;
  private lastUiSecond: number = -1;
  private released: boolean = false;
  /** 当前挂起的状态等待（同一时刻只有一条起播流程，故单槽即可） */
  private waiter: StateWaiter | null = null;

  isReady(): boolean {
    const p: media.AVPlayer | null = this.player;
    return p !== null && canControl(p.state);
  }

  /** 当前播放位置（秒）：未经 UI 节流，供进度上报取用 */
  positionSec(): number {
    return this.latestSec;
  }

  /** 视频画面载体（XComponent surface）。onLoad 早于首帧时也要能接住，故由播放器缓存 */
  setSurface(surfaceId: string): void {
    this.surfaceId = surfaceId;
    const p: media.AVPlayer | null = this.player;
    // idle/error 态赋 surfaceId 会被系统打回（5400102），缓存下来由 open() 补赋
    if (p !== null && p.state !== 'idle' && p.state !== 'error' && p.state !== 'released') {
      p.surfaceId = surfaceId;
    }
  }

  /**
   * 起播/换源。换源（切清晰度、签名续签）与首次起播同一条路径：
   * 先 reset 回 idle 再赋 url，否则在 prepared/playing 态赋 url 会被系统拒绝。
   */
  async open(url: string, startMs: number, autoPlay: boolean): Promise<void> {
    if (this.released) {
      return;
    }
    this.errorMessage = '';
    this.phase = 'preparing';
    this.buffering = false;
    this.pendingSeekMs = startMs > 0 ? startMs : 0;
    this.latestSec = this.pendingSeekMs / 1000;
    this.lastUiSecond = -1;
    this.positionMs = this.pendingSeekMs;
    this.durationMs = 0;

    try {
      const p: media.AVPlayer = await this.ensurePlayer();
      if (this.released) {
        return;
      }
      if (p.state !== 'idle') {
        await p.reset();
      }
      // url 赋值触发的 idle→initialized 是**异步**迁移：写完立刻 prepare 会撞
      // 「The current state is idle. Prepare operation only supports stopped/initialized」。
      // 而 surfaceId 也只在 initialized 态可写，故等状态落地后再赋 surface 并 prepare。
      const initialized: Promise<void> = this.waitForState('initialized');
      p.url = url;
      await initialized;
      if (this.released) {
        return;
      }
      if (this.surfaceId.length > 0) {
        p.surfaceId = this.surfaceId;
      }
      // prepare() 的 Promise 只表示「指令已下发」，并不等于已到 prepared（实测本 SDK 立即 resolve，
      // 而 HLS 拉清单一拍后可到）。故同样挂 stateChange 等落地，否则后续 play 会撞
      // 「current state is initialized. Play operation only supports prepared/paused/completed」。
      const prepared: Promise<void> = this.waitForState('prepared');
      await p.prepare();
      await prepared;
      if (this.released) {
        return;
      }
      if (this.pendingSeekMs > 0) {
        // seek 无返回值（本 SDK 的 d.ts 为 void），只能同步 try 捕获同步异常；
        // 异步结果看 seekDone —— 少了这个日志，「换清晰度后从头播」会表现为静默丢进度。
        const target: number = this.pendingSeekMs;
        try {
          p.seek(target, SEEK_CLOSEST);
          Logger.info(TAG, `换源后跳转到 ${target}ms 已下发`);
        } catch (err) {
          const be: BusinessError = err as BusinessError;
          Logger.error(TAG, `换源后跳转 ${target}ms 抛出：${be.code} ${be.message}`);
        }
      }
      if (this.rate !== 1) {
        p.setPlaybackRate(this.rate);
      }
      this.phase = 'ready';
      if (autoPlay) {
        await p.play();
      }
    } catch (err) {
      this.disposeWaiter();
      const be: BusinessError = err as BusinessError;
      Logger.error(TAG, `起播失败：${be.code} ${be.message}`);
      this.fail(`播放失败（${be.code}）`);
    }
  }

  async play(): Promise<void> {
    const p: media.AVPlayer | null = this.player;
    if (p === null || !canControl(p.state)) {
      return;
    }
    try {
      await p.play();
    } catch (err) {
      this.fail(`恢复播放失败（${(err as BusinessError).code}）`);
    }
  }

  async pause(): Promise<void> {
    const p: media.AVPlayer | null = this.player;
    if (p === null || p.state !== 'playing') {
      return;
    }
    try {
      await p.pause();
    } catch (err) {
      Logger.warn(TAG, `暂停失败：${(err as BusinessError).message}`);
    }
  }

  /** 播放/暂停切换（单击、双击手势的落点） */
  async toggle(): Promise<void> {
    const p: media.AVPlayer | null = this.player;
    if (p === null) {
      return;
    }
    if (p.state === 'playing') {
      await this.pause();
    } else if (canControl(p.state)) {
      await this.play();
    }
  }

  /** 跳转到指定毫秒（拖动进度）。结果经 seekDone 事件回来，失败在此记日志 */
  seekTo(ms: number): void {
    const p: media.AVPlayer | null = this.player;
    if (p === null || !canControl(p.state)) {
      return;
    }
    const clamped: number = Math.max(0, Math.min(ms, this.durationMs > 0 ? this.durationMs : ms));
    this.positionMs = clamped;
    this.latestSec = clamped / 1000;
    this.pendingSeekMs = clamped;
    try {
      p.seek(clamped, SEEK_CLOSEST);
    } catch (err) {
      const be: BusinessError = err as BusinessError;
      Logger.error(TAG, `跳转 ${clamped}ms 抛出：${be.code} ${be.message}`);
    }
  }

  /** 注意：HLS 直播不支持倍速，本端只消费 VOD */
  setRate(rate: number): void {
    this.rate = rate;
    const p: media.AVPlayer | null = this.player;
    if (p === null || !canControl(p.state)) {
      return;
    }
    try {
      p.setPlaybackRate(rate);
    } catch (err) {
      Logger.warn(TAG, `设置倍速失败：${(err as BusinessError).message}`);
    }
  }

  setVolume(volume: number): void {
    this.volume = Math.max(0, Math.min(1, volume));
    if (this.player !== null) {
      this.player.setVolume(this.volume);
    }
  }

  async release(): Promise<void> {
    this.released = true;
    const w: StateWaiter | null = this.waiter;
    if (w !== null) {
      w.settle(false);
    }
    const p: media.AVPlayer | null = this.player;
    this.player = null;
    this.phase = 'idle';
    this.buffering = false;
    if (p === null) {
      return;
    }
    try {
      p.off('stateChange');
      p.off('error');
      p.off('timeUpdate');
      p.off('durationUpdate');
      p.off('bufferingUpdate');
      p.off('videoSizeChange');
      p.off('seekDone');
      p.off('playbackRateDone');
      await p.release();
      Logger.info(TAG, '播放器已释放');
    } catch (err) {
      Logger.warn(TAG, `释放播放器失败：${(err as BusinessError).message}`);
    }
  }

  private async ensurePlayer(): Promise<media.AVPlayer> {
    const existing: media.AVPlayer | null = this.player;
    if (existing !== null) {
      return existing;
    }
    try {
      const p: media.AVPlayer = await media.createAVPlayer();
      p.loop = false;
      p.setVolume(this.volume);
      this.bind(p);
      this.player = p;
      // 不在此处赋 surfaceId：新建的播放器处于 idle，而该属性仅在 initialized 态可写，
      // 在 idle 赋值会被系统以 5400102 打回并置错误。真正的赋值在 open() 等状态落地后。
      return p;
    } catch (err) {
      // 创建失败上抛给 open() 统一落到 error 相位，页面据此给重试入口
      const be: BusinessError = err as BusinessError;
      Logger.error(TAG, `创建播放器失败：${be.code} ${be.message}`);
      throw new Error(`createAVPlayer failed: ${be.code}`);
    }
  }

  private bind(p: media.AVPlayer): void {
    p.on('stateChange', (state: media.AVPlayerState, reason: media.StateChangeReason) => {
      this.onState(state, reason);
    });
    p.on('error', (err: BusinessError) => {
      Logger.error(TAG, `播放器错误：${err.code} ${err.message}`);
      this.fail(`播放出错（${err.code}）`);
    });
    p.on('timeUpdate', (time: number) => {
      this.onTime(time);
    });
    p.on('durationUpdate', (duration: number) => {
      this.durationMs = duration;
    });
    p.on('bufferingUpdate', (infoType: media.BufferingInfoType, value: number) => {
      if (infoType === BUFFERING_START) {
        this.buffering = true;
      } else if (infoType === BUFFERING_END) {
        this.buffering = false;
      }
    });
    p.on('videoSizeChange', (width: number, height: number) => {
      this.videoWidth = width;
      this.videoHeight = height;
    });
    p.on('seekDone', (time: number) => {
      Logger.info(TAG, `跳转完成：${time}ms`);
      this.positionMs = time;
      this.latestSec = time / 1000;
      this.pendingSeekMs = time;
    });
    p.on('playbackRateDone', (rate: number) => {
      Logger.info(TAG, `倍速已生效：${rate}`);
    });
  }

  private onState(state: media.AVPlayerState, reason: media.StateChangeReason): void {
    if (this.released) {
      return;
    }
    // BACKGROUND 原因由系统主动停播（如切后台），保留相位交给页面的生命周期处理
    Logger.debug(TAG, `状态：${state}（reason=${reason}）`);
    this.settleWaiter(state);
    if (state === 'playing') {
      this.phase = 'playing';
    } else if (state === 'paused') {
      this.phase = 'paused';
    } else if (state === 'completed') {
      this.phase = 'ended';
      this.buffering = false;
      this.onEnded();
    } else if (state === 'error') {
      this.fail(this.errorMessage.length > 0 ? this.errorMessage : '播放出错');
    } else if (state === 'prepared') {
      if (this.phase === 'preparing') {
        this.phase = 'ready';
      }
    } else if (state === 'idle' || state === 'stopped') {
      if (this.phase !== 'preparing' && this.phase !== 'error') {
        this.phase = 'idle';
      }
    }
  }

  private onTime(time: number): void {
    if (this.released) {
      return;
    }
    this.latestSec = time / 1000;
    const second: number = Math.floor(time / 1000);
    if (second !== this.lastUiSecond) {
      this.lastUiSecond = second;
      this.positionMs = time;
      if (this.durationMs === 0) {
        const p: media.AVPlayer | null = this.player;
        if (p !== null && p.duration > 0) {
          this.durationMs = p.duration;
        }
      }
    }
  }

  /**
   * 等待播放器落到目标状态。AVPlayer 的部分迁移（url → initialized、reset → idle）
   * 是异步的，靠 `await` 属性赋值或固定延时都不稳；这里挂在 stateChange 上等，超时抛错。
   */
  private waitForState(target: media.AVPlayerState): Promise<void> {
    const p: media.AVPlayer | null = this.player;
    if (p !== null && p.state === target) {
      return Promise.resolve();
    }
    return new Promise<void>((resolve, reject) => {
      const timer: number = setTimeout(() => {
        this.waiter = null;
        reject(new Error(`等待播放器状态 ${target} 超时`));
      }, STATE_TIMEOUT_MS);
      this.waiter = {
        target: target,
        settle: (ok: boolean): void => {
          clearTimeout(timer);
          this.waiter = null;
          if (ok) {
            resolve();
          } else {
            reject(new Error(`播放器未进入 ${target} 即出错`));
          }
        },
        dispose: (): void => {
          clearTimeout(timer);
        }
      };
    });
  }

  /** 释放等待槽位而不结算 Promise（调用方已走异常分支，无人 await） */
  private disposeWaiter(): void {
    const w: StateWaiter | null = this.waiter;
    if (w !== null) {
      this.waiter = null;
      w.dispose();
    }
  }

  /** stateChange 到达时唤醒等待者；落 error 一并唤醒（以失败收场），否则调用方会挂到超时 */
  private settleWaiter(state: media.AVPlayerState): void {
    const w: StateWaiter | null = this.waiter;
    if (w === null) {
      return;
    }
    if (state === w.target) {
      w.settle(true);
    } else if (state === 'error') {
      w.settle(false);
    }
  }

  private fail(message: string): void {
    const w: StateWaiter | null = this.waiter;
    if (w !== null) {
      w.settle(false);
    }
    this.errorMessage = message;
    this.phase = 'error';
    this.buffering = false;
    this.onError(message);
  }
}

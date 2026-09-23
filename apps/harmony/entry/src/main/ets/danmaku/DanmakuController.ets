import { ApiError } from '../model/Api';
import { DanmakuBlockItem, DanmakuItem, DanmakuMode } from '../model/Danmaku';
import { DanmakuApi } from '../service/DanmakuApi';
import { DanmakuEngine, DanmakuFrame } from './DanmakuEngine';
import { DanmakuSettings, dmSettings } from './DanmakuSettings';
import { DanmakuSocket, DmSocketState } from './DanmakuSocket';
import { VideoPlayer } from '../media/VideoPlayer';
import { ErrCode } from '../common/constants/ErrorMessages';
import { Logger } from '../common/utils/Logger';

const TAG: string = 'Danmaku';

/** 服务端分段长度兜底（首次响应会带回真实值，见 DanmakuSegmentResult.segment_ms） */
const DEFAULT_SEGMENT_MS: number = 6 * 60 * 1000;
/**
 * 迟到窗口：只上屏「最近 LATE 毫秒内」该出现的弹幕。
 * 分段拉取与实时弹幕都可能迟到（尤其回退轮询），若不加窗口，一次前进式 seek 会把整段几百条
 * 全部塞进引擎（绝大多数早已过期），白跑一轮绘制与轨道分配。
 */
const LATE_WINDOW_MS: number = 12000;
/** WS 降级后的分段轮询间隔（DM-15 回退语义：延迟可见优于不可见） */
const POLL_INTERVAL_MS: number = 15000;
/** 列表面板分页大小（服务端上限 200） */
const LIST_PAGE_SIZE: number = 50;

/**
 * 弹幕控制器：把「数据（分段/实时/屏蔽/发送）+ 渲染引擎」收在一处，播放页只负责接线与 UI。
 *
 * 与 Web 端 `useDanmakuController + DanmakuLayer` 分工一致，差异在三点：
 * ① 渲染交给 `DanmakuEngine`（Canvas 轨道渲染）而不是逐条 DOM 节点；
 * ② 实时通道独立成 `DanmakuSocket`，重连次数用尽后由本类启动分段轮询兜底；
 * ③ 屏蔽对**实时帧**也走本地过滤——服务端只对 HTTP 读接口过滤，WS 帧是不过滤的。
 */
@ObservedV2
export class DanmakuController {
  /** 实时通道状态（UI 据此提示「实时弹幕已断开」） */
  @Trace socketState: DmSocketState = 'idle';
  /** 关键词屏蔽（服务端账号级，登录态下发） */
  @Trace blockWords: string[] = [];
  /** 发送者屏蔽（UID 哈希） */
  @Trace blockHashes: string[] = [];
  /** 列表面板 */
  @Trace listItems: DanmakuItem[] = [];
  @Trace listTotal: number = 0;
  @Trace listLoading: boolean = false;

  private engine: DanmakuEngine = new DanmakuEngine();
  private socket: DanmakuSocket = new DanmakuSocket();
  private player: VideoPlayer | null = null;
  private bvid: string = '';

  private segmentMs: number = DEFAULT_SEGMENT_MS;
  private loadedSegs: Set<number> = new Set<number>();
  private pool: Map<number, DanmakuItem[]> = new Map<number, DanmakuItem[]>();
  /** 已上屏（或已判定不需上屏）的弹幕 id，防重复 */
  private shown: Set<string> = new Set<string>();
  /** 每段已扫描到的下标：段内按 time_ms 有序，正常播放时不必每帧从头扫（池可上万条） */
  private cursors: Map<number, number> = new Map<number, number>();
  private lastSeg: number = -1;
  private pollTimer: number = -1;
  private listPage: number = 1;

  /** 播放位置的本地外推锚点：timeUpdate 约 100ms 一次，直接拿它画会「一格一格跳」 */
  private lastSampleMs: number = 0;
  private lastSampleWall: number = 0;

  // ---- 生命周期 ----

  /** 播放页起播前调用：接上播放器、拉屏蔽、连实时通道 */
  start(bvid: string, player: VideoPlayer): void {
    this.bvid = bvid;
    this.player = player;
    this.socket.onItem = (item: DanmakuItem) => {
      this.onRealtime(item);
    };
    this.socket.onState = (state: DmSocketState) => {
      this.socketState = state;
      this.syncPoll(state);
    };
    this.loadBlocks();
    this.socket.connect(bvid);
  }

  /** 离开页面：断开通道、停轮询、清引擎 */
  stop(): void {
    this.socket.close();
    this.stopPoll();
    this.engine.clear();
    this.engine.detach();
  }

  attachCanvas(ctx: CanvasRenderingContext2D): void {
    this.engine.attach(ctx);
  }

  /** 画面区域/画幅变化（进出全屏、视频尺寸就绪）后调用：旧轨道号在新区间可能越界，清掉重画 */
  onFrameChanged(): void {
    this.engine.clear();
    this.shown.clear();
    this.cursors.clear();
    this.engine.markDirty();
  }

  /** 展示设置变化（不透明度/字号等不影响轨道的项）：暂停时也要重画，否则看不到变化 */
  markDirty(): void {
    this.engine.markDirty();
  }

  /** 播放位置跳变：清屏并按新位置重放（弹幕没有「历史包袱」，回退就该重看） */
  onSeeked(videoMs: number): void {
    this.engine.clear();
    this.shown.clear();
    this.cursors.clear();
    this.lastSampleMs = -1; // 强制下一帧重新采样，避免外推把旧位置续上新位置
    this.lastSampleWall = 0;
    this.tick(videoMs);
  }

  /**
   * 当前位置（毫秒）——**给渲染与发送用的合成时间**：
   * 以播放器采样值为锚点，播放中用墙钟按倍速外推，保证 60fps 下位移连续。
   */
  videoMs(nowWallMs: number): number {
    const player: VideoPlayer | null = this.player;
    if (player === null) {
      return 0;
    }
    const sample: number = player.positionSec() * 1000;
    if (sample !== this.lastSampleMs) {
      this.lastSampleMs = sample;
      this.lastSampleWall = nowWallMs;
      return sample;
    }
    if (player.phase !== 'playing') {
      return sample;
    }
    const drift: number = Math.min(nowWallMs - this.lastSampleWall, 500);
    return sample + drift * player.rate;
  }

  // ---- 分段拉取与上屏 ----

  /** 每帧调用：补分段、把到点的弹幕交给引擎 */
  tick(videoMs: number): void {
    const seg: number = Math.floor(videoMs / this.segmentMs);
    if (seg !== this.lastSeg) {
      this.lastSeg = seg;
      this.ensureSegment(seg);
      this.ensureSegment(seg + 1); // 预取下一段（DM-10）
    }
    if (!dmSettings.enabled) {
      return;
    }
    const items: DanmakuItem[] = this.pool.get(seg) ?? [];
    if (items.length === 0) {
      return;
    }
    const from: number = videoMs - LATE_WINDOW_MS;
    let index: number = this.cursors.get(seg) ?? 0;
    // 跳过早已过期的前缀（只在窗口向右推进时前进，故整体是 O(新上屏条数)）
    while (index < items.length && items[index].time_ms < from) {
      index++;
    }
    this.cursors.set(seg, index);
    for (let i = index; i < items.length; i++) {
      const item: DanmakuItem = items[i];
      if (item.time_ms > videoMs) {
        break;
      }
      if (this.shown.has(item.id)) {
        continue;
      }
      this.shown.add(item.id);
      if (!this.isBlocked(item)) {
        this.engine.push(item);
      }
    }
  }

  /** 每帧调用：分配轨道并绘制（引擎自行判断是否真的需要重画） */
  draw(videoMs: number, frame: DanmakuFrame, settings: DanmakuSettings): void {
    this.engine.spawnDue(videoMs, frame, settings);
    this.engine.draw(videoMs, frame, settings);
  }

  private async ensureSegment(seg: number): Promise<void> {
    if (seg < 0 || this.loadedSegs.has(seg) || this.bvid.length === 0) {
      return;
    }
    this.loadedSegs.add(seg);
    try {
      const res = await DanmakuApi.list(this.bvid, seg);
      this.segmentMs = res.segment_ms > 0 ? res.segment_ms : this.segmentMs;
      this.pool.set(seg, res.list);
      Logger.info(TAG, `弹幕分段 ${seg} 就绪：${res.list.length} 条（段长 ${this.segmentMs}ms）`);
    } catch (err) {
      this.loadedSegs.delete(seg); // 失败允许下个心跳重试
      Logger.warn(TAG, `弹幕分段 ${seg} 拉取失败：${(err as ApiError).describe()}`);
    }
  }

  /** 发送成功后的乐观上屏（服务端广播会排除发送者本人，故不会重复） */
  inject(item: DanmakuItem): void {
    if (this.shown.has(item.id)) {
      return;
    }
    this.shown.add(item.id);
    const seg: number = Math.floor(item.time_ms / this.segmentMs);
    const list: DanmakuItem[] = this.pool.get(seg) ?? [];
    list.push(item);
    this.pool.set(seg, list);
    this.engine.push(item);
  }

  private onRealtime(item: DanmakuItem): void {
    if (this.shown.has(item.id)) {
      return; // 乐观上屏已登记
    }
    this.shown.add(item.id);
    // 登记进池：屏蔽与举报要按 id 找到原始条目
    const seg: number = Math.floor(item.time_ms / this.segmentMs);
    const list: DanmakuItem[] = this.pool.get(seg) ?? [];
    list.push(item);
    this.pool.set(seg, list);
    // WS 帧**不经服务端屏蔽过滤**，本地过滤是唯一防线
    if (dmSettings.enabled && !this.isBlocked(item)) {
      this.engine.push(item);
    }
  }

  /** 按 id 取条目（屏蔽/举报要拿内容与发送者哈希） */
  findById(id: string): DanmakuItem | null {
    for (const list of this.pool.values()) {
      for (const item of list) {
        if (item.id === id) {
          return item;
        }
      }
    }
    for (const item of this.listItems) {
      if (item.id === id) {
        return item;
      }
    }
    return null;
  }

  /** 正在上屏的条数（实测核对用） */
  activeCount(): number {
    return this.engine.activeCount();
  }

  // ---- 实时通道降级 ----

  private syncPoll(state: DmSocketState): void {
    if (state === 'degraded') {
      if (this.pollTimer === -1) {
        Logger.info(TAG, '实时弹幕不可用，改用分段轮询兜底');
        this.pollTimer = setInterval(() => {
          this.pollOnce();
        }, POLL_INTERVAL_MS);
      }
      return;
    }
    if (state === 'open') {
      this.stopPoll();
    }
  }

  private stopPoll(): void {
    if (this.pollTimer !== -1) {
      clearInterval(this.pollTimer);
      this.pollTimer = -1;
    }
  }

  /** 回退轮询：重取当前段（发送会让服务端段缓存失效，故能拿到新弹幕） */
  private pollOnce(): void {
    const seg: number = this.lastSeg >= 0 ? this.lastSeg : 0;
    this.loadedSegs.delete(seg);
    this.ensureSegment(seg);
  }

  // ---- 屏蔽（DM-20 / DM-21） ----

  async loadBlocks(): Promise<void> {
    try {
      const res = await DanmakuApi.blocks();
      this.blockWords = res.list.filter((b: DanmakuBlockItem) => b.block_type === 1)
        .map((b: DanmakuBlockItem) => b.keyword ?? '');
      this.blockHashes = res.list.filter((b: DanmakuBlockItem) => b.block_type === 2)
        .map((b: DanmakuBlockItem) => b.block_hash ?? '');
    } catch (err) {
      // 游客（/blocks 需登录）与网络失败：保持空列表，不阻塞弹幕展示
      Logger.warn(TAG, `屏蔽列表不可用：${(err as ApiError).describe()}`);
    }
  }

  /** 屏蔽判定：自己的弹幕始终可见（否则发完自己先看不见，观感像发失败了） */
  isBlocked(item: DanmakuItem): boolean {
    if (item.is_self === true) {
      return false;
    }
    const hash: string = item.sender_hash ?? '';
    if (hash.length > 0 && this.blockHashes.includes(hash)) {
      return true;
    }
    for (const word of this.blockWords) {
      if (word.length > 0 && item.content.includes(word)) {
        return true;
      }
    }
    return false;
  }

  /** 屏蔽某发送者（服务端账号级生效，跨端有效） */
  async blockSender(item: DanmakuItem): Promise<void> {
    const hash: string = item.sender_hash ?? '';
    if (hash.length === 0) {
      throw new ApiError(ErrCode.LOCAL_REJECT, '这条弹幕没有可屏蔽的发送者', '', false);
    }
    await DanmakuApi.addBlock({ block_type: 2, block_hash: hash });
    if (!this.blockHashes.includes(hash)) {
      this.blockHashes = [...this.blockHashes, hash];
    }
  }

  async addKeyword(keyword: string): Promise<void> {
    const word: string = keyword.trim();
    if (word.length === 0) {
      throw new ApiError(ErrCode.LOCAL_REJECT, '请输入屏蔽词', '', false);
    }
    await DanmakuApi.addBlock({ block_type: 1, keyword: word });
    if (!this.blockWords.includes(word)) {
      this.blockWords = [...this.blockWords, word];
    }
  }

  async removeBlock(blockType: number, value: string): Promise<void> {
    const res = await DanmakuApi.blocks();
    const hit: DanmakuBlockItem | undefined = res.list.find((b: DanmakuBlockItem) =>
      b.block_type === blockType && (blockType === 1 ? b.keyword === value : b.block_hash === value));
    if (hit === undefined) {
      return;
    }
    await DanmakuApi.deleteBlock(hit.id);
    if (blockType === 1) {
      this.blockWords = this.blockWords.filter((w: string) => w !== value);
    } else {
      this.blockHashes = this.blockHashes.filter((h: string) => h !== value);
    }
  }

  // ---- 发送（DM-01~04 / HMY-21） ----

  /**
   * 发送弹幕并乐观上屏。等级门槛/频控/去重全部由服务端裁决，端侧只翻译错误码，
   * 故不做本地倒计时——端侧与服务端的时间窗不同步，本地拦反而会误伤。
   */
  async send(content: string, mode: DanmakuMode, color: number): Promise<DanmakuItem> {
    const item: DanmakuItem = await DanmakuApi.send(this.bvid, {
      content: content.trim(),
      time_ms: Math.max(0, Math.floor(this.videoMs(Date.now()))),
      mode: mode,
      color: color
    });
    this.inject(item);
    return item;
  }

  // ---- 列表面板（DM-16 / HMY-24） ----

  /** 弹幕列表：首屏 reset，之后追加（服务端按 id 倒序＝最新在前） */
  async loadList(reset: boolean): Promise<void> {
    if (reset) {
      this.listPage = 1;
      this.listItems = [];
    }
    this.listLoading = true;
    try {
      const res = await DanmakuApi.listAll(this.bvid, this.listPage, LIST_PAGE_SIZE);
      this.listItems = reset ? res.list : [...this.listItems, ...res.list];
      this.listTotal = res.total;
      this.listPage++;
    } finally {
      this.listLoading = false;
    }
  }
}

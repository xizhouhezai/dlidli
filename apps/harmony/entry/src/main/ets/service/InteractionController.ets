import { ApiError } from '../model/Api';
import { CollectionItem, InteractionState, TripleResult } from '../model/Interaction';
import { VideoDetail } from '../model/Video';
import { InteractionApi } from '../service/InteractionApi';
import { TokenStore } from '../store/TokenStore';
import { Logger } from '../common/utils/Logger';

const TAG: string = 'Interaction';

/**
 * 互动控制器（M4-HMY-08）：点赞/投币/收藏/长按三连（幂等 + 原生动画）/收藏夹弹层。
 *
 * 与 Web 端 useVideoActions 分工一致，差异三点：
 * ① 长按三连由 ActionBar 的 LongPressGesture(1500ms) 驱动（spec ITR-30 要求 1.5s；Web 端是 800ms）；
 * ② 三连动画交 ArkUI 原生动画（见 ActionBar 组件）；
 * ③ 幂等：服务端互动接口本身幂等（点赞/收藏开关、投币唯一键、三连先扣币失败退款），
 *    端侧再以 `acting` 串行化 + 乐观回滚兜底——重复提交（双击/网络重试）在服务端只生效一次。
 */
@ObservedV2
export class InteractionController {
  /** 点赞状态 */
  @Trace liked: boolean = false;
  /** 已投币数（0=未投，1/2=已投） */
  @Trace coinCount: number = 0;
  /** 收藏状态 */
  @Trace faved: boolean = false;
  /** 计数（从详情 seed，本地修正） */
  @Trace likeCnt: number = 0;
  @Trace coinCnt: number = 0;
  @Trace favCnt: number = 0;
  /** 三连动画触发标记（ActionBar 监听此值播原生动画，播完清零） */
  @Trace tripleAnimTick: number = 0;
  /** 收藏夹弹层 */
  @Trace favPopVisible: boolean = false;
  @Trace coinPopVisible: boolean = false;
  @Trace collections: CollectionItem[] = [];

  /** 串行化互动请求，防双击/快速连点造成并发提交（服务端幂等兜底） */
  private acting: boolean = false;
  private bvid: string = '';

  /** 播放页装载详情后调用：seed 计数并拉互动状态（登录态才有意义） */
  async load(bvid: string, detail: VideoDetail): Promise<void> {
    this.bvid = bvid;
    this.likeCnt = detail.stat.like;
    this.coinCnt = detail.stat.coin;
    this.favCnt = detail.stat.fav;
    if (!TokenStore.isLoggedIn()) {
      return;
    }
    try {
      const st: InteractionState = await InteractionApi.interactionState(bvid);
      this.liked = st.liked;
      this.coinCount = st.coin_count;
      this.faved = st.faved;
    } catch (err) {
      // 互动状态读失败不阻塞播放页——按全 false 展示，用户操作时再与服务端对账
      Logger.warn(TAG, `互动状态读取失败：${(err as ApiError).describe()}`);
    }
  }

  /** 登录前置：未登录提示并跳登录页；返回是否已登录 */
  private requireLogin(): boolean {
    return TokenStore.isLoggedIn();
  }

  // ---- 点赞（ITR-01） ----

  async toggleLike(): Promise<void> {
    if (!this.requireLogin() || this.acting) {
      return;
    }
    this.acting = true;
    const before: boolean = this.liked;
    const beforeCnt: number = this.likeCnt;
    // 乐观更新（服务端幂等：同一请求并发/重放只生效一次，故先改本地再按服务端回值对齐）
    this.liked = !before;
    this.likeCnt = beforeCnt + (before ? -1 : 1);
    try {
      const res = await InteractionApi.toggleLike(this.bvid);
      this.liked = res.liked;
      // 服务端回值 = 当前真实状态；本地计数据此重算（与 before 比较得出净增量）
      this.likeCnt = beforeCnt + (res.liked === before ? 0 : res.liked ? 1 : -1);
    } catch (err) {
      // 失败回滚
      this.liked = before;
      this.likeCnt = beforeCnt;
      throw (err as Error);
    } finally {
      this.acting = false;
    }
  }

  // ---- 投币（ITR-10） ----

  async coin(count: number): Promise<void> {
    if (!this.requireLogin() || this.acting) {
      return;
    }
    if (this.coinCount > 0) {
      // 已投过：服务端唯一键会拦，端侧提前提示（50001 已有文案）
      return;
    }
    this.acting = true;
    try {
      await InteractionApi.coin(this.bvid, count);
      this.coinCount = count;
      this.coinCnt += count;
    } catch (err) {
      throw (err as Error);
    } finally {
      this.acting = false;
    }
  }

  // ---- 收藏（ITR-20/21） ----

  async toggleFavorite(collectionId: string): Promise<void> {
    if (!this.requireLogin() || this.acting) {
      return;
    }
    this.acting = true;
    const before: boolean = this.faved;
    const beforeCnt: number = this.favCnt;
    this.faved = !before;
    this.favCnt = beforeCnt + (before ? -1 : 1);
    try {
      const res = await InteractionApi.toggleFavorite(this.bvid, collectionId);
      this.faved = res.faved;
      this.favCnt = beforeCnt + (res.faved === before ? 0 : res.faved ? 1 : -1);
    } catch (err) {
      this.faved = before;
      this.favCnt = beforeCnt;
      throw (err as Error);
    } finally {
      this.acting = false;
    }
  }

  /** 收藏夹列表（收藏弹层打开时拉取） */
  async loadCollections(): Promise<void> {
    if (!this.requireLogin()) {
      return;
    }
    try {
      this.collections = await InteractionApi.listCollections();
    } catch (err) {
      Logger.warn(TAG, `收藏夹列表失败：${(err as ApiError).describe()}`);
      this.collections = [];
    }
  }

  async createCollection(name: string): Promise<void> {
    const col = await InteractionApi.createCollection(name);
    this.collections = [...this.collections, col];
  }

  // ---- 三连（ITR-30，长按 1.5s） ----

  /**
   * 一键三连。由 ActionBar 的 `LongPressGesture({duration: 1500})` 直接驱动——
   * 不再用控制器内的定时器：手势组件自带时长判定，重复计时会把手感拉成 3s。
   */
  async doTriple(): Promise<void> {
    if (!this.requireLogin() || this.acting) {
      return;
    }
    this.acting = true;
    try {
      const res: TripleResult = await InteractionApi.triple(this.bvid);
      this.liked = res.liked;
      this.coinCount = res.coin_count;
      this.faved = res.faved;
      this.likeCnt += res.like_delta;
      this.coinCnt += res.coin_delta;
      this.favCnt += res.fav_delta;
      // 触发原生动画反馈（ActionBar 监听 tripleAnimTick）
      this.tripleAnimTick++;
      Logger.info(TAG, `三连完成：like=${res.liked} coin=${res.coin_count} fav=${res.faved}`);
    } catch (err) {
      Logger.warn(TAG, `三连请求失败：${(err as ApiError).describe()}`);
      throw (err as Error);
    } finally {
      this.acting = false;
    }
  }

  /** 切换稿件后重置 */
  reset(): void {
    this.liked = false;
    this.coinCount = 0;
    this.faved = false;
    this.favPopVisible = false;
    this.coinPopVisible = false;
    this.collections = [];
    this.tripleAnimTick = 0;
  }
}

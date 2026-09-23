import webSocket from '@ohos.net.webSocket';
import { BusinessError } from '@kit.BasicServicesKit';
import { DanmakuItem } from '../model/Danmaku';
import { DanmakuApi } from '../service/DanmakuApi';
import { ApiConfig } from '../common/constants/ApiConfig';
import { TokenStore } from '../store/TokenStore';
import { Logger } from '../common/utils/Logger';

const TAG: string = 'DmSocket';

/** 重连上限（对齐 Web 端 DanmakuLayer：5 次后放弃，交回 HTTP 分段拉取兜底） */
const MAX_RETRY: number = 5;
/** 退避上限（毫秒） */
const RETRY_MAX_MS: number = 15000;

/** 服务端下发帧（**不是**统一包裹）：{"type":"danmaku","data":Item} */
interface WsFrame {
  type: string;
  data: DanmakuItem;
}

/** 连接状态：degraded 表示重连已放弃，调用方须回退分段轮询 */
export type DmSocketState = 'idle' | 'connecting' | 'open' | 'degraded';

/**
 * 弹幕实时通道（DM-15 / HMY-22）。
 *
 * 契约要点（服务端 `danmaku/hub.go`）：
 * ① 鉴权只能走 `?token=`（WS 握手无法自定义 header 之外的 Cookie 语义，服务端也只读 query）；
 *    令牌无效不会报错，而是**静默按游客连接**——游客拿不到服务端屏蔽过滤，故端侧必须本地过滤；
 * ② 上行数据被服务端直接丢弃，发送一律走 HTTP（本类只读）；
 * ③ 服务端 30s 发一次协议层 Ping、读超时 60s，Pong 回不上来会被断开；
 * ④ 重连只能客户端自己做：失败 5 次后置 degraded，由调用方按 DM-15 回退分段拉取。
 *
 * 关于握手与 Origin（实测，2026-09-22 本机 API 26 模拟器 + 本机 Go 服务）：
 * ① ArkTS 的 WebSocket **自生成 Origin**，默认形态为 `http://<host>`（**不带端口**，`connect()` 的同名
 *    header 覆盖不掉）；API 26 起可用 `supportOriginPort: true` 让它带上端口；
 * ② 服务端 `CheckOrigin` 对 `allow_origins` 做**精确匹配**（dev 为 `http://localhost:5173~5175`），
 *    因此直连 `ws://10.0.2.2:8000` 时 Origin=`http://10.0.2.2` 不在白名单 → 握手 403 → 按失败重连、
 *    5 次后 degraded 回退分段轮询（这条降级链路已实测）；
 * ③ 端侧 WS 栈本身与 Go Hub 互通无碍：把 WS 地址临时指向设备侧 `localhost:5173`
 *    （`hdc rport tcp:5173 tcp:8000` 反向转发到本机 8000）并置 `supportOriginPort: true`，
 *    Origin 即 `http://localhost:5173` 命中 dev 白名单——握手成功、`?token=` 鉴权生效、
 *    收到真实广播帧并上屏（含屏蔽命中时本地过滤）；
 * ④ 故 dev/局域网环境的唯一阻塞是**服务端白名单未含客户端来源**，端侧无从绕过；
 *    生产若同源（App 连 `wss://<正式域名>`）则 Origin 与白名单条目一致。上线前需服务端确认此项。
 */
export class DanmakuSocket {
  private ws: webSocket.WebSocket | null = null;
  private retry: number = 0;
  private timer: number = -1;
  private stopped: boolean = true;
  private bvid: string = '';
  private state: DmSocketState = 'idle';

  /** 收到一条实时弹幕 */
  onItem: (item: DanmakuItem) => void = () => {};
  /** 连接状态变化（调用方据此启停分段轮询回退） */
  onState: (state: DmSocketState) => void = () => {};

  connect(bvid: string): void {
    this.bvid = bvid;
    this.stopped = false;
    this.retry = 0;
    this.openSocket();
  }

  close(): void {
    this.stopped = true;
    if (this.timer !== -1) {
      clearTimeout(this.timer);
      this.timer = -1;
    }
    this.releaseSocket();
    this.applyState('idle');
  }

  /** 拼接完整 WS 地址：http→ws 换算 + 令牌 query */
  private url(): string {
    const base: string = ApiConfig.BASE_URL;
    const wsBase: string = base.startsWith('https://')
      ? `wss://${base.substring('https://'.length)}`
      : `ws://${base.substring('http://'.length)}`;
    const token: string = TokenStore.getAccessToken();
    const query: string = token.length > 0 ? `?token=${encodeURIComponent(token)}` : '';
    return `${wsBase}${DanmakuApi.wsPath(this.bvid)}${query}`;
  }

  private openSocket(): void {
    if (this.stopped) {
      return;
    }
    this.applyState('connecting');
    const ws: webSocket.WebSocket = webSocket.createWebSocket();
    this.ws = ws;
    ws.on('open', (err: BusinessError, value: Object) => {
      if (this.stopped || this.ws !== ws) {
        return;
      }
      this.retry = 0;
      Logger.info(TAG, `弹幕通道已连接：${this.bvid}`);
      this.applyState('open');
    });
    ws.on('message', (err: BusinessError, value: string | ArrayBuffer) => {
      if (this.stopped || this.ws !== ws) {
        return;
      }
      this.onFrame(value);
    });
    ws.on('close', (err: BusinessError, value: webSocket.CloseResult) => {
      if (this.stopped || this.ws !== ws) {
        return;
      }
      Logger.warn(TAG, `弹幕通道关闭：code=${value.code} reason=${value.reason}`);
      this.ws = null;
      this.scheduleRetry();
    });
    ws.on('error', (err: BusinessError) => {
      if (this.stopped || this.ws !== ws) {
        return;
      }
      // 握手被服务端拒（如 Origin 不在白名单、房间满员）时只会走这里：close 回调不保证到达，
      // 若不当成一次失败尝试，重连链会就此停住、也不会降级到分段轮询。
      // 置空 ws 后同一条连接随后的 close 会被上面的守卫拦掉，不会重复排一次重连。
      Logger.warn(TAG, `弹幕通道异常：code=${err.code} message=${err.message}，按失败尝试重连`);
      this.ws = null;
      this.scheduleRetry();
    });
    ws.connect(this.url()).then((ok: boolean) => {
      if (!ok && !this.stopped && this.ws === ws) {
        Logger.warn(TAG, '弹幕通道连接未成功，等待 close 回调后重试');
      }
    }).catch((err: BusinessError) => {
      if (this.stopped || this.ws !== ws) {
        return;
      }
      // 握手被拒（如房间满员 1008、服务端未启动）走这里，不会再有 close 回调
      Logger.warn(TAG, `弹幕通道连接失败：${err.code} ${err.message}`);
      this.ws = null;
      this.scheduleRetry();
    });
  }

  private onFrame(value: string | ArrayBuffer): void {
    if (typeof value !== 'string') {
      return; // 服务端只下发文本帧，二进制帧忽略
    }
    try {
      const frame: WsFrame = JSON.parse(value) as WsFrame;
      if (frame.type === 'danmaku') {
        Logger.debug(TAG, `收到实时弹幕帧：id=${frame.data.id} t=${frame.data.time_ms}`);
        this.onItem(frame.data);
      }
    } catch (err) {
      Logger.warn(TAG, `非法弹幕帧已忽略：${value.substring(0, 64)}`);
    }
  }

  /** 指数退避重连；次数用尽即置 degraded，不再自行重试（由分段轮询兜底） */
  private scheduleRetry(): void {
    if (this.stopped || this.timer !== -1) {
      return;
    }
    if (this.retry >= MAX_RETRY) {
      Logger.warn(TAG, `弹幕通道重连 ${MAX_RETRY} 次仍未成功，回退分段轮询`);
      this.applyState('degraded');
      return;
    }
    this.retry++;
    const delay: number = Math.min(1000 * Math.pow(2, this.retry), RETRY_MAX_MS);
    Logger.info(TAG, `弹幕通道 ${delay}ms 后第 ${this.retry} 次重连`);
    this.timer = setTimeout(() => {
      this.timer = -1;
      this.openSocket();
    }, delay);
  }

  private releaseSocket(): void {
    const ws: webSocket.WebSocket | null = this.ws;
    this.ws = null;
    if (ws === null) {
      return;
    }
    try {
      ws.off('message');
      ws.off('close');
      ws.off('error');
      ws.close();
    } catch (err) {
      Logger.warn(TAG, `关闭弹幕通道异常：${JSON.stringify(err)}`);
    }
  }

  private applyState(next: DmSocketState): void {
    if (this.state === next) {
      return;
    }
    this.state = next;
    this.onState(next);
  }
}

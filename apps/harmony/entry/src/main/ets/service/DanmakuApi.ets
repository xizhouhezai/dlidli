import { HttpClient } from './HttpClient';
import {
  AddDanmakuBlockReq,
  DanmakuBlockListResult,
  DanmakuItem,
  DanmakuListResult,
  DanmakuSegmentResult,
  SendDanmakuReq
} from '../model/Danmaku';

const PATH_VIDEOS: string = '/api/v1/videos';

/**
 * 屏蔽接口挂在 `/videos/:bvid/danmaku/blocks` 下，但服务端只认当前用户、**忽略 bvid**，
 * 故用一个占位 bvid（与 Web 端同一约定），避免端侧为了屏蔽再去查一次真实 bvid。
 */
const BLOCK_BVID: string = '_';

function danmakuPath(bvid: string, suffix: string): string {
  return `${PATH_VIDEOS}/${encodeURIComponent(bvid)}/danmaku${suffix}`;
}

/**
 * 弹幕域接口。读接口（分段/全量/WS）走带凭证请求——**这是端侧屏蔽生效的前提**：
 * 服务端按令牌识别观众并过滤其屏蔽项，游客拿到的是未过滤列表。
 */
export class DanmakuApi {
  /** 按段拉取（段长由响应给出，服务端 6min/段；登录用户服务端已过滤屏蔽项） */
  static list(bvid: string, segment: number): Promise<DanmakuSegmentResult> {
    const query: Map<string, string> = new Map<string, string>();
    query.set('segment', `${segment}`);
    return HttpClient.get<DanmakuSegmentResult>(danmakuPath(bvid, ''), { query: query });
  }

  /**
   * 全量列表（列表面板，按 id 倒序即最新在前）。
   * **分页参数名是 `size`**，不是 `page_size`——服务端 pagination 只读 `size`，
   * 传错会被静默忽略而退回默认 50，表现为「翻页拿到同一批数据」。
   */
  static listAll(bvid: string, page: number, size: number): Promise<DanmakuListResult> {
    const query: Map<string, string> = new Map<string, string>();
    query.set('page', `${page}`);
    query.set('size', `${size}`);
    return HttpClient.get<DanmakuListResult>(danmakuPath(bvid, '/list'), { query: query });
  }

  /** 发送（需登录；等级门槛、频控、去重均由服务端裁决，端侧只做前置提示） */
  static send(bvid: string, req: SendDanmakuReq): Promise<DanmakuItem> {
    return HttpClient.post<DanmakuItem>(danmakuPath(bvid, ''), req);
  }

  /** WS 路径（协议与主机由 `DanmakuSocket` 拼接） */
  static wsPath(bvid: string): string {
    return danmakuPath(bvid, '/ws');
  }

  /** 当前用户屏蔽列表（需登录；游客调用会 401） */
  static async blocks(): Promise<DanmakuBlockListResult> {
    return HttpClient.get<DanmakuBlockListResult>(danmakuPath(BLOCK_BVID, '/blocks'));
  }

  /** 新增屏蔽（幂等；关键词 1~64 字、上限 200 条由服务端校验） */
  static addBlock(req: AddDanmakuBlockReq): Promise<void> {
    return HttpClient.post<void>(danmakuPath(BLOCK_BVID, '/blocks'), req);
  }

  /** 删除屏蔽项（仅本人） */
  static deleteBlock(id: string): Promise<void> {
    return HttpClient.del<void>(danmakuPath(BLOCK_BVID, `/blocks/${encodeURIComponent(id)}`));
  }
}

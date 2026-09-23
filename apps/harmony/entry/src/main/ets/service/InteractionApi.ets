import { HttpClient } from './HttpClient';
import {
  AddCommentReq,
  CollectionItem,
  CommentItem,
  CommentListResult,
  FavoritesResult,
  FavedResult,
  InteractionState,
  LikedResult,
  TripleResult
} from '../model/Interaction';
import { VideoCard } from '../model/Video';

const PATH_VIDEOS: string = '/api/v1/videos';
const PATH_COMMENTS: string = '/api/v1/comments';
const PATH_ME: string = '/api/v1/users/me';

function videoPath(bvid: string, suffix: string): string {
  return `${PATH_VIDEOS}/${encodeURIComponent(bvid)}${suffix}`;
}

function commentPath(id: string, suffix: string): string {
  return `${PATH_COMMENTS}/${encodeURIComponent(id)}${suffix}`;
}

/**
 * 互动域接口（M4-HMY-08）。
 *
 * 与 Web 端 `packages/api-client` 的 interaction 模块同契约，差异两点：
 * ① 收藏夹 id 用字符串传输（雪花 id，防 JS Number 精度丢失；服务端按 int64 解析，空串=默认夹）；
 * ② 互动状态 `interactionState` 走带凭证请求——游客不带令牌时服务端 uid<=0 恒回全 false/0。
 */
export class InteractionApi {
  // ---- 互动状态 ----

  /** 当前用户对稿件的互动状态（赞/币/藏聚合，播放页一次取齐）。需登录才有意义 */
  static interactionState(bvid: string): Promise<InteractionState> {
    return HttpClient.get<InteractionState>(videoPath(bvid, '/interaction'));
  }

  // ---- 点赞 ----

  /** 视频点赞/取消，返回当前是否已赞（ITR-01）。需登录 */
  static toggleLike(bvid: string): Promise<LikedResult> {
    return HttpClient.post<LikedResult>(videoPath(bvid, '/like'), null);
  }

  /** 评论点赞/取消（ITR-02）。需登录 */
  static toggleCommentLike(id: string): Promise<LikedResult> {
    return HttpClient.post<LikedResult>(commentPath(id, '/like'), null);
  }

  // ---- 投币（ITR-10） ----

  /** 投币（自制 ≤2、转载 ≤1，由服务端裁决；需登录）。无响应体 */
  static coin(bvid: string, count: number): Promise<void> {
    const body: Record<string, number> = { 'count': count };
    return HttpClient.post<void>(videoPath(bvid, '/coin'), body);
  }

  // ---- 收藏（ITR-20/21） ----

  /** 收藏/取消；collectionId 为空串表示默认收藏夹（服务端懒创建）。需登录 */
  static toggleFavorite(bvid: string, collectionId: string): Promise<FavedResult> {
    const body: Record<string, string> = { 'collection_id': collectionId };
    return HttpClient.post<FavedResult>(videoPath(bvid, '/favorite'), body);
  }

  /**
   * 收藏夹列表（默认夹在前）。需登录。
   * 说明：服务端 `/users/me/collections` 的 data 直接是数组（非包裹对象），故 GET 的泛型取 CollectionItem[]。
   */
  static listCollections(): Promise<CollectionItem[]> {
    return HttpClient.get<CollectionItem[]>(`${PATH_ME}/collections`);
  }

  /** 新建收藏夹（1~50 字）。需登录 */
  static createCollection(name: string): Promise<CollectionItem> {
    const body: Record<string, string> = { 'name': name };
    return HttpClient.post<CollectionItem>(`${PATH_ME}/collections`, body);
  }

  /**
   * 重命名/删除收藏夹本期不做（ITR-21 属收藏管理 P1，Web 端有；鸿蒙端观看端首期聚焦默认夹一键收藏，
   * 多收藏夹管理归后续版本）。对应接口若在后续接入，用 HttpClient.del（已有）与新增的 put 支持。
   */

  // ---- 三连（ITR-30） ----

  /** 一键三连：点赞 + 投币（默认 2 枚、不足则 1 枚）+ 收藏（默认夹），返回三项状态与 delta。需登录 */
  static triple(bvid: string): Promise<TripleResult> {
    return HttpClient.post<TripleResult>(videoPath(bvid, '/triple'), null);
  }

  // ---- 评论（CMT-01~04） ----

  /** 一级评论分页（一级评论附前 2 条回复预览）。sort: hot|new；游客可读 */
  static async listComments(bvid: string, sort: string, page: number, size: number): Promise<CommentItem[]> {
    const query: Map<string, string> = new Map<string, string>();
    query.set('sort', sort);
    query.set('page', `${page}`);
    query.set('page_size', `${size}`);
    const res: CommentListResult = await HttpClient.get<CommentListResult>(videoPath(bvid, '/comments'), { query: query });
    return res.list;
  }

  /** 一级评论分页（带总数，用于页头「N 条评论」）。与 listComments 同一次请求，但调用方通常同时需要 total */
  static listCommentsPaged(bvid: string, sort: string, page: number, size: number): Promise<CommentListResult> {
    const query: Map<string, string> = new Map<string, string>();
    query.set('sort', sort);
    query.set('page', `${page}`);
    query.set('page_size', `${size}`);
    return HttpClient.get<CommentListResult>(videoPath(bvid, '/comments'), { query: query });
  }

  /** 楼中楼分页（按时间升序）。游客可读 */
  static listReplies(rootId: string, page: number, size: number): Promise<CommentListResult> {
    const query: Map<string, string> = new Map<string, string>();
    query.set('page', `${page}`);
    query.set('page_size', `${size}`);
    return HttpClient.get<CommentListResult>(commentPath(rootId, '/replies'), { query: query });
  }

  /** 发表评论/回复（需登录；内容 ≤1000 字、机审由服务端裁决） */
  static addComment(bvid: string, req: AddCommentReq): Promise<CommentItem> {
    return HttpClient.post<CommentItem>(videoPath(bvid, '/comments'), req);
  }

  /** 删除评论（作者本人或稿件 UP 主，服务端校验）。需登录 */
  static deleteComment(id: string): Promise<void> {
    return HttpClient.del<void>(commentPath(id, ''));
  }

  // ---- 我的收藏（HMY-42 个人中心用，本期顺带备齐） ----

  /** 我的收藏视频（默认收藏夹，按收藏时间倒序）。需登录 */
  static async favorites(page: number, size: number): Promise<FavoritesResult> {
    const query: Map<string, string> = new Map<string, string>();
    query.set('page', `${page}`);
    query.set('page_size', `${size}`);
    return HttpClient.get<FavoritesResult>(`${PATH_ME}/favorites`, { query: query });
  }
}

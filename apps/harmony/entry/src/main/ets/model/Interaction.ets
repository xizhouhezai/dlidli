/**
 * 互动与评论域端侧模型（M4-HMY-08，字段名以 server 实现为准，见 plan §4）。
 *
 * 契约核对要点（2026-09-22 以 server/internal/module/interaction 为准）：
 *  - 评论雪花 id 在 JSON 里是**字符串**（`json:"id,string"`），端侧统一 `id: string`；
 *  - `is_top` 服务端为 bool；`is_self` 标记当前用户（供删除入口展示）；
 *  - 互动状态的硬币为「已投数」，0 表示未投；三连响应的 delta 用于前端本地修正计数；
 *  - 收藏开关 collection_id 用字符串传输（雪花 id，防 JS Number 精度丢失；空串=默认收藏夹）。
 */

/** 评论作者摘要（对应 server UserBrief）。id 为字符串化的雪花 id。 */
export interface CommentUser {
  id: string;
  nickname: string;
  avatar: string;
  level: number;
}

/** 评论条目（对应 server CommentItem）。replies 仅在一级评论且带预览时有值。 */
export interface CommentItem {
  id: string;
  content: string;
  user: CommentUser;
  like_cnt: number;
  reply_cnt: number;
  is_top: boolean;
  is_self: boolean;
  created_at: string;
  replies?: CommentItem[];
}

/** 评论列表响应包裹（GET /videos/:bvid/comments 与 /comments/:id/replies 同形态） */
export interface CommentListResult {
  list: CommentItem[];
  total: number;
}

/** 发表/回复评论请求（≤ 1000 字；root_id/parent_id 为空表示一级评论） */
export interface AddCommentReq {
  content: string;
  root_id?: string;
  parent_id?: string;
}

/** 互动状态聚合（对应 server State：赞/币/藏）。coin_count 为已投币数，0=未投 */
export interface InteractionState {
  liked: boolean;
  coin_count: number;
  faved: boolean;
}

/** 三连结果（对应 server TripleResult）。delta 供前端本地修正计数 */
export interface TripleResult {
  liked: boolean;
  coin_count: number;
  faved: boolean;
  like_delta: number;
  coin_delta: number;
  fav_delta: number;
}

/** 点赞开关响应（视频/评论共用形态） */
export interface LikedResult {
  liked: boolean;
}

/** 收藏开关响应 */
export interface FavedResult {
  faved: boolean;
}

/** 收藏夹（对应 server Collection）。is_default=1 的默认夹不可删改 */
export interface CollectionItem {
  id: string;
  name: string;
  is_default: number;
  created_at: string;
}

/** 我的收藏视频列表响应（GET /users/me/favorites；list 为 video.Card，见 model/Video.ets） */
export interface FavoritesResult {
  list: import('./Video').VideoCard[];
  total: number;
}

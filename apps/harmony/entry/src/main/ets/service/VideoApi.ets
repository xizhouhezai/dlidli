import { HttpClient } from './HttpClient';
import {
  Category,
  HistoryResult,
  PartListResult,
  ProgressResult,
  VideoCard,
  VideoDetail,
  VideoListResult,
  VideoPart
} from '../model/Video';

const PATH_CATEGORIES: string = '/api/v1/categories';
const PATH_VIDEOS: string = '/api/v1/videos';
const PATH_RECOMMEND: string = '/api/v1/recommend/videos';

/** 稿件级子资源的路径（进度、播放上报） */
function videoPath(bvid: string, suffix: string): string {
  return `${PATH_VIDEOS}/${encodeURIComponent(bvid)}${suffix}`;
}

/**
 * 视频域接口。信息公开读接口带令牌时服务端会按用户过滤（推荐流的已看过滤、负反馈等），
 * 故走默认的带凭证请求而非 getPublic。
 */
export class VideoApi {
  /** 分区列表（含一级与二级，页面自行取 parent_id === 0 的一级分区） */
  static listCategories(): Promise<Category[]> {
    return HttpClient.get<Category[]>(PATH_CATEGORIES);
  }

  /**
   * 分区/排序信息流：/videos?category_id&sort=new|hot&page&page_size。
   * 该接口**不返回 total**，调用方以「满页即还有下一页」推断 hasMore。
   */
  static async listVideos(categoryId: number, sort: string, page: number, size: number): Promise<VideoCard[]> {
    const query: Map<string, string> = new Map<string, string>();
    query.set('category_id', `${categoryId}`);
    query.set('sort', sort);
    query.set('page', `${page}`);
    query.set('page_size', `${size}`);
    const res: VideoListResult = await HttpClient.get<VideoListResult>(PATH_VIDEOS, { query: query });
    return res.list;
  }

  /**
   * 首页推荐信息流（REC-01，与 Web 端首页「推荐」同一接口）。
   * 注意分页参数名与 /videos 不同（这里用 size），且不支持分区过滤——选定分区后须走 listVideos。
   */
  static async listRecommended(page: number, size: number): Promise<VideoCard[]> {
    const query: Map<string, string> = new Map<string, string>();
    query.set('page', `${page}`);
    query.set('size', `${size}`);
    const res: VideoListResult = await HttpClient.get<VideoListResult>(PATH_RECOMMEND, { query: query });
    return res.list;
  }

  /**
   * 稿件详情（含全部画质档位的签名播放地址）。
   * 服务端不按画质排序之外的规则过滤：`streams` 以 quality 降序下发（`quality=0` 原画排在最后），
   * 与 Web 端一致地取 `streams[0]` 作为默认档。无可用流时为 `null`（须按可空处理）。
   */
  static detail(bvid: string): Promise<VideoDetail> {
    return HttpClient.get<VideoDetail>(videoPath(bvid, ''));
  }

  /** 分P列表（单P稿件返回空数组）；多P时每P各带自己的 streams */
  static async parts(bvid: string): Promise<VideoPart[]> {
    const res: PartListResult = await HttpClient.get<PartListResult>(videoPath(bvid, '/parts'));
    return res.list;
  }

  /** 观看历史（最近观看倒序）。该接口不返回 total，以「满页即还有下一页」推断 */
  static async history(page: number, size: number): Promise<VideoCard[]> {
    const query: Map<string, string> = new Map<string, string>();
    query.set('page', `${page}`);
    query.set('page_size', `${size}`);
    const res: HistoryResult = await HttpClient.get<HistoryResult>(`${PATH_VIDEOS}/history`, { query: query });
    return res.list;
  }

  /** 读取跨端续播位置（秒）；服务端无记录返回 0。需登录 */
  static async getProgress(bvid: string): Promise<number> {
    const res: ProgressResult = await HttpClient.get<ProgressResult>(videoPath(bvid, '/progress'));
    return res.position;
  }

  /** 上报观看进度（秒）。服务端为覆盖式写入、无幂等键，节流由调用方负责 */
  static saveProgress(bvid: string, position: number): Promise<void> {
    const body: Record<string, number> = { 'position': Math.max(0, Math.floor(position)) };
    return HttpClient.post<void>(videoPath(bvid, '/progress'), body);
  }

  /**
   * 有效播放上报（PLY-05）：服务端按「登录 uid / 游客 IP」去重、8h 内只计一次，故端侧只在
   * 累计观看超过 5s 时调一次即可。走带凭证请求——游客不带令牌时服务端按 IP 计数（可选鉴权）。
   */
  static reportView(bvid: string): Promise<void> {
    return HttpClient.post<void>(videoPath(bvid, '/view'), null);
  }
}

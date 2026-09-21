import { HttpClient } from './HttpClient';
import { Category, VideoCard, VideoListResult } from '../model/Video';

const PATH_CATEGORIES: string = '/api/v1/categories';
const PATH_VIDEOS: string = '/api/v1/videos';
const PATH_RECOMMEND: string = '/api/v1/recommend/videos';

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
}

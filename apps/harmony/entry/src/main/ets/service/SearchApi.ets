import { HttpClient } from './HttpClient';
import { PagedResult, VideoCard } from '../model/Video';
import { Profile } from '../model/Auth';

const PATH_SEARCH: string = '/api/v1/search';

/**
 * 搜索域接口：/search?keyword&type=video|user&page&page_size，返回 total（可据此判断是否还有下一页）。
 *
 * 上游缺口（本期零后端改动，端侧只能如实降级）：
 * - SRH-02 排序与筛选：接口无 sort/时长/分区/发布时间参数（M2-SRH-03 已标注「随 ES 接入补」）；
 * - SRH-04 搜索历史：全仓无历史接口（M2-SRH-04 标注「历史待补」），故历史只落端侧本地。
 */
export class SearchApi {
  static searchVideos(keyword: string, page: number, size: number): Promise<PagedResult<VideoCard>> {
    return SearchApi.search<VideoCard>(keyword, 'video', page, size);
  }

  static searchUsers(keyword: string, page: number, size: number): Promise<PagedResult<Profile>> {
    return SearchApi.search<Profile>(keyword, 'user', page, size);
  }

  private static search<T>(keyword: string, type: string, page: number, size: number): Promise<PagedResult<T>> {
    const query: Map<string, string> = new Map<string, string>();
    query.set('keyword', keyword);
    query.set('type', type);
    query.set('page', `${page}`);
    query.set('page_size', `${size}`);
    return HttpClient.get<PagedResult<T>>(PATH_SEARCH, { query: query });
  }
}

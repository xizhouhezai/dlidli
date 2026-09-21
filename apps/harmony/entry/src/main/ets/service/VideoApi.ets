import { HttpClient } from './HttpClient';
import { Category } from '../model/Video';

const PATH_CATEGORIES: string = '/api/v1/categories';

/**
 * 视频域接口。本期仅落首页联通性验证所需的分类列表，
 * 信息流/详情/签名播放地址等随 M4-HMY-05、M4-HMY-06 在此文件按域补齐。
 */
export class VideoApi {
  /** 分区列表（公开接口，无需登录） */
  static listCategories(): Promise<Category[]> {
    return HttpClient.get<Category[]>(PATH_CATEGORIES);
  }
}

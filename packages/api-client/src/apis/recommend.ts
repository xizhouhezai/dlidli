import type { HttpClient } from '../http'
import type { VideoCard } from './video'

/** 推荐接口（对应后端 recommend 模块，M3-REC）。 */
export function createRecommendApi(http: HttpClient) {
  return {
    /** 首页推荐信息流（混合召回；游客/关闭个性化时退化为热度榜） */
    recommend: (page = 1, pageSize = 20) =>
      http.get<{ list: VideoCard[] }>('/api/v1/recommend/videos', {
        page,
        page_size: pageSize,
      }),

    /** 全站/分区热度榜（加权分） */
    hot: (params: { category_id?: number, page?: number, page_size?: number }) =>
      http.get<{ list: VideoCard[] }>('/api/v1/videos/hot', params),

    /** 行为上报（1曝光 2点击 3播放 4互动，批量） */
    reportBehavior: (items: Array<{ video_id: string, action: 1 | 2 | 3 | 4 }>) =>
      http.post<null>('/api/v1/behaviors', { items }),

    /** 负反馈（1内容 2UP主 3分区） */
    addDislike: (targetType: 1 | 2 | 3, targetId: string) =>
      http.post<null>('/api/v1/dislikes', { target_type: targetType, target_id: targetId }),

    /** 个性化推荐开关状态 */
    recommendSetting: () =>
      http.get<{ enabled: boolean }>('/api/v1/users/me/recommend-settings'),

    /** 开关个性化推荐（关闭后退化为热度榜，合规） */
    setRecommendSetting: (enabled: boolean) =>
      http.put<null>('/api/v1/users/me/recommend-settings', { enabled }),
  }
}

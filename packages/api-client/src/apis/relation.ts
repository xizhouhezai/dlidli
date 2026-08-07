import type { HttpClient } from '../http'

/** 用户简况（关注/粉丝列表条目，同 Profile 结构） */
export interface UserBrief {
  id: string
  nickname: string
  avatar: string
  level: number
  signature?: string
}

export interface RelationStat {
  following: boolean
  follower_cnt: number
  following_cnt: number
}

/** 关系链接口（对应后端 relation 模块，挂载于 /space/:id）。 */
export function createRelationApi(http: HttpClient) {
  return {
    /** 公开用户资料（个人空间头部） */
    profile: (uid: string) => http.get<UserBrief>(`/api/v1/space/${uid}/profile`),

    /** 关注/取关开关 */
    follow: (uid: string) => http.post<{ following: boolean }>(`/api/v1/space/${uid}/follow`),

    /** 拉黑/取消拉黑（MSG-12） */
    block: (uid: string) => http.post<{ blocked: boolean }>(`/api/v1/space/${uid}/block`),

    /** 双向拉黑状态 */
    blockStatus: (uid: string) =>
      http.get<{ i_blocked: boolean; blocked_me: boolean }>(`/api/v1/space/${uid}/block-status`),

    /** 关系状态（是否已关注 + 关注/粉丝数） */
    stat: (uid: string) => http.get<RelationStat>(`/api/v1/space/${uid}/relation`),

    followings: (uid: string, page = 1, pageSize = 20) =>
      http.get<{ list: UserBrief[]; total: number }>(`/api/v1/space/${uid}/followings`, {
        page,
        page_size: pageSize,
      }),

    followers: (uid: string, page = 1, pageSize = 20) =>
      http.get<{ list: UserBrief[]; total: number }>(`/api/v1/space/${uid}/followers`, {
        page,
        page_size: pageSize,
      }),
  }
}

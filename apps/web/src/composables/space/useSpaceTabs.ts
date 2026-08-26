import { ref, watch, type Ref, type ComputedRef } from 'vue'
import type { UserBrief, VideoCard as VideoCardData } from '@dlidli/api-client'
import { api } from '@/api'

export type SpaceTabKey = 'videos' | 'collections' | 'followings' | 'followers' | 'favorites'

/**
 * 空间 Tab 内容加载（SpaceView 拆分，M3-ENG-09）：
 * 投稿 / 合集 / 关注 / 粉丝 / 收藏 五个 Tab 的数据拉取。
 */
export function useSpaceTabs(
  uid: ComputedRef<string> | Ref<string>,
  hooks: { onCollectionsTab: () => Promise<void> | void },
) {
  const activeTab = ref<SpaceTabKey>('videos')
  const videos = ref<VideoCardData[]>([])
  const users = ref<UserBrief[]>([])
  const usersTotal = ref(0)
  const tabLoading = ref(false)

  async function loadTab() {
    tabLoading.value = true
    videos.value = []
    users.value = []
    usersTotal.value = 0
    try {
      switch (activeTab.value) {
        case 'videos': {
          const res = await api.video.list({ uid: uid.value, sort: 'new', page_size: 24 })
          videos.value = res.list
          break
        }
        case 'collections': {
          await hooks.onCollectionsTab()
          break
        }
        case 'followings': {
          const res = await api.relation.followings(uid.value, 1, 50)
          users.value = res.list
          usersTotal.value = res.total
          break
        }
        case 'followers': {
          const res = await api.relation.followers(uid.value, 1, 50)
          users.value = res.list
          usersTotal.value = res.total
          break
        }
        case 'favorites': {
          const res = await api.interaction.favorites(1, 24)
          videos.value = res.list
          break
        }
      }
    } finally {
      tabLoading.value = false
    }
  }

  // Tab 切换自动重载
  watch(activeTab, loadTab)

  return { activeTab, videos, users, usersTotal, tabLoading, loadTab }
}

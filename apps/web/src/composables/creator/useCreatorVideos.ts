import { ref } from 'vue'
import type { CreatorVideoStat } from '@dlidli/api-client'
import { api } from '@/api'

/**
 * 稿件数据分页（CreatorView 拆分，M3-ENG-10）。
 */
export function useCreatorVideos() {
  const videos = ref<CreatorVideoStat[]>([])
  const videosTotal = ref(0)
  const videosPage = ref(1)
  const videosLoading = ref(false)
  const videosLoaded = ref(false)

  async function loadVideos(reset = false) {
    if (reset) {
      videosPage.value = 1
      videos.value = []
    }
    videosLoading.value = true
    try {
      const res = await api.creator.videos(videosPage.value, 10)
      videos.value = res.list ?? []
      videosTotal.value = res.total
    } finally {
      videosLoading.value = false
      if (reset) videosLoaded.value = true
    }
  }

  function onVideosPage(page: number) {
    videosPage.value = page
    void loadVideos()
  }

  return { videos, videosTotal, videosPage, videosLoading, videosLoaded, loadVideos, onVideosPage }
}

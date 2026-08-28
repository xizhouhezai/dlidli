import { ref } from 'vue'
import type { VideoCard } from '@dlidli/api-client'
import { api } from '@/api'

/**
 * 首页推荐区（HomeView 拆分，M3-ENG-08）：
 * 优先用运营位配置 Banner（M3-OPS-01），无配置回退最热前 8；右侧 2×2 网格用最热填充。
 */
export function useHomeBanners() {
  const banners = ref<VideoCard[]>([]) // 左侧大图轮播
  const sideRecos = ref<VideoCard[]>([]) // 右侧 2×2 推荐网格

  async function loadBanners() {
    try {
      const configured = await api.video.banners()
      if (configured.list.length > 0) {
        banners.value = configured.list.slice(0, 4).map((b) => ({
          id: b.id,
          bvid: b.bvid,
          title: b.title,
          cover: b.image,
          duration: 0,
          status: 4,
          published_at: null,
          created_at: '',
          owner: { id: '', nickname: '', avatar: '' },
          stat: { view: 0, like: 0, coin: 0, fav: 0, danmaku: 0, comment: 0, share: 0 },
        }))
        // 右侧 2×2 网格用最热前 4 填充（不空白）
        try {
          const res = await api.video.list({ sort: 'hot', page: 1, page_size: 4 })
          sideRecos.value = res.list
        } catch {
          sideRecos.value = []
        }
        return
      }
    } catch {
      // 接口失败回退最热
    }
    try {
      const res = await api.video.list({ sort: 'hot', page: 1, page_size: 8 })
      banners.value = res.list.slice(0, 4)
      sideRecos.value = res.list.slice(4, 8)
    } catch {
      banners.value = []
      sideRecos.value = []
    }
  }

  return { banners, sideRecos, loadBanners }
}

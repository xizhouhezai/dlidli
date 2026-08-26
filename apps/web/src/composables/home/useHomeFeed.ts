import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import type { CategoryItem, VideoCard } from '@dlidli/api-client'
import { api } from '@/api'
import { useUserStore } from '@/stores/user'

const PAGE_SIZE = 24

/**
 * 首页视频流（HomeView 拆分，M3-ENG-08）：
 * 分区导航 / 排序切换 / 无限滚动分页 / 推荐曝光上报 / 不感兴趣。
 */
export function useHomeFeed() {
  const router = useRouter()
  const userStore = useUserStore()

  const categories = ref<CategoryItem[]>([])
  const activeCategory = ref(0) // 0 = 全部
  const sort = ref<'recommend' | 'new' | 'hot'>('recommend')
  const videos = ref<VideoCard[]>([])
  const page = ref(1)
  const hasMore = ref(false)
  const loading = ref(true)
  const loadingMore = ref(false)
  // 无限滚动状态
  const backendDown = ref(false)

  // 无限滚动：监听窗口滚动，距底 <400px 自动加载下一页
  function onScroll() {
    const doc = document.documentElement
    if (doc.scrollTop + window.innerHeight >= doc.scrollHeight - 400) loadMore()
  }

  async function loadList(reset = true) {
    if (reset) {
      page.value = 1
      loading.value = true
    } else {
      loadingMore.value = true
    }
    try {
      let list: VideoCard[]
      if (sort.value === 'recommend') {
        const res = await api.recommend.recommend(page.value, PAGE_SIZE)
        list = res.list
      } else {
        const res = await api.video.list({
          category_id: activeCategory.value || undefined,
          sort: sort.value,
          page: page.value,
          page_size: PAGE_SIZE,
        })
        list = res.list
      }
      videos.value = reset ? list : [...videos.value, ...list]
      hasMore.value = list.length === PAGE_SIZE
      // 推荐 Tab：曝光上报（登录用户，节流到每次加载）
      if (sort.value === 'recommend' && reset) reportExpose(list)
    } catch {
      backendDown.value = true
    } finally {
      loading.value = false
      loadingMore.value = false
    }
  }

  // 曝光上报（旁路，失败静默）
  function reportExpose(list: VideoCard[]) {
    if (!userStore.token || list.length === 0) return
    api.recommend
      .reportBehavior(list.map((v) => ({ video_id: v.id, action: 1 as const })))
      .catch(() => {})
  }

  // 触底自动加载下一页（防并发：加载中/无更多时不触发）
  function loadMore() {
    if (!hasMore.value || loading.value || loadingMore.value) return
    page.value++
    loadList(false)
  }

  function switchSort(s: 'recommend' | 'new' | 'hot') {
    if (sort.value === s) return
    sort.value = s
    loadList()
  }

  // 不感兴趣（推荐 Tab 卡片更多菜单）
  const dislikePending = ref(false)
  function onCardMenu(cmd: string, v: VideoCard) {
    if (cmd === 'dislike') void onDislike(v)
  }

  async function onDislike(v: VideoCard) {
    if (!userStore.token) {
      router.push('/login')
      return
    }
    if (dislikePending.value) return
    dislikePending.value = true
    try {
      await api.recommend.addDislike(1, v.id)
      videos.value = videos.value.filter((x) => x.bvid !== v.bvid)
      ElMessage.success('已减少此类推荐')
    } catch (err) {
      ElMessage.error(err instanceof Error ? err.message : '操作失败')
    } finally {
      dislikePending.value = false
    }
  }

  async function loadCategories() {
    try {
      categories.value = (await api.video.categories()).filter((c) => c.parent_id === 0)
    } catch {
      backendDown.value = true
    }
  }

  function onCategoryClick(id: number) {
    activeCategory.value = id
    loadList()
  }

  /** 进入页面：加载分区 + 首屏列表 + 挂滚动监听；离开时移除。 */
  async function start() {
    await loadCategories()
    await loadList()
    window.addEventListener('scroll', onScroll, { passive: true })
  }
  function stop() {
    window.removeEventListener('scroll', onScroll)
  }

  return {
    categories,
    activeCategory,
    sort,
    videos,
    hasMore,
    loading,
    loadingMore,
    backendDown,
    dislikePending,
    switchSort,
    onCategoryClick,
    onCardMenu,
    start,
    stop,
  }
}

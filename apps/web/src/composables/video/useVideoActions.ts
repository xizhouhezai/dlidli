import { ref, type Ref } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { ApiError, type CollectionItem, type VideoDetail } from '@dlidli/api-client'
import { api } from '@/api'
import { useUserStore } from '@/stores/user'

/**
 * 视频互动控制器（VideoView 拆分，M3-ENG-06）：
 * 点赞/投币/收藏（含收藏夹弹层）/一键三连（长按）/关注/转发。
 */
export function useVideoActions(detail: Ref<VideoDetail | null>) {
  const router = useRouter()
  const userStore = useUserStore()

  // 三连：赞 / 币 / 藏
  const liked = ref(false)
  const coined = ref(0) // 已投币数
  const faved = ref(false)
  const acting = ref(false)
  const coinPopVisible = ref(false)

  // 关注（UP 主卡片）
  const following = ref(false)
  const followerCnt = ref(0)
  const followPending = ref(false)

  // 收藏夹弹层
  const favPopVisible = ref(false)
  const collections = ref<CollectionItem[]>([])
  const newColName = ref('')

  function requireLogin(): boolean {
    if (!userStore.token) {
      router.push('/login')
      return false
    }
    return true
  }

  async function toggleLike() {
    if (!requireLogin() || !detail.value || acting.value) return
    acting.value = true
    try {
      const res = await api.interaction.likeVideo(detail.value.bvid)
      liked.value = res.liked
      detail.value.stat.like += res.liked ? 1 : -1
    } catch (err) {
      ElMessage.error(err instanceof ApiError ? err.message : '操作失败')
    } finally {
      acting.value = false
    }
  }

  function loadRelation(ownerID: string) {
    api.relation
      .stat(ownerID)
      .then((st) => {
        following.value = st.following
        followerCnt.value = st.follower_cnt
      })
      .catch(() => {})
  }

  async function toggleFollow() {
    if (!requireLogin() || !detail.value || followPending.value) return
    followPending.value = true
    try {
      const res = await api.relation.follow(detail.value.owner.id)
      following.value = res.following
      followerCnt.value += res.following ? 1 : -1
      ElMessage.success(res.following ? '关注成功，可以召唤 TA 的更多更新啦' : '已取消关注')
    } catch (err) {
      ElMessage.error(err instanceof ApiError ? err.message : '操作失败')
    } finally {
      followPending.value = false
    }
  }

  // 长按点赞 0.8s 触发一键三连（B 站同款交互）
  let pressTimer: ReturnType<typeof setTimeout> | null = null
  let longPressed = false

  function startPress() {
    if (!userStore.token || !detail.value) return
    longPressed = false
    pressTimer = setTimeout(() => {
      longPressed = true
      void doTriple()
    }, 800)
  }

  function endPress(isClick: boolean) {
    if (pressTimer) {
      clearTimeout(pressTimer)
      pressTimer = null
    }
    if (isClick && !longPressed) void toggleLike()
  }

  async function doTriple() {
    if (!requireLogin() || !detail.value || acting.value) return
    acting.value = true
    try {
      const res = await api.interaction.triple(detail.value.bvid)
      liked.value = res.liked
      coined.value = res.coin_count
      faved.value = res.faved
      detail.value.stat.like += res.like_delta
      detail.value.stat.coin += res.coin_delta
      detail.value.stat.fav += res.fav_delta
      ElMessage.success(res.coin_delta > 0 ? '三连成功，感谢支持！' : '三连成功！')
      userStore.refreshProfile()
    } catch (err) {
      ElMessage.error(err instanceof ApiError ? err.message : '操作失败')
    } finally {
      acting.value = false
    }
  }

  function openCoinPop() {
    if (!requireLogin() || !detail.value) return
    if (coined.value > 0) {
      ElMessage.info('已经投过币啦')
      return
    }
    coinPopVisible.value = !coinPopVisible.value
  }

  async function doCoin(count: 1 | 2) {
    coinPopVisible.value = false
    if (!detail.value || acting.value) return
    acting.value = true
    try {
      await api.interaction.coinVideo(detail.value.bvid, count)
      coined.value = count
      detail.value.stat.coin += count
      ElMessage.success(`投了 ${count} 枚硬币～`)
      userStore.refreshProfile()
    } catch (err) {
      ElMessage.error(err instanceof ApiError ? err.message : '投币失败')
    } finally {
      acting.value = false
    }
  }

  /** 转发到动态 */
  async function openShare() {
    if (!requireLogin() || !detail.value) return
    let content = ''
    try {
      const res = await ElMessageBox.prompt('说点什么（可留空）', '转发到动态', {
        confirmButtonText: '转发',
        cancelButtonText: '取消',
        inputPlaceholder: '分享给关注你的人~',
      })
      content = res.value?.trim() ?? ''
    } catch {
      return
    }
    try {
      await api.dynamic.shareVideo(detail.value.bvid, content)
      detail.value.stat.share += 1
      ElMessage.success('已转发到动态')
    } catch (err) {
      ElMessage.error(err instanceof ApiError ? err.message : '转发失败')
    }
  }

  async function toggleFav() {
    if (!requireLogin() || !detail.value || acting.value) return
    if (faved.value) {
      // 已收藏 → 直接取消
      void doFav('0')
      return
    }
    // 未收藏 → 弹层选收藏夹
    favPopVisible.value = !favPopVisible.value
    if (favPopVisible.value) void loadCollections()
  }

  async function loadCollections() {
    try {
      collections.value = await api.interaction.listCollections()
    } catch {
      collections.value = []
    }
  }

  async function createCollection() {
    const name = newColName.value.trim()
    if (!name) return
    try {
      const col = await api.interaction.createCollection(name)
      collections.value = [...collections.value, col]
      newColName.value = ''
    } catch (err) {
      ElMessage.error(err instanceof ApiError ? err.message : '创建失败')
    }
  }

  async function doFav(collectionId: string) {
    favPopVisible.value = false
    if (!detail.value || acting.value) return
    acting.value = true
    try {
      const res = await api.interaction.toggleFavorite(detail.value.bvid, collectionId)
      faved.value = res.faved
      detail.value.stat.fav += res.faved ? 1 : -1
      ElMessage.success(res.faved ? '已收藏' : '已取消收藏')
    } catch (err) {
      ElMessage.error(err instanceof ApiError ? err.message : '操作失败')
    } finally {
      acting.value = false
    }
  }

  /** 切换稿件后重置互动状态。 */
  function reset() {
    liked.value = false
    coined.value = 0
    faved.value = false
    coinPopVisible.value = false
    following.value = false
    followerCnt.value = 0
    followPending.value = false
    favPopVisible.value = false
    collections.value = []
    newColName.value = ''
  }

  return {
    liked,
    coined,
    faved,
    acting,
    coinPopVisible,
    following,
    followerCnt,
    followPending,
    favPopVisible,
    collections,
    newColName,
    toggleLike,
    loadRelation,
    toggleFollow,
    startPress,
    endPress,
    doTriple,
    openCoinPop,
    doCoin,
    openShare,
    toggleFav,
    loadCollections,
    createCollection,
    doFav,
    reset,
  }
}

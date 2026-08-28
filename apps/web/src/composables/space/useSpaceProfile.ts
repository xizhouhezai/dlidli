import { ref, type Ref, type ComputedRef } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { ApiError, type UserBrief } from '@dlidli/api-client'
import { api } from '@/api'
import { useUserStore } from '@/stores/user'

/**
 * 空间头部资料与关注关系（SpaceView 拆分，M3-ENG-09）。
 */
export function useSpaceProfile(uid: ComputedRef<string> | Ref<string>) {
  const router = useRouter()
  const userStore = useUserStore()

  const profile = ref<UserBrief | null>(null)
  const notFound = ref(false)
  const following = ref(false)
  const followerCnt = ref(0)
  const followingCnt = ref(0)
  const followPending = ref(false)

  async function loadHead() {
    notFound.value = false
    profile.value = null
    try {
      profile.value = await api.relation.profile(uid.value)
      document.title = `${profile.value.nickname} 的个人空间 - DliDli`
    } catch {
      notFound.value = true
      return
    }
    api.relation
      .stat(uid.value)
      .then((st) => {
        following.value = st.following
        followerCnt.value = st.follower_cnt
        followingCnt.value = st.following_cnt
      })
      .catch(() => {})
  }

  async function toggleFollow() {
    if (!userStore.token) {
      router.push('/login')
      return
    }
    if (followPending.value) return
    followPending.value = true
    try {
      const res = await api.relation.follow(uid.value)
      following.value = res.following
      followerCnt.value += res.following ? 1 : -1
    } catch (err) {
      ElMessage.error(err instanceof ApiError ? err.message : '操作失败')
    } finally {
      followPending.value = false
    }
  }

  return {
    profile,
    notFound,
    following,
    followerCnt,
    followingCnt,
    followPending,
    loadHead,
    toggleFollow,
  }
}

import { ref } from 'vue'
import { ElMessage } from 'element-plus'
import { api } from '@/api'

/**
 * 私信拉黑（MessagesView 拆分，M3-ENG-11）：拉黑状态查询与切换（MSG-12）。
 */
export function useBlockActions(getPeer: () => string) {
  const blockedMe = ref(false)
  const iBlocked = ref(false)
  const blockPending = ref(false)

  async function loadBlockStatus() {
    const peer = getPeer()
    if (!peer) return
    try {
      const s = await api.relation.blockStatus(peer)
      iBlocked.value = s.i_blocked
      blockedMe.value = s.blocked_me
    } catch {
      iBlocked.value = false
      blockedMe.value = false
    }
  }

  async function toggleBlock() {
    const peer = getPeer()
    if (!peer || blockPending.value) return
    blockPending.value = true
    try {
      const r = await api.relation.block(peer)
      iBlocked.value = r.blocked
      ElMessage.success(r.blocked ? '已拉黑对方，TA 将无法给你发私信' : '已取消拉黑')
    } catch {
      // http 层已弹错误提示
    } finally {
      blockPending.value = false
    }
  }

  function reset() {
    blockedMe.value = false
    iBlocked.value = false
  }

  return { blockedMe, iBlocked, blockPending, loadBlockStatus, toggleBlock, reset }
}

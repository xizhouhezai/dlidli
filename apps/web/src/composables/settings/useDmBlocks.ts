import { ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { ApiError, type DanmakuBlockItem } from '@dlidli/api-client'
import { api } from '@/api'

/**
 * 弹幕屏蔽管理（SettingsView 拆分，M3-ENG-12，M2-DM-02）：关键词 + 屏蔽用户。
 */
export function useDmBlocks() {
  const dmBlocks = ref<DanmakuBlockItem[]>([])
  const dmBlockLoading = ref(false)
  const dmBlockInput = ref('')
  const dmBlockAdding = ref(false)

  async function loadDmBlocks() {
    dmBlockLoading.value = true
    try {
      dmBlocks.value = (await api.danmaku.blocks()).list
    } catch {
      // 静默失败
    } finally {
      dmBlockLoading.value = false
    }
  }

  async function addDmBlock() {
    const kw = dmBlockInput.value.trim()
    if (!kw) return
    dmBlockAdding.value = true
    try {
      await api.danmaku.addBlock({ block_type: 1, keyword: kw })
      dmBlockInput.value = ''
      ElMessage.success('已添加屏蔽词')
      await loadDmBlocks()
    } catch (err) {
      ElMessage.error(err instanceof ApiError ? err.message : '操作失败')
    } finally {
      dmBlockAdding.value = false
    }
  }

  async function removeDmBlock(b: DanmakuBlockItem) {
    try {
      await api.danmaku.deleteBlock(b.id)
      dmBlocks.value = dmBlocks.value.filter((x) => x.id !== b.id)
      ElMessage.success('已删除')
    } catch (err) {
      ElMessage.error(err instanceof ApiError ? err.message : '删除失败')
    }
  }

  async function clearDmBlockHash(hash: string) {
    await ElMessageBox.confirm('确定解除对该用户的弹幕屏蔽吗？', '解除屏蔽', { type: 'warning' })
    const b = dmBlocks.value.find((x) => x.block_type === 2 && x.block_hash === hash)
    if (b) await removeDmBlock(b)
  }

  return { dmBlocks, dmBlockLoading, dmBlockInput, dmBlockAdding, loadDmBlocks, addDmBlock, removeDmBlock, clearDmBlockHash }
}

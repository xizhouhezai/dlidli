import { ref, type Ref, type ComputedRef } from 'vue'
import { ElMessage } from 'element-plus'
import { ApiError, type CollectionCard } from '@dlidli/api-client'
import { api } from '@/api'

/**
 * 空间合集（SpaceView 拆分，M3-ENG-09）：合集列表加载 / 新建合集（M3-CRT-05）。
 */
export function useSpaceCollections(uid: ComputedRef<string> | Ref<string>) {
  const collections = ref<CollectionCard[]>([])
  const collectionsLoaded = ref(false)
  const createColVisible = ref(false)
  const createColForm = ref({ title: '', description: '' })
  const createColSaving = ref(false)

  async function loadCollections() {
    try {
      collections.value = (await api.collection.list(uid.value)).list ?? []
      collectionsLoaded.value = true
    } catch {
      collections.value = []
    }
  }

  async function createCollection() {
    if (!createColForm.value.title.trim()) {
      ElMessage.warning('请填写合集标题')
      return
    }
    createColSaving.value = true
    try {
      await api.collection.create({
        title: createColForm.value.title.trim(),
        description: createColForm.value.description.trim(),
      })
      createColVisible.value = false
      createColForm.value = { title: '', description: '' }
      ElMessage.success('合集已创建')
      void loadCollections()
    } catch (err) {
      ElMessage.error(err instanceof ApiError ? err.message : '创建失败')
    } finally {
      createColSaving.value = false
    }
  }

  return {
    collections,
    collectionsLoaded,
    createColVisible,
    createColForm,
    createColSaving,
    loadCollections,
    createCollection,
  }
}

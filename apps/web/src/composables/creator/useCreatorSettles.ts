import { ref } from 'vue'
import type { SettlementItem } from '@dlidli/api-client'
import { api } from '@/api'

/**
 * 收益明细分页（CreatorView 拆分，M3-ENG-10）。
 */
export function useCreatorSettles() {
  const settles = ref<SettlementItem[]>([])
  const settlesTotal = ref(0)
  const settlesPage = ref(1)
  const settlesLoading = ref(false)
  const settlesLoaded = ref(false)

  async function loadSettles(reset = false) {
    if (reset) {
      settlesPage.value = 1
      settles.value = []
    }
    settlesLoading.value = true
    try {
      const res = await api.creator.settlements(settlesPage.value, 10)
      settles.value = res.list ?? []
      settlesTotal.value = res.total
    } finally {
      settlesLoading.value = false
      if (reset) settlesLoaded.value = true
    }
  }

  function onSettlesPage(page: number) {
    settlesPage.value = page
    void loadSettles()
  }

  return {
    settles,
    settlesTotal,
    settlesPage,
    settlesLoading,
    settlesLoaded,
    loadSettles,
    onSettlesPage,
  }
}

import { computed, ref } from 'vue'
import type { CreatorOverview } from '@dlidli/api-client'
import { api } from '@/api'

/**
 * 创作者概览（CreatorView 拆分，M3-ENG-10）：统计卡数据与累计收益（分→元）。
 */
export function useCreatorOverview() {
  const overview = ref<CreatorOverview | null>(null)
  const loading = ref(true)

  // 收益展示：分 → 元
  const earningsYuan = computed(() => ((overview.value?.earnings ?? 0) / 100).toFixed(2))

  async function load() {
    loading.value = true
    try {
      overview.value = await api.creator.overview()
    } finally {
      loading.value = false
    }
  }

  return { overview, loading, earningsYuan, load }
}

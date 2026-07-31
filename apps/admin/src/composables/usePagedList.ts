// 分页列表加载：统一 page/total/loading/list + reload/search/onPageChange。
import { ref, onMounted } from 'vue'

interface PagedResult<T> {
  list: T[]
  total: number
}

/**
 * @param fetcher (page, pageSize) => { list, total }
 * @param opts.pageSize 每页条数，默认 20；immediate 是否挂载即加载，默认 true
 */
export function usePagedList<T>(
  fetcher: (page: number, pageSize: number) => Promise<PagedResult<T>>,
  opts: { pageSize?: number, immediate?: boolean } = {},
) {
  const pageSize = opts.pageSize ?? 20
  const list = ref<T[]>([]) as { value: T[] }
  const total = ref(0)
  const loading = ref(false)
  const page = ref(1)

  async function load() {
    loading.value = true
    try {
      const res = await fetcher(page.value, pageSize)
      list.value = res.list
      total.value = res.total
    } finally {
      loading.value = false
    }
  }

  /** 重置到第 1 页并加载（查询条件变化时用）。 */
  function search() {
    page.value = 1
    load()
  }

  function onPageChange(p: number) {
    page.value = p
    load()
  }

  if (opts.immediate !== false) onMounted(load)

  return { list, total, loading, page, pageSize, load, search, onPageChange }
}

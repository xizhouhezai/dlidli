// 分页列表加载：统一 page/total/loading/list + reload/search/onPageChange。
import { ref, onMounted, type Ref } from 'vue'
import { ElMessage } from 'element-plus'
import { apiErrorMessage } from '@dlidli/api-client'

interface PagedResult<T> {
  list: T[]
  total: number
}

/**
 * @param fetcher (page, pageSize) => { list, total }
 * @param opts.pageSize 每页条数，默认 20；immediate 是否挂载即加载，默认 true
 * @param opts.errorText 加载失败 toast 的兜底文案
 */
export function usePagedList<T>(
  fetcher: (page: number, pageSize: number) => Promise<PagedResult<T>>,
  opts: { pageSize?: number; immediate?: boolean; errorText?: string } = {},
) {
  const pageSize = opts.pageSize ?? 20
  // Ref<T[]> 断言（Vue 官方推荐的泛型 ref 写法）：保住 Ref 品牌让模板自动解包
  const list = ref<T[]>([]) as Ref<T[]>
  const total = ref(0)
  const loading = ref(false)
  const page = ref(1)

  async function load() {
    loading.value = true
    try {
      const res = await fetcher(page.value, pageSize)
      list.value = res.list
      total.value = res.total
    } catch (err) {
      // 列表加载失败需可见：toast 兜底，避免静默空列表
      ElMessage.error(apiErrorMessage(err, opts.errorText ?? '列表加载失败，请稍后再试'))
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

import { reactive, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { ApiError, type CategoryItem, type VideoDetail } from '@dlidli/api-client'
import { api } from '@/api'

/**
 * 稿件信息表单（UploadView 拆分，M3-ENG-07）：
 * 分区加载 / 标签增删 / 投稿提交 / 成功态 / 重置。
 */
export function useUploadForm() {
  const categories = ref<CategoryItem[]>([])
  const form = reactive({
    title: '',
    description: '',
    categoryId: undefined as number | undefined,
    tags: [] as string[],
    copyright: 1 as 1 | 2,
  })
  const tagInput = ref('')
  const submitting = ref(false)
  const published = ref<VideoDetail | null>(null)

  async function loadCategories() {
    try {
      categories.value = (await api.video.categories()).filter((c) => c.parent_id === 0)
    } catch {
      ElMessage.error('分区加载失败')
    }
  }

  function addTag() {
    const t = tagInput.value.trim()
    if (!t) return
    if (form.tags.includes(t)) {
      tagInput.value = ''
      return
    }
    if (form.tags.length >= 10) {
      ElMessage.warning('最多 10 个标签')
      return
    }
    form.tags.push(t)
    tagInput.value = ''
  }

  async function submit(
    fileId: string,
    parts: { file_id: string; title: string }[],
    _cover: string,
  ): Promise<boolean> {
    const validParts = parts.filter((p) => p.file_id)
    if (!fileId && validParts.length === 0) {
      ElMessage.warning('请先上传视频文件')
      return false
    }
    if (!form.title.trim() || !form.categoryId || form.tags.length === 0) {
      ElMessage.warning('请填写标题、选择分区并至少添加 1 个标签')
      return false
    }
    submitting.value = true
    try {
      published.value = await api.video.submit({
        file_id: fileId,
        title: form.title.trim(),
        description: form.description,
        category_id: form.categoryId,
        tags: form.tags,
        copyright: form.copyright,
        cover: _cover,
        // 多P：有已上传分P时提交 parts（后端按分P建 video_part + 各自流）
        parts: validParts.map((p) => ({ file_id: p.file_id, title: p.title })),
      })
      return true
    } catch (err) {
      ElMessage.error(err instanceof ApiError ? err.message : '投稿失败，请重试')
      return false
    } finally {
      submitting.value = false
    }
  }

  function reset() {
    published.value = null
    form.title = ''
    form.description = ''
    form.categoryId = undefined
    form.tags = []
    form.copyright = 1
    tagInput.value = ''
  }

  return { categories, form, tagInput, submitting, published, loadCategories, addTag, submit, reset }
}

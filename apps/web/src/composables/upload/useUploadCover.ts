import { ref } from 'vue'
import { ElMessage } from 'element-plus'
import { api } from '@/api'
import { captureVideoPoster } from '@/utils/poster'

/**
 * 封面管理（UploadView 拆分，M3-ENG-07）：
 * 首帧 poster（并行截取）→ 自定义上传 → 默认封面 三级兜底；投稿前解析最终封面 URL。
 */
export function useUploadCover() {
  const posterBlob = ref<Blob | null>(null)
  const posterUrl = ref('')
  const coverFile = ref<File | null>(null)
  const coverUrl = ref('')
  const selectedCover = ref<'poster' | 'custom' | ''>('')

  const coverInput = ref<HTMLInputElement>()

  function pickCover() {
    coverInput.value?.click()
  }

  function onCoverChange(e: Event) {
    const f = (e.target as HTMLInputElement).files?.[0]
    if (!f) return
    if (f.size > 5 * 1024 * 1024) {
      ElMessage.warning('封面大小须在 5MB 以内')
      return
    }
    coverFile.value = f
    coverUrl.value = URL.createObjectURL(f)
    selectedCover.value = 'custom'
    if (coverInput.value) coverInput.value.value = ''
  }

  /** 并行截取首帧作为封面首选（mkv/flv 等浏览器不支持的格式会失败，静默回退）。 */
  async function capturePoster(f: File) {
    const blob = await captureVideoPoster(f)
    if (blob) {
      posterBlob.value = blob
      posterUrl.value = URL.createObjectURL(blob)
      if (!selectedCover.value) selectedCover.value = 'poster'
    }
  }

  /** 投稿前解析封面 URL：poster 优先 → 上传封面 → 空（展示端用默认封面）。 */
  async function resolveCover(): Promise<string> {
    if (selectedCover.value === 'poster' && posterBlob.value) {
      return (await api.video.uploadCover(posterBlob.value, 'poster.jpg')).cover
    }
    if (selectedCover.value === 'custom' && coverFile.value) {
      return (await api.video.uploadCover(coverFile.value)).cover
    }
    return ''
  }

  function reset() {
    posterBlob.value = null
    posterUrl.value = ''
    coverFile.value = null
    coverUrl.value = ''
    selectedCover.value = ''
  }

  return {
    coverInput,
    posterBlob,
    posterUrl,
    coverFile,
    coverUrl,
    selectedCover,
    pickCover,
    onCoverChange,
    capturePoster,
    resolveCover,
    reset,
  }
}

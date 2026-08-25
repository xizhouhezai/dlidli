import { ref } from 'vue'
import { ElMessage } from 'element-plus'
import { ApiError } from '@dlidli/api-client'
import { uploadVideoFile, type UploadProgress } from '@/utils/uploader'

/** 支持的视频扩展名（模板 :accept 也引用）。 */
export const ACCEPT_EXTS = ['.mp4', '.mov', '.mkv', '.flv', '.avi']

export interface PartDraft {
  title: string
  fileId: string
  uploading: boolean
  progress: UploadProgress | null
}

/**
 * 分P管理（UploadView 拆分，M3-ENG-07）：分P列表的增删与单个分P上传。
 */
export function useUploadParts() {
  const parts = ref<PartDraft[]>([])

  function addPart() {
    if (parts.value.length >= 10) {
      ElMessage.warning('最多 10 个分P')
      return
    }
    parts.value.push({ title: '', fileId: '', uploading: false, progress: null })
  }

  function removePart(i: number) {
    parts.value.splice(i, 1)
  }

  async function uploadPart(i: number, f: File) {
    const ext = f.name.slice(f.name.lastIndexOf('.')).toLowerCase()
    if (!ACCEPT_EXTS.includes(ext)) {
      ElMessage.warning(`仅支持 ${ACCEPT_EXTS.join(' / ')} 格式`)
      return
    }
    const p = parts.value[i]
    p.uploading = true
    p.progress = null
    try {
      const res = await uploadVideoFile(f, (pr) => (p.progress = pr))
      p.fileId = res.fileId
      if (!p.title) p.title = f.name.slice(0, f.name.lastIndexOf('.')).slice(0, 80)
      ElMessage.success(`分P${i + 1} 上传完成`)
    } catch (err) {
      ElMessage.error(err instanceof ApiError ? err.message : '上传失败，请重试')
    } finally {
      p.uploading = false
    }
  }

  return { parts, addPart, removePart, uploadPart }
}

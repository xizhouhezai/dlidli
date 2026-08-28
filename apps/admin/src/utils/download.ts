// 浏览器端文件保存：配合 adminApi.http.download 使用
import type { DownloadResult } from '@dlidli/api-client'

/** 触发浏览器保存已下载的文件（临时 objectURL 用后即释放） */
export function saveBlob({ blob, filename }: DownloadResult) {
  const a = document.createElement('a')
  a.href = URL.createObjectURL(blob)
  a.download = filename
  a.click()
  URL.revokeObjectURL(a.href)
}

// 视频文件上传器：增量 SHA-256（hash-wasm）→ init（秒传/续传）→ 并发分片 → 合并
import { createSHA256 } from 'hash-wasm'
import { api } from '@/api'

export interface UploadProgress {
  /** hash 计算 / upload 分片上传 / merge 合并校验 */
  stage: 'hash' | 'upload' | 'merge'
  /** 当前阶段进度 0-100 */
  percent: number
}

const HASH_SLICE = 4 * 1024 * 1024
const CONCURRENCY = 3

/** 增量计算文件 SHA-256（避免大文件整体载入内存） */
async function hashFile(file: File, onProgress: (p: UploadProgress) => void): Promise<string> {
  const hasher = await createSHA256()
  hasher.init()
  for (let offset = 0; offset < file.size; offset += HASH_SLICE) {
    const buf = await file.slice(offset, offset + HASH_SLICE).arrayBuffer()
    hasher.update(new Uint8Array(buf))
    onProgress({ stage: 'hash', percent: Math.min(100, Math.round(((offset + HASH_SLICE) / file.size) * 100)) })
  }
  return hasher.digest('hex')
}

/** 上传视频文件，返回可用于投稿的 file_id */
export async function uploadVideoFile(
  file: File,
  onProgress: (p: UploadProgress) => void,
): Promise<{ fileId: string }> {
  const hash = await hashFile(file, onProgress)

  const init = await api.upload.init(file.name, file.size, hash)
  if (init.fast && init.file_id) {
    onProgress({ stage: 'upload', percent: 100 })
    return { fileId: init.file_id }
  }

  const uploadId = init.upload_id!
  const chunkSize = init.chunk_size!
  const chunkCount = init.chunk_count!
  const uploaded = new Set(init.uploaded ?? [])

  let finished = uploaded.size
  let cursor = 0

  const worker = async () => {
    for (;;) {
      const i = cursor++
      if (i >= chunkCount) return
      if (uploaded.has(i)) continue
      const chunk = file.slice(i * chunkSize, Math.min((i + 1) * chunkSize, file.size))
      await api.upload.uploadPart(uploadId, i, chunk)
      finished++
      onProgress({ stage: 'upload', percent: Math.round((finished / chunkCount) * 100) })
    }
  }
  await Promise.all(Array.from({ length: CONCURRENCY }, worker))

  onProgress({ stage: 'merge', percent: 100 })
  const res = await api.upload.complete(uploadId)
  return { fileId: res.file_id }
}

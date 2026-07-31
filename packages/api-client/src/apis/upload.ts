import type { HttpClient } from '../http'

export interface UploadInitResp {
  fast: boolean
  file_id?: string
  store_key?: string
  upload_id?: string
  chunk_size?: number
  chunk_count?: number
  uploaded?: number[]
}

/** 分片上传接口（对应后端 /api/v1/upload）。 */
export function createUploadApi(http: HttpClient) {
  return {
    init: (fileName: string, fileSize: number, fileHash: string) =>
      http.post<UploadInitResp>('/api/v1/upload/init', {
        file_name: fileName,
        file_size: fileSize,
        file_hash: fileHash,
      }),

    uploadPart: (uploadId: string, index: number, chunk: Blob) =>
      http.putRaw<null>(`/api/v1/upload/${uploadId}/parts/${index}`, chunk),

    progress: (uploadId: string) => http.get<UploadInitResp>(`/api/v1/upload/${uploadId}`),

    complete: (uploadId: string) =>
      http.post<{ file_id: string; store_key: string }>(`/api/v1/upload/${uploadId}/complete`),
  }
}

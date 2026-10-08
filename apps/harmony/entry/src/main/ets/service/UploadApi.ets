import { HttpClient } from './HttpClient';
import { UploadCompleteResult, UploadInitResult, UploadProgressResult } from '../model/Upload';

const PATH_INIT: string = '/api/v1/upload/init';

/**
 * 上传域接口（HMY-51）。四个接口**全部需要登录**（后端 `v1.Group("/upload", auth)`），
 * 故一律走带凭证的 HttpClient，401 由 HttpClient 静默续期后重放。
 *
 * 注意分片接口 `PUT /upload/{id}/parts/{index}` **不在此处**：它的 body 是**裸二进制**，
 * 而 HttpClient 固定发 JSON——分片由 `UploadController` 经 `request.uploadFile` 走 A 路线发出
 * （HMY-50 定论：PUT 才发裸 body，POST 会发 multipart 信封被后端原样落盘而报「分片大小不合法」）。
 */
export class UploadApi {
  /** 初始化：秒传命中（fast=true）直接返回可用的 file_id；否则返回会话与已传分片用于续传 */
  static init(fileName: string, fileSize: number, fileHash: string): Promise<UploadInitResult> {
    const body: Record<string, string | number> = {
      'file_name': fileName,
      'file_size': fileSize,
      // 后端 binding 为 required,len=64 且按小写比对，端侧 hash 返回大写 hex，此处统一转小写
      'file_hash': fileHash.toLowerCase()
    };
    return HttpClient.post<UploadInitResult>(PATH_INIT, body);
  }

  /** 查询已传分片（断点续传时校准；正常流程直接用 init 的 uploaded 即可） */
  static progress(uploadId: string): Promise<UploadProgressResult> {
    return HttpClient.get<UploadProgressResult>(`/api/v1/upload/${encodeURIComponent(uploadId)}`);
  }

  /**
   * 收口合并。后端先校验「已传分片数 == chunk_count」并校验合并结果的 SHA-256，
   * 不满足直接返回 30003（ErrUploadIncomplete），故调用方必须补齐全部缺失分片后再调。
   */
  static complete(uploadId: string): Promise<UploadCompleteResult> {
    return HttpClient.post<UploadCompleteResult>(
      `/api/v1/upload/${encodeURIComponent(uploadId)}/complete`, null);
  }
}

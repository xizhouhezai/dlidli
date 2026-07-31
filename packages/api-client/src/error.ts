import { ApiError } from './http'

/** 从异常提取展示文案：ApiError 用其 message，否则用兜底文案。 */
export function apiErrorMessage(err: unknown, fallback = '操作失败'): string {
  return err instanceof ApiError ? err.message : fallback
}

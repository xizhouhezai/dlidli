import { ErrorMessages } from '../common/constants/ErrorMessages';

/**
 * 后端统一响应包裹，对应 server/internal/pkg/response.Body。
 * 业务错误走 HTTP 200 + 非 0 code，仅 401/403/404/429/500 使用 HTTP 状态码。
 */
export interface ApiBody<T> {
  code: number;
  message: string;
  data: T;
  trace_id: string;
}

export class ApiError extends Error {
  readonly code: number;
  readonly traceId: string;
  /** 仅网络类失败（离线/超时/网关异常）可重试，业务错误重试无意义 */
  readonly retryable: boolean;

  constructor(code: number, message: string, traceId: string, retryable: boolean) {
    super(message);
    this.name = 'ApiError';
    this.code = code;
    this.traceId = traceId;
    this.retryable = retryable;
  }

  /** 供 hilog 使用的单行描述 */
  describe(): string {
    return `code=${this.code} retryable=${this.retryable} message=${this.message} trace_id=${this.traceId}`;
  }
}

/** 网络类失败的构造入口：文案由端侧合成码决定，traceId 取服务端响应头（可能为空） */
export function networkError(code: number, traceId: string): ApiError {
  return new ApiError(code, ErrorMessages.text(code, ''), traceId, true);
}

import { http } from '@kit.NetworkKit';
import { BusinessError } from '@kit.BasicServicesKit';
import { ApiBody, ApiError, networkError } from '../model/Api';
import { TokenPair } from '../model/Auth';
import { ApiConfig } from '../common/constants/ApiConfig';
import { ErrCode, ErrorMessages } from '../common/constants/ErrorMessages';
import { TokenStore } from '../store/TokenStore';
import { Logger } from '../common/utils/Logger';

const TAG: string = 'HttpClient';

/** http 模块超时错误码（2300028 operation timeout），其余网络异常统一按「网络不可用」处理 */
const HTTP_TIMEOUT_ERR: number = 2300028;

export interface HttpOptions {
  /** 查询参数，值为空串的项会被跳过 */
  query?: Map<string, string>;
  /** 是否注入 Authorization 并参与 401 续期重放，默认 true；登录/续期类接口传 false */
  auth?: boolean;
  /** 内部使用：已重放过一次，避免续期死循环 */
  replayed?: boolean;
}

function queryString(query?: Map<string, string>): string {
  if (query === undefined || query.size === 0) {
    return '';
  }
  const parts: string[] = [];
  query.forEach((value: string, key: string) => {
    if (value.length > 0) {
      parts.push(`${encodeURIComponent(key)}=${encodeURIComponent(value)}`);
    }
  });
  return parts.length > 0 ? `?${parts.join('&')}` : '';
}

function sleep(ms: number): Promise<void> {
  return new Promise<void>((resolve: () => void) => {
    setTimeout(resolve, ms);
  });
}

/**
 * 统一 HTTP 出口：负责任务边界内的四件事——响应包裹解析、错误码文案、超时与退避重试、401 静默续期重放。
 * 业务错误（4xx/5xx 之外的 code）不重试；仅网络类失败与网关异常可重试，由 ApiError.retryable 标记给 UI。
 */
export class HttpClient {
  private static refreshing: Promise<boolean> | null = null;

  static get<T>(path: string, options?: HttpOptions): Promise<T> {
    return HttpClient.request<T>(http.RequestMethod.GET, path, null, options);
  }

  static post<T>(path: string, body: object | null, options?: HttpOptions): Promise<T> {
    return HttpClient.request<T>(http.RequestMethod.POST, path, body, options);
  }

  static del<T>(path: string, options?: HttpOptions): Promise<T> {
    return HttpClient.request<T>(http.RequestMethod.DELETE, path, null, options);
  }

  /** 不注入凭证、不参与续期重放的请求（登录、续期、登出等自证接口用） */
  static getPublic<T>(path: string, options?: HttpOptions): Promise<T> {
    return HttpClient.request<T>(http.RequestMethod.GET, path, null, HttpClient.publicOptions(options));
  }

  static postPublic<T>(path: string, body: object | null, options?: HttpOptions): Promise<T> {
    return HttpClient.request<T>(http.RequestMethod.POST, path, body, HttpClient.publicOptions(options));
  }

  private static publicOptions(options?: HttpOptions): HttpOptions {
    const merged: HttpOptions = {};
    merged.query = options?.query;
    merged.auth = false;
    return merged;
  }

  private static async request<T>(
    method: http.RequestMethod,
    path: string,
    body: object | null,
    options?: HttpOptions
  ): Promise<T> {
    const opts: HttpOptions = options ?? {};
    try {
      return await HttpClient.sendWithRetry<T>(method, path, body, opts);
    } catch (err) {
      const apiErr: ApiError = err as ApiError;
      // 401 且非自证接口：静默续期后重放一次原请求
      if (apiErr.code !== ErrCode.UNAUTHORIZED || opts.auth === false || opts.replayed === true) {
        throw apiErr;
      }
      const refreshed: boolean = await HttpClient.ensureRefreshed();
      if (!refreshed) {
        await TokenStore.clear();
        throw new ApiError(ErrCode.UNAUTHORIZED, ErrorMessages.text(20005, ''), apiErr.traceId, false);
      }
      const replayed: HttpOptions = { query: opts.query, auth: opts.auth, replayed: true };
      return await HttpClient.sendWithRetry<T>(method, path, body, replayed);
    }
  }

  private static async sendWithRetry<T>(
    method: http.RequestMethod,
    path: string,
    body: object | null,
    options: HttpOptions
  ): Promise<T> {
    let attempt: number = 0;
    while (true) {
      attempt++;
      try {
        return await HttpClient.send<T>(method, path, body, options);
      } catch (err) {
        const apiErr: ApiError = err as ApiError;
        if (!apiErr.retryable || attempt >= ApiConfig.MAX_ATTEMPTS) {
          throw apiErr;
        }
        const delay: number = ApiConfig.RETRY_BASE_DELAY_MS * Math.pow(2, attempt - 1);
        Logger.warn(TAG, `请求失败将重试（第 ${attempt} 次，${delay}ms 后）：${path} ${apiErr.describe()}`);
        await sleep(delay);
      }
    }
  }

  private static async send<T>(
    method: http.RequestMethod,
    path: string,
    body: object | null,
    options: HttpOptions
  ): Promise<T> {
    const url: string = ApiConfig.BASE_URL + path + queryString(options.query);
    const header: Record<string, string> = { 'Content-Type': 'application/json' };
    if (options.auth !== false) {
      const token: string = TokenStore.getAccessToken();
      if (token.length > 0) {
        header['Authorization'] = `Bearer ${token}`;
      }
    }
    const reqOptions: http.HttpRequestOptions = {
      method: method,
      header: header,
      connectTimeout: ApiConfig.CONNECT_TIMEOUT_MS,
      readTimeout: ApiConfig.READ_TIMEOUT_MS,
      expectDataType: http.HttpDataType.STRING
    };
    if (body !== null) {
      reqOptions.extraData = JSON.stringify(body);
    }

    const req: http.HttpRequest = http.createHttp();
    try {
      const resp: http.HttpResponse = await req.request(url, reqOptions);
      return HttpClient.parse<T>(resp, path);
    } catch (err) {
      // parse() 抛出的是已定级的业务错误，必须原样上抛：若在此重写为网络失败，
      // 401 会被降级成「网络不可用」并触发无意义重试，续期分支永远走不到。
      if (err instanceof ApiError) {
        throw err;
      }
      const be: BusinessError = err as BusinessError;
      const code: number = be.code === HTTP_TIMEOUT_ERR ? ErrCode.NET_TIMEOUT : ErrCode.NET_OFFLINE;
      Logger.warn(TAG, `请求未送达：${path} errCode=${be.code} -> ${code}`);
      throw networkError(code, '');
    } finally {
      req.destroy();
    }
  }

  private static parse<T>(resp: http.HttpResponse, path: string): T {
    const status: number = resp.responseCode;
    const text: string = typeof resp.result === 'string' ? resp.result as string : '';

    let body: ApiBody<T> | null = null;
    try {
      body = JSON.parse(text) as ApiBody<T>;
    } catch (err) {
      body = null;
    }

    if (body === null || typeof body.code !== 'number') {
      // 非契约响应：网关/代理错误页，或服务未按统一包裹返回
      Logger.error(TAG, `响应不符合统一包裹：${path} status=${status}`);
      throw networkError(ErrCode.BAD_RESPONSE, '');
    }

    if (body.code !== 0) {
      // 5xx 为服务端瞬时故障，可重试；4xx 与业务码重试无意义
      const retryable: boolean = status >= 500;
      const message: string = ErrorMessages.text(body.code, body.message ?? '');
      Logger.warn(TAG, `业务失败：${path} status=${status} code=${body.code} trace_id=${body.trace_id}`);
      throw new ApiError(body.code, message, body.trace_id ?? '', retryable);
    }

    return body.data;
  }

  /**
   * 单一飞行续期：并发 401 只触发一次 /auth/refresh，其余请求复用同一 Promise。
   * 续期成功后 TokenStore 内已是新令牌，被挂起的请求各自重放。
   */
  private static ensureRefreshed(): Promise<boolean> {
    if (HttpClient.refreshing === null) {
      HttpClient.refreshing = HttpClient.doRefresh()
        .then((ok: boolean) => {
          HttpClient.refreshing = null;
          return ok;
        })
        .catch((err: Object) => {
          HttpClient.refreshing = null;
          Logger.warn(TAG, `续期异常：${JSON.stringify(err)}`);
          return false;
        });
    }
    return HttpClient.refreshing;
  }

  private static async doRefresh(): Promise<boolean> {
    const refreshToken: string = TokenStore.getRefreshToken();
    if (refreshToken.length === 0) {
      Logger.warn(TAG, '未持有 refresh_token，无法续期');
      return false;
    }
    const payload: Record<string, string> = { 'refresh_token': refreshToken };
    const pair: TokenPair = await HttpClient.postPublic<TokenPair>(ApiConfig.REFRESH_PATH, payload);
    if (pair.access_token.length === 0) {
      return false;
    }
    await TokenStore.save(pair.access_token, pair.refresh_token);
    Logger.info(TAG, '访问令牌已静默续期');
    return true;
  }
}

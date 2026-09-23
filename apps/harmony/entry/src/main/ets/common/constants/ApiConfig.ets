/**
 * 接口层配置：宿主地址与超时/重试策略集中在此，联调时只改这一处。
 *
 * 模拟器为 QEMU 用户态网络，宿主 loopback 以 10.0.2.2 暴露；真机联调改为宿主局域网 IP。
 */
export class ApiConfig {
  static readonly BASE_URL: string = 'http://10.0.2.2:8000';
  static readonly API_PREFIX: string = '/api/v1';

  /**
   * H5 站点地址（站外分享链接用，SHR-02 / HMY-32）。
   * 与 BASE_URL 分开是因为二者来源不同域是常态：本地联调时后端在 10.0.2.2:8000，
   * 而 Web 端 dev server 在宿主 5173；生产则各自换成正式域名。
   */
  static readonly WEB_BASE_URL: string = 'http://10.0.2.2:5173';

  static readonly CONNECT_TIMEOUT_MS: number = 5000;
  static readonly READ_TIMEOUT_MS: number = 10000;

  /** 总尝试次数（首次 + 重试），仅对网络类失败生效，业务错误不重试 */
  static readonly MAX_ATTEMPTS: number = 3;
  /** 指数退避基数：第 n 次重试等待 RETRY_BASE_DELAY_MS * 2^(n-1) */
  static readonly RETRY_BASE_DELAY_MS: number = 300;

  static readonly REFRESH_PATH: string = '/api/v1/auth/refresh';
  static readonly LOGOUT_PATH: string = '/api/v1/auth/logout';
}

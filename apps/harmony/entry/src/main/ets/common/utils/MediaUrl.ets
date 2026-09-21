import { ApiConfig } from '../constants/ApiConfig';

/** 后端本地存储驱动（storage.baseUrl）返回的是宿主机 loopback 绝对地址，模拟器/真机访问不到 */
const LOOPBACK_ORIGIN: RegExp = /^https?:\/\/(localhost|127\.0\.0\.1)(:\d+)?/;

/** 封面、头像等静态资源地址归一：loopback 源替换为 ApiConfig.BASE_URL */
export function resolveMediaUrl(url: string): string {
  if (url.length === 0) {
    return '';
  }
  return url.replace(LOOPBACK_ORIGIN, ApiConfig.BASE_URL);
}

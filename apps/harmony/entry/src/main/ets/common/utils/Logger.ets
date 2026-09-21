import { hilog } from '@kit.PerformanceAnalysisKit';

const DOMAIN: number = 0xD11D;

/** hilog 封装：统一 domain/tag，避免日志散落各处以字符串硬编码 */
export class Logger {
  static debug(tag: string, message: string): void {
    hilog.debug(DOMAIN, tag, '%{public}s', message);
  }

  static info(tag: string, message: string): void {
    hilog.info(DOMAIN, tag, '%{public}s', message);
  }

  static warn(tag: string, message: string): void {
    hilog.warn(DOMAIN, tag, '%{public}s', message);
  }

  static error(tag: string, message: string): void {
    hilog.error(DOMAIN, tag, '%{public}s', message);
  }
}

import { preferences } from '@kit.ArkData';
import { common } from '@kit.AbilityKit';
import { Logger } from '../common/utils/Logger';

const TAG: string = 'TokenStore';
const STORE_NAME: string = 'dlidli_prefs';
const KEY_ACCESS: string = 'auth.access_token';
const KEY_REFRESH: string = 'auth.refresh_token';

/**
 * 令牌偏好存储。key 命名与 plan §3 数据模型一致，与 M4-HMY-04 的登录流程共用同一份存储。
 *
 * 内存镜像供请求时同步取用，避免每次请求都做一次异步读盘；启动时 restore() 回填。
 * 偏好存储异常一律降级为「仅内存生效」并记录日志——存储不可用不应让请求链路抛错，
 * 更不能在其上叠加出与真实错误无关的异常。
 */
export class TokenStore {
  private static store: preferences.Preferences | null = null;
  private static context: common.Context | null = null;
  private static accessToken: string = '';
  private static refreshToken: string = '';
  private static restored: boolean = false;

  static init(context: common.Context): void {
    TokenStore.context = context;
  }

  private static async open(): Promise<preferences.Preferences | null> {
    if (TokenStore.store !== null) {
      return TokenStore.store;
    }
    const ctx: common.Context | null = TokenStore.context;
    if (ctx === null) {
      Logger.error(TAG, '未初始化，需在 EntryAbility.onCreate 注入 Context');
      return null;
    }
    try {
      TokenStore.store = await preferences.getPreferences(ctx, STORE_NAME);
      return TokenStore.store;
    } catch (err) {
      Logger.error(TAG, `打开偏好存储失败：${JSON.stringify(err)}`);
      return null;
    }
  }

  /** 启动时回填内存镜像；读盘失败不阻断启动，按未登录处理 */
  static async restore(): Promise<void> {
    if (TokenStore.restored) {
      return;
    }
    TokenStore.restored = true;
    const store: preferences.Preferences | null = await TokenStore.open();
    if (store === null) {
      return;
    }
    try {
      TokenStore.accessToken = await store.get(KEY_ACCESS, '') as string;
      TokenStore.refreshToken = await store.get(KEY_REFRESH, '') as string;
    } catch (err) {
      Logger.error(TAG, `读取凭证失败：${JSON.stringify(err)}`);
      return;
    }
    Logger.info(TAG, `凭证已恢复，登录态=${TokenStore.accessToken.length > 0}`);
  }

  /** 等待回填完成：restore() 在 onCreate 异步发起，读登录态前先对齐，避免读到尚未回填的空凭证 */
  static async ready(): Promise<void> {
    await TokenStore.restore();
  }

  static getAccessToken(): string {
    return TokenStore.accessToken;
  }

  static getRefreshToken(): string {
    return TokenStore.refreshToken;
  }

  static isLoggedIn(): boolean {
    return TokenStore.accessToken.length > 0;
  }

  static async save(accessToken: string, refreshToken: string): Promise<void> {
    TokenStore.accessToken = accessToken;
    TokenStore.refreshToken = refreshToken;
    const store: preferences.Preferences | null = await TokenStore.open();
    if (store === null) {
      return;
    }
    try {
      await store.put(KEY_ACCESS, accessToken);
      await store.put(KEY_REFRESH, refreshToken);
      await store.flush();
      Logger.info(TAG, '凭证已写入偏好存储');
    } catch (err) {
      Logger.error(TAG, `凭证持久化失败（仅内存生效）：${JSON.stringify(err)}`);
    }
  }

  /** 退出登录与续期失败时调用，内存与磁盘一并清理 */
  static async clear(): Promise<void> {
    TokenStore.accessToken = '';
    TokenStore.refreshToken = '';
    const store: preferences.Preferences | null = await TokenStore.open();
    if (store === null) {
      return;
    }
    try {
      await store.delete(KEY_ACCESS);
      await store.delete(KEY_REFRESH);
      await store.flush();
      Logger.info(TAG, '凭证已清理');
    } catch (err) {
      Logger.error(TAG, `凭证清理失败（内存已清）：${JSON.stringify(err)}`);
    }
  }
}

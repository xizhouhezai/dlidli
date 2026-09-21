import { preferences } from '@kit.ArkData';
import { common } from '@kit.AbilityKit';
import { Logger } from '../common/utils/Logger';

const TAG: string = 'PreferenceStore';
const STORE_NAME: string = 'dlidli_prefs';

/**
 * 偏好存储的统一打开入口（令牌、弹幕设置、搜索历史共用一个 preferences 文件，见 plan §3）。
 * Context 由 EntryAbility.onCreate 注入；打开失败一律降级为「仅内存生效」并记日志——
 * 存储不可用不应让领域层抛错，更不能在其上叠加出与真实错误无关的异常。
 */
export class PreferenceStore {
  private static store: preferences.Preferences | null = null;
  private static context: common.Context | null = null;

  static init(context: common.Context): void {
    PreferenceStore.context = context;
  }

  static async open(): Promise<preferences.Preferences | null> {
    if (PreferenceStore.store !== null) {
      return PreferenceStore.store;
    }
    const ctx: common.Context | null = PreferenceStore.context;
    if (ctx === null) {
      Logger.error(TAG, '未初始化，需在 EntryAbility.onCreate 注入 Context');
      return null;
    }
    try {
      PreferenceStore.store = await preferences.getPreferences(ctx, STORE_NAME);
      return PreferenceStore.store;
    } catch (err) {
      Logger.error(TAG, `打开偏好存储失败：${JSON.stringify(err)}`);
      return null;
    }
  }
}

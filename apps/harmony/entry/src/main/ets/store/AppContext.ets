import { common } from '@kit.AbilityKit';
import { Logger } from '../common/utils/Logger';

const TAG: string = 'AppContext';

/**
 * UIAbilityContext 的单点持有：`request.uploadFile` 要 BaseContext、剩余空间校验要 `cacheDir`，
 * 二者都要一个 Context，而上传控制器由页面构造、页面又拿不到 Ability 实例。
 * 由 EntryAbility.onCreate 注入一次，避免把 Context 沿页面层级逐层传参
 * （与 PreferenceStore.init 同一模式，见 EntryAbility）。
 */
export class AppContext {
  private static ctx: common.UIAbilityContext | null = null;

  static init(context: common.UIAbilityContext): void {
    AppContext.ctx = context;
  }

  /** Context 未注入属编程错误（上传必须发生在 UI 生命周期内），此处直接抛，不做静默降级 */
  static require(): common.UIAbilityContext {
    const ctx: common.UIAbilityContext | null = AppContext.ctx;
    if (ctx === null) {
      Logger.error(TAG, 'Context 未注入，需在 EntryAbility.onCreate 调用 AppContext.init');
      throw new Error('应用上下文未就绪');
    }
    return ctx;
  }
}

import { Logger } from './Logger';

const TAG: string = 'AppRouter';

/** 路由名集中在此，避免各处硬编码字符串 */
export class RouteName {
  static readonly LOGIN: string = 'login';
}

/**
 * 壳层 Navigation 的路由栈代理：Index.ets 启动时绑定，
 * 子页（如「我的」）无需持有壳层即可跳转，避免把 NavPathStack 逐层传递。
 */
export class AppRouter {
  private static stack: NavPathStack | null = null;

  static bind(stack: NavPathStack): void {
    AppRouter.stack = stack;
  }

  static push(name: string): void {
    const stack: NavPathStack | null = AppRouter.stack;
    if (stack === null) {
      Logger.warn(TAG, `路由栈未绑定，忽略跳转：${name}`);
      return;
    }
    stack.pushPath({ name: name });
  }

  static pop(): void {
    const stack: NavPathStack | null = AppRouter.stack;
    if (stack !== null) {
      stack.pop();
    }
  }
}

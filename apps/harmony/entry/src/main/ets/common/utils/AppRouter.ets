import { Logger } from './Logger';

const TAG: string = 'AppRouter';

/** 路由名集中在此，避免各处硬编码字符串 */
export class RouteName {
  static readonly LOGIN: string = 'login';
  static readonly PLAY: string = 'play';
}

/** 播放页路由参数：bvid 是播放页唯一的入参（Index.ets 的 destination 分支按此解包） */
export interface PlayRouteParam {
  bvid: string;
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

  /** param 经 NavPathInfo 透传给压栈页的 @Param（如播放页的 bvid） */
  static push(name: string, param?: Object): void {
    const stack: NavPathStack | null = AppRouter.stack;
    if (stack === null) {
      Logger.warn(TAG, `路由栈未绑定，忽略跳转：${name}`);
      return;
    }
    stack.pushPath({ name: name, param: param });
  }

  static pop(): void {
    const stack: NavPathStack | null = AppRouter.stack;
    if (stack !== null) {
      stack.pop();
    }
  }
}

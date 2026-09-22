/**
 * 应用级前后台事件（EntryAbility 发、页面订阅）。
 * 用 emitter 而非全局状态：播放页是压栈的 NavDestination，拿不到 @Entry 页的生命周期回调，
 * 切后台必须靠 Ability 侧广播出来。
 */
export class AppEvent {
  static readonly ID_FOREGROUND: number = 1001;
  static readonly ID_BACKGROUND: number = 1002;
}

import { ApiError } from '../model/Api';
import { Profile, TokenPair } from '../model/Auth';
import { AuthApi } from '../service/AuthApi';
import { ApiConfig } from '../common/constants/ApiConfig';
import { ErrCode } from '../common/constants/ErrorMessages';
import { TokenStore } from './TokenStore';
import { Logger } from '../common/utils/Logger';

const TAG: string = 'Session';

/** 后端本地存储驱动返回的绝对地址指向宿主机 loopback，模拟器/真机访问不到，需换成宿主地址 */
const LOOPBACK_ORIGIN = /^https?:\/\/(localhost|127\.0\.0\.1)(:\d+)?/;

/**
 * 会话状态（跨页共享，故为 @ObservedV2 单例）：令牌实体在 TokenStore，这里维护可供 UI 观察的登录态。
 * 页面读 `session.loggedIn/nickname/avatar` 即可，无需自己拼装请求。
 */
@ObservedV2
export class Session {
  @Trace loggedIn: boolean = false;
  @Trace nickname: string = '';
  @Trace avatar: string = '';
  @Trace level: number = 0;

  /** 登录成功：落凭证 + 填充本地身份 */
  async onLoggedIn(pair: TokenPair): Promise<void> {
    await TokenStore.save(pair.access_token, pair.refresh_token);
    this.applyProfile(pair.user);
    Logger.info(TAG, `登录成功，uid=${pair.user.id} level=${pair.user.level}`);
  }

  /**
   * 启动/切回「我的」时校正会话：凭证在则拉一次资料确认可用。
   * 401 说明凭证已不可用（HttpClient 续期失败时已清 TokenStore），同步清本地态；
   * 网络类失败不动登录态——令牌还在，只是这次没拿到资料。
   */
  async hydrate(): Promise<void> {
    await TokenStore.ready();
    this.loggedIn = TokenStore.isLoggedIn();
    if (!this.loggedIn) {
      this.reset();
      return;
    }
    try {
      this.applyProfile(await AuthApi.me());
    } catch (err) {
      const apiErr: ApiError = err as ApiError;
      Logger.warn(TAG, `拉取用户资料失败：${apiErr.describe()}`);
      if (apiErr.code === ErrCode.UNAUTHORIZED) {
        await TokenStore.clear();
        this.reset();
      }
    }
  }

  /** 主动退出：先吊销服务端刷新令牌（失败不阻塞），再清本地凭证与身份 */
  async logout(): Promise<void> {
    const refreshToken: string = TokenStore.getRefreshToken();
    if (refreshToken.length > 0) {
      try {
        await AuthApi.logout(refreshToken);
      } catch (err) {
        const apiErr: ApiError = err as ApiError;
        Logger.warn(TAG, `吊销刷新令牌失败，仍清本地凭证：${apiErr.describe()}`);
      }
    }
    await TokenStore.clear();
    this.reset();
    Logger.info(TAG, '已退出登录');
  }

  private applyProfile(profile: Profile): void {
    this.loggedIn = true;
    this.nickname = profile.nickname;
    this.avatar = Session.resolveAvatar(profile.avatar);
    this.level = profile.level;
  }

  private reset(): void {
    this.loggedIn = false;
    this.nickname = '';
    this.avatar = '';
    this.level = 0;
  }

  private static resolveAvatar(avatar: string): string {
    if (avatar.length === 0) {
      return '';
    }
    return avatar.replace(LOOPBACK_ORIGIN, ApiConfig.BASE_URL);
  }
}

/** 模块级单例：@ObservedV2 实例不可在 build 内创建，须由外部持有 */
export const session: Session = new Session();

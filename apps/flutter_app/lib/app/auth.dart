/// 鉴权状态：登录 / 登出 / 启动恢复（FLT-01、FLT-02）。
///
/// 关键行为：
/// - 登录成功**立即**把令牌写进安全存储（不是"先返回再异步存"），
///   否则用户马上杀进程会丢登录态。
/// - 启动时读安全存储：有 token 就拉 `/users/me` 校验有效性 ——
///   **不能只看本地有没有 token 就当已登录**（令牌可能已过期/被吊销）。
/// - 登出：先调后端吊销刷新令牌，再清本地（顺序反了会让令牌在服务端残留）。
/// - 登出接口失败也**必须**清本地：否则用户"退不出去"。
library;

import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../core/api_error.dart';
import '../../core/token_store.dart';
import '../../data/api/auth_api.dart';
import '../../data/models/account.dart';
import 'providers.dart';

/// 鉴权状态。
sealed class AuthState {
  const AuthState();
}

class AuthUnknown extends AuthState {
  const AuthUnknown();
}

class AuthLoggedOut extends AuthState {
  const AuthLoggedOut({this.reason});
  final String? reason;
}

class AuthLoggedIn extends AuthState {
  const AuthLoggedIn({required this.profile, required this.expiresIn});
  final Profile profile;
  final int expiresIn;
}

class AuthNotifier extends StateNotifier<AuthState> {
  AuthNotifier({
    required AuthApi api,
    required TokenStore tokenStore,
  })  : _api = api,
        _tokenStore = tokenStore,
        super(const AuthUnknown()) {
    restore();
  }

  final AuthApi _api;
  final TokenStore _tokenStore;

  /// 启动恢复：本地有令牌就去后端校验，避免"假登录态"。
  Future<void> restore() async {
    final access = await _tokenStore.readAccessToken();
    if (access == null || access.isEmpty) {
      state = const AuthLoggedOut();
      return;
    }
    try {
      final profile = await _api.me();
      state = AuthLoggedIn(profile: profile, expiresIn: 0);
    } on ApiError catch (e) {
      if (e.needsLogin) {
        // 令牌确实失效：清理并回到未登录（ApiClient 已在 401 续期失败时清过）
        await _tokenStore.clear();
        state = const AuthLoggedOut(reason: '登录已过期，请重新登录');
      } else {
        // 网络原因：**不清理令牌**，否则一次断网就把用户踢下线
        state = const AuthLoggedOut(reason: '网络异常，无法校验登录态');
      }
    }
  }

  /// 验证码登录。
  Future<void> loginBySms({
    required String phone,
    required String code,
    String? inviteCode,
  }) async {
    final pair = await _api.loginBySms(phone: phone, code: code, inviteCode: inviteCode);
    await _persist(pair);
  }

  /// 密码登录（需图形验证码）。
  Future<void> loginByPassword({
    required String account,
    required String password,
    required String captchaId,
    required String captchaCode,
  }) async {
    final pair = await _api.loginByPassword(
      account: account,
      password: password,
      captchaId: captchaId,
      captchaCode: captchaCode,
    );
    await _persist(pair);
  }

  /// 登出：服务端吊销 + 本地清理。
  Future<void> logout() async {
    final refresh = await _tokenStore.readRefreshToken();
    if (refresh != null && refresh.isNotEmpty) {
      try {
        await _api.logout(refresh);
      } catch (_) {
        // 服务端吊销失败不阻断退出，本地仍必须清理
      }
    }
    await _tokenStore.clear();
    state = const AuthLoggedOut();
  }

  /// 令牌落库 + 状态切换。**先存后切**：切了状态但存储失败会导致
  /// "界面显示已登录、重启后掉线"，比直接报错更难排查。
  Future<void> _persist(TokenPair pair) async {
    if (pair.accessToken.isEmpty || pair.refreshToken.isEmpty) {
      throw ApiError(
        kind: ApiErrorKind.malformed,
        message: '登录响应缺少令牌',
      );
    }
    await _tokenStore.save(
      accessToken: pair.accessToken,
      refreshToken: pair.refreshToken,
    );
    profileCache = pair.user ?? await _api.me();
    state = AuthLoggedIn(profile: profileCache!, expiresIn: pair.expiresIn);
  }

  /// 最近一次成功登录/校验拿到的资料（供 UI 免二次请求）。
  Profile? profileCache;
}

final authApiProvider = Provider<AuthApi>((ref) => AuthApi(ref.watch(apiClientProvider)));

final authProvider = StateNotifierProvider<AuthNotifier, AuthState>((ref) {
  return AuthNotifier(
    api: ref.watch(authApiProvider),
    tokenStore: ref.watch(tokenStoreProvider),
  );
});

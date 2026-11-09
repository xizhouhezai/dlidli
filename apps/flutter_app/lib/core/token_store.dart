/// 令牌存储抽象：让网络层不依赖具体存储实现，测试可注入内存实现。
///
/// 实现要求（FLT-02）：走平台安全存储（Android Keystore / iOS Keychain），
/// **不得**明文写入日志或普通缓存文件。
library;

abstract class TokenStore {
  Future<String?> readAccessToken();
  Future<String?> readRefreshToken();
  Future<void> save({
    required String accessToken,
    required String refreshToken,
  });
  Future<void> clear();
}

/// 内存实现：仅用于单元测试与「未登录态」演示，**不得**用于真实运行。
class InMemoryTokenStore implements TokenStore {
  String? _access;
  String? _refresh;

  @override
  Future<String?> readAccessToken() async => _access;

  @override
  Future<String?> readRefreshToken() async => _refresh;

  @override
  Future<void> save({
    required String accessToken,
    required String refreshToken,
  }) async {
    _access = accessToken;
    _refresh = refreshToken;
  }

  @override
  Future<void> clear() async {
    _access = null;
    _refresh = null;
  }
}

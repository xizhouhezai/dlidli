/// 安全存储实现：Android Keystore / iOS Keychain（FLT-02）。
///
/// **为什么不用 shared_preferences**：那是明文 plist/xml，令牌等同于可被随意读取；
/// `flutter_secure_storage` 在 Android 走 EncryptedSharedPreferences + Keystore，
/// 在 iOS 走 Keychain，满足「不明文落盘」的要求。
library;

import 'package:flutter_secure_storage/flutter_secure_storage.dart';

import 'token_store.dart';

class SecureTokenStore implements TokenStore {
  SecureTokenStore({FlutterSecureStorage? storage})
    : _storage =
          storage ??
          const FlutterSecureStorage(
            aOptions: AndroidOptions(encryptedSharedPreferences: true),
            iOptions: IOSOptions(
              accessibility: KeychainAccessibility.first_unlock,
            ),
          );

  final FlutterSecureStorage _storage;

  static const _kAccess = 'dlidli.access_token';
  static const _kRefresh = 'dlidli.refresh_token';

  @override
  Future<String?> readAccessToken() => _storage.read(key: _kAccess);

  @override
  Future<String?> readRefreshToken() => _storage.read(key: _kRefresh);

  @override
  Future<void> save({
    required String accessToken,
    required String refreshToken,
  }) async {
    await _storage.write(key: _kAccess, value: accessToken);
    await _storage.write(key: _kRefresh, value: refreshToken);
  }

  @override
  Future<void> clear() async {
    await _storage.delete(key: _kAccess);
    await _storage.delete(key: _kRefresh);
  }
}

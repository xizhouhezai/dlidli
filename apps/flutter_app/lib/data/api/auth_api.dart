/// 账号鉴权接口（复用既有后端契约，无需新增接口）。
///
/// 契约要点（实测确认，见 docs/specs/flutter/plan.md）：
/// - 验证码登录：`POST /auth/login/sms` `{phone, code, invite_code?}`
/// - 密码登录：`POST /auth/login/password` `{account, password, captcha_id, captcha_code}`（**必须带图形验证码**）
/// - 发码：`POST /auth/sms-code` `{phone}`；**dev 环境返回 `debug_code`**（生产不回）
/// - 续期：`POST /auth/refresh` `{refresh_token}`
/// - 登出：`POST /auth/logout` `{refresh_token}`（吊销刷新令牌）
/// - 响应统一 `TokenPair{access_token, refresh_token, expires_in, user}`
library;

import '../../core/api_client.dart';
import '../models/account.dart';

class AuthApi {
  AuthApi(this._client);

  final ApiClient _client;

  /// 发送短信验证码。dev 环境会返回 `debugCode`（生产为 null）。
  Future<String?> sendSmsCode(String phone) async {
    final data = await _client.post('/api/v1/auth/sms-code', body: {'phone': phone});
    if (data is Map<String, dynamic>) return data['debug_code'] as String?;
    return null;
  }

  /// 验证码登录。
  Future<TokenPair> loginBySms({
    required String phone,
    required String code,
    String? inviteCode,
  }) async {
    final data = await _client.post('/api/v1/auth/login/sms', body: {
      'phone': phone,
      'code': code,
      if (inviteCode != null && inviteCode.isNotEmpty) 'invite_code': inviteCode,
    });
    return TokenPair.fromJson(_asMap(data));
  }

  /// 密码登录（需图形验证码）。
  Future<TokenPair> loginByPassword({
    required String account,
    required String password,
    required String captchaId,
    required String captchaCode,
  }) async {
    final data = await _client.post('/api/v1/auth/login/password', body: {
      'account': account,
      'password': password,
      'captcha_id': captchaId,
      'captcha_code': captchaCode,
    });
    return TokenPair.fromJson(_asMap(data));
  }

  /// 图形验证码：后端返回 `{id, svg}`，SVG 字符串由端侧渲染（不是图片 URL）。
  Future<Captcha> captcha() async {
    final data = await _client.get('/api/v1/auth/captcha');
    return Captcha.fromJson(_asMap(data));
  }

  /// 当前用户资料。
  Future<Profile> me() async {
    final data = await _client.get('/api/v1/users/me');
    return Profile.fromJson(_asMap(data));
  }

  /// 登出：吊销刷新令牌（失败不阻断本地清理）。
  Future<void> logout(String refreshToken) async {
    await _client.post('/api/v1/auth/logout', body: {'refresh_token': refreshToken});
  }

  Map<String, dynamic> _asMap(Object? data) =>
      data is Map<String, dynamic> ? data : <String, dynamic>{};
}

/// 账号模型：与后端 TokenPair / Profile 契约对齐。
library;

/// 登录成功返回的令牌对。
class TokenPair {
  const TokenPair({
    required this.accessToken,
    required this.refreshToken,
    required this.expiresIn,
    this.user,
  });

  final String accessToken;
  final String refreshToken;

  /// access token 有效期（秒）。后端实测 7200。
  final int expiresIn;

  final Profile? user;

  factory TokenPair.fromJson(Map<String, dynamic> json) => TokenPair(
        accessToken: (json['access_token'] as String?) ?? '',
        refreshToken: (json['refresh_token'] as String?) ?? '',
        expiresIn: (json['expires_in'] as num?)?.toInt() ?? 0,
        user: json['user'] is Map<String, dynamic>
            ? Profile.fromJson(json['user'] as Map<String, dynamic>)
            : null,
      );
}

/// 用户资料。
class Profile {
  const Profile({
    this.id,
    this.nickname = '',
    this.avatar = '',
    this.signature = '',
    this.level = 0,
    this.isVip = false,
  });

  final String? id;
  final String nickname;
  final String avatar;
  final String signature;
  final int level;
  final bool isVip;

  factory Profile.fromJson(Map<String, dynamic> json) => Profile(
        id: json['id']?.toString(),
        nickname: (json['nickname'] as String?) ?? '',
        avatar: (json['avatar'] as String?) ?? '',
        signature: (json['signature'] as String?) ?? '',
        level: (json['level'] as num?)?.toInt() ?? 0,
        isVip: json['is_vip'] == true,
      );
}

/// 图形验证码：后端给的是 **SVG 字符串**（不是图片 URL）。
class Captcha {
  const Captcha({required this.id, required this.svg});

  final String id;
  final String svg;

  factory Captcha.fromJson(Map<String, dynamic> json) => Captcha(
        id: (json['id'] as String?) ?? '',
        svg: (json['svg'] as String?) ?? '',
      );
}

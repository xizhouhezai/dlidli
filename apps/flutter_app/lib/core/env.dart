/// 端侧运行环境与后端基址。
///
/// **为什么需要平台区分**（FLT-03「端侧差异」）：
/// Android 模拟器（含 MuMu/AVD）里 `localhost` 指向**设备自身**，访问宿主机后端必须用
/// `10.0.2.2` 这个特殊映射地址；iOS 模拟器则是真·共享宿主网络，可直接 `localhost`。
/// 真机两者都不成立，必须走局域网 IP 或线上域名，故支持 `--dart-define=DLIDLI_API_BASE=` 覆盖。
///
/// 用法：
///   flutter run --dart-define=DLIDLI_API_BASE=http://192.168.1.10:8000
library;

import 'dart:io' show Platform;

import 'package:flutter/foundation.dart';

enum AppFlavor { dev, staging, prod }

class Env {
  Env._();

  /// 编译期注入的后端基址（真机/CI 用）。
  static const String _injectedBase = String.fromEnvironment('DLIDLI_API_BASE');

  /// 编译期注入的环境（dev/staging/prod）。
  static const String _injectedFlavor = String.fromEnvironment(
    'DLIDLI_FLAVOR',
    defaultValue: 'dev',
  );

  static AppFlavor get flavor {
    switch (_injectedFlavor) {
      case 'prod':
        return AppFlavor.prod;
      case 'staging':
        return AppFlavor.staging;
      default:
        return AppFlavor.dev;
    }
  }

  /// 后端基址（不带尾部斜杠）。
  ///
  /// 优先级：`--dart-define` 注入 > 平台默认（模拟器口径）。
  static String get apiBase {
    if (_injectedBase.isNotEmpty) return _stripTrailingSlash(_injectedBase);
    if (kIsWeb) return 'http://localhost:8000';
    // Android（含模拟器）：10.0.2.2 是宿主回环映射；MuMu 同样适用。
    if (Platform.isAndroid) return 'http://10.0.2.2:8000';
    // iOS / macOS 模拟器与宿主共享网络栈。
    return 'http://localhost:8000';
  }

  static String _stripTrailingSlash(String v) =>
      v.endsWith('/') ? v.substring(0, v.length - 1) : v;

  /// 是否输出网络日志（dev 才打，避免生产泄露 token/参数）。
  static bool get verboseNetworkLog => flavor == AppFlavor.dev;
}

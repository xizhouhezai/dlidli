/// DliDli Flutter 移动端（Android + iOS）入口。
///
/// 阶段：M5 一期 T-01 骨架 / T-02 网络层 / T-03 登录与会话。
/// 规格见 docs/specs/flutter/；本文件只做装配，业务在 features/ 下。
library;

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'app/root_shell.dart';
import 'core/env.dart';
import 'core/theme.dart';

void main() {
  runApp(const ProviderScope(child: DliDliApp()));
}

class DliDliApp extends StatelessWidget {
  const DliDliApp({super.key});

  @override
  Widget build(BuildContext context) {
    return MaterialApp(
      title: 'DliDli',
      debugShowCheckedModeBanner: Env.flavor != AppFlavor.prod,
      // 主题 token 与鸿蒙端 Theme.ets 逐值对齐（见 core/theme.dart）
      theme: DliTheme.light(),
      // FLT-05：鸿蒙「沉浸光感」在 Android/iOS 无等价能力，
      // 这里用普通背景 + 卡片层级降级，不做材质模拟。
      home: const RootShell(),
    );
  }
}

/// 品牌与设计 token：**与鸿蒙端 `common/constants/Theme.ets` 逐值对齐**。
///
/// 为什么要照搬而不是自定：本项目 Web/H5/Admin/鸿蒙四端的品牌 token 同源
///（鸿蒙侧注释亦写明"与 Web 端 _variables.scss 同源"），Flutter 若要成为第五端并保持
/// 视觉一致，token 必须是同一套数值，否则四端切换会有色差与间距跳动。
///
/// 对齐来源：`apps/harmony/entry/src/main/ets/common/constants/Theme.ets`（2026-11-05 核对）。
/// 单位换算：鸿蒙用 vp，Flutter 用逻辑像素 dp → **1:1 直接映射**（两者都是密度无关单位）。
library;

import 'package:flutter/material.dart';

/// 品牌色与中性色阶（逐值对齐 DliColor）。
class DliColors {
  DliColors._();

  static const Color primary = Color(0xFFFB7299);
  static const Color primaryLight = Color(0xFFFFD9E4);
  static const Color primaryHover = Color(0xFFFC8BAB);
  static const Color onPrimary = Color(0xFFFFFFFF);

  static const Color text1 = Color(0xFF18191C);
  static const Color text2 = Color(0xFF61666D);
  static const Color text3 = Color(0xFF9499A0);

  static const Color border = Color(0xFFE3E5E7);
  static const Color bgPage = Color(0xFFF6F7F8);
  static const Color bgSurface = Color(0xFFFFFFFF);

  /// 半透明遮罩（对齐 MASK: '#CC000000'）。
  static const Color mask = Color(0xCC000000);

  /// 时长角标底色（鸿蒙卡片用 '#99000000'）。
  static const Color durationBadge = Color(0x99000000);

  /// 次级表面：鸿蒙材质方案的卡片地基色（surface_secondary，取中性灰而非品牌粉）。
  /// 鸿蒙侧明确要求「赋色取中性、不取品牌色」，避免玻璃与品牌粉互相稀释。
  static const Color surfaceSecondary = Color(0xFFF2F3F5);
}

/// 圆角（逐值对齐 DliRadius）。
class DliRadius {
  DliRadius._();
  static const double sm = 6;
  static const double md = 8;
  static const double lg = 12;
  static const double full = 999;
}

/// 间距（逐值对齐 DliSpace）。
class DliSpace {
  DliSpace._();
  static const double xs = 4;
  static const double sm = 8;
  static const double md = 12;
  static const double lg = 16;
  static const double xl = 24;

  /// 页面左右留白（鸿蒙 PAGE = 12）。
  static const double page = 12;
}

/// 字号（逐值对齐 DliFont）。
class DliFont {
  DliFont._();
  static const double caption = 11;
  static const double bodySm = 13;
  static const double body = 15;
  static const double titleSm = 17;
  static const double title = 20;
  static const double tabLabel = 12;
}

/// 尺寸（逐值对齐 DliSize）。
class DliSize {
  DliSize._();

  static const double tabIcon = 24;
  static const double titleBarHeight = 56;

  /// 底栏悬浮胶囊与页面底部的间距（TAB_BOTTOM_MARGIN）。
  static const double tabBottomMargin = 16;

  /// 悬浮胶囊压过内容，滚动内容需留出的底部空白（TAB_RESERVED）。
  static const double tabReserved = 96;

  static const double tabWidthSmall = 294;
  static const double tabWidth = 328;
}

/// 文字样式（由 token 派生，避免各页散落硬编码）。
class DliTextStyles {
  DliTextStyles._();

  static const TextStyle title = TextStyle(
    fontSize: DliFont.title,
    color: DliColors.text1,
    fontWeight: FontWeight.w600,
  );

  static const TextStyle titleSm = TextStyle(
    fontSize: DliFont.titleSm,
    color: DliColors.text1,
    fontWeight: FontWeight.w600,
  );

  static const TextStyle body = TextStyle(
    fontSize: DliFont.body,
    color: DliColors.text1,
  );

  /// 卡片标题：两行截断（对齐鸿蒙 VideoCardItem 的 BODY_SM + maxLines 2）。
  static const TextStyle cardTitle = TextStyle(
    fontSize: DliFont.bodySm,
    color: DliColors.text1,
    height: 1.35,
  );

  /// 卡片 meta：播放量 · UP 主 · 日期（CAPTION + text_tertiary）。
  static const TextStyle caption = TextStyle(
    fontSize: DliFont.caption,
    color: DliColors.text3,
  );

  static const TextStyle tabLabel = TextStyle(fontSize: DliFont.tabLabel);
}

/// 全局主题：把上述 token 装进 Material 3 的 ColorScheme/ThemeData。
///
/// 注意 **不做材质模拟**：鸿蒙的「沉浸光感」是系统级 `uiMaterial`，Android/iOS 无等价能力
/// （spec FLT-05 已明确），此处按规范降级为「普通背景 + 卡片层级 + 极淡品牌洗色」。
class DliTheme {
  DliTheme._();

  static ThemeData light() {
    final scheme = ColorScheme.fromSeed(
      seedColor: DliColors.primary,
      brightness: Brightness.light,
    ).copyWith(
      primary: DliColors.primary,
      onPrimary: DliColors.onPrimary,
      surface: DliColors.bgSurface,
      surfaceContainerLowest: DliColors.bgSurface,
      outline: DliColors.border,
    );

    return ThemeData(
      useMaterial3: true,
      colorScheme: scheme,
      scaffoldBackgroundColor: DliColors.bgPage,
      appBarTheme: const AppBarTheme(
        backgroundColor: DliColors.bgPage,
        surfaceTintColor: Colors.transparent,
        elevation: 0,
        scrolledUnderElevation: 0,
        centerTitle: false,
        titleTextStyle: DliTextStyles.titleSm,
        foregroundColor: DliColors.text1,
      ),
      navigationBarTheme: NavigationBarThemeData(
        backgroundColor: DliColors.bgSurface,
        surfaceTintColor: Colors.transparent,
        elevation: 0,
        height: 64,
        indicatorColor: DliColors.primaryLight,
        labelTextStyle: WidgetStateProperty.resolveWith((states) {
          final selected = states.contains(WidgetState.selected);
          return DliTextStyles.tabLabel.copyWith(
            color: selected ? DliColors.primary : DliColors.text2,
            fontWeight: selected ? FontWeight.w600 : FontWeight.w400,
          );
        }),
        iconTheme: WidgetStateProperty.resolveWith((states) {
          final selected = states.contains(WidgetState.selected);
          return IconThemeData(
            size: DliSize.tabIcon,
            color: selected ? DliColors.primary : DliColors.text2,
          );
        }),
      ),
      filledButtonTheme: FilledButtonThemeData(
        style: FilledButton.styleFrom(
          backgroundColor: DliColors.primary,
          foregroundColor: DliColors.onPrimary,
          shape: const RoundedRectangleBorder(
            borderRadius: BorderRadius.all(Radius.circular(DliRadius.full)),
          ),
          minimumSize: const Size(0, 44),
          textStyle: const TextStyle(
            fontSize: DliFont.body,
            fontWeight: FontWeight.w600,
          ),
        ),
      ),
      outlinedButtonTheme: OutlinedButtonThemeData(
        style: OutlinedButton.styleFrom(
          foregroundColor: DliColors.text1,
          side: const BorderSide(color: DliColors.border),
          shape: const RoundedRectangleBorder(
            borderRadius: BorderRadius.all(Radius.circular(DliRadius.full)),
          ),
          minimumSize: const Size(0, 44),
        ),
      ),
      inputDecorationTheme: const InputDecorationTheme(
        border: OutlineInputBorder(
          borderRadius: BorderRadius.all(Radius.circular(DliRadius.md)),
        ),
        enabledBorder: OutlineInputBorder(
          borderRadius: BorderRadius.all(Radius.circular(DliRadius.md)),
          borderSide: BorderSide(color: DliColors.border),
        ),
        focusedBorder: OutlineInputBorder(
          borderRadius: BorderRadius.all(Radius.circular(DliRadius.md)),
          borderSide: BorderSide(color: DliColors.primary),
        ),
      ),
      dividerTheme: const DividerThemeData(
        color: DliColors.border,
        thickness: 0.5,
        space: 0.5,
      ),
      cardTheme: const CardThemeData(
        color: DliColors.bgSurface,
        elevation: 0,
        margin: EdgeInsets.zero,
        shape: RoundedRectangleBorder(
          borderRadius: BorderRadius.all(Radius.circular(DliRadius.lg)),
        ),
      ),
    );
  }
}

/// 数字格式化：与鸿蒙 `common/utils/Format.ets` 同口径
///（播放量按 万/亿 收敛，时长 mm:ss / h:mm:ss）。
class DliFormat {
  DliFormat._();

  /// 播放量：<1万 原样；≥1万 显示 x.x万；≥1亿 显示 x.x亿。
  static String count(int n) {
    if (n < 10000) return '$n';
    if (n < 100000000) {
      final v = n / 10000;
      return '${_trim(v)}万';
    }
    final v = n / 100000000;
    return '${_trim(v)}亿';
  }

  static String _trim(double v) {
    final s = v.toStringAsFixed(1);
    return s.endsWith('.0') ? s.substring(0, s.length - 2) : s;
  }

  /// 时长：秒 → mm:ss（≥1 小时 h:mm:ss）。
  static String duration(int seconds) {
    if (seconds <= 0) return '';
    final h = seconds ~/ 3600;
    final m = (seconds % 3600) ~/ 60;
    final s = seconds % 60;
    final mm = m.toString().padLeft(2, '0');
    final ss = s.toString().padLeft(2, '0');
    return h > 0 ? '$h:$mm:$ss' : '$mm:$ss';
  }
}

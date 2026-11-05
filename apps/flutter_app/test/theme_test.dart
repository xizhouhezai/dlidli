/// 设计 token 与鸿蒙端**逐值一致性**守护测试。
///
/// 为什么需要它：Flutter 端 UI 要与鸿蒙端保持一致，靠人眼比对不可持续——
/// 一旦有人改了 `core/theme.dart` 的色值/间距而没同步鸿蒙（或反之），
/// 两端就会悄悄漂移。这个测试把「对齐来源」写死成期望值，
/// 改 token 必须同时改这里，从而强制改动人意识到"这是跨端约定"。
///
/// 基准：`apps/harmony/entry/src/main/ets/common/constants/Theme.ets`（2026-11-05 核对）。
library;

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:dlidli_app/core/theme.dart';

void main() {
  group('品牌色与中性色阶（对齐 DliColor）', () {
    test('主色与状态色', () {
      expect(DliColors.primary, const Color(0xFFFB7299));
      expect(DliColors.primaryLight, const Color(0xFFFFD9E4));
      expect(DliColors.primaryHover, const Color(0xFFFC8BAB));
      expect(DliColors.onPrimary, const Color(0xFFFFFFFF));
    });

    test('文字三级色阶', () {
      expect(DliColors.text1, const Color(0xFF18191C));
      expect(DliColors.text2, const Color(0xFF61666D));
      expect(DliColors.text3, const Color(0xFF9499A0));
    });

    test('边框/页面底/表面', () {
      expect(DliColors.border, const Color(0xFFE3E5E7));
      expect(DliColors.bgPage, const Color(0xFFF6F7F8));
      expect(DliColors.bgSurface, const Color(0xFFFFFFFF));
      expect(DliColors.mask, const Color(0xCC000000));
    });

    test('时长角标底色（鸿蒙卡片用 99000000）', () {
      expect(DliColors.durationBadge, const Color(0x99000000));
    });
  });

  group('圆角（对齐 DliRadius）', () {
    test('sm/md/lg/full', () {
      expect(DliRadius.sm, 6);
      expect(DliRadius.md, 8);
      expect(DliRadius.lg, 12);
      expect(DliRadius.full, 999);
    });
  });

  group('间距（对齐 DliSpace）', () {
    test('xs/sm/md/lg/xl/page', () {
      expect(DliSpace.xs, 4);
      expect(DliSpace.sm, 8);
      expect(DliSpace.md, 12);
      expect(DliSpace.lg, 16);
      expect(DliSpace.xl, 24);
      expect(DliSpace.page, 12);
    });
  });

  group('字号（对齐 DliFont）', () {
    test('caption/bodySm/body/titleSm/title/tabLabel', () {
      expect(DliFont.caption, 11);
      expect(DliFont.bodySm, 13);
      expect(DliFont.body, 15);
      expect(DliFont.titleSm, 17);
      expect(DliFont.title, 20);
      expect(DliFont.tabLabel, 12);
    });
  });

  group('尺寸（对齐 DliSize）', () {
    test('底栏与标题栏', () {
      expect(DliSize.tabIcon, 24);
      expect(DliSize.titleBarHeight, 56);
      expect(DliSize.tabBottomMargin, 16);
      expect(DliSize.tabReserved, 96);
      expect(DliSize.tabWidthSmall, 294);
      expect(DliSize.tabWidth, 328);
    });
  });

  group('主题装配', () {
    test('品牌色进入 ColorScheme，页面底色为 bgPage', () {
      final t = DliTheme.light();
      expect(t.colorScheme.primary, DliColors.primary);
      expect(t.scaffoldBackgroundColor, DliColors.bgPage);
      expect(t.useMaterial3, isTrue);
    });
  });

  group('数字格式化（对齐鸿蒙 Format.ets 口径）', () {
    test('播放量 万/亿 收敛', () {
      expect(DliFormat.count(0), '0');
      expect(DliFormat.count(9999), '9999');
      expect(DliFormat.count(10000), '1万');
      expect(DliFormat.count(12345), '1.2万');
      expect(DliFormat.count(123456789), '1.2亿');
    });

    test('时长 mm:ss 与 h:mm:ss', () {
      expect(DliFormat.duration(0), '');
      expect(DliFormat.duration(59), '00:59');
      expect(DliFormat.duration(61), '01:01');
      expect(DliFormat.duration(3661), '1:01:01');
    });
  });
}

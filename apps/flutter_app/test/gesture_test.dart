/// 播放手势判定单元测试（T-12）。
///
/// 手势冲突（横拖 vs 竖拖 vs 轻触）是最容易写错、且真机上很难复现调试的地方，
/// 所以把判定逻辑抽成纯函数 [GestureResolver] 并在这里穷举各种偏移组合。
library;

import 'package:flutter_test/flutter_test.dart';

import 'package:dlidli_app/features/player/widgets/playback_gesture_layer.dart';

void main() {
  const resolver = GestureResolver(slop: 12);
  const width = 1000.0;

  group('未达阈值 → undecided（防止轻触就触发）', () {
    test('几乎没动', () {
      expect(
        resolver.resolve(startX: 500, width: width, dx: 3, dy: 2),
        GestureKind.undecided,
      );
    });

    test('刚好在阈值边界内', () {
      expect(
        resolver.resolve(startX: 500, width: width, dx: 11, dy: 11),
        GestureKind.undecided,
      );
    });

    test('一个方向刚好到位、另一个更大 → 按主轴判定', () {
      // dx=13 超过阈值，dy=20 更大 → 竖向手势
      expect(
        resolver.resolve(startX: 100, width: width, dx: 13, dy: 20),
        GestureKind.brightness,
      );
    });
  });

  group('水平占优 → seek（无论起始位置）', () {
    test('左半屏横拖', () {
      expect(
        resolver.resolve(startX: 100, width: width, dx: 80, dy: 5),
        GestureKind.seek,
      );
    });

    test('右半屏横拖', () {
      expect(
        resolver.resolve(startX: 900, width: width, dx: -80, dy: 5),
        GestureKind.seek,
      );
    });

    test('正中间横拖', () {
      expect(
        resolver.resolve(startX: 500, width: width, dx: 50, dy: 0),
        GestureKind.seek,
      );
    });
  });

  group('垂直占优 → 按起始半屏区分亮度/音量', () {
    test('左半屏竖拖 → 亮度', () {
      expect(
        resolver.resolve(startX: 200, width: width, dx: 5, dy: 60),
        GestureKind.brightness,
      );
    });

    test('右半屏竖拖 → 音量', () {
      expect(
        resolver.resolve(startX: 800, width: width, dx: 5, dy: 60),
        GestureKind.volume,
      );
    });

    test('向下竖拖同样是亮度/音量（方向由调用方按 dy 符号处理）', () {
      expect(
        resolver.resolve(startX: 200, width: width, dx: 0, dy: -60),
        GestureKind.brightness,
      );
      expect(
        resolver.resolve(startX: 800, width: width, dx: 0, dy: -60),
        GestureKind.volume,
      );
    });

    test('恰好在半屏分界线上 → 归为右半屏（音量）', () {
      // x == width/2 不满足 < width/2，故走 volume
      expect(
        resolver.resolve(startX: 500, width: width, dx: 0, dy: 60),
        GestureKind.volume,
      );
    });
  });

  group('不同屏宽下的半屏判定', () {
    test('窄屏（360dp）', () {
      expect(
        resolver.resolve(startX: 100, width: 360, dx: 0, dy: 40),
        GestureKind.brightness,
      );
      expect(
        resolver.resolve(startX: 300, width: 360, dx: 0, dy: 40),
        GestureKind.volume,
      );
    });

    test('横屏（1920 宽）', () {
      expect(
        resolver.resolve(startX: 500, width: 1920, dx: 0, dy: 40),
        GestureKind.brightness,
      );
      expect(
        resolver.resolve(startX: 1500, width: 1920, dx: 0, dy: 40),
        GestureKind.volume,
      );
    });
  });

  group('对角线接近 45° 时的稳定性', () {
    test('dx 略大于 dy → seek', () {
      expect(
        resolver.resolve(startX: 200, width: width, dx: 51, dy: 50),
        GestureKind.seek,
      );
    });

    test('dy 略大于 dx → 亮度/音量', () {
      expect(
        resolver.resolve(startX: 200, width: width, dx: 50, dy: 51),
        GestureKind.brightness,
      );
    });
  });
}

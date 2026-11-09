/// 播放手势覆盖层（T-12 / FLT-11、FLT-15）。
///
/// 交互口径对齐鸿蒙端播放页与主流播放器习惯：
/// - **单击**：显示/隐藏控制层（不暂停，避免误触打断观看）
/// - **左右横拖**：快进/快退（拖动中只显示预览时间，松手才 seek，减少无谓 seek 抖动）
/// - **左半屏竖拖**：调亮度；**右半屏竖拖**：调音量
/// - **双击左/右**：快退/快进 10 秒
///
/// 为什么把判定逻辑单独抽出来：手势冲突（横拖 vs 竖拖）是最容易写错的地方，
/// 抽成纯函数后可以用单测覆盖各种偏移组合，不必上真机试。
library;

import 'dart:async';

import 'package:flutter/material.dart';
import 'package:screen_brightness/screen_brightness.dart';
import 'package:video_player/video_player.dart';
import 'package:volume_controller/volume_controller.dart';

import '../../../core/theme.dart';

/// 手势类型判定结果。
enum GestureKind {
  /// 尚未达到判定阈值。
  undecided,

  /// 水平拖动 → 快进/快退。
  seek,

  /// 垂直拖动（左半屏）→ 亮度。
  brightness,

  /// 垂直拖动（右半屏）→ 音量。
  volume,
}

/// 手势判定：根据起止偏移与起始位置决定这是哪一类手势。
///
/// 判定规则（对齐主流播放器）：
/// - 先看**主轴**：|dx| 与 |dy| 谁大；水平占优 → seek，垂直占优 → 亮度/音量
/// - 阈值 [slop]：位移小于该值视为「未决定」，避免手指轻触就触发
/// - 垂直手势再按**起始点所在半屏**区分亮度（左）/音量（右）
class GestureResolver {
  const GestureResolver({this.slop = 12});

  /// 触发判定的最小位移（逻辑像素）。
  final double slop;

  /// [startX] 手势起始横坐标，[width] 屏幕（或手势区）宽度，
  /// [dx]/[dy] 为相对起始点的累计位移。
  GestureKind resolve({
    required double startX,
    required double width,
    required double dx,
    required double dy,
  }) {
    if (dx.abs() < slop && dy.abs() < slop) return GestureKind.undecided;

    if (dx.abs() > dy.abs()) return GestureKind.seek;

    // 垂直手势：按起始半屏区分亮度/音量
    return startX < width / 2 ? GestureKind.brightness : GestureKind.volume;
  }
}

/// 拖动中显示的预览信息（时间 / 亮度 / 音量）。
class GesturePreview {
  const GesturePreview({
    required this.kind,
    required this.label,
    this.icon,
  });

  final GestureKind kind;
  final String label;
  final IconData? icon;
}

/// 手势覆盖层：包在播放器画面之上，负责单击/双击/拖动。
class PlaybackGestureLayer extends StatefulWidget {
  const PlaybackGestureLayer({
    super.key,
    required this.controller,
    required this.child,
    this.onToggleControls,
    this.onBrightnessChanged,
    this.onVolumeChanged,
  });

  final VideoPlayerController controller;
  final Widget child;

  /// 单击：切换控制层显隐。
  final VoidCallback? onToggleControls;

  /// 亮度/音量变化回调（供父级更新 UI 状态）。
  final ValueChanged<double>? onBrightnessChanged;
  final ValueChanged<double>? onVolumeChanged;

  @override
  State<PlaybackGestureLayer> createState() => _PlaybackGestureLayerState();
}

class _PlaybackGestureLayerState extends State<PlaybackGestureLayer> {
  static const _resolver = GestureResolver();

  GestureKind _kind = GestureKind.undecided;
  Offset _start = Offset.zero;
  Size _size = Size.zero;

  /// 拖动起点时的播放位置（秒）。
  int _seekBaseSeconds = 0;

  /// 拖动中目标位置（秒）。
  int _seekTargetSeconds = 0;

  /// 拖动起点时的亮度/音量（0~1）。
  double _baseLevel = 0;
  double _currentLevel = 0;

  GesturePreview? _preview;
  Timer? _doubleTapTimer;
  bool _seekCommitting = false;

  @override
  void dispose() {
    _doubleTapTimer?.cancel();
    super.dispose();
  }

  void _onPanStart(DragStartDetails d) {
    _start = d.localPosition;
    _size = context.size ?? Size.zero;
    _kind = GestureKind.undecided;

    final c = widget.controller;
    _seekBaseSeconds = c.value.position.inSeconds;
    _seekTargetSeconds = _seekBaseSeconds;
  }

  void _onPanUpdate(DragUpdateDetails d) {
    final dx = d.localPosition.dx - _start.dx;
    final dy = d.localPosition.dy - _start.dy;

    if (_kind == GestureKind.undecided) {
      _kind = _resolver.resolve(
        startX: _start.dx,
        width: _size.width,
        dx: dx,
        dy: dy,
      );
      if (_kind == GestureKind.undecided) return;
      _initBaseLevel(_kind);
    }

    switch (_kind) {
      case GestureKind.seek:
        _updateSeek(dx);
      case GestureKind.brightness:
      case GestureKind.volume:
        _updateLevel(dy);
      case GestureKind.undecided:
        break;
    }
  }

  /// 记录拖动起点时的亮度/音量基准。
  void _initBaseLevel(GestureKind kind) {
    if (kind == GestureKind.brightness) {
      _baseLevel = _currentLevel;
    } else if (kind == GestureKind.volume) {
      _baseLevel = _currentLevel;
    }
  }

  /// 水平拖动 → 换算目标时间。
  ///
  /// 映射口径：拖满整个宽度 = 快进/快退一个「屏幕宽度对应时长」，
  /// 上限取 120 秒，避免短视频轻微一划就跳到片尾。
  void _updateSeek(double dx) {
    final c = widget.controller;
    final total = c.value.duration.inSeconds;
    if (total <= 0) return;

    final width = _size.width <= 0 ? 1.0 : _size.width;
    final span = (total * 0.25).clamp(10, 120).toDouble();
    final delta = (dx / width) * span;

    final target = (_seekBaseSeconds + delta).round().clamp(0, total);
    _seekTargetSeconds = target;

    final sign = target >= _seekBaseSeconds ? '+' : '-';
    final diff = (target - _seekBaseSeconds).abs();
    setState(() {
      _preview = GesturePreview(
        kind: GestureKind.seek,
        label: '$sign${diff}s  ${_fmt(target)} / ${_fmt(total)}',
        icon: sign == '+' ? Icons.fast_forward : Icons.fast_rewind,
      );
    });
  }

  /// 垂直拖动 → 换算亮度/音量（向上增、向下减）。
  void _updateLevel(double dy) {
    final height = _size.height <= 0 ? 1.0 : _size.height;
    // 拖满整个高度 = 变化 100%，手感与主流播放器一致
    final delta = -(dy / height);
    final level = (_baseLevel + delta).clamp(0.0, 1.0);
    _currentLevel = level;

    final isBrightness = _kind == GestureKind.brightness;
    // 写入系统；失败不影响手势反馈（部分设备有权限限制）
    if (isBrightness) {
      ScreenBrightness().setApplicationScreenBrightness(level).catchError((_) {});
      widget.onBrightnessChanged?.call(level);
    } else {
      VolumeController.instance.setVolume(level).catchError((_) {});
      widget.onVolumeChanged?.call(level);
    }

    setState(() {
      _preview = GesturePreview(
        kind: isBrightness ? GestureKind.brightness : GestureKind.volume,
        label: '${(level * 100).round()}%',
        icon: isBrightness ? Icons.brightness_6 : Icons.volume_up,
      );
    });
  }

  Future<void> _onPanEnd(DragEndDetails d) async {
    if (_kind == GestureKind.seek && !_seekCommitting) {
      _seekCommitting = true;
      try {
        await widget.controller.seekTo(Duration(seconds: _seekTargetSeconds));
      } catch (_) {
        // seek 失败不崩：保持原位置继续播放
      } finally {
        _seekCommitting = false;
      }
    }
    if (!mounted) return;
    setState(() {
      _preview = null;
      _kind = GestureKind.undecided;
    });
  }

  /// 双击：左退 10 秒 / 右进 10 秒。
  void _onDoubleTapDown(TapDownDetails d) {
    _doubleTapTimer?.cancel();
    final width = context.size?.width ?? 1;
    final isLeft = d.localPosition.dx < width / 2;
    final c = widget.controller;
    final total = c.value.duration.inSeconds;
    if (total <= 0) return;

    final base = c.value.position.inSeconds;
    final target = (base + (isLeft ? -10 : 10)).clamp(0, total);
    c.seekTo(Duration(seconds: target));

    setState(() {
      _preview = GesturePreview(
        kind: GestureKind.seek,
        label: isLeft ? '« 10s' : '10s »',
        icon: isLeft ? Icons.replay_10 : Icons.forward_10,
      );
    });
    _doubleTapTimer = Timer(const Duration(milliseconds: 600), () {
      if (!mounted) return;
      setState(() => _preview = null);
    });
  }

  String _fmt(int seconds) {
    final m = seconds ~/ 60;
    final s = seconds % 60;
    return '$m:${s.toString().padLeft(2, '0')}';
  }

  @override
  Widget build(BuildContext context) {
    return GestureDetector(
      behavior: HitTestBehavior.opaque,
      onPanStart: _onPanStart,
      onPanUpdate: _onPanUpdate,
      onPanEnd: _onPanEnd,
      onDoubleTapDown: _onDoubleTapDown,
      onTap: widget.onToggleControls,
      child: Stack(
        fit: StackFit.expand,
        children: [
          widget.child,
          if (_preview != null) _PreviewOverlay(preview: _preview!),
        ],
      ),
    );
  }
}

/// 拖动/双击时的中央提示（半透明胶囊）。
class _PreviewOverlay extends StatelessWidget {
  const _PreviewOverlay({required this.preview});

  final GesturePreview preview;

  @override
  Widget build(BuildContext context) {
    return IgnorePointer(
      child: Center(
        child: Container(
          padding: const EdgeInsets.symmetric(
            horizontal: DliSpace.lg,
            vertical: DliSpace.sm,
          ),
          decoration: const BoxDecoration(
            color: Color(0xCC000000),
            borderRadius: BorderRadius.all(Radius.circular(DliRadius.md)),
          ),
          child: Row(
            mainAxisSize: MainAxisSize.min,
            children: [
              if (preview.icon != null) ...[
                Icon(preview.icon, color: Colors.white, size: 20),
                const SizedBox(width: DliSpace.sm),
              ],
              Text(
                preview.label,
                style: const TextStyle(
                  color: Colors.white,
                  fontSize: DliFont.bodySm,
                  fontWeight: FontWeight.w600,
                ),
              ),
            ],
          ),
        ),
      ),
    );
  }
}

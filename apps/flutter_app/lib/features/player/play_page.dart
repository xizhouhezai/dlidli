/// 播放页（T-11 / FLT-10、FLT-14）。
///
/// 本期做**播放核心链路**：拉稿件详情 → 取签名 HLS → 起播 → 清晰度切换 → 进度保存。
/// 弹幕、点赞投币收藏、分享等留给后续任务（弹幕见 T-12、互动见 T-30）。
///
/// 播放器内核：**`video_player`（ExoPlayer/AVPlayer）** —— T-10 实测定案，
/// 数据与理由见 `docs/specs/flutter/plan.md` §2.4。
///
/// 三条关键约束（都是踩过的坑）：
/// 1. **播放地址必须走 UrlRewriter**：后端下发的是服务端视角的 localhost，端机不可路由；
/// 2. **切换清晰度要保留播放进度**：换流会重置到 0，用户体验上是"从头开始"，
///    故切换前记录 position，切换后 seek 回去；
/// 3. **进度上报节流**：每 3 秒上报一次即可，逐帧上报会把后端打爆。
library;

import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:video_player/video_player.dart';

import '../../app/providers.dart';
import '../../core/api_error.dart';
import '../../core/theme.dart';
import '../../core/url_rewriter.dart';
import '../../data/models/video.dart';

/// 进度上报间隔（秒）。
const _progressInterval = Duration(seconds: 3);

/// 进度上报的最小变化阈值（秒）：拖动导致的微小差异不上报，避免无意义写入。
const _progressMinDelta = 3;

/// 秒 → mm:ss（续播提示用；与 DliFormat.duration 同口径，此处独立以免耦合展示层）。
String _formatDuration(int seconds) {
  final m = seconds ~/ 60;
  final s = seconds % 60;
  return '$m:${s.toString().padLeft(2, '0')}';
}

class PlayPage extends ConsumerStatefulWidget {
  const PlayPage({super.key, required this.bvid});

  final String bvid;

  @override
  ConsumerState<PlayPage> createState() => _PlayPageState();
}

class _PlayPageState extends ConsumerState<PlayPage> {
  VideoPlayerController? _controller;
  Timer? _progressTimer;

  VideoCard? _video;
  List<VideoStream> _streams = const [];
  int _currentIndex = 0;

  bool _loading = true;
  String? _error;
  int _resumePosition = 0;

  /// 进度是否已恢复完成为 true。恢复期间不自动起播 ——
  /// 否则会出现"先从头播起、随即跳到历史位置"的闪断，观感很差。
  bool _progressRestored = false;

  @override
  void initState() {
    super.initState();
    _load();
  }

  @override
  void dispose() {
    _progressTimer?.cancel();
    _controller?.dispose();
    super.dispose();
  }

  Future<void> _load() async {
    setState(() {
      _loading = true;
      _error = null;
    });
    try {
      final api = ref.read(videoApiProvider);
      final video = await api.detail(widget.bvid);
      final streams = await api.streams(widget.bvid);

      // 只保留可用的 HLS 档位（mp4 直链也可播，但本期聚焦 HLS）
      final hls = streams.where((s) => s.format == 'hls' && s.url.isNotEmpty).toList()
        ..sort((a, b) => b.quality.compareTo(a.quality));
      if (hls.isEmpty) {
        setState(() {
          _loading = false;
          _error = '该稿件暂无可播放的清晰度';
        });
        return;
      }

      // 默认选**用户可看**的档位：优先最高档，若带会员标记则看是否已登录
      final index = _pickDefaultIndex(hls);

      if (!mounted) return;
      setState(() {
        _video = video;
        _streams = hls;
        _currentIndex = index;
      });

      await _restoreProgress();
      // async gap 之后必须重新确认 mounted：期间用户可能已退出播放页，
      // 此时用 context 调 ScaffoldMessenger 会抛异常。
      if (!mounted) return;
      if (_resumePosition > 0) {
        // 续播提示：让用户知道是从上次的位置接着看，而不是莫名跳了一段
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(
            content: Text('已续播到 ${_formatDuration(_resumePosition)}'),
            duration: const Duration(seconds: 2),
          ),
        );
      }
      await _openStream(index, seekTo: _resumePosition);
    } on ApiError catch (e) {
      if (!mounted) return;
      setState(() {
        _loading = false;
        _error = e.message;
      });
    } catch (e) {
      if (!mounted) return;
      setState(() {
        _loading = false;
        _error = '加载失败：$e';
      });
    }
  }

  /// 选默认档位：最高画质且非会员限定。
  int _pickDefaultIndex(List<VideoStream> hls) {
    for (var i = 0; i < hls.length; i++) {
      if (!hls[i].isVipOnly) return i;
    }
    return 0;
  }

  /// 读取历史进度（需登录；未登录静默跳过）。
  Future<void> _restoreProgress() async {
    try {
      final pos = await ref.read(videoApiProvider).progress(widget.bvid);
      if (!mounted) return;
      setState(() {
        _resumePosition = pos;
        _progressRestored = true;
      });
    } catch (_) {
      // 未登录或网络失败：不影响播放，从头开始即可
      if (!mounted) return;
      setState(() => _progressRestored = true);
    }
  }

  /// 打开指定档位。
  ///
  /// [seekTo] 用于两处：① 首次播放用历史进度；② 切换清晰度时保留当前位置。
  Future<void> _openStream(int index, {int seekTo = 0}) async {
    final stream = _streams[index];
    final url = UrlRewriter.rewrite(stream.url);
    if (url.isEmpty) {
      setState(() => _error = '播放地址为空');
      return;
    }

    final old = _controller;
    // 换流前先记下当前进度，切换后要 seek 回去（否则用户以为从头开始）
    final keep = old != null && _controller!.value.isInitialized
        ? old.value.position.inSeconds
        : seekTo;

    setState(() {
      _loading = true;
      _error = null;
    });

    try {
      final c = VideoPlayerController.networkUrl(Uri.parse(url));
      await c.initialize();
      await c.setLooping(false);
      if (keep > 0) {
        final maxMs = c.value.duration.inMilliseconds;
        // 进度可能超过新流时长（例如换了清晰度时长略有差异），越界会抛异常
        if (keep * 1000 < maxMs) {
          await c.seekTo(Duration(seconds: keep));
        }
      }
      // 进度还没恢复完就先不自动播：否则会出现"从头播起→随即跳到历史位置"的闪断。
      // 首次进入时 _progressRestored 已为 true（恢复完成才走到这里），切换清晰度时也已是 true。
      if (_progressRestored) {
        await c.play();
      }
      if (!mounted) {
        await c.dispose();
        return;
      }
      setState(() {
        _controller = c;
        _currentIndex = index;
        _loading = false;
      });
      old?.dispose();
      _startProgressTimer();
    } catch (e) {
      if (!mounted) return;
      setState(() {
        _loading = false;
        _error = '起播失败：$e';
      });
      old?.dispose();
    }
  }

  void _startProgressTimer() {
    _progressTimer?.cancel();
    _progressTimer = Timer.periodic(_progressInterval, (_) => _reportProgress());
  }

  int _lastReported = -1;

  Future<void> _reportProgress() async {
    final c = _controller;
    if (c == null || !c.value.isInitialized) return;
    final pos = c.value.position.inSeconds;
    if (pos <= 0) return;
    if ((pos - _lastReported).abs() < _progressMinDelta) return;
    _lastReported = pos;
    try {
      await ref.read(videoApiProvider).saveProgress(widget.bvid, pos);
    } catch (_) {
      // 上报失败不打断播放
    }
  }

  Future<void> _switchQuality(int index) async {
    if (index == _currentIndex) return;
    await _reportProgress();
    await _openStream(index);
  }

  @override
  Widget build(BuildContext context) {
    final c = _controller;
    return AnnotatedRegion<SystemUiOverlayStyle>(
      // 播放页是深色沉浸背景，状态栏用浅色图标
      value: const SystemUiOverlayStyle(
        statusBarColor: Colors.transparent,
        statusBarIconBrightness: Brightness.light,
        statusBarBrightness: Brightness.dark,
      ),
      child: Scaffold(
        backgroundColor: Colors.black,
        body: _loading && c == null
            ? const Center(
                child: CircularProgressIndicator(color: DliColors.primary),
              )
            : _error != null && c == null
                ? _ErrorView(error: _error!, onRetry: _load)
                : _buildContent(c),
      ),
    );
  }

  Widget _buildContent(VideoPlayerController? c) {
    final video = _video;
    return Column(
      children: [
        // 播放器区域（16:9）
        AspectRatio(
          aspectRatio: 16 / 9,
          child: c != null && c.value.isInitialized
              ? Stack(
                  fit: StackFit.expand,
                  children: [
                    VideoPlayer(c),
                    if (_loading)
                      const Center(
                        child: CircularProgressIndicator(color: DliColors.primary),
                      ),
                    _Controls(controller: c),
                  ],
                )
              : const ColoredBox(color: Colors.black),
        ),
        Expanded(
          child: Container(
            color: DliColors.bgPage,
            child: ListView(
              padding: const EdgeInsets.all(DliSpace.page),
              children: [
                Text(video?.title ?? '', style: DliTextStyles.titleSm),
                const SizedBox(height: DliSpace.sm),
                _metaRow(video),
                const SizedBox(height: DliSpace.md),
                _qualitySelector(),
                const SizedBox(height: DliSpace.md),
                if (video != null && video.description.isNotEmpty) ...[
                  Text('简介', style: DliTextStyles.body),
                  const SizedBox(height: DliSpace.xs),
                  Text(video.description, style: DliTextStyles.caption),
                  const SizedBox(height: DliSpace.md),
                ],
                const SizedBox(height: DliSpace.xl),
              ],
            ),
          ),
        ),
      ],
    );
  }

  Widget _metaRow(VideoCard? video) {
    if (video == null) return const SizedBox.shrink();
    final stat = video.stat;
    return Row(
      children: [
        Expanded(
          child: Text(
            video.ownerName,
            style: DliTextStyles.caption,
            overflow: TextOverflow.ellipsis,
          ),
        ),
        Text('播放 ${DliFormat.count(stat.view)}', style: DliTextStyles.caption),
        const SizedBox(width: DliSpace.md),
        Text('点赞 ${DliFormat.count(stat.like)}', style: DliTextStyles.caption),
        const SizedBox(width: DliSpace.md),
        Text('投币 ${DliFormat.count(stat.coin)}', style: DliTextStyles.caption),
      ],
    );
  }

  /// 清晰度选择（对齐鸿蒙播放页的清晰度面板：横向选项 + 当前项高亮）。
  Widget _qualitySelector() {
    return Row(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text('清晰度', style: DliTextStyles.body),
        const SizedBox(width: DliSpace.md),
        Expanded(
          child: Wrap(
            spacing: DliSpace.sm,
            runSpacing: DliSpace.sm,
            children: [
              for (var i = 0; i < _streams.length; i++)
                GestureDetector(
                  onTap: _loading ? null : () => _switchQuality(i),
                  child: Container(
                    padding: const EdgeInsets.symmetric(
                      horizontal: DliSpace.md,
                      vertical: DliSpace.xs,
                    ),
                    decoration: BoxDecoration(
                      color: i == _currentIndex
                          ? DliColors.primary
                          : DliColors.bgSurface,
                      borderRadius: BorderRadius.circular(DliRadius.full),
                      border: Border.all(
                        color: i == _currentIndex
                            ? DliColors.primary
                            : DliColors.border,
                      ),
                    ),
                    child: Text(
                      _streams[i].qualityLabel,
                      style: TextStyle(
                        fontSize: DliFont.bodySm,
                        color: i == _currentIndex
                            ? DliColors.onPrimary
                            : DliColors.text2,
                      ),
                    ),
                  ),
                ),
            ],
          ),
        ),
      ],
    );
  }
}

/// 播放控件：播放/暂停 + 进度条 + 时间。
class _Controls extends StatefulWidget {
  const _Controls({required this.controller});
  final VideoPlayerController controller;

  @override
  State<_Controls> createState() => _ControlsState();
}

class _ControlsState extends State<_Controls> {
  @override
  Widget build(BuildContext context) {
    final c = widget.controller;
    return GestureDetector(
      behavior: HitTestBehavior.opaque,
      onTap: () {
        setState(() {
          c.value.isPlaying ? c.pause() : c.play();
        });
      },
      child: Stack(
        children: [
          Positioned.fill(
            child: Center(
              child: c.value.isPlaying
                  ? const SizedBox.shrink()
                  : Container(
                      padding: const EdgeInsets.all(DliSpace.md),
                      decoration: const BoxDecoration(
                        color: Color(0x99000000),
                        shape: BoxShape.circle,
                      ),
                      child: const Icon(Icons.play_arrow,
                          color: Colors.white, size: 36),
                    ),
            ),
          ),
          Positioned(
            left: 0,
            right: 0,
            bottom: 0,
            child: VideoProgressIndicator(
              c,
              allowScrubbing: true,
              padding: const EdgeInsets.symmetric(vertical: DliSpace.xs),
              colors: const VideoProgressColors(
                playedColor: DliColors.primary,
                bufferedColor: Color(0x66FFFFFF),
                backgroundColor: Color(0x33FFFFFF),
              ),
            ),
          ),
        ],
      ),
    );
  }
}

class _ErrorView extends StatelessWidget {
  const _ErrorView({required this.error, required this.onRetry});

  final String error;
  final VoidCallback onRetry;

  @override
  Widget build(BuildContext context) {
    return Center(
      child: Padding(
        padding: const EdgeInsets.all(DliSpace.xl),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            const Icon(Icons.error_outline, size: 40, color: DliColors.text3),
            const SizedBox(height: DliSpace.md),
            Text(error, textAlign: TextAlign.center, style: DliTextStyles.body),
            const SizedBox(height: DliSpace.lg),
            FilledButton(onPressed: onRetry, child: const Text('重试')),
          ],
        ),
      ),
    );
  }
}

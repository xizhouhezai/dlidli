/// 播放器探针页（T-10 选型结论的**回归探针**）。
///
/// 选型已定案 `video_player`（实测数据见 `docs/specs/flutter/plan.md` §2.4）：
/// 起播比 `media_kit` 快 1.6~2.2 倍、包体小 43MB、集成成本低（无需升级 Kotlin 工具链）。
///
/// 本页保留的价值：改播放器相关依赖/引擎版本后，**再跑一次就能对比起播耗时是否劣化**。
/// 因此保留 `media_kit` 对比会引入 +43MB 包体，得不偿失 —— 改为只测 `video_player`，
/// 对比数据从历史截图与 plan 记录中查。
library;

import 'dart:async';

import 'package:flutter/material.dart';
import 'package:video_player/video_player.dart';

import '../../core/theme.dart';

/// 实测结果。
class PlayerProbeResult {
  PlayerProbeResult({required this.ok, this.loadMs, this.error, this.detail});

  final bool ok;
  final int? loadMs;
  final String? error;
  final String? detail;
}

class PlayerProbePage extends StatefulWidget {
  const PlayerProbePage({super.key, required this.hlsUrl, this.title = ''});

  final String hlsUrl;
  final String title;

  @override
  State<PlayerProbePage> createState() => _PlayerProbePageState();
}

class _PlayerProbePageState extends State<PlayerProbePage> {
  PlayerProbeResult? _result;
  VideoPlayerController? _vp;
  bool _ready = false;

  @override
  void initState() {
    super.initState();
    _probe();
  }

  @override
  void dispose() {
    _vp?.dispose();
    super.dispose();
  }

  Future<void> _probe() async {
    final sw = Stopwatch()..start();
    try {
      final c = VideoPlayerController.networkUrl(Uri.parse(widget.hlsUrl));
      await c.initialize();
      sw.stop();
      _vp = c;
      _ready = true;
      await c.play();
      if (!mounted) return;
      setState(() {
        _result = PlayerProbeResult(
          ok: true,
          loadMs: sw.elapsedMilliseconds,
          detail:
              '${c.value.size.width.toInt()}x${c.value.size.height.toInt()} '
              'duration=${c.value.duration.inSeconds}s',
        );
      });
    } catch (e) {
      sw.stop();
      if (!mounted) return;
      setState(() {
        _result = PlayerProbeResult(
          ok: false,
          loadMs: sw.elapsedMilliseconds,
          error: '$e',
        );
      });
    }
  }

  @override
  Widget build(BuildContext context) {
    final r = _result;
    return Scaffold(
      appBar: AppBar(title: const Text('播放器探针（video_player）')),
      body: ListView(
        padding: const EdgeInsets.all(DliSpace.lg),
        children: [
          Text(
            widget.title.isEmpty ? 'HLS 片源' : widget.title,
            style: DliTextStyles.cardTitle,
          ),
          const SizedBox(height: DliSpace.xs),
          Text(
            widget.hlsUrl,
            style: const TextStyle(fontFamily: 'monospace', fontSize: 10),
            maxLines: 3,
            overflow: TextOverflow.ellipsis,
          ),
          const SizedBox(height: DliSpace.lg),
          Card(
            margin: const EdgeInsets.only(bottom: DliSpace.sm),
            child: ListTile(
              leading: Icon(
                r == null
                    ? Icons.hourglass_empty
                    : (r.ok ? Icons.check_circle : Icons.cancel),
                color:
                    r == null
                        ? DliColors.text3
                        : (r.ok ? Colors.green : Colors.red),
              ),
              title: Text(
                r == null ? '探测中…' : (r.ok ? '起播成功' : '起播失败'),
                style: DliTextStyles.cardTitle,
              ),
              subtitle: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  if (r?.loadMs != null)
                    Text('起播耗时 ${r!.loadMs} ms', style: DliTextStyles.caption),
                  if (r?.detail != null)
                    Text(r!.detail!, style: DliTextStyles.caption),
                  if (r?.error != null)
                    Text(
                      r!.error!,
                      style: const TextStyle(fontSize: 10, color: Colors.red),
                      maxLines: 4,
                      overflow: TextOverflow.ellipsis,
                    ),
                ],
              ),
              isThreeLine: true,
            ),
          ),
          const Divider(),
          const SizedBox(height: DliSpace.sm),
          Text('画面', style: DliTextStyles.body),
          const SizedBox(height: DliSpace.sm),
          _ready && _vp != null
              ? AspectRatio(
                aspectRatio: _vp!.value.aspectRatio,
                child: VideoPlayer(_vp!),
              )
              : Container(
                height: 160,
                alignment: Alignment.center,
                decoration: BoxDecoration(
                  color: DliColors.surfaceSecondary,
                  borderRadius: BorderRadius.circular(DliRadius.md),
                ),
                child: Text('未就绪', style: DliTextStyles.caption),
              ),
        ],
      ),
    );
  }
}

/// 网络自检页（开发辅助，T-01/T-02 的验收面板）。
///
/// 把「能不能连通真实后端」这件事做成可点开就能看到绿勾的页面，
/// 便于真机取证，也方便日后排查"是不是后端地址配错了"。
library;

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../app/providers.dart';
import '../../core/api_error.dart';
import '../../core/theme.dart';

class DiagPage extends ConsumerStatefulWidget {
  const DiagPage({super.key});

  @override
  ConsumerState<DiagPage> createState() => _DiagPageState();
}

class _DiagPageState extends ConsumerState<DiagPage> {
  final List<_Row> _rows = [];
  bool _running = false;

  @override
  void initState() {
    super.initState();
    _run();
  }

  Future<void> _run() async {
    setState(() {
      _running = true;
      _rows.clear();
    });

    final client = ref.read(apiClientProvider);
    final api = ref.read(videoApiProvider);

    void add(String name, bool ok, String detail) {
      if (!mounted) return;
      setState(() => _rows.add(_Row(name: name, ok: ok, detail: detail)));
    }

    add('后端基址', true, client.baseUrl);

    try {
      final data = await client.get('/health');
      final m = data is Map ? data : const {};
      final ready = m['ready'] == true;
      add(
        '/health',
        ready,
        'ready=$ready mysql=${(m['components'] as Map?)?['mysql']}',
      );
    } catch (e) {
      add('/health', false, '$e');
    }

    try {
      final cats = await api.categories();
      add('分区列表', cats.isNotEmpty, 'count=${cats.length}');
    } catch (e) {
      add('分区列表', false, e is ApiError ? e.message : '$e');
    }

    try {
      final paged = await api.list(page: 1, pageSize: 3);
      add('视频列表', paged.list.isNotEmpty, 'count=${paged.list.length}');
      if (paged.list.isNotEmpty) {
        final streams = await api.streams(paged.list.first.bvid);
        final hls = streams.where((s) => s.format == 'hls');
        final sample =
            hls.isNotEmpty
                ? hls.first
                : (streams.isEmpty ? null : streams.first);
        final signed =
            sample != null &&
            sample.url.contains('e=') &&
            sample.url.contains('s=');
        add(
          '签名播放地址',
          signed,
          sample == null
              ? 'streams 为空'
              : '${streams.length} 档 / ${sample.qualityLabel} / 含签名=$signed',
        );
      }
    } catch (e) {
      add('视频列表', false, e is ApiError ? '${e.kind}: ${e.message}' : '$e');
    }

    if (mounted) setState(() => _running = false);
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(
        title: const Text('网络自检'),
        actions: [
          IconButton(
            icon: const Icon(Icons.play_arrow),
            onPressed: _running ? null : _run,
          ),
        ],
      ),
      body: ListView(
        padding: const EdgeInsets.all(DliSpace.lg),
        children: [
          if (_running) const LinearProgressIndicator(color: DliColors.primary),
          const SizedBox(height: DliSpace.sm),
          for (final r in _rows)
            ListTile(
              dense: true,
              leading: Icon(
                r.ok ? Icons.check_circle : Icons.error,
                color: r.ok ? Colors.green : Colors.red,
              ),
              title: Text(r.name, style: DliTextStyles.body),
              subtitle: Text(
                r.detail,
                style: const TextStyle(
                  fontFamily: 'monospace',
                  fontSize: DliFont.caption,
                ),
              ),
            ),
        ],
      ),
    );
  }
}

class _Row {
  _Row({required this.name, required this.ok, required this.detail});
  final String name;
  final bool ok;
  final String detail;
}

/// 首页信息流（T-40 的第一步：真实接口 + 三态渲染 + 触底加载）。
///
/// 本页刻意做成「最小可用」：先把**网络层与三态渲染**在真机上跑通，
/// 双列瀑布流、分区栏、下拉刷新在 T-40 正式任务里补齐。
library;

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../app/providers.dart';
import '../../core/api_error.dart';
import '../../data/models/video.dart';

class FeedPage extends ConsumerStatefulWidget {
  const FeedPage({super.key});

  @override
  ConsumerState<FeedPage> createState() => _FeedPageState();
}

class _FeedPageState extends ConsumerState<FeedPage> {
  final _scroll = ScrollController();

  @override
  void initState() {
    super.initState();
    _scroll.addListener(_onScroll);
  }

  @override
  void dispose() {
    _scroll.removeListener(_onScroll);
    _scroll.dispose();
    super.dispose();
  }

  void _onScroll() {
    if (!_scroll.hasClients) return;
    // 距底部 400px 触发加载更多
    if (_scroll.position.pixels >= _scroll.position.maxScrollExtent - 400) {
      ref.read(feedProvider.notifier).loadMore();
    }
  }

  @override
  Widget build(BuildContext context) {
    final async = ref.watch(feedProvider);

    return Scaffold(
      appBar: AppBar(
        title: const Text('DliDli'),
        actions: [
          IconButton(
            tooltip: '网络自检',
            icon: const Icon(Icons.wifi_tethering),
            onPressed: () => Navigator.of(context).push(
              MaterialPageRoute(builder: (_) => const DiagPage()),
            ),
          ),
          IconButton(
            tooltip: '刷新',
            icon: const Icon(Icons.refresh),
            onPressed: () => ref.read(feedProvider.notifier).loadFirstPage(),
          ),
        ],
      ),
      // AsyncValue.when 原生覆盖三态：loading / error / data（FLT-03 不白屏要求）
      body: async.when(
        loading: () => const Center(child: CircularProgressIndicator()),
        error: (err, _) => _ErrorView(
          error: err,
          onRetry: () => ref.read(feedProvider.notifier).loadFirstPage(),
        ),
        data: (state) {
          if (state.items.isEmpty) {
            return const Center(child: Text('暂无内容'));
          }
          return RefreshIndicator(
            onRefresh: () => ref.read(feedProvider.notifier).loadFirstPage(),
            child: ListView.separated(
              controller: _scroll,
              itemCount: state.items.length + (state.loadingMore ? 1 : 0),
              separatorBuilder: (_, __) => const Divider(height: 1),
              itemBuilder: (context, i) {
                if (i >= state.items.length) {
                  return const Padding(
                    padding: EdgeInsets.symmetric(vertical: 16),
                    child: Center(child: CircularProgressIndicator(strokeWidth: 2)),
                  );
                }
                final v = state.items[i] as VideoCard;
                return _VideoTile(card: v);
              },
            ),
          );
        },
      ),
    );
  }
}

class _VideoTile extends StatelessWidget {
  const _VideoTile({required this.card});

  final VideoCard card;

  @override
  Widget build(BuildContext context) {
    return ListTile(
      leading: SizedBox(
        width: 112,
        height: 63,
        child: card.cover.isEmpty
            ? Container(color: Colors.black12)
            : Image.network(
                card.cover,
                fit: BoxFit.cover,
                // 封面加载失败不能崩，给个占位（FLT-03 健壮性）
                errorBuilder: (_, __, ___) => Container(color: Colors.black12),
              ),
      ),
      title: Text(card.title, maxLines: 2, overflow: TextOverflow.ellipsis),
      subtitle: Text('${card.ownerName} · 播放 ${card.viewCount}'),
    );
  }
}

class _ErrorView extends StatelessWidget {
  const _ErrorView({required this.error, required this.onRetry});

  final Object error;
  final VoidCallback onRetry;

  @override
  Widget build(BuildContext context) {
    final api = error is ApiError ? error as ApiError : null;
    final msg = api?.message ?? '加载失败';
    final canRetry = api?.retryable ?? true;
    return Center(
      child: Padding(
        padding: const EdgeInsets.all(24),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            const Icon(Icons.cloud_off, size: 48, color: Colors.grey),
            const SizedBox(height: 12),
            Text(msg, textAlign: TextAlign.center),
            if (api?.traceId != null)
              Padding(
                padding: const EdgeInsets.only(top: 4),
                child: Text(
                  'trace: ${api!.traceId}',
                  style: const TextStyle(fontSize: 11, color: Colors.grey),
                ),
              ),
            if (canRetry) ...[
              const SizedBox(height: 16),
              FilledButton(onPressed: onRetry, child: const Text('重试')),
            ],
          ],
        ),
      ),
    );
  }
}

/// 网络自检页：把 T-01/T-02 的验收项可视化，便于真机取证。
///
/// 检查项：后端基址 / `/health` / 视频列表 / 签名播放地址（FLT-14 前提）。
class DiagPage extends ConsumerStatefulWidget {
  const DiagPage({super.key});

  @override
  ConsumerState<DiagPage> createState() => _DiagPageState();
}

class _DiagPageState extends ConsumerState<DiagPage> {
  final List<_DiagRow> _rows = [];
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
      setState(() => _rows.add(_DiagRow(name: name, ok: ok, detail: detail)));
    }

    add('后端基址', true, client.baseUrl);

    // 1) /health
    try {
      final data = await client.get('/health');
      final m = data is Map ? data : const {};
      final ready = m['ready'] == true;
      add('/health', ready, 'ready=$ready mysql=${(m['components'] as Map?)?['mysql']}');
    } catch (e) {
      add('/health', false, '$e');
    }

    // 2) 视频列表（真实契约：{list,total}）
    try {
      final paged = await api.list(page: 1, pageSize: 3);
      add('视频列表', paged.list.isNotEmpty, 'count=${paged.list.length} total=${paged.total}');
      if (paged.list.isNotEmpty) {
        // 3) 签名播放地址（data.streams[].url，带 e=/s=）
        final streams = await api.streams(paged.list.first.bvid);
        final hls = streams.where((s) => s.format == 'hls');
        final sample = hls.isNotEmpty ? hls.first : (streams.isEmpty ? null : streams.first);
        final signed = sample != null && sample.url.contains('e=') && sample.url.contains('s=');
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
        padding: const EdgeInsets.all(16),
        children: [
          if (_running) const LinearProgressIndicator(),
          const SizedBox(height: 8),
          for (final r in _rows)
            ListTile(
              dense: true,
              leading: Icon(
                r.ok ? Icons.check_circle : Icons.error,
                color: r.ok ? Colors.green : Colors.red,
              ),
              title: Text(r.name),
              subtitle: Text(
                r.detail,
                style: const TextStyle(fontFamily: 'monospace', fontSize: 11),
              ),
            ),
        ],
      ),
    );
  }
}

class _DiagRow {
  _DiagRow({required this.name, required this.ok, required this.detail});
  final String name;
  final bool ok;
  final String detail;
}

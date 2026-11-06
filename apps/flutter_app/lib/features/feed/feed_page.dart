/// 首页：分区栏 + 推荐/最新/最热 + 双列信息流。
///
/// **布局逐项对齐鸿蒙端 `pages/home/HomePage.ets`**：
/// - 双列 `columnsTemplate('1fr 1fr')` → Flutter 用 `SliverGrid` 两列
/// - columnsGap = DliSpace.md(12)、rowsGap = DliSpace.lg(16)、页面左右 padding = DliSpace.page(12)
/// - 分区栏横向滚动 + 排序 Tab（首页才有「推荐」，选了分区就没有推荐，与鸿蒙同口径）
/// - 顶部极淡品牌洗色（鸿蒙 AmbientBackdrop；Flutter 无系统材质，降级为静态渐变）
library;

import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../app/providers.dart';
import '../../core/api_error.dart';
import '../../core/theme.dart';
import '../../data/models/video.dart';
import '../dev/diag_page.dart';
import '../player/player_probe_page.dart';
import 'widgets/video_card_item.dart';

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
    if (_scroll.position.pixels >= _scroll.position.maxScrollExtent - 400) {
      ref.read(feedProvider.notifier).loadMore();
    }
  }

  @override
  Widget build(BuildContext context) {
    final async = ref.watch(feedProvider);

    return Container(
      // 极淡品牌洗色氛围底（对齐鸿蒙 AmbientBackdrop 的视觉意图；
      // 鸿蒙用实时模糊 + 系统材质，Flutter 无此能力，降级为静态线性渐变）
      decoration: const BoxDecoration(
        gradient: LinearGradient(
          begin: Alignment.topCenter,
          end: Alignment.bottomCenter,
          colors: [Color(0xFFFFF5F8), DliColors.bgPage],
          stops: [0.0, 0.35],
        ),
      ),
      child: AnnotatedRegion<SystemUiOverlayStyle>(
        // 页面底色是浅色（极淡品牌洗色），状态栏图标/文字必须用深色，
        // 否则浅底 + 浅色图标会糊成一片（首版在真机截图上就是这个现象）。
        value: const SystemUiOverlayStyle(
          statusBarColor: Colors.transparent,
          statusBarIconBrightness: Brightness.dark, // Android
          statusBarBrightness: Brightness.light, // iOS
        ),
        child: SafeArea(
          bottom: false,
          child: Column(
            children: [
              _Header(
                onRefresh: () => ref.read(feedProvider.notifier).loadFirstPage(),
                onDiag: () => Navigator.of(context).push(
                  MaterialPageRoute<void>(builder: (_) => const DiagPage()),
                ),
              ),
              Expanded(
                child: async.when(
                  loading: () => const Center(
                    child: CircularProgressIndicator(color: DliColors.primary),
                  ),
                  error: (err, _) => _ErrorView(
                    error: err,
                    onRetry: () => ref.read(feedProvider.notifier).loadFirstPage(),
                  ),
                  data: (state) => _FeedGrid(
                    state: state,
                    controller: _scroll,
                    onRefresh: () => ref.read(feedProvider.notifier).loadFirstPage(),
                  ),
                ),
              ),
            ],
          ),
        ),
      ),
    );
  }
}

/// 顶部：品牌标题 + 分区栏 + 排序 Tab。
class _Header extends ConsumerWidget {
  const _Header({required this.onRefresh, required this.onDiag});

  final VoidCallback onRefresh;
  final VoidCallback onDiag;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final feed = ref.watch(feedProvider).valueOrNull;
    final selectedCategory = ref.watch(selectedCategoryProvider);
    final sort = ref.watch(sortProvider);
    final categoriesAsync = ref.watch(categoriesProvider);

    return Container(
      padding: const EdgeInsets.only(
        left: DliSpace.page,
        right: DliSpace.page,
        top: DliSpace.sm,
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              const Text('DliDli', style: DliTextStyles.title),
              const Spacer(),
              IconButton(
                tooltip: '网络自检',
                icon: const Icon(Icons.wifi_tethering, size: 22),
                color: DliColors.text2,
                onPressed: onDiag,
              ),
              IconButton(
                tooltip: '刷新',
                icon: const Icon(Icons.refresh, size: 22),
                color: DliColors.text2,
                onPressed: onRefresh,
              ),
            ],
          ),
          // 分区栏：横向滚动、无滚动条（对齐鸿蒙 scrollable(Horizontal) + BarState.Off）
          SizedBox(
            height: 34,
            child: categoriesAsync.when(
              loading: () => const SizedBox.shrink(),
              error: (_, __) => const SizedBox.shrink(),
              data: (cats) {
                return ListView.separated(
                  scrollDirection: Axis.horizontal,
                  itemCount: cats.length + 1,
                  separatorBuilder: (_, __) => const SizedBox(width: DliSpace.lg),
                  itemBuilder: (context, i) {
                    final isAll = i == 0;
                    final id = isAll ? 0 : cats[i - 1].id;
                    final label = isAll ? '首页' : cats[i - 1].name;
                    return _CategoryChip(
                      label: label,
                      active: selectedCategory == id,
                      onTap: () =>
                          ref.read(selectedCategoryProvider.notifier).state = id,
                    );
                  },
                );
              },
            ),
          ),
          const SizedBox(height: DliSpace.sm),
          // 排序 Tab：首页(全站)才有「推荐」，选了分区就只有 最新/最热（与鸿蒙同口径）
          Row(
            children: [
              if (selectedCategory == 0)
                _SortTab(
                  label: '推荐',
                  active: sort == FeedSort.recommend,
                  onTap: () =>
                      ref.read(sortProvider.notifier).state = FeedSort.recommend,
                ),
              _SortTab(
                label: '最新',
                active: sort == FeedSort.newest,
                onTap: () => ref.read(sortProvider.notifier).state = FeedSort.newest,
              ),
              _SortTab(
                label: '最热',
                active: sort == FeedSort.hot,
                onTap: () => ref.read(sortProvider.notifier).state = FeedSort.hot,
              ),
              const Spacer(),
              if (feed != null) Text('共 ${feed.items.length} 项', style: DliTextStyles.caption),
            ],
          ),
          const SizedBox(height: DliSpace.sm),
        ],
      ),
    );
  }
}

class _CategoryChip extends StatelessWidget {
  const _CategoryChip({required this.label, required this.active, required this.onTap});

  final String label;
  final bool active;
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) {
    return GestureDetector(
      onTap: onTap,
      behavior: HitTestBehavior.opaque,
      child: Center(
        child: Text(
          label,
          style: TextStyle(
            fontSize: DliFont.bodySm,
            // 品牌粉只用在选中态（对齐鸿蒙「品牌色只留选中态」的约束）
            color: active ? DliColors.primary : DliColors.text2,
            fontWeight: active ? FontWeight.w600 : FontWeight.w400,
          ),
        ),
      ),
    );
  }
}

class _SortTab extends StatelessWidget {
  const _SortTab({required this.label, required this.active, required this.onTap});

  final String label;
  final bool active;
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) {
    return GestureDetector(
      onTap: onTap,
      behavior: HitTestBehavior.opaque,
      child: Padding(
        padding: const EdgeInsets.only(right: DliSpace.lg, top: 2, bottom: 2),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            Text(
              label,
              style: TextStyle(
                fontSize: DliFont.bodySm,
                color: active ? DliColors.text1 : DliColors.text3,
                fontWeight: active ? FontWeight.w600 : FontWeight.w400,
              ),
            ),
            const SizedBox(height: 3),
            AnimatedContainer(
              duration: const Duration(milliseconds: 150),
              width: active ? 16 : 0,
              height: 2.5,
              decoration: const BoxDecoration(
                color: DliColors.primary,
                borderRadius: BorderRadius.all(Radius.circular(DliRadius.full)),
              ),
            ),
          ],
        ),
      ),
    );
  }
}

/// 双列信息流。
class _FeedGrid extends StatelessWidget {
  const _FeedGrid({
    required this.state,
    required this.controller,
    required this.onRefresh,
  });

  final FeedState state;
  final ScrollController controller;
  final Future<void> Function() onRefresh;

  @override
  Widget build(BuildContext context) {
    if (state.items.isEmpty) {
      return const Center(child: Text('暂无内容', style: DliTextStyles.caption));
    }

    return RefreshIndicator(
      onRefresh: onRefresh,
      color: DliColors.primary,
      child: CustomScrollView(
        controller: controller,
        slivers: [
          SliverPadding(
            padding: const EdgeInsets.symmetric(horizontal: DliSpace.page),
            sliver: SliverGrid(
              // 对齐鸿蒙 Grid：两列等宽、列间距 12、行间距 16
              //
              // **用 mainAxisExtent 而不是 childAspectRatio**：封面高度随屏宽线性变化
              //（宽 * 9/16），而文本块高度基本固定，固定宽高比在窄屏/宽屏都会算出偏差，
              // 表现为卡片底部留白（首版 0.68、0.94、0.97 三轮截图均可见）。
              // 这里按「实际可用宽度 → 封面高 + 固定文本块高」精确算出格子高度。
              gridDelegate: SliverGridDelegateWithFixedCrossAxisCount(
                crossAxisCount: 2,
                crossAxisSpacing: DliSpace.md,
                mainAxisSpacing: DliSpace.lg,
                mainAxisExtent: _cardExtent(context),
              ),
              delegate: SliverChildBuilderDelegate(
                (context, i) {
                  final v = state.items[i];
                  return VideoCardItem(
                    video: v,
                    showDate: true,
                    onTap: (card) => _openVideo(context, card),
                  );
                },
                childCount: state.items.length,
              ),
            ),
          ),
          SliverToBoxAdapter(
            child: _LoadMoreHint(loading: state.loadingMore, done: !state.hasMore),
          ),
          const SliverToBoxAdapter(child: SizedBox(height: DliSpace.xl)),
        ],
      ),
    );
  }

  void _openVideo(BuildContext context, VideoCard card) {
    // T-10 阶段：点卡片进入播放器选型对比页（同一片源跑两个播放器）。
    // T-11 落地播放页后，这里改为进入正式播放页。
    showDialog<void>(
      context: context,
      barrierDismissible: true,
      builder: (_) => const Center(
        child: CircularProgressIndicator(color: DliColors.primary),
      ),
    );
    _openProbe(context, card);
  }

  Future<void> _openProbe(BuildContext context, VideoCard card) async {
    // 用 ProviderScope 容器直接读取（本类不是 ConsumerWidget，拿不到 ref）
    final container = ProviderScope.containerOf(context, listen: false);
    try {
      final streams = await container.read(videoApiProvider).streams(card.bvid);
      if (!context.mounted) return;
      Navigator.of(context).pop(); // 关掉 loading
      final hls = streams.where((s) => s.format == 'hls').toList();
      if (hls.isEmpty) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text('该稿件的 HLS 片源缺失：${card.title}')),
        );
        return;
      }
      // 取最高清晰度做对比（更能暴露解码压力）
      hls.sort((a, b) => b.quality.compareTo(a.quality));
      await Navigator.of(context).push(
        MaterialPageRoute<void>(
          builder: (_) => PlayerProbePage(
            hlsUrl: hls.first.url,
            title: '${card.title}  (${hls.first.qualityLabel})',
          ),
        ),
      );
    } catch (e) {
      if (!context.mounted) return;
      Navigator.of(context).pop();
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(content: Text('获取播放地址失败：$e')),
      );
    }
  }

  /// 计算卡片格子高度：封面(宽*9/16) + 文本块 + 卡片内边距。
  ///
  /// 文本块构成（与 VideoCardItem 保持一致）：
  ///   标题两行 13*1.35*2 ≈ 35 + 间距 2 + meta 11*1.2 ≈ 14 ≈ 51
  /// 卡片内边距上下 8*2 = 16；封面与文本间距 4。
  double _cardExtent(BuildContext context) {
    final screenWidth = MediaQuery.of(context).size.width;
    // 两列 + 页面左右 padding + 列间距
    final cardWidth =
        (screenWidth - DliSpace.page * 2 - DliSpace.md) / 2;
    const textBlock = 52.0;
    const paddings = DliSpace.sm * 2 + DliSpace.xs;
    return cardWidth * 9 / 16 + textBlock + paddings;
  }
}

class _LoadMoreHint extends StatelessWidget {
  const _LoadMoreHint({required this.loading, required this.done});

  final bool loading;
  final bool done;

  @override
  Widget build(BuildContext context) {
    if (!loading && !done) return const SizedBox(height: DliSpace.lg);
    return Padding(
      padding: const EdgeInsets.symmetric(vertical: DliSpace.lg),
      child: Center(
        child: loading
            ? const SizedBox(
                width: 18,
                height: 18,
                child: CircularProgressIndicator(strokeWidth: 2, color: DliColors.primary),
              )
            : const Text('没有更多了', style: DliTextStyles.caption),
      ),
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
        padding: const EdgeInsets.all(DliSpace.xl),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            const Icon(Icons.cloud_off, size: 40, color: DliColors.text3),
            const SizedBox(height: DliSpace.md),
            Text(msg, textAlign: TextAlign.center, style: DliTextStyles.body),
            if (api?.traceId != null)
              Padding(
                padding: const EdgeInsets.only(top: DliSpace.xs),
                child: Text('trace: ${api!.traceId}', style: DliTextStyles.caption),
              ),
            if (canRetry) ...[
              const SizedBox(height: DliSpace.lg),
              FilledButton(onPressed: onRetry, child: const Text('重试')),
            ],
          ],
        ),
      ),
    );
  }
}

/// Riverpod 依赖装配：把 ApiClient / TokenStore / 各 API 串起来。
library;

import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../core/api_client.dart';
import '../core/secure_token_store.dart';
import '../core/token_store.dart';
import '../data/api/video_api.dart';
import '../data/models/video.dart';

/// 令牌存储。测试时用 `overrideWithValue(InMemoryTokenStore())` 替换。
final tokenStoreProvider = Provider<TokenStore>((ref) => SecureTokenStore());

final apiClientProvider = Provider<ApiClient>((ref) {
  return ApiClient(tokenStore: ref.watch(tokenStoreProvider));
});

final videoApiProvider = Provider<VideoApi>((ref) {
  return VideoApi(ref.watch(apiClientProvider));
});

/// 排序口径。对齐鸿蒙 `SORT_RECOMMEND/SORT_NEW/SORT_HOT`：
/// 后端 `sort` 取值 `recommend` / `new` / `hot`。
enum FeedSort {
  recommend('recommend', '推荐'),
  newest('new', '最新'),
  hot('hot', '最热');

  const FeedSort(this.wire, this.label);
  final String wire;
  final String label;
}

/// 当前选中的分区（0 = 全站/首页）。
final selectedCategoryProvider = StateProvider<int>((ref) => 0);

/// 当前排序。
final sortProvider = StateProvider<FeedSort>((ref) => FeedSort.recommend);

/// 分区列表（横向分区栏）。
final categoriesProvider = FutureProvider<List<Category>>((ref) async {
  return ref.watch(videoApiProvider).categories();
});

/// 首页信息流（分页累加）。
///
/// 用 StateNotifier 而不是 FutureProvider：FLT-40 需要下拉刷新 + 触底加载，
/// 属于「可累加的分页状态」，不是一次性结果。
class FeedState {
  const FeedState({
    required this.items,
    required this.page,
    required this.hasMore,
    this.loadingMore = false,
  });

  final List<VideoCard> items;
  final int page;
  final bool hasMore;
  final bool loadingMore;

  FeedState copyWith({
    List<VideoCard>? items,
    int? page,
    bool? hasMore,
    bool? loadingMore,
  }) => FeedState(
    items: items ?? this.items,
    page: page ?? this.page,
    hasMore: hasMore ?? this.hasMore,
    loadingMore: loadingMore ?? this.loadingMore,
  );
}

class FeedNotifier extends StateNotifier<AsyncValue<FeedState>> {
  FeedNotifier({
    required VideoApi api,
    required FeedSort sort,
    required int categoryId,
  }) : _api = api,
       _sort = sort,
       _categoryId = categoryId,
       super(const AsyncValue.loading()) {
    loadFirstPage();
  }

  final VideoApi _api;
  final FeedSort _sort;
  final int _categoryId;
  static const _pageSize = 20;

  Future<void> loadFirstPage() async {
    state = const AsyncValue.loading();
    try {
      final paged = await _api.list(
        page: 1,
        pageSize: _pageSize,
        sort: _sort.wire,
        categoryId: _categoryId == 0 ? null : _categoryId,
      );
      state = AsyncValue.data(
        FeedState(
          items: paged.list,
          page: 1,
          // 后端列表接口不返回可靠 total，以「满页即可能还有」推断（与鸿蒙/Web 同口径）
          hasMore: paged.list.length >= _pageSize,
        ),
      );
    } catch (e, st) {
      state = AsyncValue.error(e, st);
    }
  }

  /// 触底加载下一页。已在加载或没有更多时直接返回（防重复请求）。
  Future<void> loadMore() async {
    final cur = state.valueOrNull;
    if (cur == null || cur.loadingMore || !cur.hasMore) return;
    state = AsyncValue.data(cur.copyWith(loadingMore: true));
    try {
      final nextPage = cur.page + 1;
      final paged = await _api.list(
        page: nextPage,
        pageSize: _pageSize,
        sort: _sort.wire,
        categoryId: _categoryId == 0 ? null : _categoryId,
      );
      state = AsyncValue.data(
        cur.copyWith(
          items: [...cur.items, ...paged.list],
          page: nextPage,
          hasMore: paged.list.length >= _pageSize,
          loadingMore: false,
        ),
      );
    } catch (e) {
      // 加载更多失败：保留已有内容，只退出 loadingMore 态（不整页报错）
      state = AsyncValue.data(cur.copyWith(loadingMore: false));
      // ignore: avoid_print
      print('[feed] loadMore failed: $e');
    }
  }
}

/// 信息流：**依赖排序与分区**，切换时自动重建并重新拉第一页。
final feedProvider =
    StateNotifierProvider.autoDispose<FeedNotifier, AsyncValue<FeedState>>((
      ref,
    ) {
      return FeedNotifier(
        api: ref.watch(videoApiProvider),
        sort: ref.watch(sortProvider),
        categoryId: ref.watch(selectedCategoryProvider),
      );
    });

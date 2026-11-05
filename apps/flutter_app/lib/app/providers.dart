/// Riverpod 依赖装配：把 ApiClient / TokenStore / 各 API 串起来。
library;

import 'package:flutter/foundation.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../core/api_client.dart';
import '../core/secure_token_store.dart';
import '../core/token_store.dart';
import '../data/api/video_api.dart';

/// 令牌存储。测试时用 `overrideWithValue(InMemoryTokenStore())` 替换。
final tokenStoreProvider = Provider<TokenStore>((ref) => SecureTokenStore());

final apiClientProvider = Provider<ApiClient>((ref) {
  return ApiClient(tokenStore: ref.watch(tokenStoreProvider));
});

final videoApiProvider = Provider<VideoApi>((ref) {
  return VideoApi(ref.watch(apiClientProvider));
});

/// 首页信息流（分页累加）。
///
/// 用 `AsyncNotifier` 而不是 FutureProvider：FLT-40 需要下拉刷新 + 触底加载，
/// 属于「可累加的分页状态」，不是一次性结果。
class FeedState {
  const FeedState({
    required this.items,
    required this.page,
    required this.hasMore,
    this.loadingMore = false,
  });

  final List<dynamic> items;
  final int page;
  final bool hasMore;
  final bool loadingMore;

  FeedState copyWith({
    List<dynamic>? items,
    int? page,
    bool? hasMore,
    bool? loadingMore,
  }) =>
      FeedState(
        items: items ?? this.items,
        page: page ?? this.page,
        hasMore: hasMore ?? this.hasMore,
        loadingMore: loadingMore ?? this.loadingMore,
      );
}

class FeedNotifier extends StateNotifier<AsyncValue<FeedState>> {
  FeedNotifier(this._api) : super(const AsyncValue.loading()) {
    loadFirstPage();
  }

  final VideoApi _api;
  static const _pageSize = 20;

  Future<void> loadFirstPage() async {
    state = const AsyncValue.loading();
    try {
      final paged = await _api.list(page: 1, pageSize: _pageSize);
      state = AsyncValue.data(
        FeedState(
          items: paged.list,
          page: 1,
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
      final paged = await _api.list(page: nextPage, pageSize: _pageSize);
      state = AsyncValue.data(
        cur.copyWith(
          items: [...cur.items, ...paged.list],
          page: nextPage,
          // 后端 list 接口不一定回 total，故用「满页即可能还有」的保守口径
          hasMore: paged.list.length >= _pageSize,
          loadingMore: false,
        ),
      );
    } catch (e) {
      // 加载更多失败：保留已有内容，只退出 loadingMore 态（不整页报错）
      state = AsyncValue.data(cur.copyWith(loadingMore: false));
      // 不用 AsyncValue.error 传：那会让整页变错误态，现有内容被清掉。
      debugPrint('[feed] loadMore failed: $e');
    }
  }
}

final feedProvider = StateNotifierProvider<FeedNotifier, AsyncValue<FeedState>>(
  (ref) => FeedNotifier(ref.watch(videoApiProvider)),
);

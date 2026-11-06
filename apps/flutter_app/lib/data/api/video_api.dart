/// 视频接口：直接复用既有后端契约，无需新增接口（spec §5）。
library;

import '../../core/api_client.dart';
import '../../core/api_error.dart';
import '../models/video.dart';

class VideoApi {
  VideoApi(this._client);

  final ApiClient _client;

  /// 分区列表（首页横向分区栏）。后端返回 `{list:[...]}`。
  Future<List<Category>> categories() async {
    final data = await _client.get('/api/v1/categories');
    final raw = (data is Map<String, dynamic>) ? data['list'] : data;
    if (raw is! List) return const [];
    return raw
        .whereType<Map<String, dynamic>>()
        .map(Category.fromJson)
        .toList(growable: false);
  }

  /// 首页/列表。后端返回 `{list, total}`。
  Future<PagedVideos> list({
    int page = 1,
    int pageSize = 20,
    String? sort,
    int? categoryId,
  }) async {
    final data = await _client.get(
      '/api/v1/videos',
      query: {
        'page': page,
        'page_size': pageSize,
        if (sort != null) 'sort': sort,
        if (categoryId != null) 'category_id': categoryId,
      },
    );
    if (data is! Map<String, dynamic>) return const PagedVideos(list: [], total: 0);
    return PagedVideos.fromJson(data);
  }

  /// 详情（含签名播放地址 streams[]）。
  Future<List<VideoStream>> streams(String bvid) async {
    final data = await _client.get('/api/v1/videos/$bvid');
    if (data is! Map<String, dynamic>) return const [];
    final raw = data['streams'];
    if (raw is! List) return const [];
    return raw
        .whereType<Map<String, dynamic>>()
        .map(VideoStream.fromJson)
        .toList(growable: false);
  }

  /// 稿件详情（含标题、UP 主、互动计数、简介、播放流）。
  Future<VideoCard> detail(String bvid) async {
    final data = await _client.get('/api/v1/videos/$bvid');
    if (data is! Map<String, dynamic>) {
      throw ApiError(kind: ApiErrorKind.malformed, message: '稿件详情为空');
    }
    return VideoCard.fromJson(data);
  }

  /// 读取播放进度（秒）。需登录；未登录时服务端返回业务错误，调用方应静默忽略。
  Future<int> progress(String bvid) async {
    final data = await _client.get('/api/v1/videos/$bvid/progress');
    if (data is! Map<String, dynamic>) return 0;
    return (data['position'] as num?)?.toInt() ?? 0;
  }

  /// 保存播放进度（秒）。需登录。
  Future<void> saveProgress(String bvid, int position) async {
    await _client.post('/api/v1/videos/$bvid/progress', body: {'position': position});
  }
}

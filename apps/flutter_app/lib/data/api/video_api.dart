/// 视频接口：直接复用既有后端契约，无需新增接口（spec §5）。
library;

import '../../core/api_client.dart';
import '../models/video.dart';

class VideoApi {
  VideoApi(this._client);

  final ApiClient _client;

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
}

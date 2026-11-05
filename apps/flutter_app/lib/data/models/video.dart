/// 视频相关数据模型（对应后端 /api/v1/videos 系列）。
///
/// 注：只声明端侧当前真正消费的字段；新增字段需与后端契约同步（SDD：先改 spec）。
library;

import '../../core/url_rewriter.dart';

/// 视频列表项（卡片）。
class VideoCard {
  const VideoCard({
    required this.bvid,
    required this.title,
    required this.cover,
    required this.duration,
    required this.ownerName,
    this.viewCount = 0,
    this.publishedAt,
    this.categoryId,
  });

  final String bvid;
  final String title;
  final String cover;
  final int duration;
  final String ownerName;
  final int viewCount;

  /// 发布日期（信息流卡片 meta 行用；鸿蒙侧同样取 published_at ?? created_at）。
  final DateTime? publishedAt;

  final int? categoryId;

  factory VideoCard.fromJson(Map<String, dynamic> json) {
    final owner = json['owner'];
    final stat = json['stat'];
    return VideoCard(
      bvid: (json['bvid'] as String?) ?? '',
      title: (json['title'] as String?) ?? '',
      // 后端下发的是服务端视角地址（localhost），端机不可路由 → 必须重写
      cover: UrlRewriter.rewrite((json['cover'] as String?) ?? ''),
      duration: (json['duration'] as num?)?.toInt() ?? 0,
      ownerName: (owner is Map ? owner['nickname'] as String? : null) ?? '',
      viewCount: (stat is Map ? (stat['view'] as num?)?.toInt() : null) ?? 0,
      publishedAt: _parseTime(json['published_at'] ?? json['created_at']),
      categoryId: (json['category_id'] as num?)?.toInt(),
    );
  }

  /// 后端时间形如 `2026-09-09T16:58:39+08:00`；解析失败返回 null（不让一条脏数据搞崩列表）。
  static DateTime? _parseTime(Object? v) {
    if (v is! String || v.isEmpty) return null;
    return DateTime.tryParse(v);
  }
}

/// 分区（首页横向分区栏）。后端 `/api/v1/categories` 返回。
///
/// 对齐鸿蒙 `model/Video.ets` 的 `Category`：`parent_id == 0` 为顶级分区。
class Category {
  const Category({required this.id, required this.name, this.parentId});

  final int id;
  final String name;
  final int? parentId;

  bool get isTopLevel => (parentId ?? 0) == 0;

  factory Category.fromJson(Map<String, dynamic> json) => Category(
        id: (json['id'] as num?)?.toInt() ?? 0,
        name: (json['name'] as String?) ?? '',
        parentId: (json['parent_id'] as num?)?.toInt(),
      );
}

/// 分页结果。
class PagedVideos {
  const PagedVideos({required this.list, required this.total});

  final List<VideoCard> list;
  final int total;

  factory PagedVideos.fromJson(Map<String, dynamic> json) {
    final raw = json['list'];
    final list = <VideoCard>[];
    if (raw is List) {
      for (final e in raw) {
        if (e is Map<String, dynamic>) list.add(VideoCard.fromJson(e));
      }
    }
    return PagedVideos(
      list: list,
      total: (json['total'] as num?)?.toInt() ?? list.length,
    );
  }
}

/// 播放流（清晰度档位）。真实契约是 `data.streams[]`，**不是** `parts[].play_url`。
class VideoStream {
  const VideoStream({
    required this.quality,
    required this.format,
    required this.url,
  });

  final int quality;
  final String format;
  final String url;

  /// 清晰度中文标签（0 原画 / 360 / 720 / 1080 / 2160）。
  String get qualityLabel {
    switch (quality) {
      case 0:
        return '原画';
      case 2160:
        return '4K';
      default:
        return '${quality}P';
    }
  }

  factory VideoStream.fromJson(Map<String, dynamic> json) => VideoStream(
        quality: (json['quality'] as num?)?.toInt() ?? 0,
        format: (json['format'] as String?) ?? '',
        // 播放地址同样是服务端视角的 localhost，且带签名参数（e/q/u/s），
        // 重写只换 host:port，绝不重排 query —— 否则签名校验失败。
        url: UrlRewriter.rewrite((json['url'] as String?) ?? ''),
      );
}

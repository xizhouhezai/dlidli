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
    this.stat = const VideoStat(),
    this.description = '',
  });

  final String bvid;
  final String title;
  final String cover;
  final int duration;
  final String ownerName;
  final int viewCount;

  /// 互动计数（播放页展示播放/点赞/投币）。
  final VideoStat stat;

  /// 稿件简介。
  final String description;

  /// 发布日期（信息流卡片 meta 行用；鸿蒙侧同样取 published_at ?? created_at）。
  final DateTime? publishedAt;

  final int? categoryId;

  factory VideoCard.fromJson(Map<String, dynamic> json) {
    final owner = json['owner'];
    final statJson = json['stat'];
    final stat = statJson is Map
        ? VideoStat.fromJson(statJson.cast<String, dynamic>())
        : const VideoStat();
    return VideoCard(
      bvid: (json['bvid'] as String?) ?? '',
      title: (json['title'] as String?) ?? '',
      // 后端下发的是服务端视角地址（localhost），端机不可路由 → 必须重写
      cover: UrlRewriter.rewrite((json['cover'] as String?) ?? ''),
      duration: (json['duration'] as num?)?.toInt() ?? 0,
      ownerName: (owner is Map ? owner['nickname'] as String? : null) ?? '',
      viewCount: stat.view,
      stat: stat,
      description: (json['description'] as String?) ?? '',
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

/// 互动计数（后端 `stat` 对象：view/like/coin/fav/danmaku/comment/share）。
class VideoStat {
  const VideoStat({
    this.view = 0,
    this.like = 0,
    this.coin = 0,
    this.fav = 0,
    this.danmaku = 0,
    this.comment = 0,
    this.share = 0,
  });

  final int view;
  final int like;
  final int coin;
  final int fav;
  final int danmaku;
  final int comment;
  final int share;

  factory VideoStat.fromJson(Map<String, dynamic> json) => VideoStat(
        view: (json['view'] as num?)?.toInt() ?? 0,
        like: (json['like'] as num?)?.toInt() ?? 0,
        coin: (json['coin'] as num?)?.toInt() ?? 0,
        fav: (json['fav'] as num?)?.toInt() ?? 0,
        danmaku: (json['danmaku'] as num?)?.toInt() ?? 0,
        comment: (json['comment'] as num?)?.toInt() ?? 0,
        share: (json['share'] as num?)?.toInt() ?? 0,
      );
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
    this.isVipOnly = false,
  });

  final int quality;
  final String format;
  final String url;

  /// 是否会员限定档位（服务端按用户身份下发时带 `vip_only` 标记）。
  final bool isVipOnly;

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
        isVipOnly: json['vip_only'] == true || json['is_vip_only'] == true,
        // 播放地址同样是服务端视角的 localhost，且带签名参数（e/q/u/s），
        // 重写只换 host:port，绝不重排 query —— 否则签名校验失败。
        url: UrlRewriter.rewrite((json['url'] as String?) ?? ''),
      );
}

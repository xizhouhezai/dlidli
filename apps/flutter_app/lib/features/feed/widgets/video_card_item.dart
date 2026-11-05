/// 视频卡片：**版式逐项对齐鸿蒙端 `components/VideoCardItem.ets`**。
///
/// 对齐项（不要随意改动，改了两端就不像了）：
/// - 封面 16:9，圆角 DliRadius.md（8）
/// - 时长角标：右下、CAPTION(11) 字号、白字、`#99000000` 底、圆角 4、距右下各 6
/// - 标题：BODY_SM(13)、两行截断、text_primary
/// - meta：CAPTION(11)、text_tertiary、单行截断，内容为「播放量 · UP 主 [· 日期]」
/// - 卡片：内边距 SM(8)、圆角 LG(12)、表面色（鸿蒙是系统材质，Flutter 降级为纯色 surface）
library;

import 'package:flutter/material.dart';

import '../../../core/theme.dart';
import '../../../core/url_rewriter.dart';
import '../../../data/models/video.dart';

class VideoCardItem extends StatelessWidget {
  const VideoCardItem({
    super.key,
    required this.video,
    this.showDate = false,
    this.onTap,
  });

  final VideoCard video;

  /// 是否在 meta 行追加发布日期（信息流为 true，搜索结果可为 false）。
  final bool showDate;

  final ValueChanged<VideoCard>? onTap;

  String get _metaText {
    final parts = <String>[DliFormat.count(video.viewCount)];
    if (video.ownerName.isNotEmpty) parts.add(video.ownerName);
    if (showDate) {
      final d = video.publishedAt;
      if (d != null) {
        final s = '${d.year}-${d.month.toString().padLeft(2, '0')}';
        parts.add(s);
      }
    }
    return parts.join(' · ');
  }

  @override
  Widget build(BuildContext context) {
    return GestureDetector(
      onTap: onTap == null ? null : () => onTap!(video),
      child: Container(
        padding: const EdgeInsets.all(DliSpace.sm),
        decoration: const BoxDecoration(
          // 鸿蒙：backgroundColor(DliMaterial.surface()) + systemMaterial(card)
          // Flutter：无系统材质能力（FLT-05），降级为纯色表面 + 圆角
          color: DliColors.bgSurface,
          borderRadius: BorderRadius.all(Radius.circular(DliRadius.lg)),
        ),
        // 卡片内容按"封面 + 固定高度文本块"排布，文本块高度固定，
        // 这样在 SliverGrid 的固定宽高比格子里不会出现底部大片空白
        //（首版用 Column + MainAxisSize.min，格子比内容高就留白，真机截图可见）。
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            _cover(),
            const SizedBox(height: DliSpace.xs),
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                mainAxisAlignment: MainAxisAlignment.start,
                children: [
                  Text(
                    video.title,
                    maxLines: 2,
                    overflow: TextOverflow.ellipsis,
                    style: DliTextStyles.cardTitle,
                  ),
                  const SizedBox(height: 2),
                  Text(
                    _metaText,
                    maxLines: 1,
                    overflow: TextOverflow.ellipsis,
                    style: DliTextStyles.caption,
                  ),
                ],
              ),
            ),
          ],
        ),
      ),
    );
  }

  Widget _cover() {
    final url = UrlRewriter.rewrite(video.cover);
    return ClipRRect(
      borderRadius: const BorderRadius.all(Radius.circular(DliRadius.md)),
      child: AspectRatio(
        aspectRatio: 16 / 9,
        child: Stack(
          fit: StackFit.expand,
          children: [
            if (url.isEmpty)
              _defaultCover()
            else
              Image.network(
                url,
                fit: BoxFit.cover,
                // 加载失败用品牌默认封面兜底（与鸿蒙端同口径）。
                // 实测踩到：库里存在封面记录但文件已丢失的稿件（如 VIP-02 验收样片），
                // 只画中性底色的话，用户看到的是"空白卡片"，与"本来没封面"无法区分。
                errorBuilder: (_, __, ___) => _defaultCover(),
                loadingBuilder: (_, child, progress) => progress == null
                    ? child
                    : const ColoredBox(color: DliColors.surfaceSecondary),
              ),
            if (video.duration > 0)
              Align(
                alignment: Alignment.bottomRight,
                child: Container(
                  margin: const EdgeInsets.only(right: 6, bottom: 6),
                  padding: const EdgeInsets.symmetric(horizontal: 6, vertical: 1),
                  decoration: const BoxDecoration(
                    color: DliColors.durationBadge,
                    borderRadius: BorderRadius.all(Radius.circular(4)),
                  ),
                  child: Text(
                    DliFormat.duration(video.duration),
                    style: const TextStyle(fontSize: DliFont.caption, color: Colors.white),
                  ),
                ),
              ),
          ],
        ),
      ),
    );
  }

  /// 品牌默认封面（与鸿蒙端 default_cover.png 同一份字节，md5 已核对）。
  Widget _defaultCover() {
    return Image.asset(
      'assets/images/default_cover.png',
      fit: BoxFit.cover,
      // 资产本身缺失时退回中性底色，避免连兜底都崩
      errorBuilder: (_, __, ___) => const ColoredBox(color: DliColors.surfaceSecondary),
    );
  }
}

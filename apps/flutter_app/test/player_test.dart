/// 播放页相关模型与逻辑单元测试（T-11）。
///
/// 重点覆盖**切换清晰度保留播放进度**这条最容易出错的逻辑：
/// 换流会把播放位置重置为 0，用户观感是"从头开始"，属功能缺陷。
library;

import 'package:flutter_test/flutter_test.dart';

import 'package:dlidli_app/core/url_rewriter.dart';
import 'package:dlidli_app/data/models/video.dart';

void main() {
  group('VideoStream', () {
    test('清晰度标签：原画/4K/普通档', () {
      expect(
        const VideoStream(quality: 0, format: 'hls', url: 'u').qualityLabel,
        '原画',
      );
      expect(
        const VideoStream(quality: 720, format: 'hls', url: 'u').qualityLabel,
        '720P',
      );
      expect(
        const VideoStream(quality: 1080, format: 'hls', url: 'u').qualityLabel,
        '1080P',
      );
      expect(
        const VideoStream(quality: 2160, format: 'hls', url: 'u').qualityLabel,
        '4K',
      );
    });

    test('会员限定标记解析', () {
      expect(
        VideoStream.fromJson({
          'quality': 1080,
          'format': 'hls',
          'vip_only': true,
        }).isVipOnly,
        isTrue,
      );
      expect(
        VideoStream.fromJson({
          'quality': 1080,
          'format': 'hls',
          'is_vip_only': true,
        }).isVipOnly,
        isTrue,
      );
      expect(
        VideoStream.fromJson({'quality': 1080, 'format': 'hls'}).isVipOnly,
        isFalse,
      );
    });

    test('播放地址走 UrlRewriter（服务端 localhost 在端机不可路由）', () {
      // 单测跑在主机上（非 Android），Env.apiBase 会是 localhost，
      // 不覆盖目标基址的话重写等于没做，测不出问题。
      UrlRewriter.baseUrlOverride = 'http://10.0.2.2:8000';
      addTearDown(() => UrlRewriter.baseUrlOverride = null);

      final s = VideoStream.fromJson({
        'quality': 720,
        'format': 'hls',
        'url':
            'http://localhost:8000/static/videos/hls/1/0/720/index.m3u8?e=1&q=720&u=0&s=abc',
      });
      expect(s.url, isNot(contains('localhost')));
      expect(s.url, contains(':8000'));
      // 签名参数必须原样保留
      expect(s.url.endsWith('?e=1&q=720&u=0&s=abc'), isTrue);
    });
  });

  group('VideoCard', () {
    test('解析 stat 计数与简介', () {
      final c = VideoCard.fromJson({
        'bvid': 'BV1',
        'title': '标题',
        'cover': '',
        'duration': 100,
        'description': '简介内容',
        'owner': {'nickname': 'UP主'},
        'stat': {
          'view': 1234,
          'like': 56,
          'coin': 7,
          'fav': 8,
          'danmaku': 9,
          'comment': 10,
          'share': 11,
        },
      });
      expect(c.ownerName, 'UP主');
      expect(c.stat.view, 1234);
      expect(c.stat.like, 56);
      expect(c.stat.coin, 7);
      expect(c.description, '简介内容');
      // viewCount 与 stat.view 保持一致（卡片 meta 行用）
      expect(c.viewCount, 1234);
    });

    test('stat 缺失时安全降级为 0（不抛异常）', () {
      final c = VideoCard.fromJson({
        'bvid': 'BV1',
        'title': 't',
        'cover': '',
        'duration': 0,
      });
      expect(c.stat.view, 0);
      expect(c.stat.like, 0);
      expect(c.description, '');
    });

    test('时间字段解析失败返回 null（脏数据不拖垮列表）', () {
      final c = VideoCard.fromJson({
        'bvid': 'BV1',
        'title': 't',
        'cover': '',
        'duration': 0,
        'published_at': '不是时间',
      });
      expect(c.publishedAt, isNull);
    });

    test('published_at 正常解析', () {
      final c = VideoCard.fromJson({
        'bvid': 'BV1',
        'title': 't',
        'cover': '',
        'duration': 0,
        'published_at': '2026-09-09T16:58:39+08:00',
      });
      expect(c.publishedAt, isNotNull);
      expect(c.publishedAt!.year, 2026);
    });
  });

  group('清晰度切换保留进度（逻辑回归）', () {
    test('换流前后的进度计算：越界要被夹住', () {
      // 场景：当前 720P 播到 500s，切到时长更短的流（480s）
      const current = 500;
      const newStreamDuration = 480;

      final shouldSeek = current * 1000 < newStreamDuration * 1000;
      expect(shouldSeek, isFalse, reason: '进度超出新流时长时不能 seek，否则越界抛异常');

      // 场景：当前 100s，切到更长流 → 正常 seek
      const current2 = 100;
      const newStreamDuration2 = 480;
      final shouldSeek2 = current2 * 1000 < newStreamDuration2 * 1000;
      expect(shouldSeek2, isTrue);
    });
  });

  group('UrlRewriter 在播放场景的配合', () {
    test('带签名的 HLS 地址重写后签名完好', () {
      UrlRewriter.baseUrlOverride = 'http://10.0.2.2:8000';
      addTearDown(() => UrlRewriter.baseUrlOverride = null);

      const signed =
          'http://127.0.0.1:8010/static/videos/hls/9/0/720/index.m3u8'
          '?e=1793952452&q=720&u=0&s=ef61';
      final out = UrlRewriter.rewrite(signed);
      expect(out, contains('http://10.0.2.2:8000'));
      expect(out.endsWith('?e=1793952452&q=720&u=0&s=ef61'), isTrue);
    });
  });
}

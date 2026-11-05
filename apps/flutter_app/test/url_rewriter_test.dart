/// URL 重写单元测试：保证「后端下发 localhost 地址」在端机可用，
/// 且**不改动签名 query**（FLT-14 的前提）。
library;

import 'package:flutter_test/flutter_test.dart';

import 'package:dlidli_app/core/url_rewriter.dart';

void main() {
  // 单测跑在主机上（非 Android），Env.apiBase 会是 localhost，
  // 故这里显式指定「端侧实际后端基址」，与真机（10.0.2.2）一致。
  setUpAll(() {
    UrlRewriter.baseUrlOverride = 'http://10.0.2.2:8000';
  });

  tearDownAll(() {
    UrlRewriter.baseUrlOverride = null;
  });

  group('UrlRewriter', () {
    test('localhost 主机被替换为端侧后端主机（保留端口与路径）', () {
      final out = UrlRewriter.rewrite('http://localhost:8000/static/covers/a.jpg');
      expect(out, contains('/static/covers/a.jpg'));
      expect(out, isNot(contains('localhost')));
      expect(out, contains(':8000'));
    });

    test('签名 query 原样保留（不重排、不重新编码）', () {
      const signed =
          'http://localhost:8000/static/videos/hls/1/0/720/index.m3u8?e=1793871003&q=720&u=0&s=8feddb5a1d4a';
      final out = UrlRewriter.rewrite(signed);
      expect(
        out.endsWith('?e=1793871003&q=720&u=0&s=8feddb5a1d4a'),
        isTrue,
        reason: '签名参数顺序与内容必须逐字一致，否则服务端验签失败',
      );
      expect(out, isNot(contains('localhost')));
    });

    test('127.0.0.1 / 0.0.0.0 同样被替换', () {
      for (final host in ['127.0.0.1', '0.0.0.0']) {
        final out = UrlRewriter.rewrite('http://$host:8000/static/x.jpg');
        expect(out, isNot(contains(host)), reason: '$host 应被替换');
      }
    });

    test('真实域名不被改写（生产环境不得掩盖服务端配置问题）', () {
      const real = 'https://cdn.dlidli.com/static/covers/a.jpg';
      expect(UrlRewriter.rewrite(real), real);
    });

    test('空串与相对路径原样返回', () {
      expect(UrlRewriter.rewrite(''), '');
      expect(UrlRewriter.rewrite('/static/a.jpg'), '/static/a.jpg');
    });

    test('无端口的 localhost 也能正确替换', () {
      final out = UrlRewriter.rewrite('http://localhost/static/a.jpg');
      expect(out, isNot(contains('localhost')));
      expect(out, contains('/static/a.jpg'));
    });
  });
}

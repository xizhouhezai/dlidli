/// 核心层单元测试：统一信封、错误分类、401 单飞续期。
///
/// 这些是 FLT-03 的验收依据 —— 不依赖真机/网络，`flutter test` 即可复跑。
///
/// 实现说明：用**替换 HttpClientAdapter** 的方式伪造传输层（dio 官方推荐做法）。
/// 不用拦截器伪造，因为拦截器 resolver.resolve 在部分路径下仍会走到真实网络兜底，
/// 导致「本该 401 却报网络错误」这种假失败。
library;

import 'dart:convert';
import 'dart:typed_data';

import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:dlidli_app/core/api_client.dart';
import 'package:dlidli_app/core/api_error.dart';
import 'package:dlidli_app/core/token_store.dart';

/// 可编程的假传输层：根据请求返回预设状态码与 JSON。
class FakeAdapter implements HttpClientAdapter {
  FakeAdapter(this.handler);

  final (int, Map<String, dynamic>) Function(RequestOptions o) handler;

  /// 记录每次请求的 Authorization 头，便于断言 token 注入与重放。
  final List<String?> seenAuth = [];
  final List<String> seenPaths = [];

  @override
  Future<ResponseBody> fetch(
    RequestOptions options,
    Stream<Uint8List>? requestStream,
    Future<void>? cancelFuture,
  ) async {
    seenPaths.add(options.path);
    seenAuth.add(options.headers['Authorization'] as String?);
    final (status, body) = handler(options);
    return ResponseBody.fromString(
      jsonEncode(body),
      status,
      headers: {
        Headers.contentTypeHeader: [Headers.jsonContentType],
      },
    );
  }

  @override
  void close({bool force = false}) {}
}

Dio dioWith(FakeAdapter adapter) {
  final dio = Dio(
    BaseOptions(
      baseUrl: 'http://test.local',
      validateStatus: (_) => true,
    ),
  );
  dio.httpClientAdapter = adapter;
  return dio;
}

void main() {
  group('ApiEnvelope', () {
    test('code=0 视为成功', () {
      final env = ApiEnvelope.fromJson({
        'code': 0,
        'message': 'ok',
        'data': {'a': 1},
        'trace_id': 't1',
      });
      expect(env.isOk, isTrue);
      expect(env.traceId, 't1');
    });

    test('code≠0 即使 HTTP 200 也视为失败（本项目关键口径）', () {
      final env = ApiEnvelope.fromJson({'code': 92303, 'message': '重复打款'});
      expect(env.isOk, isFalse);
      expect(env.code, 92303);
    });

    test('缺字段时安全降级，不抛异常', () {
      final env = ApiEnvelope.fromJson({});
      expect(env.code, -1);
      expect(env.message, '');
    });
  });

  group('ErrorMessages', () {
    test('已知业务码用映射文案', () {
      expect(ErrorMessages.forCode(20003, 'whatever'), '账号或密码错误');
    });

    test('未知业务码回退后端文案', () {
      expect(ErrorMessages.forCode(99999, '后端原始文案'), '后端原始文案');
    });

    test('未知业务码且无文案时兜底', () {
      expect(ErrorMessages.forCode(99999, ''), '请求失败（99999）');
    });
  });

  group('ApiClient 基本行为', () {
    test('成功响应直接返回 data', () async {
      final adapter = FakeAdapter((_) => (200, {
            'code': 0,
            'message': 'ok',
            'data': {'x': 42},
          }));
      final client = ApiClient(tokenStore: InMemoryTokenStore(), dio: dioWith(adapter));
      final data = await client.get('/anything');
      expect(data, {'x': 42});
    });

    test('业务失败抛 ApiError(business) 并带 traceId', () async {
      final adapter = FakeAdapter((_) => (200, {
            'code': 20003,
            'message': '账号或密码错误',
            'trace_id': 'trace-abc',
          }));
      final client = ApiClient(tokenStore: InMemoryTokenStore(), dio: dioWith(adapter));

      await expectLater(
        client.get('/x'),
        throwsA(
          isA<ApiError>()
              .having((e) => e.kind, 'kind', ApiErrorKind.business)
              .having((e) => e.code, 'code', 20003)
              .having((e) => e.traceId, 'traceId', 'trace-abc')
              .having((e) => e.retryable, 'retryable', isFalse),
        ),
      );
    });

    test('HTTP 5xx 归为 server 且可重试', () async {
      final adapter = FakeAdapter((_) => (500, {'code': 50000, 'message': '内部错误'}));
      final client = ApiClient(tokenStore: InMemoryTokenStore(), dio: dioWith(adapter));

      await expectLater(
        client.get('/x'),
        throwsA(
          isA<ApiError>()
              .having((e) => e.kind, 'kind', ApiErrorKind.server)
              .having((e) => e.retryable, 'retryable', isTrue),
        ),
      );
    });

    test('请求头自动带 Bearer token', () async {
      final store = InMemoryTokenStore();
      await store.save(accessToken: 'tk-123', refreshToken: 'rf-1');
      final adapter = FakeAdapter((_) => (200, {'code': 0, 'message': 'ok', 'data': null}));
      final client = ApiClient(tokenStore: store, dio: dioWith(adapter));

      await client.get('/x');
      expect(adapter.seenAuth.single, 'Bearer tk-123');
    });
  });

  group('401 单飞续期', () {
    test('并发 3 个请求只触发一次 refresh，且全部重放成功', () async {
      final store = InMemoryTokenStore();
      await store.save(accessToken: 'old', refreshToken: 'rf-old');

      var refreshCalls = 0;

      final mainAdapter = FakeAdapter((o) {
        final bearer = o.headers['Authorization'];
        if (bearer == 'Bearer old') {
          return (401, {'code': 20002, 'message': '令牌已过期'});
        }
        return (200, {'code': 0, 'message': 'ok', 'data': 'ok'});
      });

      final refreshAdapter = FakeAdapter((_) {
        refreshCalls++;
        return (200, {
          'code': 0,
          'message': 'ok',
          'data': {'access_token': 'new-token', 'refresh_token': 'rf-new'},
        });
      });

      final client = ApiClient(
        tokenStore: store,
        dio: dioWith(mainAdapter),
        refreshDio: dioWith(refreshAdapter),
      );

      final results = await Future.wait([
        client.get('/a'),
        client.get('/b'),
        client.get('/c'),
      ]);

      expect(results, everyElement('ok'));
      expect(refreshCalls, 1, reason: '并发 401 必须只续期一次（单飞），否则会互相踢掉');
      expect(await store.readAccessToken(), 'new-token');
      // 每个请求各打一次：首次 + 重放 = 6 次
      expect(mainAdapter.seenPaths.length, 6);
    });

    test('续期失败抛 unauthorized 并清空凭证', () async {
      final store = InMemoryTokenStore();
      await store.save(accessToken: 'old', refreshToken: 'rf-old');

      final mainAdapter = FakeAdapter((_) => (401, {'code': 20002, 'message': '令牌已过期'}));
      // refresh 端点也失败
      final refreshAdapter = FakeAdapter((_) => (200, {'code': 20002, 'message': '刷新令牌无效'}));

      final client = ApiClient(
        tokenStore: store,
        dio: dioWith(mainAdapter),
        refreshDio: dioWith(refreshAdapter),
      );

      await expectLater(
        client.get('/x'),
        throwsA(isA<ApiError>().having((e) => e.kind, 'kind', ApiErrorKind.unauthorized)),
      );
      expect(await store.readAccessToken(), isNull, reason: '续期失败必须清理本地凭证');
    });
  });
}

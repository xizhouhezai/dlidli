/// 鉴权状态机单元测试（T-03 / FLT-01、FLT-02）。
///
/// 重点覆盖两类「看起来成功、实际很伤」的行为：
/// 1. **网络异常不能把用户踢下线** —— 断网时若清理令牌，用户下次打开就得重新登录；
/// 2. **登出必须清本地** —— 服务端吊销失败也不能让用户「退不出去」。
library;

import 'dart:convert';
import 'dart:typed_data';

import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:dlidli_app/app/auth.dart';
import 'package:dlidli_app/core/api_client.dart';
import 'package:dlidli_app/core/api_error.dart';
import 'package:dlidli_app/core/token_store.dart';
import 'package:dlidli_app/data/api/auth_api.dart';

/// 可编程假传输层（替换 HttpClientAdapter，官方推荐做法）。
class FakeAdapter implements HttpClientAdapter {
  FakeAdapter(this.handler);

  final (int, Map<String, dynamic>) Function(RequestOptions o) handler;

  final List<String> seenPaths = [];

  @override
  Future<ResponseBody> fetch(
    RequestOptions options,
    Stream<Uint8List>? requestStream,
    Future<void>? cancelFuture,
  ) async {
    seenPaths.add(options.path);
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

Dio dioWith(HttpClientAdapter adapter) {
  final dio = Dio(BaseOptions(baseUrl: 'http://test.local', validateStatus: (_) => true));
  dio.httpClientAdapter = adapter;
  return dio;
}

/// 构造一个注入了假传输层的 AuthNotifier。
AuthNotifier makeNotifier(TokenStore store, HttpClientAdapter adapter) => AuthNotifier(
      api: AuthApi(ApiClient(tokenStore: store, dio: dioWith(adapter))),
      tokenStore: store,
    );

/// 等状态机跑完启动恢复。
Future<void> settle() => Future<void>.delayed(const Duration(milliseconds: 120));

void main() {
  group('启动恢复 restore()', () {
    test('本地无令牌 → 未登录', () async {
      final store = InMemoryTokenStore();
      final n = makeNotifier(store, FakeAdapter((_) => (200, {'code': 0, 'message': 'ok', 'data': null})));
      await settle();
      expect(n.state, isA<AuthLoggedOut>());
    });

    test('本地有令牌且后端校验通过 → 已登录', () async {
      final store = InMemoryTokenStore();
      await store.save(accessToken: 'a', refreshToken: 'r');

      final adapter = FakeAdapter((o) {
        if (o.path.contains('/users/me')) {
          return (200, {
            'code': 0,
            'message': 'ok',
            'data': {'id': '1', 'nickname': '测试用户', 'level': 3},
          });
        }
        return (200, {'code': 0, 'message': 'ok', 'data': null});
      });

      final n = makeNotifier(store, adapter);
      await settle();

      expect(n.state, isA<AuthLoggedIn>());
      expect((n.state as AuthLoggedIn).profile.nickname, '测试用户');
    });

    test('网络异常时**不清理**令牌（断网不应把用户踢下线）', () async {
      final store = InMemoryTokenStore();
      await store.save(accessToken: 'a', refreshToken: 'r');

      final dio = Dio(BaseOptions(baseUrl: 'http://test.local'));
      dio.interceptors.add(InterceptorsWrapper(onRequest: (o, h) {
        h.reject(DioException(requestOptions: o, type: DioExceptionType.connectionError));
      }));

      final n = AuthNotifier(
        api: AuthApi(ApiClient(tokenStore: store, dio: dio)),
        tokenStore: store,
      );
      await settle();

      expect(n.state, isA<AuthLoggedOut>());
      expect(
        await store.readAccessToken(),
        'a',
        reason: '网络原因不得清理令牌，否则一次断网就要重新登录',
      );
    });

    test('令牌确已失效（401 且续期失败）→ 清理并回未登录', () async {
      final store = InMemoryTokenStore();
      await store.save(accessToken: 'a', refreshToken: 'r');

      // 主接口 401，refresh 也 401 → ApiClient 会清理凭证并抛 unauthorized
      final adapter = FakeAdapter((o) {
        if (o.path.contains('/auth/refresh')) {
          return (200, {'code': 20002, 'message': '刷新令牌无效'});
        }
        return (401, {'code': 20002, 'message': '令牌已过期'});
      });

      final n = makeNotifier(store, adapter);
      await settle();

      expect(n.state, isA<AuthLoggedOut>());
      expect(await store.readAccessToken(), isNull);
    });
  });

  group('登出 logout()', () {
    test('服务端吊销失败也必须清本地（否则用户退不出去）', () async {
      final store = InMemoryTokenStore();
      await store.save(accessToken: 'a', refreshToken: 'r');

      final adapter = FakeAdapter((o) {
        if (o.path.contains('/auth/logout')) {
          return (500, {'code': 50000, 'message': '服务端错误'});
        }
        return (200, {'code': 0, 'message': 'ok', 'data': null});
      });

      final n = makeNotifier(store, adapter);
      await settle();
      await n.logout();

      expect(n.state, isA<AuthLoggedOut>());
      expect(await store.readAccessToken(), isNull);
      expect(await store.readRefreshToken(), isNull);
    });
  });

  group('登录 _persist', () {
    test('响应缺令牌时抛错且不写入存储', () async {
      final store = InMemoryTokenStore();
      final adapter = FakeAdapter((_) => (200, {
            'code': 0,
            'message': 'ok',
            'data': {'access_token': '', 'refresh_token': '', 'expires_in': 0},
          }));

      final n = makeNotifier(store, adapter);
      await settle();

      await expectLater(
        n.loginBySms(phone: '13800000000', code: '123456'),
        throwsA(isA<ApiError>()),
      );
      expect(await store.readAccessToken(), isNull);
    });
  });
}

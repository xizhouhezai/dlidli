/// HTTP 客户端：统一信封解析、按 code 判成败、401 单飞续期重放。
///
/// 关键设计（对应 FLT-03 / spec §3 / plan §2.3）：
/// 1. **按 `code` 判成败**，不看 HTTP 状态码（后端业务失败可能是 200 + code≠0）。
/// 2. **401 单飞续期**：并发的多个请求同时 401 时，只发起**一次** refresh，
///    其余请求等这一次的结果再重放。否则会出现「N 个请求各刷一次 token」，
///    在 token 轮换（旧 token 立即失效）的服务端上必然互相踢掉。
/// 3. 续期失败 → 清理凭证 + 抛 `ApiError(needsLogin: true)`，由上层引导登录。
library;

import 'dart:async';

import 'package:dio/dio.dart';

import 'api_error.dart';
import 'env.dart';
import 'token_store.dart';

class ApiClient {
  ApiClient({
    required TokenStore tokenStore,
    String? baseUrl,
    Dio? dio,
    Dio? refreshDio,
  }) : _tokenStore = tokenStore,
       _dio =
           dio ??
           Dio(
             BaseOptions(
               baseUrl: baseUrl ?? Env.apiBase,
               connectTimeout: const Duration(seconds: 10),
               receiveTimeout: const Duration(seconds: 20),
               sendTimeout: const Duration(seconds: 20),
               // 只要不抛异常就行：业务成败由信封 code 决定，故这里统一允许，
               // 状态码判断放在下面 fromDioError 里做（便于区分 401/403/5xx）。
               validateStatus: (_) => true,
             ),
           ),
       _refreshDio =
           refreshDio ??
           Dio(
             BaseOptions(
               baseUrl: baseUrl ?? Env.apiBase,
               connectTimeout: const Duration(seconds: 10),
               receiveTimeout: const Duration(seconds: 20),
               validateStatus: (_) => true,
             ),
           ) {
    _dio.interceptors.add(
      InterceptorsWrapper(
        onRequest: (options, handler) async {
          final token = await _tokenStore.readAccessToken();
          if (token != null && token.isNotEmpty) {
            options.headers['Authorization'] = 'Bearer $token';
          }
          handler.next(options);
        },
      ),
    );
  }

  final Dio _dio;

  /// 续期专用 Dio：**不走**本客户端的拦截器，避免 401 递归。
  /// 可注入（测试或特殊网络栈需要替换传输层）。
  final Dio _refreshDio;

  final TokenStore _tokenStore;

  /// 续期中的 Future：用于「单飞」——并发 401 只刷新一次。
  Future<bool>? _refreshing;

  bool _isRefreshing = false;

  String get baseUrl => _dio.options.baseUrl;

  // ---------------------------------------------------------------- 公开方法

  Future<Object?> get(String path, {Map<String, dynamic>? query}) =>
      _request(path, method: 'GET', query: query);

  Future<Object?> post(
    String path, {
    Object? body,
    Map<String, dynamic>? query,
  }) => _request(path, method: 'POST', body: body, query: query);

  Future<Object?> put(
    String path, {
    Object? body,
    Map<String, dynamic>? query,
  }) => _request(path, method: 'PUT', body: body, query: query);

  Future<Object?> delete(String path, {Object? body}) =>
      _request(path, method: 'DELETE', body: body);

  // ---------------------------------------------------------------- 内部实现

  Future<Object?> _request(
    String path, {
    required String method,
    Object? body,
    Map<String, dynamic>? query,
    bool retried = false,
  }) async {
    Response<dynamic> resp;
    try {
      resp = await _dio.request<dynamic>(
        path,
        data: body,
        queryParameters: query,
        options: Options(method: method),
      );
    } on DioException catch (e) {
      throw _fromDioException(e);
    }

    final status = resp.statusCode ?? 0;

    // 401：尝试单飞续期并重放一次
    if (status == 401 && !retried) {
      final ok = await _refreshOnce();
      if (ok) {
        return _request(
          path,
          method: method,
          body: body,
          query: query,
          retried: true,
        );
      }
      await _tokenStore.clear();
      throw ApiError(
        kind: ApiErrorKind.unauthorized,
        message: ErrorMessages.forStatus(401),
        statusCode: 401,
      );
    }

    final data = resp.data;
    if (data is! Map<String, dynamic>) {
      throw ApiError(
        kind: ApiErrorKind.malformed,
        message: '响应格式异常',
        statusCode: status,
      );
    }

    final envelope = ApiEnvelope.fromJson(data);

    // **按 code 判成败**（不看 2xx/4xx）
    if (envelope.isOk) return envelope.data;

    // 业务失败：用映射表或后端文案
    throw ApiError(
      kind: _kindForBusiness(status, envelope.code),
      message: ErrorMessages.forCode(envelope.code, envelope.message),
      code: envelope.code,
      traceId: envelope.traceId,
      statusCode: status,
    );
  }

  ApiErrorKind _kindForBusiness(int status, int code) {
    if (status == 401) return ApiErrorKind.unauthorized;
    if (status == 403) return ApiErrorKind.forbidden;
    if (status >= 500) return ApiErrorKind.server;
    return ApiErrorKind.business;
  }

  /// 单飞续期：并发调用只发一次 refresh 请求。
  Future<bool> _refreshOnce() {
    if (_isRefreshing && _refreshing != null) return _refreshing!;

    _isRefreshing = true;
    final future = _doRefresh().whenComplete(() {
      _isRefreshing = false;
      _refreshing = null;
    });
    _refreshing = future;
    return future;
  }

  Future<bool> _doRefresh() async {
    final refreshToken = await _tokenStore.readRefreshToken();
    if (refreshToken == null || refreshToken.isEmpty) return false;

    try {
      // 用**干净**的 Dio 发续期请求：避免走本客户端的 401 拦截造成递归。
      final resp = await _refreshDio.post<dynamic>(
        '/api/v1/auth/refresh',
        data: {'refresh_token': refreshToken},
      );
      final data = resp.data;
      if (data is! Map<String, dynamic>) return false;
      final env = ApiEnvelope.fromJson(data);
      if (!env.isOk) return false;

      final payload = env.data;
      if (payload is! Map<String, dynamic>) return false;
      final access = payload['access_token'] as String?;
      final refresh = (payload['refresh_token'] as String?) ?? refreshToken;
      if (access == null || access.isEmpty) return false;

      await _tokenStore.save(accessToken: access, refreshToken: refresh);
      return true;
    } catch (_) {
      return false;
    }
  }

  ApiError _fromDioException(DioException e) {
    switch (e.type) {
      case DioExceptionType.connectionTimeout:
      case DioExceptionType.sendTimeout:
      case DioExceptionType.receiveTimeout:
        return ApiError(kind: ApiErrorKind.timeout, message: '网络超时，请检查网络后重试');
      case DioExceptionType.connectionError:
        return ApiError(kind: ApiErrorKind.network, message: '网络连接失败，请检查网络后重试');
      case DioExceptionType.badResponse:
        final status = e.response?.statusCode ?? 0;
        return ApiError(
          kind:
              status == 401
                  ? ApiErrorKind.unauthorized
                  : (status >= 500
                      ? ApiErrorKind.server
                      : ApiErrorKind.business),
          message: ErrorMessages.forStatus(status),
          statusCode: status,
        );
      case DioExceptionType.cancel:
        return ApiError(kind: ApiErrorKind.network, message: '请求已取消');
      case DioExceptionType.badCertificate:
        return ApiError(kind: ApiErrorKind.network, message: '证书校验失败');
      // 兜底：dio 后续版本可能新增类型（例如 transformTimeout），
      // 用 default 而不是穷尽匹配，避免升级 dio 就编译不过。
      default:
        return ApiError(kind: ApiErrorKind.network, message: '网络异常，请稍后重试');
    }
  }
}

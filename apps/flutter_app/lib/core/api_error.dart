/// 后端统一响应信封与错误模型。
///
/// **判据铁律（FLT-03 与 spec §3）**：本项目后端**业务失败也可能返回 HTTP 200 + code≠0**
/// （例：提现重复打款 = 200 + `code=92303`）。因此：
///   - **一律按 `code` 判成败**，不得只看 HTTP 状态码；
///   - 把「被正确拦下」按状态码判定会误报成缺陷（admin-fill 卡首版就踩过这个坑）。
library;

/// 后端统一信封：`{code, message, data, trace_id}`。
class ApiEnvelope {
  const ApiEnvelope({
    required this.code,
    required this.message,
    this.data,
    this.traceId,
  });

  final int code;
  final String message;
  final Object? data;
  final String? traceId;

  bool get isOk => code == 0;

  factory ApiEnvelope.fromJson(Map<String, dynamic> json) => ApiEnvelope(
        code: (json['code'] as num?)?.toInt() ?? -1,
        message: (json['message'] as String?) ?? '',
        data: json['data'],
        traceId: json['trace_id'] as String?,
      );
}

/// 错误分类：决定 UI 该展示什么、是否可重试。
enum ApiErrorKind {
  /// 网络不可用 / 连接失败（可重试）
  network,

  /// 请求超时（可重试）
  timeout,

  /// 未登录或令牌失效（需引导登录）
  unauthorized,

  /// 无权限（403 / 权限码不足）
  forbidden,

  /// 业务校验失败（后端返回 code≠0，一般是可直接展示的文案）
  business,

  /// 服务端异常（5xx）
  server,

  /// 响应无法解析
  malformed,
}

/// 统一异常：把 dio 异常、信封业务错误、HTTP 错误收敛成一种类型，UI 只需处理它。
class ApiError implements Exception {
  ApiError({
    required this.kind,
    required this.message,
    this.code,
    this.traceId,
    this.statusCode,
  });

  final ApiErrorKind kind;

  /// 已本地化的可读文案（可直接展示给用户）。
  final String message;

  /// 后端业务码（kind == business 时有值）。
  final int? code;
  final String? traceId;
  final int? statusCode;

  /// 是否值得给用户一个「重试」按钮。
  bool get retryable =>
      kind == ApiErrorKind.network ||
      kind == ApiErrorKind.timeout ||
      kind == ApiErrorKind.server;

  /// 是否需要引导用户重新登录。
  bool get needsLogin => kind == ApiErrorKind.unauthorized;

  @override
  String toString() =>
      'ApiError(kind: $kind, code: $code, status: $statusCode, message: $message)';
}

/// 错误码 → 可读文案映射。
///
/// 只覆盖端侧**会主动处理**的码；其余走后端 `message`（后端文案已是中文且面向运营/用户）。
/// 参考 `server/internal/pkg/errcode`。
class ErrorMessages {
  ErrorMessages._();

  static const Map<int, String> _known = {
    10002: '参数不合法',
    20003: '账号或密码错误',
    30001: '上传会话已失效，请重新选择文件',
    30007: '视频时长超出限制',
    91027: '截帧机审未启用',
  };

  static String forCode(int code, String fallback) {
    final hit = _known[code];
    if (hit != null) return hit;
    return fallback.isNotEmpty ? fallback : '请求失败（$code）';
  }

  static String forStatus(int status) {
    switch (status) {
      case 401:
        return '登录已过期，请重新登录';
      case 403:
        return '没有权限执行该操作';
      case 404:
        return '内容不存在或已下架';
      case 429:
        return '操作太频繁，请稍后再试';
      case 500:
      case 502:
      case 503:
        return '服务暂时不可用，请稍后重试';
      default:
        return '请求失败（HTTP $status）';
    }
  }
}

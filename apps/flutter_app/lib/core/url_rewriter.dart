/// 后端下发 URL 的端侧重写。
///
/// **为什么必须做**（实测踩到，FLT-03 端侧差异）：
/// 后端把资源地址按**服务端视角**写死成 `http://localhost:8000/static/...`（见 `cover`/`streams[].url`）。
/// 在模拟器/真机里 `localhost` 指向**设备自身**，于是：
///   - 封面全部加载失败（页面一片灰块，且不报错，极难定位）；
///   - 播放地址同理不可用。
/// 因此端侧必须把「不可路由的主机名」替换成**当前实际使用的后端主机**。
///
/// 设计要点：
/// - 只替换**保留域名**（localhost / 127.0.0.1 / 0.0.0.0），不动真实域名——生产环境后端下发的是
///   正式域名，端侧不应改写，否则会掩盖服务端配置问题。
/// - 保留原始 scheme/path/query/port，避免破坏签名（`e=/q=/u=/s=` 在 query 里，**不能**重新编码）。
///   故这里用字符串替换而非 Uri 重建（Uri 重建会重排 query，签名校验可能因此失败）。
library;

import 'env.dart';

class UrlRewriter {
  UrlRewriter._();

  /// 不可路由的主机名（服务端视角的本地地址）。
  static const List<String> _loopbackHosts = [
    'localhost',
    '127.0.0.1',
    '0.0.0.0',
  ];

  /// 目标后端基址覆盖（测试或特殊部署可设）。
  ///
  /// 默认走 [Env.apiBase]；之所以允许覆盖，是因为**单测跑在主机（Windows/macOS）上**，
  /// `Platform.isAndroid` 为 false，[Env.apiBase] 会返回 localhost —— 此时重写等于没做，
  /// 测试就测不出问题。生产代码不设置它，行为与直接读 Env 一致。
  static String? baseUrlOverride;

  static String get _effectiveBase => baseUrlOverride ?? Env.apiBase;

  /// 把后端下发 URL 里的 loopback 主机替换成端侧实际可用的后端主机。
  ///
  /// 例（Android 模拟器）：
  ///   `http://localhost:8000/static/covers/a.jpg`
  ///   → `http://10.0.2.2:8000/static/covers/a.jpg`
  static String rewrite(String url) {
    if (url.isEmpty) return url;

    final uri = Uri.tryParse(url);
    if (uri == null || !uri.hasScheme || uri.host.isEmpty) {
      // 相对路径或非法 URL：原样返回，交由调用方处理
      return url;
    }
    if (!_loopbackHosts.contains(uri.host)) return url;

    // 从生效的后端基址取出端侧应使用的主机与端口
    final base = Uri.tryParse(_effectiveBase);
    if (base == null || base.host.isEmpty) return url;
    // 目标主机本身就是 loopback（例如主机上直跑测试）→ 无从重写，原样返回
    if (_loopbackHosts.contains(base.host) && base.host == uri.host) return url;

    // 字符串替换而非重建 Uri：保住 query 原样（签名参数不能重排/重新编码）
    final authorityStart = url.indexOf('://') + 3;
    final pathStart = url.indexOf('/', authorityStart);
    final authorityEnd = pathStart < 0 ? url.length : pathStart;
    final oldAuthority = url.substring(authorityStart, authorityEnd);

    // **host 与 port 一律取当前 API 的**，不保留后端写死的端口。
    //
    // 踩过的坑：库里历史数据把封面存成 `http://127.0.0.1:8010/...`（另一套测试环境的端口），
    // 早先实现只替换 host、保留原端口 → 得到 `http://10.0.2.2:8010/...`，
    // 而 API 实际在 8000，封面全部加载失败（真机截图表现为空白卡片）。
    // 静态资源与 API 同源（同一个 Go 服务既挂 /api 也挂 /static），
    // 所以端口必须跟随 API，而不是跟随那条脏数据。
    final newAuthority = base.hasPort && base.port > 0
        ? '${base.host}:${base.port}'
        : base.host;

    if (oldAuthority == newAuthority) return url;

    return url.replaceRange(authorityStart, authorityEnd, newAuthority);
  }
}

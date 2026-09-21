/**
 * 错误码与文案：分段规则与服务端 server/internal/pkg/errcode 对齐
 * （1xxxx 通用 / 2xxxx 账号 / 3xxxx 视频 / 4xxxx 弹幕 / 5xxxx 互动 / 6xxxx 社区 / 7xxxx 消息 / 8xxxx 审核 / 9xxxx 商业化）。
 * 负数段为端侧合成码，用于「网络不可用/超时/响应不合契约」这三类可重试失败。
 */
export class ErrCode {
  /** 端侧合成：网络不可用 */
  static readonly NET_OFFLINE: number = -1;
  /** 端侧合成：请求超时 */
  static readonly NET_TIMEOUT: number = -2;
  /** 端侧合成：响应不是统一包裹（多为网关/代理返回的非契约内容） */
  static readonly BAD_RESPONSE: number = -3;

  static readonly INTERNAL: number = 10001;
  static readonly INVALID_PARAMS: number = 10002;
  static readonly UNAUTHORIZED: number = 10003;
  static readonly FORBIDDEN: number = 10004;
  static readonly NOT_FOUND: number = 10005;
  static readonly TOO_MANY_REQUESTS: number = 10006;
}

const CODE_TEXT: Map<number, string> = new Map<number, string>([
  [ErrCode.NET_OFFLINE, '网络不可用，请检查网络后重试'],
  [ErrCode.NET_TIMEOUT, '网络超时，请重试'],
  [ErrCode.BAD_RESPONSE, '服务响应异常，请稍后重试'],
  [10001, '服务器开小差了，请稍后再试'],
  [10002, '参数不合法'],
  [10003, '请先登录'],
  [10004, '没有操作权限'],
  [10005, '资源不存在'],
  [10006, '操作太频繁，请稍后再试'],
  [20001, '验证码发送太频繁，请稍后再试'],
  [20002, '验证码错误或已过期'],
  [20003, '账号或密码错误'],
  [20004, '密码错误次数过多，请 15 分钟后再试'],
  [20005, '登录已过期，请重新登录'],
  [20006, '账号已被封禁'],
  [20007, '昵称已被占用'],
  [20008, '账号不存在'],
  [20009, '硬币不足，每日登录可获得硬币'],
  [20010, '账号已被禁言，暂无法发布内容'],
  [20011, '该邮箱已注册'],
  [20012, '账号未激活，请先查收激活邮件'],
  [20013, '微信登录未启用'],
  [20014, '微信授权失败，请重试'],
  [30001, '上传任务不存在或已过期'],
  [30002, '分片序号不合法'],
  [30003, '分片未全部上传完成'],
  [30004, '文件校验失败，请重新上传'],
  [30005, '不支持的视频格式'],
  [30006, '文件超过大小限制'],
  [40001, '弹幕发送太频繁，歇一会儿再发吧'],
  [40002, 'Lv1 及以上才能发弹幕'],
  [40003, 'Lv3 及以上才能发彩色/顶部/底部弹幕'],
  [40004, '内容重复，请勿刷屏'],
  [50001, '已经投过币啦'],
  [50002, '不能给自己的稿件投币'],
  [50003, '投币数量不合法（自制最多 2 枚，转载 1 枚）'],
  [60001, '不能关注自己']
]);

/** 未知码按域兜底，避免把裸数字或空文案抛给用户 */
function domainText(code: number): string {
  if (code >= 20000 && code < 30000) {
    return '账号操作失败，请稍后再试';
  }
  if (code >= 30000 && code < 40000) {
    return '视频操作失败，请稍后再试';
  }
  if (code >= 40000 && code < 50000) {
    return '弹幕操作失败，请稍后再试';
  }
  if (code >= 50000 && code < 60000) {
    return '互动操作失败，请稍后再试';
  }
  return '操作失败，请稍后再试';
}

export class ErrorMessages {
  /**
   * 服务端已给可读 message 时优先采用（同码文案可能带上下文，如 errcode.WithMsg 的附加提示），
   * 缺文案时才回退端侧表与按域兜底。
   */
  static text(code: number, serverMessage: string): string {
    const known = CODE_TEXT.get(code);
    if (code < 0) {
      return known ?? domainText(code);
    }
    if (serverMessage.length > 0) {
      return serverMessage;
    }
    return known ?? domainText(code);
  }

  static isNetworkError(code: number): boolean {
    return code < 0;
  }
}

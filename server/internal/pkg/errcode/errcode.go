// Package errcode 定义统一业务错误码。
// 分段规则：1xxxx 通用；2xxxx 账号；3xxxx 视频；4xxxx 弹幕；5xxxx 互动；
// 6xxxx 社区；7xxxx 消息；8xxxx 审核后台；9xxxx 商业化（直播 91000+，M4-LIV）。
package errcode

import "fmt"

type Error struct {
	Code int    `json:"code"`
	Msg  string `json:"message"`
}

func (e *Error) Error() string {
	return fmt.Sprintf("[%d] %s", e.Code, e.Msg)
}

// WithMsg 返回同码不同文案的错误副本（用于附加上下文提示）。
func (e *Error) WithMsg(msg string) *Error {
	return &Error{Code: e.Code, Msg: msg}
}

func New(code int, msg string) *Error {
	return &Error{Code: code, Msg: msg}
}

// 通用错误码（1xxxx）
var (
	ErrInternal        = New(10001, "服务器开小差了，请稍后再试")
	ErrInvalidParams   = New(10002, "参数不合法")
	ErrUnauthorized    = New(10003, "请先登录")
	ErrForbidden       = New(10004, "没有操作权限")
	ErrNotFound        = New(10005, "资源不存在")
	ErrTooManyRequests = New(10006, "操作太频繁，请稍后再试")
)

// 账号错误码（2xxxx）
var (
	ErrSmsTooFrequent   = New(20001, "验证码发送太频繁，请稍后再试")
	ErrSmsCodeInvalid   = New(20002, "验证码错误或已过期")
	ErrPasswordMismatch = New(20003, "账号或密码错误")
	ErrLoginLocked      = New(20004, "密码错误次数过多，请 15 分钟后再试")
	ErrRefreshInvalid   = New(20005, "登录已过期，请重新登录")
	ErrUserBanned       = New(20006, "账号已被封禁")
	ErrNicknameTaken    = New(20007, "昵称已被占用")
	ErrAccountNotExists = New(20008, "账号不存在")
	ErrCoinNotEnough    = New(20009, "硬币不足，每日登录可获得硬币")
	ErrUserMuted        = New(20010, "账号已被禁言，暂无法发布内容")
	ErrAccountExists    = New(20011, "该邮箱已注册")
	ErrAccountInactive  = New(20012, "账号未激活，请先查收激活邮件")
	// 微信小程序登录（M3-MP-01）
	ErrWxLoginDisabled = New(20013, "微信登录未启用")
	ErrWxCodeInvalid   = New(20014, "微信授权失败，请重试")
)

// 视频/上传错误码（3xxxx）
var (
	ErrUploadNotFound     = New(30001, "上传任务不存在或已过期")
	ErrChunkIndexInvalid  = New(30002, "分片序号不合法")
	ErrUploadIncomplete   = New(30003, "分片未全部上传完成")
	ErrFileHashMismatch   = New(30004, "文件校验失败，请重新上传")
	ErrFileTypeNotAllowed = New(30005, "不支持的视频格式")
	ErrFileTooLarge       = New(30006, "文件超过大小限制")
)

// 弹幕错误码（4xxxx）
var (
	ErrDanmakuTooFrequent = New(40001, "弹幕发送太频繁，歇一会儿再发吧")
	ErrDanmakuLevelLow    = New(40002, "Lv1 及以上才能发弹幕")
	ErrDanmakuPrivilege   = New(40003, "Lv3 及以上才能发彩色/顶部/底部弹幕")
	ErrDanmakuDuplicate   = New(40004, "内容重复，请勿刷屏")
)

// 互动错误码（5xxxx）
var (
	ErrAlreadyCoined = New(50001, "已经投过币啦")
	ErrCoinSelf      = New(50002, "不能给自己的稿件投币")
	ErrCoinCount     = New(50003, "投币数量不合法（自制最多 2 枚，转载 1 枚）")
)

// 关系链错误码（6xxxx）
var (
	ErrFollowSelf = New(60001, "不能关注自己")
)

// 直播错误码（91000+，商业化段，M4-LIV）
var (
	ErrLiveNotWhitelisted  = New(91001, "暂未获得开播资格（白名单）")
	ErrLiveNotRealnamed    = New(91002, "开播需先完成实名认证")
	ErrLiveFansNotEnough   = New(91003, "粉丝数未达 100，暂不能开播")
	ErrLiveRoomExists      = New(91004, "您已创建直播间")
	ErrLiveAlreadyLive     = New(91005, "房间已开播，请勿重复开播")
	ErrLiveNotLive         = New(91006, "房间未在直播中")
	ErrLiveRoomNotFound    = New(91007, "直播间不存在")
	ErrLiveNotAnchor       = New(91008, "只有主播才能执行该操作")
	ErrLiveNotAdmin        = New(91009, "只有主播或房管才能执行该操作")
	ErrLiveAdminSelf       = New(91010, "不能对自己执行该操作")
	ErrLiveAdminIsAnchor   = New(91011, "主播无需任命为房管")
	ErrLiveCategoryInvalid = New(91012, "直播分区不存在或已停用")
	ErrLiveMuteRange       = New(91013, "禁言时长需在 1 分钟到 720 小时之间")
	ErrLiveTitleRequired   = New(91014, "房间标题不能为空（1-80 字）")
	// 直播弹幕通道（M4-LIV-03）
	ErrLiveDanmakuMuted      = New(91015, "您已被房管禁言，暂无法发言")
	ErrLiveDanmakuTooFast    = New(91016, "发言太频繁，请稍后再试")
	ErrLiveDanmakuNotIn      = New(91017, "请先进入直播间")
	ErrLiveDanmakuDuplicate  = New(91018, "请勿重复发送相同内容")
	// 礼物系统（M4-LIV-04）
	ErrLiveGiftNotFound    = New(91019, "礼物不存在或已下架")
	ErrLiveGiftNotLive     = New(91020, "房间未在直播，无法赠送礼物")
	ErrLiveMelonNotEnough  = New(91021, "金瓜子余额不足")
	ErrLiveGiftCount       = New(91022, "赠送数量不合法（1-999）")
	ErrLiveRechargeInvalid = New(91023, "直充金额不合法")
	// 推流回调（M4-LIV-01）：SRS http_hooks 鉴权失败一律以 HTTP 403 返回（非 2xx 即拒推）
	ErrLiveCallbackTokenInvalid = New(91024, "直播回调鉴权失败")
	ErrLiveStreamKeyInvalid     = New(91025, "推流密钥错误")
	ErrLiveStreamFormatInvalid  = New(91026, "推流流名不合法")
)

// Package errcode 定义统一业务错误码。
// 分段规则：1xxxx 通用；2xxxx 账号；3xxxx 视频；4xxxx 弹幕；5xxxx 互动；
// 6xxxx 社区；7xxxx 消息；8xxxx 审核后台；9xxxx 商业化。
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

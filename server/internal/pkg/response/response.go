// Package response 提供统一响应结构：{ code, message, data, trace_id }。
package response

import (
	"errors"
	"net/http"

	"github.com/dlidli/server/internal/pkg/errcode"
	"github.com/gin-gonic/gin"
)

// CtxTraceID 是 gin.Context 中 trace id 的键，由 TraceID 中间件写入。
const CtxTraceID = "trace_id"

type Body struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data"`
	TraceID string `json:"trace_id"`
}

// OK 返回成功响应。
func OK(c *gin.Context, data any) {
	c.JSON(http.StatusOK, Body{
		Code:    0,
		Message: "ok",
		Data:    data,
		TraceID: c.GetString(CtxTraceID),
	})
}

// Fail 返回失败响应；非 *errcode.Error 一律按内部错误处理，避免泄露细节。
func Fail(c *gin.Context, err error) {
	_ = c.Error(err) // 挂到 gin 错误链，由访问日志输出详情
	e := errcode.ErrInternal
	var biz *errcode.Error
	if errors.As(err, &biz) {
		e = biz
	}
	c.JSON(httpStatus(e), Body{
		Code:    e.Code,
		Message: e.Msg,
		Data:    nil,
		TraceID: c.GetString(CtxTraceID),
	})
}

// httpStatus 将业务错误码映射为 HTTP 状态码。
func httpStatus(e *errcode.Error) int {
	switch e.Code {
	case errcode.ErrUnauthorized.Code:
		return http.StatusUnauthorized
	case errcode.ErrForbidden.Code:
		return http.StatusForbidden
	case errcode.ErrNotFound.Code:
		return http.StatusNotFound
	case errcode.ErrTooManyRequests.Code:
		return http.StatusTooManyRequests
	case errcode.ErrInternal.Code:
		return http.StatusInternalServerError
	default:
		return http.StatusOK // 业务错误统一 200 + code
	}
}

package wechat

import (
	"github.com/dlidli/server/internal/pkg/errcode"
	"github.com/dlidli/server/internal/pkg/response"
	"github.com/gin-gonic/gin"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// RegisterRoutes 注册微信 JSSDK 路由（公开接口：签名与登录态无关）。
func (h *Handler) RegisterRoutes(v1 *gin.RouterGroup) {
	v1.GET("/wechat/jssdk-sign", h.jssdkSign)
}

// @Summary  微信 JSSDK 签名
// @Tags     微信
// @Produce  json
// @Param    url query string true "当前页面 URL（含参数，不含 #hash）"
// @Success  200 {object} response.Body
// @Router   /wechat/jssdk-sign [get]
func (h *Handler) jssdkSign(c *gin.Context) {
	pageURL := c.Query("url")
	if pageURL == "" {
		response.Fail(c, errcode.ErrInvalidParams.WithMsg("缺少 url 参数"))
		return
	}
	res, err := h.svc.Sign(c.Request.Context(), pageURL)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, res)
}

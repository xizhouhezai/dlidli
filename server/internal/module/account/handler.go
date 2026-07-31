package account

import (
	"regexp"

	"github.com/dlidli/server/internal/middleware"
	"github.com/dlidli/server/internal/pkg/errcode"
	"github.com/dlidli/server/internal/pkg/response"
	"github.com/dlidli/server/internal/pkg/storage"
	"github.com/gin-gonic/gin"
)

var phoneRe = regexp.MustCompile(`^1\d{10}$`)

type Handler struct {
	svc   *Service
	store storage.Storage
}

func NewHandler(svc *Service, store storage.Storage) *Handler {
	return &Handler{svc: svc, store: store}
}

// RegisterRoutes 注册账号域路由。
func (h *Handler) RegisterRoutes(v1 *gin.RouterGroup, auth gin.HandlerFunc) {
	g := v1.Group("/auth")
	{
		g.POST("/sms-code", h.sendSmsCode)
		g.POST("/login/sms", h.loginBySms)
		g.POST("/login/password", h.loginByPassword)
		g.GET("/captcha", h.captcha)
		g.POST("/refresh", h.refresh)
		g.POST("/logout", h.logout)
		g.POST("/reset-password", h.resetPassword)
	}

	users := v1.Group("/users", auth)
	{
		users.GET("/me", h.me)
		users.PUT("/me/password", h.changePassword)
		users.PATCH("/me", h.updateProfile)
		users.POST("/me/avatar", h.uploadAvatar)
	}
}

func (h *Handler) sendSmsCode(c *gin.Context) {
	var req struct {
		Phone string `json:"phone" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || !phoneRe.MatchString(req.Phone) {
		response.Fail(c, errcode.ErrInvalidParams.WithMsg("手机号格式不正确"))
		return
	}
	debugCode, err := h.svc.SendSmsCode(c.Request.Context(), req.Phone)
	if err != nil {
		response.Fail(c, err)
		return
	}
	data := gin.H{}
	if debugCode != "" {
		data["debug_code"] = debugCode // 仅 dev 环境返回
	}
	response.OK(c, data)
}

func (h *Handler) loginBySms(c *gin.Context) {
	var req struct {
		Phone string `json:"phone" binding:"required"`
		Code  string `json:"code" binding:"required,len=6"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || !phoneRe.MatchString(req.Phone) {
		response.Fail(c, errcode.ErrInvalidParams)
		return
	}
	pair, err := h.svc.LoginBySms(c.Request.Context(), req.Phone, req.Code)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, pair)
}

func (h *Handler) loginByPassword(c *gin.Context) {
	var req struct {
		Account     string `json:"account" binding:"required"`
		Password    string `json:"password" binding:"required,min=6,max=64"`
		CaptchaID   string `json:"captcha_id" binding:"required"`
		CaptchaCode string `json:"captcha_code" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, errcode.ErrInvalidParams)
		return
	}
	pair, err := h.svc.LoginByPassword(c.Request.Context(), req.Account, req.Password, req.CaptchaID, req.CaptchaCode)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, pair)
}

func (h *Handler) captcha(c *gin.Context) {
	result, err := h.svc.GenerateCaptcha(c.Request.Context())
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, result)
}

func (h *Handler) refresh(c *gin.Context) {
	var req struct {
		RefreshToken string `json:"refresh_token" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, errcode.ErrInvalidParams)
		return
	}
	pair, err := h.svc.Refresh(c.Request.Context(), req.RefreshToken)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, pair)
}

func (h *Handler) logout(c *gin.Context) {
	var req struct {
		RefreshToken string `json:"refresh_token" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, errcode.ErrInvalidParams)
		return
	}
	if err := h.svc.Logout(c.Request.Context(), req.RefreshToken); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, nil)
}

func (h *Handler) me(c *gin.Context) {
	uid := c.GetInt64(middleware.CtxUserID)
	profile, err := h.svc.Me(c.Request.Context(), uid)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, profile)
}

func (h *Handler) changePassword(c *gin.Context) {
	var req struct {
		OldPassword string `json:"old_password"`
		NewPassword string `json:"new_password" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, errcode.ErrInvalidParams)
		return
	}
	uid := c.GetInt64(middleware.CtxUserID)
	if err := h.svc.ChangePassword(c.Request.Context(), uid, req.OldPassword, req.NewPassword); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, nil)
}

func (h *Handler) resetPassword(c *gin.Context) {
	var req struct {
		Phone       string `json:"phone" binding:"required"`
		Code        string `json:"code" binding:"required"`
		NewPassword string `json:"new_password" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, errcode.ErrInvalidParams)
		return
	}
	if err := h.svc.ResetPassword(c.Request.Context(), req.Phone, req.Code, req.NewPassword); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, nil)
}

func (h *Handler) updateProfile(c *gin.Context) {
	var req UpdateProfileReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, errcode.ErrInvalidParams)
		return
	}
	uid := c.GetInt64(middleware.CtxUserID)
	profile, err := h.svc.UpdateProfile(c.Request.Context(), uid, &req)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, profile)
}

func (h *Handler) uploadAvatar(c *gin.Context) {
	fh, err := c.FormFile("file")
	if err != nil {
		response.Fail(c, errcode.ErrInvalidParams.WithMsg("请选择头像文件"))
		return
	}
	uid := c.GetInt64(middleware.CtxUserID)
	url, err := h.svc.UploadAvatar(c.Request.Context(), uid, fh, h.store)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"avatar": url})
}

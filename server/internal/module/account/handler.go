package account

import (
	"regexp"
	"strconv"

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
		g.POST("/register/email", h.registerByEmail)
		g.POST("/activate", h.activateEmail)
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
		users.GET("/me/coin-logs", h.coinLogs)
		users.GET("/me/youth-mode", h.youthMode)
		users.PUT("/me/youth-mode", h.setYouthMode)
	}
}

// sendSmsCode 发送短信验证码
// @Summary  发送短信验证码
// @Tags     账号-认证
// @Accept   json
// @Produce  json
// @Param    body body object true "phone: 手机号"
// @Success  200 {object} response.Body
// @Router   /auth/sms-code [post]
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

// registerByEmail 邮箱注册（ACC-02）
// @Summary  邮箱注册
// @Tags     账号-认证
// @Accept   json
// @Produce  json
// @Param    body body object true \"email; password\"
// @Success  200 {object} response.Body
// @Router   /auth/register/email [post]
func (h *Handler) registerByEmail(c *gin.Context) {
	var req struct {
		Email    string `json:"email" binding:"required"`
		Password string `json:"password" binding:"required,min=6,max=64"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, errcode.ErrInvalidParams)
		return
	}
	debugURL, err := h.svc.RegisterByEmail(c.Request.Context(), req.Email, req.Password)
	if err != nil {
		response.Fail(c, err)
		return
	}
	data := gin.H{"activated": false}
	if debugURL != "" {
		data["debug_activate_url"] = debugURL // 仅 dev 环境返回（mock 邮件）
	}
	response.OK(c, data)
}

// activateEmail 激活邮箱账号（ACC-02）
// @Summary  激活邮箱账号
// @Tags     账号-认证
// @Accept   json
// @Produce  json
// @Param    body body object true \"token\"
// @Success  200 {object} response.Body
// @Router   /auth/activate [post]
func (h *Handler) activateEmail(c *gin.Context) {
	var req struct {
		Token string `json:"token" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, errcode.ErrInvalidParams)
		return
	}
	if err := h.svc.ActivateEmail(c.Request.Context(), req.Token); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"activated": true})
}

// loginBySms 验证码登录（未注册自动注册）
// @Summary  验证码登录
// @Tags     账号-认证
// @Accept   json
// @Produce  json
// @Param    body body object true "phone: 手机号; code: 6位验证码"
// @Success  200 {object} response.Body "data: {access_token, refresh_token, user}"
// @Router   /auth/login/sms [post]
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

// loginByPassword 账号密码登录（需图形验证码）
// @Summary  密码登录
// @Tags     账号-认证
// @Accept   json
// @Produce  json
// @Param    body body object true "account; password; captcha_id; captcha_code"
// @Success  200 {object} response.Body
// @Router   /auth/login/password [post]
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

// captcha 获取图形验证码
// @Summary  获取图形验证码
// @Tags     账号-认证
// @Produce  json
// @Success  200 {object} response.Body "data: {captcha_id, image_base64}"
// @Router   /auth/captcha [get]
func (h *Handler) captcha(c *gin.Context) {
	result, err := h.svc.GenerateCaptcha(c.Request.Context())
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, result)
}

// refresh 刷新令牌
// @Summary  刷新访问令牌
// @Tags     账号-认证
// @Accept   json
// @Produce  json
// @Param    body body object true "refresh_token"
// @Success  200 {object} response.Body
// @Router   /auth/refresh [post]
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

// logout 退出登录（吊销刷新令牌）
// @Summary  退出登录
// @Tags     账号-认证
// @Accept   json
// @Produce  json
// @Param    body body object true "refresh_token"
// @Success  200 {object} response.Body
// @Router   /auth/logout [post]
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

// me 获取当前登录用户资料
// @Summary  当前用户资料
// @Tags     账号-资料
// @Produce  json
// @Security BearerAuth
// @Success  200 {object} response.Body
// @Router   /users/me [get]
func (h *Handler) me(c *gin.Context) {
	uid := c.GetInt64(middleware.CtxUserID)
	profile, err := h.svc.Me(c.Request.Context(), uid)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, profile)
}

// coinLogs 硬币流水分页
// @Summary  硬币流水分页
// @Tags     账号-资产
// @Produce  json
// @Security BearerAuth
// @Param    page query int false "页码（默认1）"
// @Param    size query int false "每页条数（默认20）"
// @Success  200 {object} response.Body
// @Router   /users/me/coin-logs [get]
func (h *Handler) coinLogs(c *gin.Context) {
	page, size := pagination(c)
	uid := c.GetInt64(middleware.CtxUserID)
	list, total, err := h.svc.CoinLogs(c.Request.Context(), uid, page, size)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"list": list, "total": total})
}

func pagination(c *gin.Context) (page, size int) {
	page = 1
	size = 20
	if p := c.Query("page"); p != "" {
		if v, err := strconv.Atoi(p); err == nil && v > 0 {
			page = v
		}
	}
	if s := c.Query("size"); s != "" {
		if v, err := strconv.Atoi(s); err == nil && v > 0 && v <= 100 {
			size = v
		}
	}
	return page, size
}

// youthMode 青少年模式状态
// @Summary  青少年模式状态
// @Tags     账号-设置
// @Produce  json
// @Security BearerAuth
// @Success  200 {object} response.Body
// @Router   /users/me/youth-mode [get]
func (h *Handler) youthMode(c *gin.Context) {
	uid := c.GetInt64(middleware.CtxUserID)
	on, err := h.svc.GetYouthMode(c.Request.Context(), uid)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"enabled": on})
}

// setYouthMode 开关青少年模式
// @Summary  开关青少年模式
// @Tags     账号-设置
// @Accept   json
// @Produce  json
// @Security BearerAuth
// @Param    body body object true "enabled: 是否开启"
// @Success  200 {object} response.Body
// @Router   /users/me/youth-mode [put]
func (h *Handler) setYouthMode(c *gin.Context) {
	var req struct {
		Enabled bool `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, errcode.ErrInvalidParams)
		return
	}
	uid := c.GetInt64(middleware.CtxUserID)
	if err := h.svc.SetYouthMode(c.Request.Context(), uid, req.Enabled); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, nil)
}

// changePassword 修改密码
// @Summary  修改密码
// @Tags     账号-资料
// @Accept   json
// @Produce  json
// @Security BearerAuth
// @Param    body body object true "old_password; new_password"
// @Success  200 {object} response.Body
// @Router   /users/me/password [put]
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

// resetPassword 短信验证码重置密码
// @Summary  重置密码
// @Tags     账号-认证
// @Accept   json
// @Produce  json
// @Param    body body object true "phone; code; new_password"
// @Success  200 {object} response.Body
// @Router   /auth/reset-password [post]
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

// updateProfile 更新用户资料
// @Summary  更新资料
// @Tags     账号-资料
// @Accept   json
// @Produce  json
// @Security BearerAuth
// @Param    body body UpdateProfileReq true "资料字段"
// @Success  200 {object} response.Body
// @Router   /users/me [patch]
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

// uploadAvatar 上传头像
// @Summary  上传头像
// @Tags     账号-资料
// @Accept   multipart/form-data
// @Produce  json
// @Security BearerAuth
// @Param    file formData file true "头像图片"
// @Success  200 {object} response.Body "data: {avatar}"
// @Router   /users/me/avatar [post]
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

package report

import (
	"net/http"
	"strconv"

	"github.com/dlidli/server/internal/middleware"
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

// RegisterRoutes 注册举报路由：
// C 端 POST /reports（登录）；后台 /admin/reports（管理员，report:view/handle 权限）。
func (h *Handler) RegisterRoutes(v1 *gin.RouterGroup, auth gin.HandlerFunc,
	adminAuth gin.HandlerFunc, perm func(code string) gin.HandlerFunc,
) {
	v1.POST("/reports", auth, h.submit)

	g := v1.Group("/admin/reports", adminAuth)
	{
		g.GET("", perm("report:view"), h.adminList)
		g.GET("/export", perm("report:view"), h.adminExport)
		g.POST("/:id/handle", perm("report:handle"), h.adminHandle)
	}
}

// @Summary  举报列表导出 CSV（当前状态筛选，SYS-06）
// @Tags     管理后台-举报
// @Produce  text/csv
// @Security BearerAuth
// @Param    status query int false "-1全部 0待处理 1已处理"
// @Success  200 {string} string "CSV 文件"
// @Router   /admin/reports/export [get]
func (h *Handler) adminExport(c *gin.Context) {
	status, _ := strconv.Atoi(c.DefaultQuery("status", "0"))
	data, filename, err := h.svc.ExportCSV(c.Request.Context(), int8(status))
	if err != nil {
		response.Fail(c, err)
		return
	}
	c.Header("Content-Disposition", `attachment; filename="`+filename+`"`)
	c.Data(http.StatusOK, "text/csv; charset=utf-8", data)
}

// @Summary  提交举报（视频/评论/弹幕/动态/用户）
// @Tags     举报
// @Accept   json
// @Produce  json
// @Security BearerAuth
// @Param    body body SubmitReq true "举报信息（视频 target_id 传 bvid，其余传对象 ID）"
// @Success  200 {object} response.Body
// @Router   /reports [post]
func (h *Handler) submit(c *gin.Context) {
	var req SubmitReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, errcode.ErrInvalidParams)
		return
	}
	uid := c.GetInt64(middleware.CtxUserID)
	if err := h.svc.Submit(c.Request.Context(), uid, &req); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, nil)
}

// @Summary  举报队列（状态筛选）
// @Tags     管理后台-举报
// @Produce  json
// @Security BearerAuth
// @Param    status query int false "0待处理 1已处理 2已忽略 -1全部（默认0）"
// @Param    page query int false "页码（默认1）"
// @Param    size query int false "每页条数（默认20）"
// @Success  200 {object} response.Body
// @Router   /admin/reports [get]
func (h *Handler) adminList(c *gin.Context) {
	status, _ := strconv.Atoi(c.DefaultQuery("status", "0"))
	if status < -1 || status > 2 {
		status = 0
	}
	page, size := pagination(c)
	items, total, err := h.svc.AdminList(c.Request.Context(), int8(status), page, size)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"list": items, "total": total})
}

// @Summary  处理举报（忽略/删除内容/删除并处罚）
// @Tags     管理后台-举报
// @Accept   json
// @Produce  json
// @Security BearerAuth
// @Param    id path string true "举报ID"
// @Param    body body HandleReq true "处理动作"
// @Success  200 {object} response.Body
// @Router   /admin/reports/{id}/handle [post]
func (h *Handler) adminHandle(c *gin.Context) {
	var req HandleReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, errcode.ErrInvalidParams)
		return
	}
	adminID := c.GetInt64(middleware.CtxAdminID)
	if err := h.svc.Handle(c.Request.Context(), adminID, c.Param("id"), &req); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, nil)
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

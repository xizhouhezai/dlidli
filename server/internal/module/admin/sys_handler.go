package admin

import (
	"net/http"
	"strconv"
	"time"

	"github.com/dlidli/server/internal/middleware"
	"github.com/dlidli/server/internal/pkg/errcode"
	"github.com/dlidli/server/internal/pkg/response"
	"github.com/gin-gonic/gin"
)

// ---- 审计日志（M2-SYS-01） ----

// parseAuditQuery 解析审计查询条件（操作者/动作/对象类型/时间范围）。
func parseAuditQuery(c *gin.Context) (*AuditQuery, error) {
	q := &AuditQuery{}
	if v := c.Query("admin_id"); v != "" {
		id, err := strconv.ParseInt(v, 10, 64)
		if err != nil || id <= 0 {
			return nil, errcode.ErrInvalidParams
		}
		q.AdminID = id
	}
	q.Action = c.Query("action")
	q.ObjType = c.Query("obj_type")
	if v := c.Query("from"); v != "" {
		t, err := time.ParseInLocation("2006-01-02", v, time.Local)
		if err != nil {
			return nil, errcode.ErrInvalidParams.WithMsg("起始日期格式应为 YYYY-MM-DD")
		}
		q.From = &t
	}
	if v := c.Query("to"); v != "" {
		t, err := time.ParseInLocation("2006-01-02", v, time.Local)
		if err != nil {
			return nil, errcode.ErrInvalidParams.WithMsg("截止日期格式应为 YYYY-MM-DD")
		}
		t = t.Add(24*time.Hour - time.Second) // 含当日
		q.To = &t
	}
	return q, nil
}

// @Summary  审计日志分页查询（按操作者/动作/对象/时间筛选）
// @Tags     管理后台-审计
// @Produce  json
// @Security BearerAuth
// @Param    admin_id query int false "操作者ID"
// @Param    action query string false "动作"
// @Param    obj_type query string false "对象类型"
// @Param    from query string false "起始日期 YYYY-MM-DD"
// @Param    to query string false "截止日期 YYYY-MM-DD"
// @Param    page query int false "页码"
// @Param    size query int false "每页条数"
// @Success  200 {object} response.Body
// @Router   /admin/audit-logs [get]
func (h *Handler) listAuditLogs(c *gin.Context) {
	q, err := parseAuditQuery(c)
	if err != nil {
		response.Fail(c, err)
		return
	}
	page, size := pagination(c)
	items, total, err := h.svc.AuditLogs(c.Request.Context(), q, page, size)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"list": items, "total": total})
}

// @Summary  审计日志导出 CSV（按当前筛选条件）
// @Tags     管理后台-审计
// @Produce  text/csv
// @Security BearerAuth
// @Param    admin_id query int false "操作者ID"
// @Param    action query string false "动作"
// @Param    obj_type query string false "对象类型"
// @Param    from query string false "起始日期 YYYY-MM-DD"
// @Param    to query string false "截止日期 YYYY-MM-DD"
// @Success  200 {string} string "CSV 文件"
// @Router   /admin/audit-logs/export [get]
func (h *Handler) exportAuditLogs(c *gin.Context) {
	q, err := parseAuditQuery(c)
	if err != nil {
		response.Fail(c, err)
		return
	}
	data, filename, err := h.svc.ExportAuditLogsCSV(c.Request.Context(), q)
	if err != nil {
		response.Fail(c, err)
		return
	}
	c.Header("Content-Disposition", `attachment; filename="`+filename+`"`)
	c.Data(http.StatusOK, "text/csv; charset=utf-8", data)
}

// ---- 系统配置（M2-SYS-02） ----

// @Summary  系统配置列表
// @Tags     管理后台-系统
// @Produce  json
// @Security BearerAuth
// @Success  200 {object} response.Body
// @Router   /admin/configs [get]
func (h *Handler) listConfigs(c *gin.Context) {
	list, err := h.svc.Configs(c.Request.Context())
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"list": list})
}

// @Summary  新建系统配置
// @Tags     管理后台-系统
// @Accept   json
// @Produce  json
// @Security BearerAuth
// @Param    body body SaveConfigReq true "配置项"
// @Success  200 {object} response.Body
// @Router   /admin/configs [post]
func (h *Handler) createConfig(c *gin.Context) {
	var req SaveConfigReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, errcode.ErrInvalidParams)
		return
	}
	adminID := c.GetInt64(middleware.CtxAdminID)
	if err := h.svc.CreateConfig(c.Request.Context(), &req); err != nil {
		response.Fail(c, err)
		return
	}
	h.svc.addAudit(&AuditLog{AdminID: adminID, Action: "add_config", ObjType: "config", Detail: req.ConfigKey})
	response.OK(c, nil)
}

// @Summary  编辑系统配置（热更新）
// @Tags     管理后台-系统
// @Accept   json
// @Produce  json
// @Security BearerAuth
// @Param    id path int true "配置ID"
// @Param    body body SaveConfigReq true "配置项"
// @Success  200 {object} response.Body
// @Router   /admin/configs/{id} [put]
func (h *Handler) updateConfig(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.Fail(c, errcode.ErrInvalidParams)
		return
	}
	var req SaveConfigReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, errcode.ErrInvalidParams)
		return
	}
	adminID := c.GetInt64(middleware.CtxAdminID)
	if err := h.svc.UpdateConfig(c.Request.Context(), id, &req); err != nil {
		response.Fail(c, err)
		return
	}
	h.svc.addAudit(&AuditLog{AdminID: adminID, Action: "edit_config", ObjType: "config", Oid: id, Detail: req.ConfigKey})
	response.OK(c, nil)
}

// @Summary  删除系统配置
// @Tags     管理后台-系统
// @Produce  json
// @Security BearerAuth
// @Param    id path int true "配置ID"
// @Success  200 {object} response.Body
// @Router   /admin/configs/{id} [delete]
func (h *Handler) deleteConfig(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.Fail(c, errcode.ErrInvalidParams)
		return
	}
	adminID := c.GetInt64(middleware.CtxAdminID)
	if err := h.svc.DeleteConfig(c.Request.Context(), id); err != nil {
		response.Fail(c, err)
		return
	}
	h.svc.addAudit(&AuditLog{AdminID: adminID, Action: "del_config", ObjType: "config", Oid: id})
	response.OK(c, nil)
}

// ---- 数据字典（M2-SYS-02） ----

// @Summary  数据字典分组列表
// @Tags     管理后台-系统
// @Produce  json
// @Security BearerAuth
// @Success  200 {object} response.Body
// @Router   /admin/dicts [get]
func (h *Handler) listDicts(c *gin.Context) {
	groups, err := h.svc.DictGroups(c.Request.Context())
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"groups": groups})
}

// @Summary  新建字典项
// @Tags     管理后台-系统
// @Accept   json
// @Produce  json
// @Security BearerAuth
// @Param    body body SaveDictReq true "字典项"
// @Success  200 {object} response.Body
// @Router   /admin/dicts [post]
func (h *Handler) createDict(c *gin.Context) {
	var req SaveDictReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, errcode.ErrInvalidParams)
		return
	}
	adminID := c.GetInt64(middleware.CtxAdminID)
	if err := h.svc.CreateDict(c.Request.Context(), &req); err != nil {
		response.Fail(c, err)
		return
	}
	h.svc.addAudit(&AuditLog{AdminID: adminID, Action: "add_dict", ObjType: "dict", Detail: req.DictType + ":" + req.Value})
	response.OK(c, nil)
}

// @Summary  编辑字典项
// @Tags     管理后台-系统
// @Accept   json
// @Produce  json
// @Security BearerAuth
// @Param    id path int true "字典项ID"
// @Param    body body SaveDictReq true "字典项"
// @Success  200 {object} response.Body
// @Router   /admin/dicts/{id} [put]
func (h *Handler) updateDict(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.Fail(c, errcode.ErrInvalidParams)
		return
	}
	var req SaveDictReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, errcode.ErrInvalidParams)
		return
	}
	adminID := c.GetInt64(middleware.CtxAdminID)
	if err := h.svc.UpdateDict(c.Request.Context(), id, &req); err != nil {
		response.Fail(c, err)
		return
	}
	h.svc.addAudit(&AuditLog{AdminID: adminID, Action: "edit_dict", ObjType: "dict", Oid: id, Detail: req.DictType + ":" + req.Value})
	response.OK(c, nil)
}

// @Summary  删除字典项
// @Tags     管理后台-系统
// @Produce  json
// @Security BearerAuth
// @Param    id path int true "字典项ID"
// @Success  200 {object} response.Body
// @Router   /admin/dicts/{id} [delete]
func (h *Handler) deleteDict(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.Fail(c, errcode.ErrInvalidParams)
		return
	}
	adminID := c.GetInt64(middleware.CtxAdminID)
	if err := h.svc.DeleteDict(c.Request.Context(), id); err != nil {
		response.Fail(c, err)
		return
	}
	h.svc.addAudit(&AuditLog{AdminID: adminID, Action: "del_dict", ObjType: "dict", Oid: id})
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

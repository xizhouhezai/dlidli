package live

import (
	"strconv"

	"github.com/dlidli/server/internal/middleware"
	"github.com/dlidli/server/internal/pkg/errcode"
	"github.com/dlidli/server/internal/pkg/response"
	"github.com/gin-gonic/gin"
)

// 人工巡查（M4-LIV-05，spec §5E.4）：管理端可查直播中房间、手动切断、恢复。
//
// 鉴权链与既有后台接口一致（AdminAuth + RequirePerm("live:audit")，写法对齐 gift_handler 的
// RegisterDevRoutes）：仅加路由不加权限点会让非 super 管理员全部 403，故权限点须同时落库
// （见 admin/rbac_model.go 的 builtinPerms / builtinRoles）。
type PatrolHandler struct {
	svc *Service
}

func NewPatrolHandler(svc *Service) *PatrolHandler { return &PatrolHandler{svc: svc} }

// RegisterPatrolRoutes 注册后台巡查路由（挂在已套 AdminAuth + RequirePerm 的 /admin 组上）。
func (h *PatrolHandler) RegisterPatrolRoutes(adminAuthed *gin.RouterGroup) {
	g := adminAuthed.Group("/live")
	g.GET("/rooms", h.liveRooms)
	g.POST("/rooms/:id/cut", h.cut)
	g.POST("/rooms/:id/restore", h.restore)
	g.GET("/audit/status", h.auditStatus)
}

// AuditStatus 截帧机审运行态（供巡查台判断机审是否真的在跑）。
type AuditStatus struct {
	Enabled              bool    `json:"enabled"`
	FrameIntervalSeconds int     `json:"frame_interval_seconds"`
	SrsAPIBase           string  `json:"srs_api_base"`
	FrameDir             string  `json:"frame_dir"`
	BlackYAvgMax         float64 `json:"black_yavg_max"`
	FlatRangeMax         float64 `json:"flat_range_max"`
}

// @Summary  截帧机审运行态（是否启用 + 当前配置）
// @Tags     直播巡查
// @Produce  json
// @Security BearerAuth
// @Success  200 {object} response.Body{data=live.AuditStatus}
// @Failure  200 {object} response.Body "91027 截帧机审未启用"
// @Router   /admin/live/audit/status [get]
// @Description 机审关闭时返回 91027（enabled=false），便于巡查台区分「机审在跑但全绿」与「机审根本没开」；
// @Description 这条区分是必要的：截帧失败/无法判定走 fail-open（degraded=true 且不切断），
// @Description 若不暴露开关状态，静默全绿与正常全绿在外部不可分辨（plan §2D.2 待替换点）。
func (h *PatrolHandler) auditStatus(c *gin.Context) {
	cfg := h.svc.cfg.Live.Audit
	if !cfg.Enabled {
		response.Fail(c, errcode.ErrLiveAuditDisabled)
		return
	}
	response.OK(c, AuditStatus{
		Enabled:              true,
		FrameIntervalSeconds: cfg.FrameIntervalSeconds,
		SrsAPIBase:           cfg.SrsAPIBase,
		FrameDir:             cfg.FrameDir,
		BlackYAvgMax:         cfg.BlackYAvgMax,
		FlatRangeMax:         cfg.FlatRangeMax,
	})
}

// PatrolRoom 巡查列表里的房间视图（房间信息 + 在线人数 + 最近截帧判定摘要）。
type PatrolRoom struct {
	ID        int64  `json:"id"`
	AnchorID  string `json:"anchor_id"` // 字符串避免 JS 精度丢失（与 RoomInfo 同口径）
	Title     string `json:"title"`
	Status    int8   `json:"status"`
	BanStatus int8   `json:"ban_status"`
	BanReason string `json:"ban_reason"`
	Online    int64  `json:"online"`
	StartedAt string `json:"started_at,omitempty"`
	// 最近一次截帧机审判定摘要（无记录时为 null）
	LastAudit *PatrolAuditBrief `json:"last_audit"`
}

// PatrolAuditBrief 最近一次截帧判定摘要。
type PatrolAuditBrief struct {
	ID        int64  `json:"id"`
	Pass      bool   `json:"pass"`
	Degraded  bool   `json:"degraded"`
	HitWords  string `json:"hit_words"`
	ImageKey  string `json:"image_key"`
	Width     int    `json:"width"`
	Height    int    `json:"height"`
	CostMs    int    `json:"cost_ms"`
	CreatedAt string `json:"created_at"`
}

type PatrolRoomListResult struct {
	List  []*PatrolRoom `json:"list"`
	Total int           `json:"total"`
}

// @Summary  直播中房间巡查列表（含在线人数与最近截帧判定）
// @Tags     直播巡查
// @Produce  json
// @Security BearerAuth
// @Success  200 {object} response.Body{data=live.PatrolRoomListResult}
// @Failure  200 {object} response.Body "403 无 live:audit 权限"
// @Router   /admin/live/rooms [get]
func (h *PatrolHandler) liveRooms(c *gin.Context) {
	rooms, err := h.svc.repo.ListLiveRoomsAll()
	if err != nil {
		response.Fail(c, err)
		return
	}
	list := make([]*PatrolRoom, 0, len(rooms))
	for i := range rooms {
		r := rooms[i]
		item := &PatrolRoom{
			ID:        r.ID,
			AnchorID:  strconv.FormatInt(r.AnchorID, 10), // 字符串避免 JS 精度丢失（与 RoomInfo 同口径）
			Title:     r.Title,
			Status:    r.Status,
			BanStatus: r.BanStatus,
			BanReason: r.BanReason,
		}
		if r.StartedAt != nil {
			item.StartedAt = r.StartedAt.Format("2006-01-02 15:04:05")
		}
		// 在线人数走 Redis（与房间详情同口径）；Redis 不可用时降级 DB 兜底
		if n, err := h.svc.online.Count(c.Request.Context(), r.ID); err == nil {
			item.Online = n
		}
		if rec, err := h.svc.repo.LatestFrameAudit(r.ID); err == nil && rec != nil {
			item.LastAudit = &PatrolAuditBrief{
				ID: rec.ID, Pass: rec.Pass, Degraded: rec.Degraded, HitWords: rec.HitWords,
				ImageKey: rec.ImageKey, Width: rec.Width, Height: rec.Height, CostMs: rec.CostMs,
				CreatedAt: rec.CreatedAt.Format("2006-01-02 15:04:05"),
			}
		}
		list = append(list, item)
	}
	response.OK(c, PatrolRoomListResult{List: list, Total: len(list)})
}

type cutReq struct {
	Reason string `json:"reason"`
}

// @Summary  手动切断直播（人工巡查）
// @Tags     直播巡查
// @Accept   json
// @Produce  json
// @Security BearerAuth
// @Param    id   path int     true "房间ID"
// @Param    body body cutReq  false "切断原因"
// @Success  200 {object} response.Body{data=live.CutResult}
// @Failure  200 {object} response.Body "91007 房间不存在 / 91028 房间已被封禁"
// @Router   /admin/live/rooms/{id}/cut [post]
func (h *PatrolHandler) cut(c *gin.Context) {
	adminID := c.GetInt64(middleware.CtxAdminID)
	id, ok := pathID(c)
	if !ok {
		return
	}
	var req cutReq
	_ = c.ShouldBindJSON(&req) // 原因可选；空则走默认文案
	res, err := h.svc.CutRoom(c.Request.Context(), id, ViolationSourceManual, req.Reason, 0, adminID)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, res)
}

// @Summary  恢复直播（解封，人工巡查）
// @Tags     直播巡查
// @Produce  json
// @Security BearerAuth
// @Param    id path int true "房间ID"
// @Success  200 {object} response.Body{data=object} "restored=true"
// @Failure  200 {object} response.Body "91007 房间不存在 / 91029 房间未被封禁"
// @Router   /admin/live/rooms/{id}/restore [post]
func (h *PatrolHandler) restore(c *gin.Context) {
	adminID := c.GetInt64(middleware.CtxAdminID)
	id, ok := pathID(c)
	if !ok {
		return
	}
	if err := h.svc.RestoreRoom(c.Request.Context(), id, adminID); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"restored": true})
}

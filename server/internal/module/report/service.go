package report

import (
	"context"
	"encoding/csv"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/dlidli/server/internal/module/account"
	"github.com/dlidli/server/internal/module/danmaku"
	"github.com/dlidli/server/internal/module/dynamic"
	"github.com/dlidli/server/internal/module/interaction"
	"github.com/dlidli/server/internal/module/notify"
	"github.com/dlidli/server/internal/module/video"
	"github.com/dlidli/server/internal/pkg/bvid"
	"github.com/dlidli/server/internal/pkg/errcode"
	"github.com/dlidli/server/internal/pkg/snowflake"
	"go.uber.org/zap"
)

// Service 举报域服务：C 端提交举报、后台队列与处理（删除内容/处罚/忽略）。
type Service struct {
	repo           *Repo
	videoSvc       *video.Service
	accountSvc     *account.Service
	interactionSvc *interaction.Service
	danmakuSvc     *danmaku.Service
	dynamicSvc     *dynamic.Service
	notifySvc      *notify.Service
	log            *zap.Logger
}

func NewService(repo *Repo, videoSvc *video.Service, accountSvc *account.Service,
	interactionSvc *interaction.Service, danmakuSvc *danmaku.Service,
	dynamicSvc *dynamic.Service, notifySvc *notify.Service, log *zap.Logger,
) *Service {
	return &Service{repo: repo, videoSvc: videoSvc, accountSvc: accountSvc,
		interactionSvc: interactionSvc, danmakuSvc: danmakuSvc,
		dynamicSvc: dynamicSvc, notifySvc: notifySvc, log: log}
}

// SubmitReq 提交举报请求。
type SubmitReq struct {
	TargetType int8   `json:"target_type" binding:"required,oneof=1 2 3 4 5"`
	TargetID   string `json:"target_id" binding:"required"` // 视频传 bvid，其余传对象 ID（字符串）
	ReasonType int8   `json:"reason_type" binding:"required,oneof=1 2 3 4 5 6"`
	Reason     string `json:"reason" binding:"max=500"`
}

// Submit 提交举报：对象存在性校验 → 同对象防重复 → 落库。
func (s *Service) Submit(ctx context.Context, uid int64, req *SubmitReq) error {
	targetID, err := s.resolveTarget(ctx, req.TargetType, req.TargetID)
	if err != nil {
		return err
	}

	exist, err := s.repo.FindDuplicated(uid, targetID, req.TargetType)
	if err != nil {
		return err
	}
	if exist != nil {
		return errcode.ErrInvalidParams.WithMsg("你已举报过该内容")
	}

	return s.repo.Create(&Report{
		ID: snowflake.NextID(), ReporterID: uid,
		TargetType: req.TargetType, TargetID: targetID,
		ReasonType: req.ReasonType, Reason: strings.TrimSpace(req.Reason),
	})
}

// resolveTarget 解析并校验举报目标：视频传 bvid（解码内部 ID），其余传数字 ID。
func (s *Service) resolveTarget(ctx context.Context, targetType int8, targetIDStr string) (int64, error) {
	if targetType == TargetVideo {
		id := bvid.Decode(targetIDStr)
		if id <= 0 {
			return 0, errcode.ErrInvalidParams.WithMsg("稿件标识不合法")
		}
		if _, _, err := s.videoSvc.VideoBrief(ctx, id); err != nil {
			return 0, err
		}
		return id, nil
	}

	id, err := strconv.ParseInt(targetIDStr, 10, 64)
	if err != nil || id <= 0 {
		return 0, errcode.ErrInvalidParams
	}
	switch targetType {
	case TargetComment:
		if _, _, err := s.interactionSvc.CommentBrief(ctx, id); err != nil {
			return 0, err
		}
	case TargetDanmaku:
		if _, _, err := s.danmakuSvc.DanmakuBrief(ctx, id); err != nil {
			return 0, err
		}
	case TargetDynamic:
		if _, _, err := s.dynamicSvc.DynamicBrief(ctx, id); err != nil {
			return 0, err
		}
	case TargetUser:
		if _, err := s.accountSvc.Me(ctx, id); err != nil {
			return 0, err
		}
	}
	return id, nil
}

// AdminItem 举报列表项（含对象摘要）。
type AdminItem struct {
	ID           string    `json:"id"`
	TargetType   int8      `json:"target_type"`
	TargetName   string    `json:"target_name"` // 对象类型文案
	TargetDesc   string    `json:"target_desc"` // 对象摘要（标题/内容/昵称）
	TargetID     string    `json:"target_id"`
	ReporterName string    `json:"reporter_name"`
	ReasonType   int8      `json:"reason_type"`
	ReasonName   string    `json:"reason_name"`
	Reason       string    `json:"reason"`
	Status       int8      `json:"status"`
	CreatedAt    time.Time `json:"created_at"`
}

// AdminList 举报队列（status=-1 全部）。
func (s *Service) AdminList(ctx context.Context, status int8, page, size int) ([]AdminItem, int64, error) {
	list, total, err := s.repo.ListByStatus(status, page, size)
	if err != nil {
		return nil, 0, err
	}

	items := make([]AdminItem, 0, len(list))
	for _, r := range list {
		item := AdminItem{
			ID: strconv.FormatInt(r.ID, 10), TargetType: r.TargetType,
			TargetName: TargetNames[r.TargetType], TargetID: strconv.FormatInt(r.TargetID, 10),
			ReasonType: r.ReasonType, ReasonName: ReasonNames[r.ReasonType],
			Reason: r.Reason, Status: r.Status, CreatedAt: r.CreatedAt,
		}
		item.TargetDesc = s.targetDesc(ctx, r.TargetType, r.TargetID)
		if p, err := s.accountSvc.Me(ctx, r.ReporterID); err == nil {
			item.ReporterName = p.Nickname
		}
		items = append(items, item)
	}
	return items, total, nil
}

// ExportCSV 举报列表导出（当前状态筛选，上限 10000；SYS-06）。
func (s *Service) ExportCSV(ctx context.Context, status int8) ([]byte, string, error) {
	list, _, err := s.AdminList(ctx, status, 1, 10000)
	if err != nil {
		return nil, "", err
	}
	var sb strings.Builder
	sb.WriteString("\xEF\xBB\xBF") // UTF-8 BOM
	w := csv.NewWriter(&sb)
	_ = w.Write([]string{"ID", "对象类型", "对象摘要", "对象ID", "举报人", "原因类型", "补充说明", "状态", "时间"})
	for _, r := range list {
		st := "待处理"
		if r.Status == 1 {
			st = "已处理"
		}
		_ = w.Write([]string{
			r.ID, r.TargetName, r.TargetDesc, r.TargetID, r.ReporterName,
			r.ReasonName, r.Reason, st, r.CreatedAt.Format("2006-01-02 15:04:05"),
		})
	}
	w.Flush()
	if err := w.Error(); err != nil {
		return nil, "", err
	}
	return []byte(sb.String()), fmt.Sprintf("reports-%s.csv", time.Now().Format("20060102-150405")), nil
}

// targetDesc 对象摘要（视频标题/评论弹幕动态内容/用户昵称；对象已删除时提示）。
func (s *Service) targetDesc(ctx context.Context, targetType int8, targetID int64) string {
	deleted := "（内容已删除）"
	switch targetType {
	case TargetVideo:
		if title, _, err := s.videoSvc.VideoBrief(ctx, targetID); err == nil {
			return title
		}
	case TargetComment:
		if content, _, err := s.interactionSvc.CommentBrief(ctx, targetID); err == nil {
			return content
		}
	case TargetDanmaku:
		if content, _, err := s.danmakuSvc.DanmakuBrief(ctx, targetID); err == nil {
			return content
		}
	case TargetDynamic:
		if content, _, err := s.dynamicSvc.DynamicBrief(ctx, targetID); err == nil {
			return content
		}
	case TargetUser:
		if p, err := s.accountSvc.Me(ctx, targetID); err == nil {
			return p.Nickname
		}
	}
	return deleted
}

// HandleReq 处理举报请求。
type HandleReq struct {
	Action string `json:"action" binding:"required,oneof=ignore delete punish"` // 忽略/删除内容/删除并处罚
	Note   string `json:"note" binding:"max=255"`                                // 处理备注
	Punish string `json:"punish" binding:"omitempty,oneof=mute ban"`             // punish 时的处罚类型
	Days   int    `json:"days"`                                                  // 处罚天数（ban 传 0 = 永久）
}

// Handle 处理举报：ignore 忽略；delete 删除内容；punish 删除内容并处罚作者。
// 处理结果通过站内通知反馈举报人。
func (s *Service) Handle(ctx context.Context, adminID int64, reportIDStr string, req *HandleReq) error {
	reportID, err := strconv.ParseInt(reportIDStr, 10, 64)
	if err != nil {
		return errcode.ErrInvalidParams
	}
	rep, err := s.repo.FindByID(reportID)
	if err != nil {
		return err
	}
	if rep == nil || rep.Status != StatusPending {
		return errcode.ErrNotFound.WithMsg("举报不存在或已处理")
	}

	var result string
	switch req.Action {
	case "ignore":
		result = "已忽略"
		if req.Note != "" {
			result += "：" + req.Note
		}
		if err := s.repo.UpdateHandle(reportID, adminID, StatusIgnored, result); err != nil {
			return err
		}
	case "delete", "punish":
		desc, ownerUID, err := s.deleteTarget(ctx, rep.TargetType, rep.TargetID)
		if err != nil {
			return err
		}
		result = fmt.Sprintf("已删除%s：%s", TargetNames[rep.TargetType], desc)
		if req.Action == "punish" {
			if ownerUID <= 0 {
				return errcode.ErrInvalidParams.WithMsg("该对象无关联作者，无法处罚")
			}
			if err := s.accountSvc.PunishUser(ctx, ownerUID, req.Punish, req.Days); err != nil {
				return err
			}
			result += fmt.Sprintf("；已处罚用户（%s）", req.Punish)
		}
		if req.Note != "" {
			result += "；备注：" + req.Note
		}
		if err := s.repo.UpdateHandle(reportID, adminID, StatusHandled, result); err != nil {
			return err
		}
	}

	// 处理结果反馈举报人（旁路，失败仅日志）
	s.notifySvc.Push(rep.ReporterID, adminID, notify.TypeSystem,
		"你举报的内容已处理："+result, "")
	return nil
}

// deleteTarget 删除被举报对象，返回对象摘要与作者 UID。
func (s *Service) deleteTarget(ctx context.Context, targetType int8, targetID int64) (desc string, ownerUID int64, err error) {
	switch targetType {
	case TargetVideo:
		title, owner, err := s.videoSvc.VideoBrief(ctx, targetID)
		if err != nil {
			return "", 0, err
		}
		if err := s.videoSvc.AdminDelete(ctx, bvid.Encode(targetID)); err != nil {
			return "", 0, err
		}
		return title, owner, nil
	case TargetComment:
		content, owner, err := s.interactionSvc.CommentBrief(ctx, targetID)
		if err != nil {
			return "", 0, err
		}
		if err := s.interactionSvc.AdminDeleteComment(ctx, targetID); err != nil {
			return "", 0, err
		}
		return content, owner, nil
	case TargetDanmaku:
		content, owner, err := s.danmakuSvc.DanmakuBrief(ctx, targetID)
		if err != nil {
			return "", 0, err
		}
		if err := s.danmakuSvc.AdminDeleteDanmaku(ctx, targetID); err != nil {
			return "", 0, err
		}
		return content, owner, nil
	case TargetDynamic:
		content, owner, err := s.dynamicSvc.DynamicBrief(ctx, targetID)
		if err != nil {
			return "", 0, err
		}
		if err := s.dynamicSvc.AdminDeleteDynamic(ctx, targetID); err != nil {
			return "", 0, err
		}
		return content, owner, nil
	case TargetUser:
		p, err := s.accountSvc.Me(ctx, targetID)
		if err != nil {
			return "", 0, err
		}
		return p.Nickname, targetID, nil
	}
	return "", 0, errcode.ErrInvalidParams
}

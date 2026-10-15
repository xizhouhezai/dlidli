package interaction

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/dlidli/server/internal/module/account"
	"github.com/dlidli/server/internal/module/growth"
	"github.com/dlidli/server/internal/module/notify"
	"github.com/dlidli/server/internal/module/video"
	"github.com/dlidli/server/internal/pkg/contentmod"
	"github.com/dlidli/server/internal/pkg/errcode"
	"github.com/dlidli/server/internal/pkg/snowflake"
	"go.uber.org/zap"
)

const replyPreviewCount = 2

// VipBadgeProvider 会员标识的只读提供者（M4-VIP-01 权益落地：评论带徽章）。
//
// 用「最小接口 + 注入」而非 import 会员域：读模型只需知道「谁是会员、昵称什么颜色」，
// 不因此与会员域互相依赖；也便于测试注入替身（无需真实 membership 表）。
type VipBadgeProvider interface {
	// VipColors 返回「生效中会员」的 uid → 昵称颜色（非会员不在返回值中）。
	VipColors(userIDs []int64) (map[int64]string, error)
}

type Service struct {
	repo       *Repo
	videoSvc   *video.Service
	accountSvc *account.Service
	notifySvc  *notify.Service
	growthSvc  *growth.Service
	log        *zap.Logger
	// vipBadge 会员标识提供者（router 装配注入；nil 表示不渲染会员标识）。
	vipBadge VipBadgeProvider
}

func NewService(repo *Repo, videoSvc *video.Service, accountSvc *account.Service, notifySvc *notify.Service, growthSvc *growth.Service, log *zap.Logger) *Service {
	return &Service{repo: repo, videoSvc: videoSvc, accountSvc: accountSvc, notifySvc: notifySvc, growthSvc: growthSvc, log: log}
}

// SetVipBadge 注入会员标识提供者（M4-VIP-01；nil 表示不渲染，向后兼容）。
func (s *Service) SetVipBadge(p VipBadgeProvider) { s.vipBadge = p }

// ---- 评论 ----

// AddComment 发布评论/回复；禁言/封禁拦截；命中敏感词影子屏蔽。
func (s *Service) AddComment(ctx context.Context, uid int64, bv string, req *AddCommentReq) (*CommentItem, error) {
	if err := s.accountSvc.EnsureCanPublish(ctx, uid); err != nil {
		return nil, err
	}
	videoID, err := s.videoSvc.PublishedIDByBvid(ctx, bv)
	if err != nil {
		return nil, err
	}

	content := strings.TrimSpace(req.Content)
	if content == "" {
		return nil, errcode.ErrInvalidParams
	}

	c := &Comment{
		ID:      snowflake.NextID(),
		Oid:     videoID,
		ObjType: ObjVideo,
		UserID:  uid,
		Content: content,
	}

	// 楼中楼：校验 root/parent 归属
	var replyToUID int64 // 被回复人（通知用）
	if req.RootID != "" {
		rootID, err := strconv.ParseInt(req.RootID, 10, 64)
		if err != nil {
			return nil, errcode.ErrInvalidParams
		}
		root, err := s.repo.FindComment(rootID)
		if err != nil {
			return nil, err
		}
		if root == nil || root.Status == CommentDeleted || root.RootID != 0 || root.Oid != videoID {
			return nil, errcode.ErrNotFound.WithMsg("原评论不存在")
		}
		c.RootID = rootID
		c.ParentID = rootID
		replyToUID = root.UserID
		if req.ParentID != "" && req.ParentID != req.RootID {
			parentID, err := strconv.ParseInt(req.ParentID, 10, 64)
			if err != nil {
				return nil, errcode.ErrInvalidParams
			}
			parent, err := s.repo.FindComment(parentID)
			if err != nil {
				return nil, err
			}
			if parent == nil || parent.Status == CommentDeleted || parent.RootID != rootID {
				return nil, errcode.ErrNotFound.WithMsg("回复的评论不存在")
			}
			c.ParentID = parentID
			replyToUID = parent.UserID
		}
	}

	// 机审（M2-AUD-01）：命中敏感词/联系方式规则 → 影子屏蔽（仅发送者可见）
	if !contentmod.CheckText(contentmod.SceneComment, content).Pass {
		c.Status = CommentShadow
	}

	if err := s.repo.CreateComment(c); err != nil {
		return nil, err
	}

	if c.Status == CommentNormal {
		if err := s.videoSvc.AddStat(ctx, videoID, "comment_cnt", 1); err != nil {
			s.log.Warn("评论计数回写失败", zap.Error(err))
		}
		if c.RootID != 0 {
			if err := s.repo.AddReplyCnt(c.RootID, 1); err != nil {
				s.log.Warn("回复计数回写失败", zap.Error(err))
			}
		}
		// 发表评论 +1 经验（每日上限 20 次，M2-GRW-01；影子屏蔽不奖励）
		if s.growthSvc != nil {
			s.growthSvc.AddExpWithLimit(ctx, uid, growth.ReasonCommentSend)
		}

		// 通知：回复→被回复人；一级评论→稿件 UP 主
		summary := []rune(content)
		if len(summary) > 30 {
			summary = summary[:30]
		}
		if replyToUID != 0 {
			s.notifySvc.Push(replyToUID, uid, notify.TypeComment, "回复了你的评论："+string(summary), "/video/"+bv)
		} else if owner, err := s.videoSvc.OwnerID(ctx, videoID); err == nil {
			s.notifySvc.Push(owner, uid, notify.TypeComment, "评论了你的稿件："+string(summary), "/video/"+bv)
		}
	}

	items, err := s.toItems(ctx, []Comment{*c}, uid)
	if err != nil {
		return nil, err
	}
	return &items[0], nil
}

// ListComments 一级评论分页（附前 2 条回复预览）。
func (s *Service) ListComments(ctx context.Context, viewerUID int64, bv, sort string, page, size int) ([]CommentItem, int64, error) {
	videoID, err := s.videoSvc.PublishedIDByBvid(ctx, bv)
	if err != nil {
		return nil, 0, err
	}

	roots, total, err := s.repo.ListRoot(videoID, ObjVideo, sort, page, size)
	if err != nil {
		return nil, 0, err
	}
	items, err := s.toItems(ctx, roots, viewerUID)
	if err != nil {
		return nil, 0, err
	}

	// 楼中楼预览（MVP：逐条查询，量小可接受）
	for i := range roots {
		if roots[i].ReplyCnt == 0 {
			continue
		}
		replies, _, err := s.repo.ListReplies(roots[i].ID, 1, replyPreviewCount)
		if err != nil {
			return nil, 0, err
		}
		sub, err := s.toItems(ctx, replies, viewerUID)
		if err != nil {
			return nil, 0, err
		}
		items[i].Replies = sub
	}
	return items, total, nil
}

// ListReplies 楼中楼分页。
func (s *Service) ListReplies(ctx context.Context, viewerUID, rootID int64, page, size int) ([]CommentItem, int64, error) {
	replies, total, err := s.repo.ListReplies(rootID, page, size)
	if err != nil {
		return nil, 0, err
	}
	items, err := s.toItems(ctx, replies, viewerUID)
	return items, total, err
}

// DeleteComment 删除评论：作者本人或稿件 UP 主可删。
func (s *Service) DeleteComment(ctx context.Context, uid, commentID int64) error {
	c, err := s.repo.FindComment(commentID)
	if err != nil {
		return err
	}
	if c == nil || c.Status == CommentDeleted {
		return errcode.ErrNotFound
	}

	if c.UserID != uid {
		owner, err := s.videoSvc.OwnerID(ctx, c.Oid)
		if err != nil {
			return err
		}
		if owner != uid {
			return errcode.ErrForbidden
		}
	}

	if err := s.repo.MarkCommentDeleted(commentID); err != nil {
		return err
	}
	if c.Status == CommentNormal {
		_ = s.videoSvc.AddStat(ctx, c.Oid, "comment_cnt", -1)
		if c.RootID != 0 {
			_ = s.repo.AddReplyCnt(c.RootID, -1)
		}
	}
	return nil
}

// CommentBrief 评论摘要（举报队列展示用）：内容 + 作者。
func (s *Service) CommentBrief(_ context.Context, commentID int64) (content string, userID int64, err error) {
	c, err := s.repo.FindComment(commentID)
	if err != nil {
		return "", 0, err
	}
	if c == nil || c.Status == CommentDeleted {
		return "", 0, errcode.ErrNotFound
	}
	return c.Content, c.UserID, nil
}

// AdminDeleteComment 管理员删除评论（举报处理用，绕过作者/UP 主校验）。
func (s *Service) AdminDeleteComment(ctx context.Context, commentID int64) error {
	c, err := s.repo.FindComment(commentID)
	if err != nil {
		return err
	}
	if c == nil || c.Status == CommentDeleted {
		return errcode.ErrNotFound
	}
	if err := s.repo.MarkCommentDeleted(commentID); err != nil {
		return err
	}
	if c.Status == CommentNormal {
		_ = s.videoSvc.AddStat(ctx, c.Oid, "comment_cnt", -1)
		if c.RootID != 0 {
			_ = s.repo.AddReplyCnt(c.RootID, -1)
		}
	}
	return nil
}

// ---- 点赞 ----

// ToggleVideoLike 视频点赞开关，返回当前是否已赞。
func (s *Service) ToggleVideoLike(ctx context.Context, uid int64, bv string) (bool, error) {
	videoID, err := s.videoSvc.PublishedIDByBvid(ctx, bv)
	if err != nil {
		return false, err
	}
	liked, err := s.repo.ToggleAction(uid, videoID, ObjVideo, ActionLike)
	if err != nil {
		return false, err
	}
	delta := 1
	if !liked {
		delta = -1
	}
	if err := s.videoSvc.AddStat(ctx, videoID, "like_cnt", delta); err != nil {
		s.log.Warn("点赞计数回写失败", zap.Error(err))
	}
	if liked {
		if owner, err := s.videoSvc.OwnerID(ctx, videoID); err == nil {
			s.notifySvc.Push(owner, uid, notify.TypeLike, "赞了你的稿件", "/video/"+bv)
		}
	}
	return liked, nil
}

// VideoLiked 当前用户是否已赞该视频（游客恒为 false）。
func (s *Service) VideoLiked(ctx context.Context, uid int64, bv string) (bool, error) {
	if uid <= 0 {
		return false, nil
	}
	videoID, err := s.videoSvc.PublishedIDByBvid(ctx, bv)
	if err != nil {
		return false, err
	}
	return s.repo.HasAction(uid, videoID, ObjVideo, ActionLike)
}

// ToggleCommentLike 评论点赞开关。
func (s *Service) ToggleCommentLike(_ context.Context, uid, commentID int64) (bool, error) {
	c, err := s.repo.FindComment(commentID)
	if err != nil {
		return false, err
	}
	if c == nil || c.Status == CommentDeleted {
		return false, errcode.ErrNotFound
	}
	liked, err := s.repo.ToggleAction(uid, commentID, ObjComment, ActionLike)
	if err != nil {
		return false, err
	}
	delta := 1
	if !liked {
		delta = -1
	}
	if err := s.repo.AddCommentLike(commentID, delta); err != nil {
		s.log.Warn("评论赞数回写失败", zap.Error(err))
	}
	return liked, nil
}

// ---- 读模型 ----

func (s *Service) toItems(ctx context.Context, list []Comment, viewerUID int64) ([]CommentItem, error) {
	if len(list) == 0 {
		return []CommentItem{}, nil
	}
	uids := make([]int64, 0, len(list))
	for _, c := range list {
		uids = append(uids, c.UserID)
	}
	briefs, err := s.accountSvc.Briefs(ctx, uids)
	if err != nil {
		return nil, err
	}
	// 会员标识（M4-VIP-01）：一次批量查询，避免每条评论各查一次会员表（N+1）。
	// 查询失败不阻断评论列表 —— 会员徽章是展示增强，不是数据完整性的一部分；
	// 但它必须来自服务端，故失败时降级为「无徽章」而不是报错或伪造。
	vipColors := map[int64]string{}
	if s.vipBadge != nil {
		if m, e := s.vipBadge.VipColors(uids); e == nil {
			vipColors = m
		} else {
			s.log.Warn("会员标识查询失败，评论降级为无徽章", zap.Error(e))
		}
	}

	items := make([]CommentItem, 0, len(list))
	for _, c := range list {
		b := briefs[c.UserID]
		color, isVip := vipColors[c.UserID]
		items = append(items, CommentItem{
			ID:      fmt.Sprintf("%d", c.ID),
			Content: c.Content,
			User: UserBrief{
				ID: b.ID, Nickname: b.Nickname, Avatar: b.Avatar, Level: b.Level,
				IsVip: isVip, NickColor: color,
			},
			LikeCnt:   c.LikeCnt,
			ReplyCnt:  c.ReplyCnt,
			IsTop:     c.IsTop == 1,
			IsSelf:    viewerUID > 0 && c.UserID == viewerUID,
			CreatedAt: c.CreatedAt,
		})
	}
	return items, nil
}

package im

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/dlidli/server/internal/module/account"
	"github.com/dlidli/server/internal/module/relation"
	"github.com/dlidli/server/internal/pkg/contentmod"
	"github.com/dlidli/server/internal/pkg/errcode"
	"go.uber.org/zap"
)

// maxMsgLen 单条私信最大字数（PRD MSG-10 ≤500）。
const maxMsgLen = 500

// dailyLimitUnfollow 未互关每天可发送条数（PRD MSG-11 防骚扰）。
const dailyLimitUnfollow = 1

// Service 私信服务：发送（限制+机审+会话维护）、会话/消息读取、未读、WS 推送。
type Service struct {
	repo        *Repo
	accountSvc  *account.Service
	relationSvc *relation.Service
	hub         *Hub
	log         *zap.Logger
}

func NewService(repo *Repo, accountSvc *account.Service, relationSvc *relation.Service, hub *Hub, log *zap.Logger) *Service {
	return &Service{repo: repo, accountSvc: accountSvc, relationSvc: relationSvc, hub: hub, log: log}
}

// SendReq 发送私信请求。
type SendReq struct {
	ToUID       string `json:"to_uid" binding:"required"`
	Content     string `json:"content" binding:"required"`
	ContentType int8   `json:"content_type" binding:"oneof=1 2"` // 1文字 2图片
}

// Send 发送私信：禁言/机审/发送限制 → 会话 upsert → 消息入库 → WS 推送接收方。
func (s *Service) Send(ctx context.Context, uid int64, req *SendReq) (*MessageItem, error) {
	toUID, err := parseUID(req.ToUID)
	if err != nil || toUID <= 0 || toUID == uid {
		return nil, errcode.ErrInvalidParams.WithMsg("接收方不合法")
	}
	// 禁言拦截（发送方）
	if err := s.accountSvc.EnsureCanPublish(ctx, uid); err != nil {
		return nil, err
	}
	// 机审过滤（PRD MSG-12）
	if req.ContentType == 1 {
		content := strings.TrimSpace(req.Content)
		if content == "" {
			return nil, errcode.ErrInvalidParams.WithMsg("消息不能为空")
		}
		if len([]rune(content)) > maxMsgLen {
			return nil, errcode.ErrInvalidParams.WithMsg("消息不能超过 500 字")
		}
		if !contentmod.CheckText(contentmod.SceneComment, content).Pass {
			return nil, errcode.ErrInvalidParams.WithMsg("消息包含敏感内容，已被拦截")
		}
		req.Content = content
	} else {
		if !strings.HasPrefix(req.Content, "http") {
			return nil, errcode.ErrInvalidParams.WithMsg("图片消息需为图片 URL")
		}
	}
	// 拉黑拦截（PRD MSG-12）：对方已拉黑我 → 不可发送
	blockedMe, err := s.relationSvc.IsBlocked(ctx, toUID, uid)
	if err != nil {
		return nil, err
	}
	if blockedMe {
		return nil, errcode.ErrInvalidParams.WithMsg("对方已将你拉黑，无法发送私信")
	}
	// 发送限制（PRD MSG-11）：未互关每天最多 1 条；提示语区分关注方向
	mutual, err := s.relationSvc.IsMutual(ctx, uid, toUID)
	if err != nil {
		return nil, err
	}
	if !mutual {
		cnt, err := s.repo.SentToday(uid, toUID)
		if err != nil {
			return nil, err
		}
		if cnt >= dailyLimitUnfollow {
			// 我已关注对方但对方未回关 → 提示对方；否则提示我先关注对方
			iFollow, err := s.relationSvc.IsFollowing(ctx, uid, toUID)
			if err != nil {
				return nil, err
			}
			if iFollow {
				return nil, errcode.ErrInvalidParams.WithMsg("对方未关注你，今日私信已达上限（互关后不限）")
			}
			return nil, errcode.ErrInvalidParams.WithMsg("你未关注对方，今日私信已达上限（互关后不限）")
		}
	}

	// 会话 upsert（规范化 a<b）
	conv, err := s.repo.FindConversation(uid, toUID)
	if err != nil {
		return nil, err
	}
	if conv == nil {
		a, b := normPair(uid, toUID)
		conv = &Conversation{UserA: a, UserB: b, LastAt: time.Now()}
		if err := s.repo.CreateConversation(conv); err != nil {
			return nil, err
		}
	}
	msg := &Message{ConversationID: conv.ID, SenderID: uid, Content: req.Content, ContentType: req.ContentType}
	if err := s.repo.AddMessage(msg, conv, toUID); err != nil {
		return nil, err
	}
	item := MessageItem{
		ID: msg.ID, SenderID: uid, Content: msg.Content,
		ContentType: msg.ContentType, CreatedAt: msg.CreatedAt, Mine: true,
	}
	// WS 实时推送接收方（在线时，PRD MSG-13）
	s.hub.Push(toUID, MessageItem{
		ID: msg.ID, SenderID: uid, Content: msg.Content,
		ContentType: msg.ContentType, CreatedAt: msg.CreatedAt, Mine: false,
	})
	return &item, nil
}

// Conversations 会话列表。
func (s *Service) Conversations(_ context.Context, uid int64) ([]ConversationItem, error) {
	items, err := s.repo.Conversations(uid)
	if err != nil {
		return nil, err
	}
	if items == nil {
		items = []ConversationItem{}
	}
	return items, nil
}

// Messages 与某用户的会话消息分页 + 已读。
func (s *Service) Messages(ctx context.Context, uid int64, peerUID int64, page, size int) ([]MessageItem, int64, error) {
	if size <= 0 || size > 50 {
		size = 20
	}
	conv, err := s.repo.FindConversation(uid, peerUID)
	if err != nil {
		return nil, 0, err
	}
	if conv == nil {
		return []MessageItem{}, 0, nil
	}
	list, total, err := s.repo.Messages(conv.ID, page, size)
	if err != nil {
		return nil, 0, err
	}
	_ = s.repo.MarkRead(conv.ID, uid)
	items := make([]MessageItem, 0, len(list))
	for _, m := range list {
		items = append(items, MessageItem{
			ID: m.ID, SenderID: m.SenderID, Content: m.Content,
			ContentType: m.ContentType, CreatedAt: m.CreatedAt, Mine: m.SenderID == uid,
		})
	}
	return items, total, nil
}

// UnreadTotal 总未读数。
func (s *Service) UnreadTotal(_ context.Context, uid int64) (int, error) {
	return s.repo.UnreadTotal(uid)
}

// parseUID 字符串 uid → int64。
func parseUID(s string) (int64, error) {
	return strconv.ParseInt(s, 10, 64)
}

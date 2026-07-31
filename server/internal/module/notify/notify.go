// Package notify 站内通知域：点赞/评论/关注等事件通知、未读数、通知列表。
// MVP 同库直写；规模化后改事件总线异步投递 + WebSocket 实时推送（comet）。
package notify

import (
	"context"
	"strconv"
	"time"

	"github.com/dlidli/server/internal/module/account"
	"github.com/dlidli/server/internal/pkg/snowflake"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// 通知类型
const (
	TypeLike    int8 = 1
	TypeComment int8 = 2
	TypeFollow  int8 = 3
	TypeSystem  int8 = 4
)

// Notify 对应 notify 表。
type Notify struct {
	ID        int64 `gorm:"primaryKey"`
	UserID    int64
	SenderID  int64
	Type      int8
	Content   string
	Link      string
	IsRead    int8
	CreatedAt time.Time
}

func (Notify) TableName() string { return "notify" }

// Item 通知列表条目（读模型）。
type Item struct {
	ID        string          `json:"id"`
	Type      int8            `json:"type"`
	Content   string          `json:"content"`
	Link      string          `json:"link"`
	IsRead    bool            `json:"is_read"`
	Sender    account.Profile `json:"sender"`
	CreatedAt time.Time       `json:"created_at"`
}

type Repo struct {
	db *gorm.DB
}

func NewRepo(db *gorm.DB) *Repo {
	return &Repo{db: db}
}

func (r *Repo) Create(n *Notify) error {
	return r.db.Create(n).Error
}

func (r *Repo) List(uid, cursor int64, size int) ([]Notify, error) {
	q := r.db.Where("user_id = ?", uid)
	if cursor > 0 {
		q = q.Where("id < ?", cursor)
	}
	var list []Notify
	err := q.Order("id DESC").Limit(size).Find(&list).Error
	return list, err
}

func (r *Repo) UnreadCount(uid int64) (int64, error) {
	var cnt int64
	err := r.db.Model(&Notify{}).Where("user_id = ? AND is_read = 0", uid).Count(&cnt).Error
	return cnt, err
}

func (r *Repo) MarkAllRead(uid int64) error {
	return r.db.Model(&Notify{}).Where("user_id = ? AND is_read = 0", uid).
		UpdateColumn("is_read", 1).Error
}

type Service struct {
	repo       *Repo
	accountSvc *account.Service
	log        *zap.Logger
}

func NewService(repo *Repo, accountSvc *account.Service, log *zap.Logger) *Service {
	return &Service{repo: repo, accountSvc: accountSvc, log: log}
}

// Push 投递通知（旁路逻辑：自我触发跳过，失败仅日志不影响主流程）。
func (s *Service) Push(recipient, sender int64, ntype int8, content, link string) {
	if recipient <= 0 || recipient == sender {
		return
	}
	n := &Notify{
		ID: snowflake.NextID(), UserID: recipient, SenderID: sender,
		Type: ntype, Content: content, Link: link,
	}
	if err := s.repo.Create(n); err != nil {
		s.log.Warn("通知投递失败", zap.Int64("recipient", recipient), zap.Error(err))
	}
}

// List 通知列表（游标分页）。
func (s *Service) List(ctx context.Context, uid int64, cursorStr string, size int) ([]Item, string, bool, error) {
	var cursor int64
	if cursorStr != "" {
		cursor, _ = strconv.ParseInt(cursorStr, 10, 64)
	}
	list, err := s.repo.List(uid, cursor, size)
	if err != nil {
		return nil, "", false, err
	}

	uids := make([]int64, 0, len(list))
	for _, n := range list {
		uids = append(uids, n.SenderID)
	}
	senders, err := s.accountSvc.Briefs(ctx, uids)
	if err != nil {
		return nil, "", false, err
	}

	items := make([]Item, 0, len(list))
	for _, n := range list {
		items = append(items, Item{
			ID:        strconv.FormatInt(n.ID, 10),
			Type:      n.Type,
			Content:   n.Content,
			Link:      n.Link,
			IsRead:    n.IsRead == 1,
			Sender:    senders[n.SenderID],
			CreatedAt: n.CreatedAt,
		})
	}
	next := ""
	if len(list) > 0 {
		next = strconv.FormatInt(list[len(list)-1].ID, 10)
	}
	return items, next, len(list) == size, nil
}

// UnreadCount 未读数。
func (s *Service) UnreadCount(_ context.Context, uid int64) (int64, error) {
	return s.repo.UnreadCount(uid)
}

// MarkAllRead 全部已读。
func (s *Service) MarkAllRead(_ context.Context, uid int64) error {
	return s.repo.MarkAllRead(uid)
}

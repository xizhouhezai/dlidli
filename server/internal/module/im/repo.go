package im

import (
	"errors"
	"time"

	"gorm.io/gorm"
)

type Repo struct {
	db *gorm.DB
}

func NewRepo(db *gorm.DB) *Repo {
	return &Repo{db: db}
}

// normPair 规范化用户对（小者在前）。
func normPair(a, b int64) (int64, int64) {
	if a < b {
		return a, b
	}
	return b, a
}

// FindConversation 按规范化用户对查会话；不存在返回 (nil, nil)。
func (r *Repo) FindConversation(a, b int64) (*Conversation, error) {
	u1, u2 := normPair(a, b)
	var c Conversation
	err := r.db.Where("user_a = ? AND user_b = ?", u1, u2).First(&c).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &c, nil
}

// CreateConversation 新建会话。
func (r *Repo) CreateConversation(c *Conversation) error {
	return r.db.Create(c).Error
}

// AddMessage 新增消息并更新会话（摘要/时间/接收方未读），事务。
func (r *Repo) AddMessage(msg *Message, conv *Conversation, recvUID int64) error {
	u1, u2 := normPair(conv.UserA, conv.UserB)
	recvUnread := "unread_b"
	if recvUID == u1 {
		recvUnread = "unread_a"
	}
	_ = u2
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(msg).Error; err != nil {
			return err
		}
		return tx.Model(&Conversation{}).Where("id = ?", conv.ID).Updates(map[string]any{
			"last_content": msg.Content,
			"last_at":      time.Now(),
			recvUnread:     gorm.Expr(recvUnread + " + 1"),
		}).Error
	})
}

// Conversations 会话列表（含对方用户信息与我的未读数，按 last_at 倒序）。
func (r *Repo) Conversations(uid int64) ([]ConversationItem, error) {
	var items []ConversationItem
	err := r.db.Raw(`SELECT
			CASE WHEN c.user_a = ? THEN c.user_b ELSE c.user_a END AS peer_id,
			u.nickname, u.avatar,
			c.last_content, c.last_at,
			CASE WHEN c.user_a = ? THEN c.unread_a ELSE c.unread_b END AS unread
		FROM conversation c
		JOIN user u ON u.id = CASE WHEN c.user_a = ? THEN c.user_b ELSE c.user_a END
		WHERE c.user_a = ? OR c.user_b = ?
		ORDER BY c.last_at DESC`, uid, uid, uid, uid, uid).Scan(&items).Error
	return items, err
}

// Messages 会话消息分页（旧→新），并标记接收方已读。
func (r *Repo) Messages(convID int64, page, size int) ([]Message, int64, error) {
	var total int64
	if err := r.db.Model(&Message{}).Where("conversation_id = ?", convID).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	list := make([]Message, 0)
	err := r.db.Where("conversation_id = ?", convID).
		Order("id").
		Offset((page - 1) * size).Limit(size).
		Find(&list).Error
	return list, total, err
}

// MarkRead 标记会话中某接收方消息已读并清零未读数。
func (r *Repo) MarkRead(convID, uid int64) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&Message{}).
			Where("conversation_id = ? AND sender_id <> ? AND read_at IS NULL", convID, uid).
			Update("read_at", time.Now()).Error; err != nil {
			return err
		}
		var conv Conversation
		if err := tx.First(&conv, convID).Error; err != nil {
			return err
		}
		if conv.UserA == uid {
			return tx.Model(&Conversation{}).Where("id = ?", convID).Update("unread_a", 0).Error
		}
		return tx.Model(&Conversation{}).Where("id = ?", convID).Update("unread_b", 0).Error
	})
}

// UnreadTotal 我的总未读数（头部红点）。
func (r *Repo) UnreadTotal(uid int64) (int, error) {
	var n int
	err := r.db.Model(&Conversation{}).
		Select("COALESCE(SUM(CASE WHEN user_a = ? THEN unread_a ELSE unread_b END), 0)", uid).
		Where("user_a = ? OR user_b = ?", uid, uid).
		Scan(&n).Error
	return n, err
}

// SentToday 今日与某接收方的发送数（发送限制用）。
func (r *Repo) SentToday(sender, receiver int64) (int64, error) {
	var n int64
	err := r.db.Model(&Message{}).
		Joins("JOIN conversation c ON c.id = private_message.conversation_id").
		Where("private_message.sender_id = ? AND (c.user_a = ? OR c.user_b = ?) AND private_message.created_at >= ?",
			sender, receiver, receiver, time.Now().Format("2006-01-02")).
		Count(&n).Error
	return n, err
}

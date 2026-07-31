// Package interaction 互动域：评论（两级）、点赞（视频/评论）。
// 投币/收藏/三连在 V1（M2-ITR）接入。
package interaction

import "time"

// user_action 对象类型与动作
const (
	ObjVideo   = 1
	ObjComment = 2

	ActionLike = 1
	ActionCoin = 2
	ActionFav  = 3
)

// Collection 对应 collection 表（ID 字符串化避免 JS 精度丢失）。
type Collection struct {
	ID        int64     `gorm:"primaryKey" json:"id,string"`
	UserID    int64     `json:"-"`
	Name      string    `json:"name"`
	IsDefault int8      `json:"is_default"`
	CreatedAt time.Time `json:"created_at"`
}

func (Collection) TableName() string { return "collection" }

// 评论状态
const (
	CommentNormal  = 0
	CommentShadow  = 1 // 影子屏蔽：仅发送者可见
	CommentDeleted = 2
)

// Comment 对应 comment 表。
type Comment struct {
	ID        int64 `gorm:"primaryKey"`
	Oid       int64
	ObjType   int8
	UserID    int64
	RootID    int64
	ParentID  int64
	Content   string
	LikeCnt   int
	ReplyCnt  int
	Status    int8
	IsTop     int8
	CreatedAt time.Time
}

func (Comment) TableName() string { return "comment" }

// UserAction 对应 user_action 表（点赞/投币/收藏明细）。
type UserAction struct {
	ID           int64 `gorm:"primaryKey;autoIncrement"`
	UserID       int64
	Oid          int64
	ObjType      int8
	Action       int8
	Extra        int
	CollectionID int64 // 收藏夹 ID（action=收藏时有效）
	CreatedAt    time.Time
}

func (UserAction) TableName() string { return "user_action" }

// ---- DTO ----

// AddCommentReq 发布评论请求；RootID/ParentID 为空表示一级评论。
type AddCommentReq struct {
	Content  string `json:"content" binding:"required,max=1000"`
	RootID   string `json:"root_id"`
	ParentID string `json:"parent_id"`
}

// UserBrief 评论作者信息。
type UserBrief struct {
	ID       string `json:"id"`
	Nickname string `json:"nickname"`
	Avatar   string `json:"avatar"`
	Level    int8   `json:"level"`
}

// CommentItem 评论条目。
type CommentItem struct {
	ID        string        `json:"id"`
	Content   string        `json:"content"`
	User      UserBrief     `json:"user"`
	LikeCnt   int           `json:"like_cnt"`
	ReplyCnt  int           `json:"reply_cnt"`
	IsTop     bool          `json:"is_top"`
	IsSelf    bool          `json:"is_self"`
	CreatedAt time.Time     `json:"created_at"`
	Replies   []CommentItem `json:"replies,omitempty"` // 一级评论附带前 2 条回复预览
}

package interaction

import (
	"errors"

	"github.com/dlidli/server/internal/pkg/snowflake"
	"gorm.io/gorm"
)

type Repo struct {
	db *gorm.DB
}

func NewRepo(db *gorm.DB) *Repo {
	return &Repo{db: db}
}

func (r *Repo) CreateComment(c *Comment) error {
	return r.db.Create(c).Error
}

// FindComment 按 ID 查评论；不存在返回 (nil, nil)。
func (r *Repo) FindComment(id int64) (*Comment, error) {
	var c Comment
	err := r.db.First(&c, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &c, nil
}

// ListRoot 一级评论分页：置顶优先，hot 按赞数、new 按时间倒序。
func (r *Repo) ListRoot(oid int64, objType int8, sort string, page, size int) ([]Comment, int64, error) {
	q := r.db.Model(&Comment{}).
		Where("oid = ? AND obj_type = ? AND root_id = 0 AND status = ?", oid, objType, CommentNormal)

	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	order := "is_top DESC, created_at DESC"
	if sort == "hot" {
		order = "is_top DESC, like_cnt DESC, created_at DESC"
	}
	var list []Comment
	err := q.Order(order).Offset((page - 1) * size).Limit(size).Find(&list).Error
	return list, total, err
}

// ListReplies 楼中楼分页（时间升序）。
func (r *Repo) ListReplies(rootID int64, page, size int) ([]Comment, int64, error) {
	q := r.db.Model(&Comment{}).Where("root_id = ? AND status = ?", rootID, CommentNormal)
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var list []Comment
	err := q.Order("created_at").Offset((page - 1) * size).Limit(size).Find(&list).Error
	return list, total, err
}

func (r *Repo) MarkCommentDeleted(id int64) error {
	return r.db.Model(&Comment{}).Where("id = ?", id).Update("status", CommentDeleted).Error
}

// AddReplyCnt 一级评论回复数增减（下限 0）。
func (r *Repo) AddReplyCnt(rootID int64, delta int) error {
	return r.db.Model(&Comment{}).Where("id = ?", rootID).
		UpdateColumn("reply_cnt", gorm.Expr("GREATEST(CAST(reply_cnt AS SIGNED) + ?, 0)", delta)).Error
}

// AddCommentLike 评论赞数增减（下限 0）。
func (r *Repo) AddCommentLike(id int64, delta int) error {
	return r.db.Model(&Comment{}).Where("id = ?", id).
		UpdateColumn("like_cnt", gorm.Expr("GREATEST(CAST(like_cnt AS SIGNED) + ?, 0)", delta)).Error
}

// ToggleAction 点赞开关：已存在则取消（返回 false），否则新增（返回 true）。
func (r *Repo) ToggleAction(uid, oid int64, objType, action int8) (active bool, err error) {
	err = r.db.Transaction(func(tx *gorm.DB) error {
		res := tx.Where("user_id = ? AND oid = ? AND obj_type = ? AND action = ?",
			uid, oid, objType, action).Delete(&UserAction{})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected > 0 {
			active = false
			return nil
		}
		active = true
		return tx.Create(&UserAction{UserID: uid, Oid: oid, ObjType: objType, Action: action}).Error
	})
	return active, err
}

// HasAction 是否存在某动作记录。
func (r *Repo) HasAction(uid, oid int64, objType, action int8) (bool, error) {
	var cnt int64
	err := r.db.Model(&UserAction{}).
		Where("user_id = ? AND oid = ? AND obj_type = ? AND action = ?", uid, oid, objType, action).
		Count(&cnt).Error
	return cnt > 0, err
}

// GetAction 查动作记录（含 extra，如投币数）；不存在返回 (nil, nil)。
func (r *Repo) GetAction(uid, oid int64, objType, action int8) (*UserAction, error) {
	var a UserAction
	err := r.db.Where("user_id = ? AND oid = ? AND obj_type = ? AND action = ?",
		uid, oid, objType, action).First(&a).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &a, nil
}

// CreateAction 新增动作记录；唯一键冲突返回 gorm.ErrDuplicatedKey。
func (r *Repo) CreateAction(a *UserAction) error {
	return r.db.Create(a).Error
}

// ListFavOids 收藏的稿件 ID（收藏时间倒序）。
func (r *Repo) ListFavOids(uid int64, page, size int) ([]int64, int64, error) {
	q := r.db.Model(&UserAction{}).
		Where("user_id = ? AND obj_type = ? AND action = ?", uid, ObjVideo, ActionFav)
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var ids []int64
	err := q.Order("created_at DESC").Offset((page-1)*size).Limit(size).
		Pluck("oid", &ids).Error
	return ids, total, err
}

// ---- 收藏夹 ----

func (r *Repo) ListCollections(uid int64) ([]Collection, error) {
	var list []Collection
	err := r.db.Where("user_id = ?", uid).Order("is_default DESC, created_at").Find(&list).Error
	return list, err
}

func (r *Repo) CreateCollection(c *Collection) error {
	return r.db.Create(c).Error
}

func (r *Repo) RenameCollection(uid, id int64, name string) error {
	return r.db.Model(&Collection{}).Where("id = ? AND user_id = ? AND is_default = 0", id, uid).
		Update("name", name).Error
}

func (r *Repo) DeleteCollection(uid, id int64) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		res := tx.Where("id = ? AND user_id = ? AND is_default = 0", id, uid).Delete(&Collection{})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}
		// 删除夹内收藏记录
		return tx.Where("user_id = ? AND obj_type = ? AND action = ? AND collection_id = ?",
			uid, ObjVideo, ActionFav, id).Delete(&UserAction{}).Error
	})
}

func (r *Repo) FindCollection(uid, id int64) (*Collection, error) {
	var c Collection
	err := r.db.Where("id = ? AND user_id = ?", id, uid).First(&c).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &c, err
}

// EnsureDefaultCollection 确保用户有默认收藏夹（幂等）。
func (r *Repo) EnsureDefaultCollection(uid int64) (int64, error) {
	var c Collection
	err := r.db.Where("user_id = ? AND is_default = 1", uid).First(&c).Error
	if err == nil {
		return c.ID, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return 0, err
	}
	c = Collection{ID: snowflake.NextID(), UserID: uid, Name: "默认收藏夹", IsDefault: 1}
	if err := r.db.Create(&c).Error; err != nil {
		return 0, err
	}
	return c.ID, nil
}

package collection

import (
	"context"
	"errors"

	"github.com/dlidli/server/internal/module/video"
	"github.com/dlidli/server/internal/pkg/errcode"
	"gorm.io/gorm"
)

// Service 合集服务：创建/列表/详情/归集稿件（仅 UP 主本人可管理）。
type Service struct {
	repo     *Repo
	videoSvc *video.Service
}

func NewService(repo *Repo, videoSvc *video.Service) *Service {
	return &Service{repo: repo, videoSvc: videoSvc}
}

// CreateReq 创建合集请求。
type CreateReq struct {
	Title       string `json:"title" binding:"required,max=64"`
	Description string `json:"description" binding:"max=200"`
	Cover       string `json:"cover" binding:"max=255"`
}

// Create 创建合集。
func (s *Service) Create(_ context.Context, uid int64, req *CreateReq) error {
	return s.repo.Create(&Collection{UserID: uid, Title: req.Title, Description: req.Description, Cover: req.Cover})
}

// ListByUser 某 UP 主的合集卡片。
func (s *Service) ListByUser(_ context.Context, uid int64) ([]Card, error) {
	return s.repo.ListByUser(uid)
}

// Detail 合集详情（含稿件卡片列表）。
func (s *Service) Detail(ctx context.Context, id int64) (*Collection, []video.Card, error) {
	c, err := s.repo.FindByID(id)
	if err != nil || c == nil {
		if err == nil {
			err = errcode.ErrNotFound
		}
		return nil, nil, err
	}
	ids, err := s.repo.VideoIDs(id)
	if err != nil {
		return nil, nil, err
	}
	if len(ids) == 0 {
		return c, []video.Card{}, nil
	}
	cards, err := s.videoSvc.CardsByIDs(ctx, ids)
	return c, cards, err
}

// AddVideo 合集添加稿件（仅本人，稿件须为本人已发布）。
func (s *Service) AddVideo(ctx context.Context, uid, id int64, bv string) error {
	c, err := s.repo.FindByID(id)
	if err != nil || c == nil {
		if err == nil {
			err = errcode.ErrNotFound
		}
		return err
	}
	if c.UserID != uid {
		return errcode.ErrForbidden
	}
	vid, err := s.videoSvc.PublishedIDByBvid(ctx, bv)
	if err != nil {
		return err
	}
	return s.repo.AddVideo(id, vid, 0)
}

// RemoveVideo 合集移除稿件（仅本人）。
func (s *Service) RemoveVideo(ctx context.Context, uid, id int64, bv string) error {
	c, err := s.repo.FindByID(id)
	if err != nil || c == nil {
		if err == nil {
			err = errcode.ErrNotFound
		}
		return err
	}
	if c.UserID != uid {
		return errcode.ErrForbidden
	}
	vid, err := s.videoSvc.PublishedIDByBvid(ctx, bv)
	if err != nil {
		return err
	}
	return s.repo.RemoveVideo(id, vid)
}

// Delete 删除合集（仅本人）。
func (s *Service) Delete(_ context.Context, uid, id int64) error {
	c, err := s.repo.FindByID(id)
	if err != nil {
		return err
	}
	if c == nil {
		return errcode.ErrNotFound
	}
	if c.UserID != uid {
		return errcode.ErrForbidden
	}
	return s.repo.Delete(id)
}

// ensureNoDup 唯一键冲突转友好错误。
func (s *Service) ensureNoDup(err error) error {
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return errcode.ErrInvalidParams.WithMsg("该稿件已在合集中")
	}
	return err
}

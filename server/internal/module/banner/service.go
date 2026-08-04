package banner

import (
	"context"

	"github.com/dlidli/server/internal/module/video"
	"github.com/dlidli/server/internal/pkg/errcode"
)

// Service 运营位服务：公开 Banner 读取 + admin CRUD。
type Service struct {
	repo     *Repo
	videoSvc *video.Service
}

func NewService(repo *Repo, videoSvc *video.Service) *Service {
	return &Service{repo: repo, videoSvc: videoSvc}
}

// Banners 公开 Banner 列表（image 为空时回退稿件封面）。
func (s *Service) Banners(ctx context.Context) ([]Item, error) {
	list, err := s.repo.ListEnabled()
	if err != nil {
		return nil, err
	}
	items := make([]Item, 0, len(list))
	for _, b := range list {
		item := Item{ID: b.ID, Title: b.Title, Image: b.Image, Bvid: b.Bvid}
		if item.Image == "" && b.Bvid != "" {
			if v, err := s.videoSvc.PublicDetail(ctx, b.Bvid); err == nil {
				item.Image = v.Cover
			}
		}
		items = append(items, item)
	}
	return items, nil
}

// SaveReq 新建/编辑 Banner 请求。
type SaveReq struct {
	Title  string `json:"title" binding:"max=64"`
	Image  string `json:"image" binding:"max=255"`
	Bvid   string `json:"bvid" binding:"max=16"`
	Sort   int    `json:"sort"`
	Status int8   `json:"status" binding:"oneof=0 1"`
}

// validate 校验跳转稿件存在。
func (s *Service) validate(ctx context.Context, req *SaveReq) error {
	if req.Bvid != "" {
		if _, err := s.videoSvc.PublicDetail(ctx, req.Bvid); err != nil {
			return errcode.ErrInvalidParams.WithMsg("跳转稿件不存在或未发布")
		}
	}
	return nil
}

// AdminList 全部 Banner（admin）。
func (s *Service) AdminList(_ context.Context) ([]Banner, error) {
	return s.repo.ListAll()
}

// AdminCreate 新建 Banner。
func (s *Service) AdminCreate(ctx context.Context, req *SaveReq) error {
	if err := s.validate(ctx, req); err != nil {
		return err
	}
	return s.repo.Create(&Banner{
		Title: req.Title, Image: req.Image, Bvid: req.Bvid,
		Sort: req.Sort, Status: req.Status,
	})
}

// AdminUpdate 编辑 Banner。
func (s *Service) AdminUpdate(ctx context.Context, id int64, req *SaveReq) error {
	if err := s.validate(ctx, req); err != nil {
		return err
	}
	return s.repo.Update(id, map[string]any{
		"title": req.Title, "image": req.Image, "bvid": req.Bvid,
		"sort": req.Sort, "status": req.Status,
	})
}

// AdminDelete 删除 Banner。
func (s *Service) AdminDelete(_ context.Context, id int64) error {
	return s.repo.Delete(id)
}

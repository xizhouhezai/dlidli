// Package dynamic 动态域：图文动态发布、投稿自动动态、关注流 Feed。
// MVP 拉模式（关注列表 IN 查询 + 游标分页）；规模化后推拉结合（收件箱）。
package dynamic

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/dlidli/server/internal/module/account"
	"github.com/dlidli/server/internal/module/relation"
	"github.com/dlidli/server/internal/module/video"
	"github.com/dlidli/server/internal/pkg/contentmod"
	"github.com/dlidli/server/internal/pkg/errcode"
	"github.com/dlidli/server/internal/pkg/snowflake"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// 动态类型
const (
	TypeVideo      int8 = 1 // 投稿动态（发布时自动生成）
	TypeText       int8 = 2 // 图文动态
	TypeShareVideo int8 = 3 // 转发视频动态（带转发语 + 引用视频）
)

// Dynamic 对应 dynamic 表。
type Dynamic struct {
	ID        int64 `gorm:"primaryKey"`
	UserID    int64
	Type      int8
	Content   string
	VideoID   int64
	Status    int8
	CreatedAt time.Time
}

func (Dynamic) TableName() string { return "dynamic" }

// FeedItem 动态流条目（读模型）。
type FeedItem struct {
	ID        string          `json:"id"`
	Type      int8            `json:"type"`
	Content   string          `json:"content"`
	User      account.Profile `json:"user"`
	Video     *video.Card     `json:"video,omitempty"`
	CreatedAt time.Time       `json:"created_at"`
}

type Repo struct {
	db *gorm.DB
}

func NewRepo(db *gorm.DB) *Repo {
	return &Repo{db: db}
}

func (r *Repo) Create(d *Dynamic) error {
	return r.db.Create(d).Error
}

// ListByUsers 拉模式 Feed：多作者动态按 ID 倒序 + 游标。
func (r *Repo) ListByUsers(userIDs []int64, cursor int64, size int) ([]Dynamic, error) {
	if len(userIDs) == 0 {
		return []Dynamic{}, nil
	}
	q := r.db.Where("user_id IN ? AND status = 0", userIDs)
	if cursor > 0 {
		q = q.Where("id < ?", cursor)
	}
	var list []Dynamic
	err := q.Order("id DESC").Limit(size).Find(&list).Error
	return list, err
}

// FindByID 查动态；不存在返回 (nil, nil)。
func (r *Repo) FindByID(id int64) (*Dynamic, error) {
	var d Dynamic
	err := r.db.First(&d, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &d, nil
}

// MarkDeleted 动态置删除状态（举报处理用）。
func (r *Repo) MarkDeleted(id int64) error {
	return r.db.Model(&Dynamic{}).Where("id = ?", id).UpdateColumn("status", 2).Error
}

type Service struct {
	repo        *Repo
	accountSvc  *account.Service
	videoSvc    *video.Service
	relationSvc *relation.Service
	log         *zap.Logger
}

func NewService(repo *Repo, accountSvc *account.Service, videoSvc *video.Service,
	relationSvc *relation.Service, log *zap.Logger,
) *Service {
	return &Service{repo: repo, accountSvc: accountSvc, videoSvc: videoSvc, relationSvc: relationSvc, log: log}
}

// PostText 发布图文动态。
func (s *Service) PostText(ctx context.Context, uid int64, content string) (*FeedItem, error) {
	if err := s.accountSvc.EnsureCanPublish(ctx, uid); err != nil {
		return nil, err
	}
	content = strings.TrimSpace(content)
	if content == "" || len([]rune(content)) > 1000 {
		return nil, errcode.ErrInvalidParams.WithMsg("动态内容需在 1~1000 字之间")
	}
	// 机审（M2-AUD-01）：命中敏感词/联系方式规则 → 拒绝发布
	if !contentmod.CheckText(contentmod.SceneDynamic, content).Pass {
		return nil, errcode.ErrInvalidParams.WithMsg("内容包含违规信息，请修改后再发布")
	}

	d := &Dynamic{ID: snowflake.NextID(), UserID: uid, Type: TypeText, Content: content}
	if err := s.repo.Create(d); err != nil {
		return nil, err
	}
	items, err := s.assemble(ctx, []Dynamic{*d})
	if err != nil || len(items) == 0 {
		return nil, err
	}
	return &items[0], nil
}

// OnVideoPublished 投稿发布钩子：自动生成投稿动态（幂等由调用侧保证一次触发）。
func (s *Service) OnVideoPublished(videoID, userID int64) {
	d := &Dynamic{ID: snowflake.NextID(), UserID: userID, Type: TypeVideo, VideoID: videoID}
	if err := s.repo.Create(d); err != nil {
		s.log.Warn("投稿动态生成失败", zap.Int64("video_id", videoID), zap.Error(err))
	}
}

// ShareVideo 转发视频到动态：生成带引用的新动态，同时 share_cnt +1。
func (s *Service) ShareVideo(ctx context.Context, uid int64, bv, content string) (*FeedItem, error) {
	if err := s.accountSvc.EnsureCanPublish(ctx, uid); err != nil {
		return nil, err
	}
	videoID, _, _, err := s.videoSvc.PublishedMetaByBvid(ctx, bv)
	if err != nil {
		return nil, err
	}
	content = strings.TrimSpace(content)
	if len([]rune(content)) > 1000 {
		return nil, errcode.ErrInvalidParams.WithMsg("转发语最多 1000 字")
	}
	// 机审（M2-AUD-01）：转发语命中敏感词/联系方式规则 → 拒绝转发
	if content != "" && !contentmod.CheckText(contentmod.SceneDynamic, content).Pass {
		return nil, errcode.ErrInvalidParams.WithMsg("内容包含违规信息，请修改后再转发")
	}

	d := &Dynamic{ID: snowflake.NextID(), UserID: uid, Type: TypeShareVideo, Content: content, VideoID: videoID}
	if err := s.repo.Create(d); err != nil {
		return nil, err
	}
	if err := s.videoSvc.AddStat(ctx, videoID, "share_cnt", 1); err != nil {
		s.log.Warn("转发计数回写失败", zap.Error(err))
	}

	items, err := s.assemble(ctx, []Dynamic{*d})
	if err != nil || len(items) == 0 {
		return nil, err
	}
	return &items[0], nil
}

// DynamicBrief 动态摘要（举报队列展示用）：内容 + 作者。
func (s *Service) DynamicBrief(_ context.Context, id int64) (content string, userID int64, err error) {
	d, err := s.repo.FindByID(id)
	if err != nil {
		return "", 0, err
	}
	if d == nil || d.Status != 0 {
		return "", 0, errcode.ErrNotFound
	}
	return d.Content, d.UserID, nil
}

// AdminDeleteDynamic 管理员删除动态（举报处理用）。
func (s *Service) AdminDeleteDynamic(_ context.Context, id int64) error {
	d, err := s.repo.FindByID(id)
	if err != nil {
		return err
	}
	if d == nil || d.Status != 0 {
		return errcode.ErrNotFound
	}
	return s.repo.MarkDeleted(id)
}

// Feed 关注流（含自己的动态），游标分页。
func (s *Service) Feed(ctx context.Context, uid int64, cursorStr string, size int) ([]FeedItem, string, bool, error) {
	ids, err := s.relationSvc.FollowingIDs(ctx, uid)
	if err != nil {
		return nil, "", false, err
	}
	ids = append(ids, uid) // 自己的动态也进流

	var cursor int64
	if cursorStr != "" {
		cursor, _ = strconv.ParseInt(cursorStr, 10, 64)
	}
	list, err := s.repo.ListByUsers(ids, cursor, size)
	if err != nil {
		return nil, "", false, err
	}

	items, err := s.assemble(ctx, list)
	if err != nil {
		return nil, "", false, err
	}
	next := ""
	if len(list) > 0 {
		next = strconv.FormatInt(list[len(list)-1].ID, 10)
	}
	return items, next, len(list) == size, nil
}

// assemble 拼装读模型：批量取用户资料与视频卡片。
func (s *Service) assemble(ctx context.Context, list []Dynamic) ([]FeedItem, error) {
	if len(list) == 0 {
		return []FeedItem{}, nil
	}
	uidSet := map[int64]struct{}{}
	videoIDs := make([]int64, 0, len(list))
	for _, d := range list {
		uidSet[d.UserID] = struct{}{}
		if (d.Type == TypeVideo || d.Type == TypeShareVideo) && d.VideoID > 0 {
			videoIDs = append(videoIDs, d.VideoID)
		}
	}
	uids := make([]int64, 0, len(uidSet))
	for id := range uidSet {
		uids = append(uids, id)
	}

	users, err := s.accountSvc.Briefs(ctx, uids)
	if err != nil {
		return nil, err
	}
	cardByVideoID, err := s.videoSvc.CardMapByIDs(ctx, videoIDs)
	if err != nil {
		return nil, err
	}

	items := make([]FeedItem, 0, len(list))
	for _, d := range list {
		item := FeedItem{
			ID:        strconv.FormatInt(d.ID, 10),
			Type:      d.Type,
			Content:   d.Content,
			User:      users[d.UserID],
			CreatedAt: d.CreatedAt,
		}
		if d.Type == TypeVideo || d.Type == TypeShareVideo {
			if c, ok := cardByVideoID[d.VideoID]; ok {
				item.Video = &c
			} else {
				continue // 关联稿件已删除/下架，跳过该条动态
			}
		}
		items = append(items, item)
	}
	return items, nil
}

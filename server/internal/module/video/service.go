package video

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/dlidli/server/internal/module/account"
	"github.com/dlidli/server/internal/module/growth"
	"github.com/dlidli/server/internal/module/upload"
	"github.com/dlidli/server/internal/pkg/bvid"
	"github.com/dlidli/server/internal/pkg/config"
	"github.com/dlidli/server/internal/pkg/errcode"
	"github.com/dlidli/server/internal/pkg/playsign"
	"github.com/dlidli/server/internal/pkg/snowflake"
	"github.com/dlidli/server/internal/pkg/storage"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

const coverMaxSize = 5 << 20 // 5MB

// playSignTTL 播放地址签名有效期（下发给客户端的 URL 过期时间）。
const playSignTTL = 6 * time.Hour

// signedPlayURL 拼接带 HMAC 签名与过期的播放地址。
func (s *Service) signedPlayURL(playPath string) string {
	base := strings.TrimRight(s.cfg.Storage.BaseURL, "/")
	q := playsign.Query(s.cfg.JWT.Secret, playPath, playSignTTL)
	return fmt.Sprintf("%s/%s?%s", base, playPath, q)
}

var coverExts = map[string]bool{".jpg": true, ".jpeg": true, ".png": true, ".webp": true}

type Service struct {
	repo       *Repo
	uploadSvc  *upload.Service
	accountSvc *account.Service
	growthSvc  *growth.Service
	rdb        *redis.Client
	cfg        *config.Config
	log        *zap.Logger
	// publishHook 稿件发布旁路回调（动态生成等），装配时注入
	publishHook PublishHook
}

func NewService(repo *Repo, uploadSvc *upload.Service, accountSvc *account.Service, growthSvc *growth.Service, rdb *redis.Client, cfg *config.Config, log *zap.Logger) *Service {
	return &Service{repo: repo, uploadSvc: uploadSvc, accountSvc: accountSvc, growthSvc: growthSvc, rdb: rdb, cfg: cfg, log: log}
}

// Categories 分区列表。
func (s *Service) Categories(_ context.Context) ([]Category, error) {
	return s.repo.Categories()
}

// ---- 后台分区管理（M1-ADM-06） ----

// SaveCategoryReq 新建/编辑分区请求。
type SaveCategoryReq struct {
	ParentID int    `json:"parent_id"`
	Name     string `json:"name" binding:"required,max=32"`
	Sort     int    `json:"sort"`
	Status   int8   `json:"status" binding:"oneof=0 1"`
}

// AdminCategories 后台：全部分区（含停用）。
func (s *Service) AdminCategories(_ context.Context) ([]Category, error) {
	return s.repo.AllCategories()
}

// CreateCategory 新建分区。
func (s *Service) CreateCategory(_ context.Context, req *SaveCategoryReq) (*Category, error) {
	if req.ParentID != 0 {
		parent, err := s.repo.FindCategory(req.ParentID)
		if err != nil {
			return nil, err
		}
		if parent == nil || parent.ParentID != 0 {
			return nil, errcode.ErrInvalidParams.WithMsg("父分区不存在或非一级分区")
		}
	}
	c := &Category{ParentID: req.ParentID, Name: req.Name, Sort: req.Sort, Status: req.Status}
	if err := s.repo.CreateCategory(c); err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			return nil, errcode.ErrInvalidParams.WithMsg("同级分区名已存在")
		}
		return nil, err
	}
	return c, nil
}

// UpdateCategory 编辑分区（名称/排序/状态）。
func (s *Service) UpdateCategory(_ context.Context, id int, req *SaveCategoryReq) error {
	c, err := s.repo.FindCategory(id)
	if err != nil {
		return err
	}
	if c == nil {
		return errcode.ErrNotFound
	}
	if err := s.repo.UpdateCategory(id, map[string]any{
		"name": req.Name, "sort": req.Sort, "status": req.Status,
	}); err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			return errcode.ErrInvalidParams.WithMsg("同级分区名已存在")
		}
		return err
	}
	return nil
}

// DeleteCategory 删除分区（有子分区或稿件时禁删）。
func (s *Service) DeleteCategory(_ context.Context, id int) error {
	c, err := s.repo.FindCategory(id)
	if err != nil {
		return err
	}
	if c == nil {
		return errcode.ErrNotFound
	}
	if c.ParentID == 0 {
		if n, _ := s.repo.CategoryChildCount(id); n > 0 {
			return errcode.ErrInvalidParams.WithMsg("该分区下还有子分区，请先删除子分区")
		}
	}
	if n, _ := s.repo.CategoryVideoCount(id); n > 0 {
		return errcode.ErrInvalidParams.WithMsg("该分区下还有稿件，不可删除")
	}
	return s.repo.DeleteCategory(id)
}

// Submit 投稿：禁言/封禁拦截 → 登记稿件 + 原画流；dev autoApprove 直接发布。
func (s *Service) Submit(ctx context.Context, uid int64, req *SubmitReq) (*Detail, error) {
	if err := s.accountSvc.EnsureCanPublish(ctx, uid); err != nil {
		return nil, err
	}
	fileID, err := strconv.ParseInt(req.FileID, 10, 64)
	if err != nil {
		return nil, errcode.ErrInvalidParams
	}
	file, err := s.uploadSvc.GetUserFile(ctx, fileID)
	if err != nil {
		return nil, err
	}

	if ok, err := s.repo.CategoryExists(req.CategoryID); err != nil {
		return nil, err
	} else if !ok {
		return nil, errcode.ErrInvalidParams.WithMsg("分区不存在")
	}

	for i, t := range req.Tags {
		req.Tags[i] = strings.TrimSpace(t)
		if req.Tags[i] == "" {
			return nil, errcode.ErrInvalidParams.WithMsg("标签不能为空")
		}
	}
	tagsJSON, _ := json.Marshal(req.Tags)

	id := snowflake.NextID()
	v := &Video{
		ID:          id,
		Bvid:        bvid.Encode(id),
		UserID:      uid,
		Title:       strings.TrimSpace(req.Title),
		Cover:       req.Cover,
		Description: req.Description,
		CategoryID:  req.CategoryID,
		Tags:        string(tagsJSON),
		Copyright:   req.Copyright,
	}

	// 状态机：转码开启→转码中（Worker 完成后推进）；关闭→直接待审/发布
	// TODO(M2-AUD): 标题/简介/封面接入机审
	var jobs []int16
	if s.cfg.Transcode.Enabled {
		v.Status = StatusTranscoding
		jobs = []int16{360, 720}
	} else {
		v.Status = StatusReviewing
		if s.cfg.App.AutoApprove {
			now := time.Now()
			v.Status = StatusPublished
			v.PublishedAt = &now
		}
	}

	stream := &Stream{
		Quality:  0, // 原画（源文件直出，转码完成后补充多档位）
		Format:   "mp4",
		PlayPath: file.StoreKey,
		FileSize: file.FileSize,
	}
	if err := s.repo.CreateWithStat(v, stream, jobs); err != nil {
		return nil, err
	}
	return s.detail(ctx, v, true)
}

// UploadCover 上传稿件封面（投稿前调用，返回 URL 填入 SubmitReq.Cover）。
func (s *Service) UploadCover(ctx context.Context, uid int64, fh *multipart.FileHeader, store storage.Storage) (string, error) {
	if store == nil {
		return "", errcode.ErrInternal.WithMsg("存储服务未就绪")
	}
	if fh.Size <= 0 || fh.Size > coverMaxSize {
		return "", errcode.ErrInvalidParams.WithMsg("封面大小须在 5MB 以内")
	}
	ext := strings.ToLower(filepath.Ext(fh.Filename))
	if !coverExts[ext] {
		return "", errcode.ErrInvalidParams.WithMsg("仅支持 jpg / png / webp 格式")
	}

	f, err := fh.Open()
	if err != nil {
		return "", err
	}
	defer f.Close()

	// TODO(M2-AUD): 封面接入图片内容安全机审
	key := fmt.Sprintf("covers/%d_%d%s", uid, time.Now().UnixMilli(), ext)
	return store.Save(ctx, key, io.LimitReader(f, coverMaxSize))
}

// AddView 有效播放上报（前端播放 >5s 触发）：同一观众 8h 内去重。
// TODO(M2)：改为 Redis 计数 + 异步批量回写（见 docs/architecture/backend.md §2.2）。
func (s *Service) AddView(ctx context.Context, bv, viewer string) error {
	v, err := s.repo.FindByBvid(bv)
	if err != nil {
		return err
	}
	if v == nil || v.Status != StatusPublished {
		return errcode.ErrNotFound
	}

	ok, err := s.rdb.SetNX(ctx, fmt.Sprintf("view:v:%d:%s", v.ID, viewer), 1, 8*time.Hour).Result()
	if err != nil {
		return err
	}
	if !ok {
		return nil // 8h 内重复观看不计数
	}
	return s.repo.IncView(v.ID)
}

// PublishedIDByBvid 返回已发布稿件的内部 ID（供弹幕/评论等模块校验归属）。
func (s *Service) PublishedIDByBvid(_ context.Context, bv string) (int64, error) {
	v, err := s.repo.FindByBvid(bv)
	if err != nil {
		return 0, err
	}
	if v == nil || v.Status != StatusPublished {
		return 0, errcode.ErrNotFound.WithMsg("稿件不存在或未发布")
	}
	return v.ID, nil
}

// PublishedMetaByBvid 返回已发布稿件的关键元信息（供投币规则校验）。
func (s *Service) PublishedMetaByBvid(_ context.Context, bv string) (videoID, ownerID int64, copyright int8, err error) {
	v, err := s.repo.FindByBvid(bv)
	if err != nil {
		return 0, 0, 0, err
	}
	if v == nil || v.Status != StatusPublished {
		return 0, 0, 0, errcode.ErrNotFound.WithMsg("稿件不存在或未发布")
	}
	return v.ID, v.UserID, v.Copyright, nil
}

// CardMapByIDs 按内部 ID 返回已发布稿件卡片映射（供 dynamic 等模块拼装）。
func (s *Service) CardMapByIDs(ctx context.Context, ids []int64) (map[int64]Card, error) {
	if len(ids) == 0 {
		return map[int64]Card{}, nil
	}
	list, err := s.repo.FindPublishedByIDs(ids)
	if err != nil {
		return nil, err
	}
	cards, err := s.cards(ctx, list)
	if err != nil {
		return nil, err
	}
	m := make(map[int64]Card, len(cards))
	for i, v := range list {
		m[v.ID] = cards[i]
	}
	return m, nil
}

// CardsByIDs 按 ID 集合返回已发布稿件卡片（保持入参顺序；供收藏列表等拼装）。
func (s *Service) CardsByIDs(ctx context.Context, ids []int64) ([]Card, error) {
	if len(ids) == 0 {
		return []Card{}, nil
	}
	list, err := s.repo.FindPublishedByIDs(ids)
	if err != nil {
		return nil, err
	}
	cards, err := s.cards(ctx, list)
	if err != nil {
		return nil, err
	}
	// 按入参顺序重排
	idx := make(map[int64]Card, len(cards))
	for i, v := range list {
		idx[v.ID] = cards[i]
	}
	ordered := make([]Card, 0, len(ids))
	for _, id := range ids {
		if c, ok := idx[id]; ok {
			ordered = append(ordered, c)
		}
	}
	return ordered, nil
}

// IncDanmakuCount 弹幕计数 +1（供 danmaku 模块回写）。
func (s *Service) IncDanmakuCount(_ context.Context, videoID int64) error {
	return s.repo.IncStatColumn(videoID, "danmaku_cnt")
}

// AddStat 计数列增减（供 interaction 等模块回写点赞/评论数）。
func (s *Service) AddStat(_ context.Context, videoID int64, column string, delta int) error {
	return s.repo.AddStatColumn(videoID, column, delta)
}

// OwnerID 返回稿件作者 ID（供评论删除权限校验）。
func (s *Service) OwnerID(_ context.Context, videoID int64) (int64, error) {
	v, err := s.repo.FindVideoByID(videoID)
	if err != nil {
		return 0, err
	}
	if v == nil {
		return 0, errcode.ErrNotFound
	}
	return v.UserID, nil
}

// ---- 观看进度（跨端续播）----

const progressTTL = 90 * 24 * time.Hour

// SaveProgress 保存观看进度（秒）；有效观看（≥5s）触发每日一次观看经验（M2-GRW-01）。
func (s *Service) SaveProgress(ctx context.Context, uid int64, bv string, position int) error {
	if position < 0 {
		return errcode.ErrInvalidParams
	}
	videoID, err := s.PublishedIDByBvid(ctx, bv)
	if err != nil {
		return err
	}
	if position >= 5 && s.growthSvc != nil {
		s.growthSvc.AddExpOncePerDay(ctx, uid, growth.ReasonDailyWatch)
	}
	key := fmt.Sprintf("wp:u:%d", uid)
	if err := s.rdb.HSet(ctx, key, fmt.Sprintf("%d", videoID), position).Err(); err != nil {
		return err
	}
	return s.rdb.Expire(ctx, key, progressTTL).Err()
}

// GetProgress 读取观看进度（无记录返回 0）。
func (s *Service) GetProgress(ctx context.Context, uid int64, bv string) (int, error) {
	videoID, err := s.PublishedIDByBvid(ctx, bv)
	if err != nil {
		return 0, err
	}
	pos, err := s.rdb.HGet(ctx, fmt.Sprintf("wp:u:%d", uid), fmt.Sprintf("%d", videoID)).Int()
	if err != nil {
		return 0, nil // 无记录/异常均视为从头播
	}
	return pos, nil
}

// ---- 审核（供 admin 模块调用）----

// ReviewList 待审队列（提交时间升序）。
func (s *Service) ReviewList(ctx context.Context, page, size int) ([]ReviewItem, int64, error) {
	list, total, err := s.repo.ListByStatus(StatusReviewing, page, size)
	if err != nil {
		return nil, 0, err
	}
	cards, err := s.cards(ctx, list)
	if err != nil {
		return nil, 0, err
	}

	items := make([]ReviewItem, 0, len(list))
	for i, v := range list {
		item := ReviewItem{Card: cards[i], Description: v.Description}
		// 原画预览地址
		streams, err := s.repo.StreamsByVideo(v.ID)
		if err != nil {
			return nil, 0, err
		}
		for _, st := range streams {
			if st.Quality == 0 {
				item.PlayURL = s.signedPlayURL(st.PlayPath)
				break
			}
		}
		items = append(items, item)
	}
	return items, total, nil
}

// PublishHook 稿件发布回调（动态生成等旁路逻辑，异步执行、失败不影响主流程）。
type PublishHook func(videoID, userID int64)

// SetPublishHook 注入发布钩子（router 装配时调用）。
func (s *Service) SetPublishHook(h PublishHook) {
	s.publishHook = h
}

func (s *Service) firePublish(videoID, userID int64) {
	if s.publishHook != nil {
		go s.publishHook(videoID, userID)
	}
	// 投稿发布 +10 经验（每日上限 2 次，M2-GRW-01）
	if s.growthSvc != nil {
		s.growthSvc.AddExpWithLimit(context.Background(), userID, growth.ReasonVideoUpload)
	}
}

// Review 审核定档：待审 → 发布 / 驳回。
func (s *Service) Review(_ context.Context, bv string, approve bool, reason string) error {
	v, err := s.repo.FindByBvid(bv)
	if err != nil {
		return err
	}
	if v == nil || v.Status != StatusReviewing {
		return errcode.ErrNotFound.WithMsg("稿件不存在或不在待审状态")
	}

	fields := map[string]any{}
	if approve {
		fields["status"] = StatusPublished
		fields["published_at"] = time.Now()
	} else {
		if strings.TrimSpace(reason) == "" {
			return errcode.ErrInvalidParams.WithMsg("驳回必须填写原因")
		}
		fields["status"] = StatusRejected
		fields["reject_reason"] = reason
	}
	if err := s.repo.UpdateVideoFields(v.ID, fields); err != nil {
		return err
	}
	if approve {
		s.firePublish(v.ID, v.UserID)
	}
	return nil
}

// PublicDetail 公开详情（仅已发布可见）。
func (s *Service) PublicDetail(ctx context.Context, bv string) (*Detail, error) {
	v, err := s.repo.FindByBvid(bv)
	if err != nil {
		return nil, err
	}
	if v == nil || v.Status != StatusPublished {
		return nil, errcode.ErrNotFound.WithMsg("稿件不存在或未发布")
	}
	return s.detail(ctx, v, true)
}

// Mine 我的稿件列表。
func (s *Service) Mine(ctx context.Context, uid int64, page, size int) ([]Card, int64, error) {
	list, total, err := s.repo.ListMine(uid, page, size)
	if err != nil {
		return nil, 0, err
	}
	cards, err := s.cards(ctx, list)
	return cards, total, err
}

// PublicList 首页/分区/个人空间公开列表（uid>0 时仅某 UP 主的投稿）。
func (s *Service) PublicList(ctx context.Context, categoryID int, uid int64, sort string, page, size int) ([]Card, error) {
	list, err := s.repo.ListPublished(categoryID, uid, sort, page, size)
	if err != nil {
		return nil, err
	}
	return s.cards(ctx, list)
}

// Search 标题搜索（供 search 模块）。
func (s *Service) Search(ctx context.Context, keyword string, page, size int) ([]Card, int64, error) {
	list, total, err := s.repo.SearchPublished(keyword, page, size)
	if err != nil {
		return nil, 0, err
	}
	cards, err := s.cards(ctx, list)
	if err != nil {
		return nil, 0, err
	}
	return cards, total, nil
}

// Delete 删除稿件（仅作者本人；软删除）。
func (s *Service) Delete(_ context.Context, uid int64, bv string) error {
	v, err := s.repo.FindByBvid(bv)
	if err != nil {
		return err
	}
	if v == nil || v.Status == StatusDeleted {
		return errcode.ErrNotFound
	}
	if v.UserID != uid {
		return errcode.ErrForbidden
	}
	return s.repo.SoftDelete(v)
}

// ---- 读模型拼装 ----

func (s *Service) cards(ctx context.Context, list []Video) ([]Card, error) {
	if len(list) == 0 {
		return []Card{}, nil
	}
	ids := make([]int64, 0, len(list))
	uids := make([]int64, 0, len(list))
	for _, v := range list {
		ids = append(ids, v.ID)
		uids = append(uids, v.UserID)
	}
	stats, err := s.repo.StatsByIDs(ids)
	if err != nil {
		return nil, err
	}
	owners, err := s.accountSvc.Briefs(ctx, uids)
	if err != nil {
		return nil, err
	}

	cards := make([]Card, 0, len(list))
	for _, v := range list {
		cards = append(cards, s.card(&v, stats[v.ID], owners[v.UserID]))
	}
	return cards, nil
}

func (s *Service) card(v *Video, st Stat, owner account.Profile) Card {
	return Card{
		Bvid:        v.Bvid,
		Title:       v.Title,
		Cover:       v.Cover,
		Duration:    v.Duration,
		Status:      v.Status,
		PublishedAt: v.PublishedAt,
		CreatedAt:   v.CreatedAt,
		Owner:       OwnerBrief{ID: owner.ID, Nickname: owner.Nickname, Avatar: owner.Avatar},
		Stat: StatBrief{
			View: st.ViewCnt, Like: st.LikeCnt, Coin: st.CoinCnt, Fav: st.FavCnt,
			Danmaku: st.DanmakuCnt, Comment: st.CommentCnt, Share: st.ShareCnt,
		},
	}
}

func (s *Service) detail(ctx context.Context, v *Video, withStreams bool) (*Detail, error) {
	cards, err := s.cards(ctx, []Video{*v})
	if err != nil {
		return nil, err
	}

	var tags []string
	_ = json.Unmarshal([]byte(v.Tags), &tags)

	d := &Detail{
		Card:        cards[0],
		Description: v.Description,
		CategoryID:  v.CategoryID,
		Tags:        tags,
		Copyright:   v.Copyright,
	}
	if v.RejectReason != nil {
		d.RejectReason = *v.RejectReason
	}

	if withStreams {
		streams, err := s.repo.StreamsByVideo(v.ID)
		if err != nil {
			return nil, err
		}
		for _, st := range streams {
			d.Streams = append(d.Streams, StreamItem{
				Quality: st.Quality,
				Format:  st.Format,
				// 签名下发（PLY/VID-05）：HMAC + 过期；生产环境可叠加 CDN
				URL: s.signedPlayURL(st.PlayPath),
			})
		}
	}
	return d, nil
}

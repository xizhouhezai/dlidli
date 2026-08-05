package admin

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/dlidli/server/internal/module/account"
	"github.com/dlidli/server/internal/module/video"
	"github.com/dlidli/server/internal/pkg/config"
	"github.com/dlidli/server/internal/pkg/errcode"
	"github.com/dlidli/server/internal/pkg/jwtx"
	"github.com/dlidli/server/internal/pkg/moderate"
	"github.com/dlidli/server/internal/pkg/snowflake"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

const adminTokenTTL = 8 * time.Hour

type Repo struct {
	db *gorm.DB
}

func NewRepo(db *gorm.DB) *Repo {
	return &Repo{db: db}
}

func (r *Repo) FindByUsername(username string) (*AdminUser, error) {
	var u AdminUser
	err := r.db.Where("username = ?", username).First(&u).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (r *Repo) Count() (int64, error) {
	var cnt int64
	err := r.db.Model(&AdminUser{}).Count(&cnt).Error
	return cnt, err
}

func (r *Repo) Create(u *AdminUser) error {
	return r.db.Create(u).Error
}

func (r *Repo) AddAudit(l *AuditLog) error {
	return r.db.Create(l).Error
}

// ListWords 按创建时间倒序返回全部敏感词。
func (r *Repo) ListWords() ([]SensitiveWord, error) {
	var list []SensitiveWord
	err := r.db.Order("id DESC").Find(&list).Error
	return list, err
}

// AllWordStrings 返回纯词列表（供 moderate 热加载）。
func (r *Repo) AllWordStrings() ([]string, error) {
	var words []string
	err := r.db.Model(&SensitiveWord{}).Pluck("word", &words).Error
	return words, err
}

func (r *Repo) CreateWord(w *SensitiveWord) error {
	return r.db.Create(w).Error
}

func (r *Repo) DeleteWord(id int64) error {
	return r.db.Delete(&SensitiveWord{}, id).Error
}

type Service struct {
	repo       *Repo
	videoSvc   *video.Service
	accountSvc *account.Service
	cfg        *config.Config
	log        *zap.Logger
}

func NewService(repo *Repo, videoSvc *video.Service, accountSvc *account.Service, cfg *config.Config, log *zap.Logger) *Service {
	s := &Service{repo: repo, videoSvc: videoSvc, accountSvc: accountSvc, cfg: cfg, log: log}
	s.ensureDefaultAdmin()
	s.seedRBAC()   // 初始化 RBAC 权限点/内置角色，并给默认 admin 绑 super
	s.reloadWords() // 启动时从 DB 加载敏感词库到 moderate
	return s
}

// reloadWords 从 DB 重新加载词库并热刷到 moderate（启动与增删后调用）。
func (s *Service) reloadWords() {
	words, err := s.repo.AllWordStrings()
	if err != nil {
		s.log.Warn("敏感词库加载失败，沿用内置默认词库", zap.Error(err))
		return
	}
	moderate.SetWords(words)
}

// ensureDefaultAdmin 首次启动创建默认管理员（dev 便利；生产应通过运维流程创建并改密）。
func (s *Service) ensureDefaultAdmin() {
	cnt, err := s.repo.Count()
	if err != nil || cnt > 0 {
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte("admin123"), bcrypt.DefaultCost)
	if err != nil {
		return
	}
	if err := s.repo.Create(&AdminUser{
		ID: snowflake.NextID(), Username: "admin", Password: string(hash), Role: "super",
	}); err != nil {
		s.log.Warn("创建默认管理员失败", zap.Error(err))
		return
	}
	s.log.Warn("已创建默认管理员 admin/admin123，请尽快修改密码")
}

// Login 后台登录。
func (s *Service) Login(_ context.Context, req *LoginReq) (*LoginResp, error) {
	u, err := s.repo.FindByUsername(req.Username)
	if err != nil {
		return nil, err
	}
	if u == nil || u.Status != 0 ||
		bcrypt.CompareHashAndPassword([]byte(u.Password), []byte(req.Password)) != nil {
		return nil, errcode.ErrPasswordMismatch
	}

	token, err := jwtx.GenerateAdmin(s.cfg.JWT.Secret, u.ID, adminTokenTTL)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	_ = s.repo.UpdateAdmin(u.ID, map[string]any{"last_login_at": &now})
	return &LoginResp{Token: token, Username: u.Username, Role: u.Role}, nil
}

// ReviewList 待审队列。
func (s *Service) ReviewList(ctx context.Context, page, size int) ([]video.ReviewItem, int64, error) {
	return s.videoSvc.ReviewList(ctx, page, size)
}

// VideoList 稿件管理列表（全状态 + 筛选）。
func (s *Service) VideoList(ctx context.Context, categoryID int, status int8, keyword string, page, size int) ([]video.ReviewItem, int64, error) {
	return s.videoSvc.AdminList(ctx, categoryID, status, keyword, page, size)
}

// SetVideoStatus 稿件下架/恢复并留痕。
func (s *Service) SetVideoStatus(ctx context.Context, adminID int64, bv string, status int8) error {
	if err := s.videoSvc.AdminSetStatus(ctx, bv, status); err != nil {
		return err
	}
	action := "video_offline"
	if status == video.StatusPublished {
		action = "video_restore"
	}
	oid, _ := strconv.ParseInt(bv[2:], 36, 64)
	if err := s.repo.AddAudit(&AuditLog{
		AdminID: adminID, Action: action, ObjType: "video", Oid: oid, Detail: "bvid=" + bv,
	}); err != nil {
		s.log.Warn("稿件定档审计留痕失败", zap.String("bvid", bv), zap.Error(err))
	}
	return nil
}

// DeleteVideo 删除稿件并留痕。
func (s *Service) DeleteVideo(ctx context.Context, adminID int64, bv string) error {
	if err := s.videoSvc.AdminDelete(ctx, bv); err != nil {
		return err
	}
	oid, _ := strconv.ParseInt(bv[2:], 36, 64)
	if err := s.repo.AddAudit(&AuditLog{
		AdminID: adminID, Action: "video_delete", ObjType: "video", Oid: oid, Detail: "bvid=" + bv,
	}); err != nil {
		s.log.Warn("稿件删除审计留痕失败", zap.String("bvid", bv), zap.Error(err))
	}
	return nil
}

// Review 审核稿件并留痕。
func (s *Service) Review(ctx context.Context, adminID int64, bv string, req *ReviewReq) error {
	if err := s.videoSvc.Review(ctx, bv, req.Approve, req.Reason); err != nil {
		return err
	}

	action := "approve"
	if !req.Approve {
		action = "reject"
	}
	oid, _ := strconv.ParseInt(bv[2:], 36, 64) // 仅用于索引参考，精确对象以 detail 为准
	if err := s.repo.AddAudit(&AuditLog{
		AdminID: adminID, Action: action, ObjType: "video", Oid: oid,
		Detail: "bvid=" + bv + " reason=" + req.Reason,
	}); err != nil {
		s.log.Warn("审计日志写入失败", zap.Error(err))
	}
	return nil
}

// ListWords 敏感词列表。
func (s *Service) ListWords() ([]SensitiveWord, error) {
	return s.repo.ListWords()
}

// AddWord 新增敏感词，成功后热刷词库并留痕。
func (s *Service) AddWord(adminID int64, word string) (*SensitiveWord, error) {
	w := &SensitiveWord{ID: snowflake.NextID(), Word: word}
	if err := s.repo.CreateWord(w); err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			return nil, errcode.ErrInvalidParams.WithMsg("该敏感词已存在")
		}
		return nil, err
	}
	s.reloadWords()
	_ = s.repo.AddAudit(&AuditLog{AdminID: adminID, Action: "add_word", ObjType: "sensitive_word", Oid: w.ID, Detail: word})
	return w, nil
}

// DeleteWord 删除敏感词，成功后热刷词库并留痕。
func (s *Service) DeleteWord(adminID, id int64) error {
	if err := s.repo.DeleteWord(id); err != nil {
		return err
	}
	s.reloadWords()
	_ = s.repo.AddAudit(&AuditLog{AdminID: adminID, Action: "del_word", ObjType: "sensitive_word", Oid: id})
	return nil
}

// ListUsers 后台用户查询。
func (s *Service) ListUsers(ctx context.Context, keyword string, status, page, size int) ([]account.AdminUserItem, int64, error) {
	return s.accountSvc.AdminListUsers(ctx, keyword, status, page, size)
}

// PunishUser 处罚/解除并留痕：action = mute|unmute|ban|unban。
func (s *Service) PunishUser(ctx context.Context, adminID, uid int64, action string, days int, reason string) error {
	if err := s.accountSvc.PunishUser(ctx, uid, action, days); err != nil {
		return err
	}
	_ = s.repo.AddAudit(&AuditLog{
		AdminID: adminID, Action: action, ObjType: "user", Oid: uid,
		Detail: fmt.Sprintf("days=%d reason=%s", days, reason),
	})
	return nil
}

// ---- 分区管理（M1-ADM-06，转调 video 服务） ----

// ListCategories 后台分区列表（含停用）。
func (s *Service) ListCategories(ctx context.Context) ([]video.Category, error) {
	return s.videoSvc.AdminCategories(ctx)
}

// CreateCategory 新建分区并留痕。
func (s *Service) CreateCategory(ctx context.Context, adminID int64, req *video.SaveCategoryReq) (*video.Category, error) {
	c, err := s.videoSvc.CreateCategory(ctx, req)
	if err != nil {
		return nil, err
	}
	_ = s.repo.AddAudit(&AuditLog{AdminID: adminID, Action: "add_category", ObjType: "category", Oid: int64(c.ID), Detail: req.Name})
	return c, nil
}

// UpdateCategory 编辑分区并留痕。
func (s *Service) UpdateCategory(ctx context.Context, adminID int64, id int, req *video.SaveCategoryReq) error {
	if err := s.videoSvc.UpdateCategory(ctx, id, req); err != nil {
		return err
	}
	_ = s.repo.AddAudit(&AuditLog{AdminID: adminID, Action: "edit_category", ObjType: "category", Oid: int64(id), Detail: req.Name})
	return nil
}

// DeleteCategory 删除分区并留痕。
func (s *Service) DeleteCategory(ctx context.Context, adminID int64, id int) error {
	if err := s.videoSvc.DeleteCategory(ctx, id); err != nil {
		return err
	}
	_ = s.repo.AddAudit(&AuditLog{AdminID: adminID, Action: "del_category", ObjType: "category", Oid: int64(id)})
	return nil
}

package admin

import (
	"context"
	"encoding/csv"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/dlidli/server/internal/pkg/errcode"
)

// 审计动作/对象中文映射（M2-SYS-01 展示用；未登记回退原样）。
var auditActionNames = map[string]string{
	"approve": "审核通过", "reject": "审核驳回",
	"mute": "禁言", "unmute": "解除禁言", "ban": "封禁", "unban": "解除封禁",
	"add_word": "新增敏感词", "del_word": "删除敏感词",
	"add_category": "新增分区", "edit_category": "编辑分区", "del_category": "删除分区",
	"add_permission": "新增权限点", "edit_permission": "编辑权限点", "del_permission": "删除权限点",
	"add_role": "新增角色", "edit_role": "编辑角色", "del_role": "删除角色",
	"add_admin": "新增账号", "edit_admin": "编辑账号", "del_admin": "删除账号", "reset_pwd": "重置密码",
	"add_config": "新增配置", "edit_config": "编辑配置", "del_config": "删除配置",
	"add_dict": "新增字典项", "edit_dict": "编辑字典项", "del_dict": "删除字典项",
}

var auditObjNames = map[string]string{
	"video": "稿件", "user": "用户", "sensitive_word": "敏感词", "category": "分区",
	"permission": "权限点", "role": "角色", "admin": "账号",
	"config": "配置", "dict": "字典项",
}

// AuditQuery 审计日志查询条件。
type AuditQuery struct {
	AdminID int64
	Action  string
	ObjType string
	From    *time.Time
	To      *time.Time
}

func auditItemOf(l *AuditLog, names map[int64]string) AuditItem {
	item := AuditItem{
		ID: l.ID, AdminID: l.AdminID, Action: l.Action, ObjType: l.ObjType,
		Oid: strconv.FormatInt(l.Oid, 10), Detail: l.Detail, CreatedAt: l.CreatedAt,
	}
	item.AdminName = names[l.AdminID]
	if v, ok := auditActionNames[l.Action]; ok {
		item.ActionName = v
	} else {
		item.ActionName = l.Action
	}
	if v, ok := auditObjNames[l.ObjType]; ok {
		item.ObjName = v
	} else {
		item.ObjName = l.ObjType
	}
	return item
}

// AuditLogs 审计日志分页查询。
func (s *Service) AuditLogs(_ context.Context, q *AuditQuery, page, size int) ([]AuditItem, int64, error) {
	list, total, err := s.repo.ListAuditLogs(q.AdminID, q.Action, q.ObjType, q.From, q.To, page, size)
	if err != nil {
		return nil, 0, err
	}
	ids := make([]int64, 0, len(list))
	for _, l := range list {
		ids = append(ids, l.AdminID)
	}
	names, err := s.repo.AdminNames(ids)
	if err != nil {
		return nil, 0, err
	}
	items := make([]AuditItem, 0, len(list))
	for _, l := range list {
		items = append(items, auditItemOf(&l, names))
	}
	return items, total, nil
}

// ExportAuditLogsCSV 审计日志导出（UTF-8 BOM + CSV，供 Excel 直接打开）。
func (s *Service) ExportAuditLogsCSV(ctx context.Context, q *AuditQuery) ([]byte, string, error) {
	list, err := s.repo.ExportAuditLogs(q.AdminID, q.Action, q.ObjType, q.From, q.To)
	if err != nil {
		return nil, "", err
	}
	ids := make([]int64, 0, len(list))
	for _, l := range list {
		ids = append(ids, l.AdminID)
	}
	names, err := s.repo.AdminNames(ids)
	if err != nil {
		return nil, "", err
	}

	var sb strings.Builder
	sb.WriteString("\xEF\xBB\xBF") // UTF-8 BOM
	w := csv.NewWriter(&sb)
	_ = w.Write([]string{"ID", "操作者", "动作", "对象类型", "对象ID", "详情", "时间"})
	for _, l := range list {
		item := auditItemOf(&l, names)
		_ = w.Write([]string{
			strconv.FormatInt(l.ID, 10), item.AdminName, item.ActionName, item.ObjName,
			item.Oid, l.Detail, l.CreatedAt.Format("2006-01-02 15:04:05"),
		})
	}
	w.Flush()
	if err := w.Error(); err != nil {
		return nil, "", err
	}
	return []byte(sb.String()), fmt.Sprintf("audit-log-%s.csv", time.Now().Format("20060102-150405")), nil
}

// 用户/稿件状态中文（与 admin 前端一致，SYS-06 导出用）。
var userStatusNames = map[int]string{0: "正常", 1: "禁言", 2: "封禁"}
var videoStatusNames = map[int8]string{0: "草稿", 2: "转码中", 3: "审核中", 4: "已发布", 5: "已驳回", 6: "已锁定"}

// ExportUsersCSV 用户列表导出（当前筛选，上限 10000；SYS-06）。
func (s *Service) ExportUsersCSV(ctx context.Context, keyword string, status int) ([]byte, string, error) {
	list, _, err := s.accountSvc.AdminListUsers(ctx, keyword, status, 1, 10000)
	if err != nil {
		return nil, "", err
	}
	var sb strings.Builder
	sb.WriteString("\xEF\xBB\xBF") // UTF-8 BOM
	w := csv.NewWriter(&sb)
	_ = w.Write([]string{"ID", "昵称", "手机号", "等级", "硬币", "状态", "禁言至", "封禁至"})
	for _, u := range list {
		muted, banned := "", ""
		if u.MutedUntil != nil {
			muted = u.MutedUntil.Format("2006-01-02 15:04:05")
		}
		if u.BannedUntil != nil {
			banned = u.BannedUntil.Format("2006-01-02 15:04:05")
		}
		_ = w.Write([]string{
			strconv.FormatInt(u.ID, 10), u.Nickname, u.Phone,
			strconv.Itoa(int(u.Level)), strconv.Itoa(u.Coin),
			userStatusNames[int(u.Status)], muted, banned,
		})
	}
	w.Flush()
	if err := w.Error(); err != nil {
		return nil, "", err
	}
	return []byte(sb.String()), fmt.Sprintf("users-%s.csv", time.Now().Format("20060102-150405")), nil
}

// ExportVideosCSV 稿件列表导出（当前筛选，上限 10000；SYS-06）。
func (s *Service) ExportVideosCSV(ctx context.Context, categoryID int, status int8, keyword string) ([]byte, string, error) {
	list, _, err := s.videoSvc.AdminList(ctx, categoryID, status, keyword, 1, 10000)
	if err != nil {
		return nil, "", err
	}
	// 分区 ID → 名称映射（人读友好）
	catName := map[int]string{}
	if cats, err := s.videoSvc.AdminCategories(ctx); err == nil {
		for _, c := range cats {
			catName[c.ID] = c.Name
		}
	}
	var sb strings.Builder
	sb.WriteString("\xEF\xBB\xBF") // UTF-8 BOM
	w := csv.NewWriter(&sb)
	_ = w.Write([]string{"bvid", "标题", "分区", "UP主", "状态", "播放", "点赞", "发布时间"})
	for _, v := range list {
		pub := ""
		if v.PublishedAt != nil {
			pub = v.PublishedAt.Format("2006-01-02 15:04:05")
		}
		cat := catName[v.CategoryID]
		if cat == "" {
			cat = strconv.Itoa(v.CategoryID)
		}
		_ = w.Write([]string{
			v.Bvid, v.Title, cat, v.Owner.Nickname,
			videoStatusNames[v.Status],
			strconv.FormatInt(v.Stat.View, 10), strconv.FormatInt(v.Stat.Like, 10), pub,
		})
	}
	w.Flush()
	if err := w.Error(); err != nil {
		return nil, "", err
	}
	return []byte(sb.String()), fmt.Sprintf("videos-%s.csv", time.Now().Format("20060102-150405")), nil
}

// ---- 系统配置（M2-SYS-02） ----

// SaveConfigReq 新建/编辑配置请求。
type SaveConfigReq struct {
	ConfigKey string `json:"config_key" binding:"required,max=64"`
	Name      string `json:"name" binding:"max=64"`
	Value     string `json:"value" binding:"max=500"`
	Remark    string `json:"remark" binding:"max=200"`
}

// Configs 配置列表。
func (s *Service) Configs(_ context.Context) ([]SystemConfig, error) {
	return s.repo.ListConfigs()
}

// CreateConfig 新建配置（键唯一）。
func (s *Service) CreateConfig(_ context.Context, req *SaveConfigReq) error {
	if strings.TrimSpace(req.ConfigKey) == "" {
		return errcode.ErrInvalidParams.WithMsg("配置键不能为空")
	}
	return s.repo.CreateConfig(&SystemConfig{
		ConfigKey: strings.TrimSpace(req.ConfigKey),
		Name: req.Name, Value: req.Value, Remark: req.Remark,
	})
}

// UpdateConfig 编辑配置（热更新：业务侧按键读取即生效）。
func (s *Service) UpdateConfig(_ context.Context, id int64, req *SaveConfigReq) error {
	fields := map[string]any{"name": req.Name, "value": req.Value, "remark": req.Remark}
	if req.ConfigKey != "" {
		fields["config_key"] = req.ConfigKey
	}
	return s.repo.UpdateConfig(id, fields)
}

// DeleteConfig 删除配置。
func (s *Service) DeleteConfig(_ context.Context, id int64) error {
	return s.repo.DeleteConfig(id)
}

// GetConfig 按键读取配置值（业务侧热更新读取入口）。
func (s *Service) GetConfig(_ context.Context, key string) (string, error) {
	return s.repo.GetConfigByKey(key)
}

// ---- 数据字典（M2-SYS-02） ----

// SaveDictReq 新建/编辑字典项请求。
type SaveDictReq struct {
	DictType string `json:"dict_type" binding:"required,max=32"`
	Label    string `json:"label" binding:"required,max=64"`
	Value    string `json:"value" binding:"required,max=64"`
	Sort     int    `json:"sort"`
	Remark   string `json:"remark" binding:"max=200"`
}

// DictGroups 字典分组（类型 → 项列表）。
func (s *Service) DictGroups(_ context.Context) (map[string][]DataDict, error) {
	types, err := s.repo.ListDictTypes()
	if err != nil {
		return nil, err
	}
	m := make(map[string][]DataDict, len(types))
	for _, t := range types {
		list, err := s.repo.ListDicts(t)
		if err != nil {
			return nil, err
		}
		m[t] = list
	}
	return m, nil
}

// CreateDict 新建字典项（类型+值唯一）。
func (s *Service) CreateDict(_ context.Context, req *SaveDictReq) error {
	if strings.TrimSpace(req.DictType) == "" {
		return errcode.ErrInvalidParams.WithMsg("字典类型不能为空")
	}
	return s.repo.CreateDict(&DataDict{
		DictType: strings.TrimSpace(req.DictType),
		Label: req.Label, Value: req.Value, Sort: req.Sort, Remark: req.Remark,
	})
}

// UpdateDict 编辑字典项。
func (s *Service) UpdateDict(_ context.Context, id int64, req *SaveDictReq) error {
	fields := map[string]any{"label": req.Label, "value": req.Value, "sort": req.Sort, "remark": req.Remark}
	if req.DictType != "" {
		fields["dict_type"] = req.DictType
	}
	return s.repo.UpdateDict(id, fields)
}

// DeleteDict 删除字典项。
func (s *Service) DeleteDict(_ context.Context, id int64) error {
	return s.repo.DeleteDict(id)
}

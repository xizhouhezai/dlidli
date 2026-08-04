package admin

import (
	"time"

	"gorm.io/gorm"
)

// ---- 审计日志（M2-SYS-01） ----

// ListAuditLogs 审计日志分页查询（可按操作者/动作/对象类型/时间范围筛选），新→旧。
func (r *Repo) ListAuditLogs(adminID int64, action, objType string, from, to *time.Time, page, size int) ([]AuditLog, int64, error) {
	q := r.db.Model(&AuditLog{})
	if adminID > 0 {
		q = q.Where("admin_id = ?", adminID)
	}
	if action != "" {
		q = q.Where("action = ?", action)
	}
	if objType != "" {
		q = q.Where("obj_type = ?", objType)
	}
	if from != nil {
		q = q.Where("created_at >= ?", from)
	}
	if to != nil {
		q = q.Where("created_at <= ?", to)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var list []AuditLog
	err := q.Order("id DESC").Offset((page - 1) * size).Limit(size).Find(&list).Error
	return list, total, err
}

// ExportAuditLogs 审计日志全量导出（按筛选条件，最多 1 万条）。
func (r *Repo) ExportAuditLogs(adminID int64, action, objType string, from, to *time.Time) ([]AuditLog, error) {
	q := r.db.Model(&AuditLog{})
	if adminID > 0 {
		q = q.Where("admin_id = ?", adminID)
	}
	if action != "" {
		q = q.Where("action = ?", action)
	}
	if objType != "" {
		q = q.Where("obj_type = ?", objType)
	}
	if from != nil {
		q = q.Where("created_at >= ?", from)
	}
	if to != nil {
		q = q.Where("created_at <= ?", to)
	}
	var list []AuditLog
	err := q.Order("id DESC").Limit(10000).Find(&list).Error
	return list, err
}

// AdminNames 批量查询管理员昵称。
func (r *Repo) AdminNames(ids []int64) (map[int64]string, error) {
	if len(ids) == 0 {
		return map[int64]string{}, nil
	}
	var list []AdminUser
	if err := r.db.Select("id", "nickname").Where("id IN ?", ids).Find(&list).Error; err != nil {
		return nil, err
	}
	m := make(map[int64]string, len(list))
	for _, u := range list {
		m[u.ID] = u.Nickname
	}
	return m, nil
}

// ---- 系统配置（M2-SYS-02） ----

func (r *Repo) ListConfigs() ([]SystemConfig, error) {
	var list []SystemConfig
	err := r.db.Order("id").Find(&list).Error
	return list, err
}

func (r *Repo) CreateConfig(c *SystemConfig) error {
	return r.db.Create(c).Error
}

func (r *Repo) UpdateConfig(id int64, fields map[string]any) error {
	return r.db.Model(&SystemConfig{}).Where("id = ?", id).Updates(fields).Error
}

func (r *Repo) DeleteConfig(id int64) error {
	return r.db.Delete(&SystemConfig{}, id).Error
}

// GetConfigByKey 按键读取配置值（供业务侧热更新读取）。
func (r *Repo) GetConfigByKey(key string) (string, error) {
	var c SystemConfig
	err := r.db.Where("config_key = ?", key).First(&c).Error
	if err == gorm.ErrRecordNotFound {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return c.Value, nil
}

// ---- 数据字典（M2-SYS-02） ----

// ListDictTypes 字典类型列表（去重）。
func (r *Repo) ListDictTypes() ([]string, error) {
	var types []string
	err := r.db.Model(&DataDict{}).Distinct().Order("dict_type").Pluck("dict_type", &types).Error
	return types, err
}

// ListDicts 按类型查字典项（按 sort）。
func (r *Repo) ListDicts(dictType string) ([]DataDict, error) {
	var list []DataDict
	err := r.db.Where("dict_type = ?", dictType).Order("sort, id").Find(&list).Error
	return list, err
}

func (r *Repo) CreateDict(d *DataDict) error {
	return r.db.Create(d).Error
}

func (r *Repo) UpdateDict(id int64, fields map[string]any) error {
	return r.db.Model(&DataDict{}).Where("id = ?", id).Updates(fields).Error
}

func (r *Repo) DeleteDict(id int64) error {
	return r.db.Delete(&DataDict{}, id).Error
}

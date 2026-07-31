package admin

import (
	"errors"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ---- RBAC Repo ----

// SeedRBAC 幂等初始化权限点与内置角色（每次启动 upsert）。
func (r *Repo) SeedRBAC(nextID func() int64) error {
	// 1. upsert 权限点（按 code 唯一，更新 name/path/icon/sort 等展示字段）
	for _, p := range builtinPermissions {
		perm := p
		perm.ID = nextID()
		if err := r.db.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "code"}},
			DoUpdates: clause.AssignmentColumns([]string{"name", "type", "parent", "path", "icon", "sort"}),
		}).Create(&perm).Error; err != nil {
			return err
		}
	}
	// 2. upsert 内置角色 + 绑定权限
	for _, br := range builtinRoles {
		// 判断角色是否已存在：已存在的内置角色权限交由后台维护，重启不覆盖（避免管理员编辑被 seed 冲掉）
		var existing Role
		findErr := r.db.Where("code = ?", br.Code).First(&existing).Error
		isNew := errors.Is(findErr, gorm.ErrRecordNotFound)
		role := Role{ID: nextID(), Code: br.Code, Name: br.Name, Remark: br.Remark, IsBuiltin: 1}
		if err := r.db.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "code"}},
			DoUpdates: clause.AssignmentColumns([]string{"name", "remark", "is_builtin"}),
		}).Create(&role).Error; err != nil {
			return err
		}
		// 取回真实角色 ID
		var saved Role
		if err := r.db.Where("code = ?", br.Code).First(&saved).Error; err != nil {
			return err
		}
		if br.Code == SuperRoleCode {
			continue // super 不落权限关联，鉴权时短路放行
		}
		if !isNew {
			continue // 已存在：保留当前权限（含后台编辑结果），仅首次创建时绑定默认权限
		}
		// 首次创建：绑定默认权限关联
		if err := r.db.Where("role_id = ?", saved.ID).Delete(&RolePermission{}).Error; err != nil {
			return err
		}
		var permIDs []int64
		if err := r.db.Model(&Permission{}).Where("code IN ?", br.Perms).Pluck("id", &permIDs).Error; err != nil {
			return err
		}
		for _, pid := range permIDs {
			if err := r.db.Create(&RolePermission{RoleID: saved.ID, PermissionID: pid}).Error; err != nil {
				return err
			}
		}
	}
	return nil
}

// PermCodesByAdmin 返回某管理员的权限码集合，以及是否为超管。
func (r *Repo) PermCodesByAdmin(adminID int64) (codes []string, isSuper bool, err error) {
	// 角色 ID 列表
	var roleIDs []int64
	if err = r.db.Model(&UserRole{}).Where("admin_user_id = ?", adminID).Pluck("role_id", &roleIDs).Error; err != nil {
		return nil, false, err
	}
	if len(roleIDs) == 0 {
		return nil, false, nil
	}
	// 是否含 super
	var superCnt int64
	if err = r.db.Model(&Role{}).Where("id IN ? AND code = ?", roleIDs, SuperRoleCode).Count(&superCnt).Error; err != nil {
		return nil, false, err
	}
	if superCnt > 0 {
		return nil, true, nil
	}
	// 权限码去重
	err = r.db.Model(&Permission{}).
		Distinct("admin_permission.code").
		Joins("JOIN admin_role_permission rp ON rp.permission_id = admin_permission.id").
		Where("rp.role_id IN ?", roleIDs).
		Pluck("admin_permission.code", &codes).Error
	return codes, false, err
}

// AllPermissions 权限点全集（按 sort）。
func (r *Repo) AllPermissions() ([]Permission, error) {
	var list []Permission
	err := r.db.Order("sort, id").Find(&list).Error
	return list, err
}

// ListRoles 角色列表。
func (r *Repo) ListRoles() ([]Role, error) {
	var list []Role
	err := r.db.Order("is_builtin DESC, id").Find(&list).Error
	return list, err
}

func (r *Repo) FindRole(id int64) (*Role, error) {
	var role Role
	err := r.db.First(&role, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &role, err
}

func (r *Repo) CreateRole(role *Role) error { return r.db.Create(role).Error }

func (r *Repo) UpdateRole(id int64, fields map[string]any) error {
	return r.db.Model(&Role{}).Where("id = ?", id).Updates(fields).Error
}

func (r *Repo) DeleteRole(id int64) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("role_id = ?", id).Delete(&RolePermission{}).Error; err != nil {
			return err
		}
		if err := tx.Where("role_id = ?", id).Delete(&UserRole{}).Error; err != nil {
			return err
		}
		return tx.Delete(&Role{}, id).Error
	})
}

// RolePermCodes 某角色已分配的权限码。
func (r *Repo) RolePermCodes(roleID int64) ([]string, error) {
	var codes []string
	err := r.db.Model(&Permission{}).
		Joins("JOIN admin_role_permission rp ON rp.permission_id = admin_permission.id").
		Where("rp.role_id = ?", roleID).
		Pluck("admin_permission.code", &codes).Error
	return codes, err
}

// SetRolePerms 重建角色权限关联（按权限码）。
func (r *Repo) SetRolePerms(roleID int64, codes []string) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("role_id = ?", roleID).Delete(&RolePermission{}).Error; err != nil {
			return err
		}
		if len(codes) == 0 {
			return nil
		}
		var permIDs []int64
		if err := tx.Model(&Permission{}).Where("code IN ?", codes).Pluck("id", &permIDs).Error; err != nil {
			return err
		}
		rows := make([]RolePermission, 0, len(permIDs))
		for _, pid := range permIDs {
			rows = append(rows, RolePermission{RoleID: roleID, PermissionID: pid})
		}
		return tx.Create(&rows).Error
	})
}

// CountRoleMembers 各角色成员数（role_id → count）。
func (r *Repo) CountRoleMembers() (map[int64]int64, error) {
	type row struct {
		RoleID int64
		Cnt    int64
	}
	var rows []row
	err := r.db.Model(&UserRole{}).Select("role_id, COUNT(*) AS cnt").Group("role_id").Scan(&rows).Error
	m := make(map[int64]int64, len(rows))
	for _, x := range rows {
		m[x.RoleID] = x.Cnt
	}
	return m, err
}

// ---- 管理员账号 ----

func (r *Repo) ListAdmins(page, size int) ([]AdminUser, int64, error) {
	var total int64
	if err := r.db.Model(&AdminUser{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var list []AdminUser
	err := r.db.Order("id").Offset((page - 1) * size).Limit(size).Find(&list).Error
	return list, total, err
}

func (r *Repo) FindAdmin(id int64) (*AdminUser, error) {
	var u AdminUser
	err := r.db.First(&u, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &u, err
}

func (r *Repo) UpdateAdmin(id int64, fields map[string]any) error {
	return r.db.Model(&AdminUser{}).Where("id = ?", id).Updates(fields).Error
}

func (r *Repo) DeleteAdmin(id int64) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("admin_user_id = ?", id).Delete(&UserRole{}).Error; err != nil {
			return err
		}
		return tx.Delete(&AdminUser{}, id).Error
	})
}

// RoleIDsByAdmin / RoleCodesByAdmin 账号已绑定角色。
func (r *Repo) RoleIDsByAdmin(adminID int64) ([]int64, error) {
	var ids []int64
	err := r.db.Model(&UserRole{}).Where("admin_user_id = ?", adminID).Pluck("role_id", &ids).Error
	return ids, err
}

func (r *Repo) RoleCodesByAdmin(adminID int64) ([]string, error) {
	var codes []string
	err := r.db.Model(&Role{}).
		Joins("JOIN admin_user_role ur ON ur.role_id = admin_role.id").
		Where("ur.admin_user_id = ?", adminID).
		Pluck("admin_role.code", &codes).Error
	return codes, err
}

// SetAdminRoles 重建账号-角色关联（按角色 ID）。
func (r *Repo) SetAdminRoles(adminID int64, roleIDs []int64) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("admin_user_id = ?", adminID).Delete(&UserRole{}).Error; err != nil {
			return err
		}
		if len(roleIDs) == 0 {
			return nil
		}
		rows := make([]UserRole, 0, len(roleIDs))
		for _, rid := range roleIDs {
			rows = append(rows, UserRole{AdminUserID: adminID, RoleID: rid})
		}
		return tx.Create(&rows).Error
	})
}

// FindRoleByCode 供 seed 后给默认 admin 绑 super 角色用。
func (r *Repo) FindRoleByCode(code string) (*Role, error) {
	var role Role
	err := r.db.Where("code = ?", code).First(&role).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &role, err
}

package admin

import (
	"context"
	"errors"
	"strconv"

	"github.com/dlidli/server/internal/pkg/errcode"
	"github.com/dlidli/server/internal/pkg/snowflake"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// ---- RBAC DTO ----

// MenuItem 下发给前端的菜单项。
type MenuItem struct {
	Code string `json:"code"`
	Name string `json:"name"`
	Path string `json:"path"`
	Icon string `json:"icon"`
}

// CurrentPerm 当前登录管理员的权限与菜单。
type CurrentPerm struct {
	IsSuper bool       `json:"is_super"`
	Perms   []string   `json:"perms"` // 权限码集合（super 为全集）
	Menus   []MenuItem `json:"menus"` // 有权访问的菜单（super 为全部）
}

// RoleItem 角色列表项（带成员数与权限码）。
type RoleItem struct {
	Role
	Members int64    `json:"members"`
	Perms   []string `json:"perms"`
}

// AdminItem 账号列表项（带角色）。
type AdminItem struct {
	ID          int64   `json:"id,string"`
	Username    string  `json:"username"`
	Nickname    string  `json:"nickname"`
	Status      int8    `json:"status"`
	RoleIDs     []int64 `json:"role_ids"`
	RoleNames   string  `json:"role_names"`
	CreatedAt   string  `json:"created_at"`
	LastLoginAt *string `json:"last_login_at"`
}

// ---- 请求体 ----

type SaveRoleReq struct {
	Name   string   `json:"name" binding:"required,max=32"`
	Code   string   `json:"code" binding:"max=32"`
	Remark string   `json:"remark" binding:"max=128"`
	Perms  []string `json:"perms"`
}

type SaveAdminReq struct {
	Username string   `json:"username" binding:"required,max=32"`
	Nickname string   `json:"nickname" binding:"max=32"`
	Password string   `json:"password" binding:"max=64"`
	RoleIDs  []string `json:"role_ids"` // 雪花 ID 字符串化（防 JS 精度丢失）
}

type ResetPwdReq struct {
	Password string `json:"password" binding:"required,min=6,max=64"`
}

// ---- seed & 权限查询 ----

// parseRoleIDs 将字符串角色 ID 解析为 int64（忽略非法项）。
func parseRoleIDs(ss []string) []int64 {
	ids := make([]int64, 0, len(ss))
	for _, s := range ss {
		if id, err := strconv.ParseInt(s, 10, 64); err == nil {
			ids = append(ids, id)
		}
	}
	return ids
}

// seedRBAC 初始化权限点/内置角色，并确保默认 admin 绑定 super 角色。
func (s *Service) seedRBAC() {
	if err := s.repo.SeedRBAC(snowflake.NextID); err != nil {
		s.log.Warn("RBAC 初始化失败", zap.Error(err))
		return
	}
	// 默认 admin 账号绑定 super 角色（若未绑定）
	admin, _ := s.repo.FindByUsername("admin")
	superRole, _ := s.repo.FindRoleByCode(SuperRoleCode)
	if admin != nil && superRole != nil {
		ids, _ := s.repo.RoleIDsByAdmin(admin.ID)
		if len(ids) == 0 {
			_ = s.repo.SetAdminRoles(admin.ID, []int64{superRole.ID})
		}
	}
}

// CurrentPerm 返回管理员的权限码与可见菜单。
func (s *Service) CurrentPerm(_ context.Context, adminID int64) (*CurrentPerm, error) {
	codes, isSuper, err := s.repo.PermCodesByAdmin(adminID)
	if err != nil {
		return nil, err
	}
	perms, err := s.repo.AllPermissions()
	if err != nil {
		return nil, err
	}
	allow := make(map[string]bool, len(codes))
	for _, c := range codes {
		allow[c] = true
	}
	res := &CurrentPerm{IsSuper: isSuper, Perms: []string{}, Menus: []MenuItem{}}
	for _, p := range perms {
		granted := isSuper || allow[p.Code]
		if !granted {
			continue
		}
		res.Perms = append(res.Perms, p.Code)
		if p.Type == "menu" {
			res.Menus = append(res.Menus, MenuItem{Code: p.Code, Name: p.Name, Path: p.Path, Icon: p.Icon})
		}
	}
	return res, nil
}

// HasPerm 校验管理员是否具备指定权限码（供中间件调用）。
func (s *Service) HasPerm(adminID int64, code string) (bool, error) {
	codes, isSuper, err := s.repo.PermCodesByAdmin(adminID)
	if err != nil {
		return false, err
	}
	if isSuper {
		return true, nil
	}
	for _, c := range codes {
		if c == code {
			return true, nil
		}
	}
	return false, nil
}

// AllPermissions 权限点全集（供前端角色分配树渲染）。
func (s *Service) AllPermissions() ([]Permission, error) {
	return s.repo.AllPermissions()
}

// SavePermissionReq 新建/编辑权限点请求。
type SavePermissionReq struct {
	Code   string `json:"code" binding:"max=64"`
	Name   string `json:"name" binding:"required,max=64"`
	Type   string `json:"type" binding:"omitempty,oneof=menu button"`
	Parent string `json:"parent" binding:"max=64"`
	Path   string `json:"path" binding:"max=128"`
	Icon   string `json:"icon" binding:"max=64"`
	Sort   int    `json:"sort"`
}

// CreatePermission 新建权限点（页面 menu / 按钮 button）。
func (s *Service) CreatePermission(adminID int64, req *SavePermissionReq) (*Permission, error) {
	if req.Code == "" {
		return nil, errcode.ErrInvalidParams.WithMsg("权限码必填")
	}
	if req.Type != "menu" && req.Type != "button" {
		return nil, errcode.ErrInvalidParams.WithMsg("类型只能为 menu 或 button")
	}
	if exist, _ := s.repo.FindPermissionByCode(req.Code); exist != nil {
		return nil, errcode.ErrInvalidParams.WithMsg("权限码已存在")
	}
	if req.Type == "button" {
		if req.Parent == "" {
			return nil, errcode.ErrInvalidParams.WithMsg("按钮权限必须挂到一个页面权限下")
		}
		parent, _ := s.repo.FindPermissionByCode(req.Parent)
		if parent == nil || parent.Type != "menu" {
			return nil, errcode.ErrInvalidParams.WithMsg("父权限不存在或非页面权限")
		}
	}
	p := &Permission{ID: snowflake.NextID(), Code: req.Code, Name: req.Name, Type: req.Type, Parent: req.Parent, Path: req.Path, Icon: req.Icon, Sort: req.Sort}
	if err := s.repo.CreatePermission(p); err != nil {
		return nil, err
	}
	_ = s.repo.AddAudit(&AuditLog{AdminID: adminID, Action: "add_permission", ObjType: "permission", Oid: p.ID, Detail: req.Code})
	return p, nil
}

// UpdatePermission 编辑权限点（code 锁定，不可改）。
func (s *Service) UpdatePermission(adminID, id int64, req *SavePermissionReq) error {
	p, err := s.repo.FindPermission(id)
	if err != nil {
		return err
	}
	if p == nil {
		return errcode.ErrNotFound
	}
	if req.Type == "button" && req.Parent != "" {
		parent, _ := s.repo.FindPermissionByCode(req.Parent)
		if parent == nil || parent.Type != "menu" {
			return errcode.ErrInvalidParams.WithMsg("父权限不存在或非页面权限")
		}
	}
	fields := map[string]any{"name": req.Name, "path": req.Path, "icon": req.Icon, "sort": req.Sort}
	if req.Type != "" {
		fields["type"] = req.Type
		fields["parent"] = req.Parent
	}
	if err := s.repo.UpdatePermission(id, fields); err != nil {
		return err
	}
	_ = s.repo.AddAudit(&AuditLog{AdminID: adminID, Action: "edit_permission", ObjType: "permission", Oid: id, Detail: p.Code})
	return nil
}

// DeletePermission 删除权限点（有子节点或被角色引用时禁删）。
func (s *Service) DeletePermission(adminID, id int64) error {
	p, err := s.repo.FindPermission(id)
	if err != nil {
		return err
	}
	if p == nil {
		return errcode.ErrNotFound
	}
	if p.Type == "menu" {
		if n, _ := s.repo.PermissionChildCount(p.Code); n > 0 {
			return errcode.ErrInvalidParams.WithMsg("该页面权限下还有按钮权限，请先删除子项")
		}
	}
	if n, _ := s.repo.PermissionRoleRefCount(id); n > 0 {
		return errcode.ErrInvalidParams.WithMsg("该权限点已被角色引用，请先从角色中移除")
	}
	if err := s.repo.DeletePermission(id); err != nil {
		return err
	}
	_ = s.repo.AddAudit(&AuditLog{AdminID: adminID, Action: "del_permission", ObjType: "permission", Oid: id, Detail: p.Code})
	return nil
}

// ---- 角色 ----

func (s *Service) ListRoles() ([]RoleItem, error) {
	roles, err := s.repo.ListRoles()
	if err != nil {
		return nil, err
	}
	members, _ := s.repo.CountRoleMembers()
	items := make([]RoleItem, 0, len(roles))
	for _, r := range roles {
		perms, _ := s.repo.RolePermCodes(r.ID)
		if perms == nil {
			perms = []string{}
		}
		items = append(items, RoleItem{Role: r, Members: members[r.ID], Perms: perms})
	}
	return items, nil
}

func (s *Service) CreateRole(adminID int64, req *SaveRoleReq) (*Role, error) {
	if req.Code == "" {
		return nil, errcode.ErrInvalidParams.WithMsg("角色编码必填")
	}
	role := &Role{ID: snowflake.NextID(), Code: req.Code, Name: req.Name, Remark: req.Remark}
	if err := s.repo.CreateRole(role); err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			return nil, errcode.ErrInvalidParams.WithMsg("角色编码已存在")
		}
		return nil, err
	}
	_ = s.repo.SetRolePerms(role.ID, req.Perms)
	_ = s.repo.AddAudit(&AuditLog{AdminID: adminID, Action: "add_role", ObjType: "role", Oid: role.ID, Detail: req.Name})
	return role, nil
}

func (s *Service) UpdateRole(adminID, id int64, req *SaveRoleReq) error {
	role, err := s.repo.FindRole(id)
	if err != nil {
		return err
	}
	if role == nil {
		return errcode.ErrNotFound
	}
	if err := s.repo.UpdateRole(id, map[string]any{"name": req.Name, "remark": req.Remark}); err != nil {
		return err
	}
	// super 权限恒为全集，不可编辑；其余角色（含内置）允许管理权限
	if role.Code != SuperRoleCode {
		_ = s.repo.SetRolePerms(id, req.Perms)
	}
	_ = s.repo.AddAudit(&AuditLog{AdminID: adminID, Action: "edit_role", ObjType: "role", Oid: id, Detail: req.Name})
	return nil
}

func (s *Service) DeleteRole(adminID, id int64) error {
	role, err := s.repo.FindRole(id)
	if err != nil {
		return err
	}
	if role == nil {
		return errcode.ErrNotFound
	}
	if role.IsBuiltin == 1 {
		return errcode.ErrInvalidParams.WithMsg("内置角色不可删除")
	}
	if err := s.repo.DeleteRole(id); err != nil {
		return err
	}
	_ = s.repo.AddAudit(&AuditLog{AdminID: adminID, Action: "del_role", ObjType: "role", Oid: id, Detail: role.Name})
	return nil
}

// ---- 账号 ----

func (s *Service) ListAdmins(page, size int) ([]AdminItem, int64, error) {
	admins, total, err := s.repo.ListAdmins(page, size)
	if err != nil {
		return nil, 0, err
	}
	roles, _ := s.repo.ListRoles()
	roleName := make(map[int64]string, len(roles))
	for _, r := range roles {
		roleName[r.ID] = r.Name
	}
	items := make([]AdminItem, 0, len(admins))
	for _, a := range admins {
		ids, _ := s.repo.RoleIDsByAdmin(a.ID)
		if ids == nil {
			ids = []int64{}
		}
		names := ""
		for i, rid := range ids {
			if i > 0 {
				names += "、"
			}
			names += roleName[rid]
		}
		item := AdminItem{
			ID: a.ID, Username: a.Username, Nickname: a.Nickname, Status: a.Status,
			RoleIDs: ids, RoleNames: names, CreatedAt: a.CreatedAt.Format("2006-01-02 15:04"),
		}
		if a.LastLoginAt != nil {
			t := a.LastLoginAt.Format("2006-01-02 15:04")
			item.LastLoginAt = &t
		}
		items = append(items, item)
	}
	return items, total, nil
}

func (s *Service) CreateAdmin(adminID int64, req *SaveAdminReq) (*AdminUser, error) {
	if req.Password == "" {
		return nil, errcode.ErrInvalidParams.WithMsg("初始密码必填")
	}
	if exist, _ := s.repo.FindByUsername(req.Username); exist != nil {
		return nil, errcode.ErrInvalidParams.WithMsg("用户名已存在")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}
	u := &AdminUser{ID: snowflake.NextID(), Username: req.Username, Nickname: req.Nickname, Password: string(hash), Role: "reviewer"}
	if err := s.repo.Create(u); err != nil {
		return nil, err
	}
	_ = s.repo.SetAdminRoles(u.ID, parseRoleIDs(req.RoleIDs))
	_ = s.repo.AddAudit(&AuditLog{AdminID: adminID, Action: "add_admin", ObjType: "admin", Oid: u.ID, Detail: req.Username})
	return u, nil
}

func (s *Service) UpdateAdmin(adminID, id int64, req *SaveAdminReq) error {
	u, err := s.repo.FindAdmin(id)
	if err != nil {
		return err
	}
	if u == nil {
		return errcode.ErrNotFound
	}
	if err := s.repo.UpdateAdmin(id, map[string]any{"nickname": req.Nickname}); err != nil {
		return err
	}
	_ = s.repo.SetAdminRoles(id, parseRoleIDs(req.RoleIDs))
	_ = s.repo.AddAudit(&AuditLog{AdminID: adminID, Action: "edit_admin", ObjType: "admin", Oid: id, Detail: req.Username})
	return nil
}

// ToggleAdmin 启用/停用账号（不能停用自己）。
func (s *Service) ToggleAdmin(adminID, id int64, status int8) error {
	if adminID == id {
		return errcode.ErrInvalidParams.WithMsg("不能停用当前登录账号")
	}
	u, err := s.repo.FindAdmin(id)
	if err != nil {
		return err
	}
	if u == nil {
		return errcode.ErrNotFound
	}
	if err := s.repo.UpdateAdmin(id, map[string]any{"status": status}); err != nil {
		return err
	}
	act := "enable_admin"
	if status != 0 {
		act = "disable_admin"
	}
	_ = s.repo.AddAudit(&AuditLog{AdminID: adminID, Action: act, ObjType: "admin", Oid: id})
	return nil
}

func (s *Service) DeleteAdmin(adminID, id int64) error {
	if adminID == id {
		return errcode.ErrInvalidParams.WithMsg("不能删除当前登录账号")
	}
	u, err := s.repo.FindAdmin(id)
	if err != nil {
		return err
	}
	if u == nil {
		return errcode.ErrNotFound
	}
	if err := s.repo.DeleteAdmin(id); err != nil {
		return err
	}
	_ = s.repo.AddAudit(&AuditLog{AdminID: adminID, Action: "del_admin", ObjType: "admin", Oid: id, Detail: u.Username})
	return nil
}

func (s *Service) ResetAdminPwd(adminID, id int64, newPwd string) error {
	u, err := s.repo.FindAdmin(id)
	if err != nil {
		return err
	}
	if u == nil {
		return errcode.ErrNotFound
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(newPwd), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	if err := s.repo.UpdateAdmin(id, map[string]any{"password": string(hash)}); err != nil {
		return err
	}
	_ = s.repo.AddAudit(&AuditLog{AdminID: adminID, Action: "reset_pwd", ObjType: "admin", Oid: id})
	return nil
}

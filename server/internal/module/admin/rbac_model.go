package admin

import "time"

// ---- RBAC 数据模型 ----

// Role 对应 admin_role 表。
type Role struct {
	ID        int64     `gorm:"primaryKey" json:"id,string"`
	Code      string    `json:"code"`
	Name      string    `json:"name"`
	Remark    string    `json:"remark"`
	IsBuiltin int8      `json:"is_builtin"`
	CreatedAt time.Time `json:"created_at"`
}

func (Role) TableName() string { return "admin_role" }

// Permission 对应 admin_permission 表。
type Permission struct {
	ID     int64  `gorm:"primaryKey" json:"id,string"`
	Code   string `json:"code"`
	Name   string `json:"name"`
	Type   string `json:"type"`
	Parent string `json:"parent"`
	Path   string `json:"path"`
	Icon   string `json:"icon"`
	Sort   int    `json:"sort"`
}

func (Permission) TableName() string { return "admin_permission" }

// UserRole 对应 admin_user_role 表。
type UserRole struct {
	AdminUserID int64 `gorm:"column:admin_user_id"`
	RoleID      int64 `gorm:"column:role_id"`
}

func (UserRole) TableName() string { return "admin_user_role" }

// RolePermission 对应 admin_role_permission 表。
type RolePermission struct {
	RoleID       int64 `gorm:"column:role_id"`
	PermissionID int64 `gorm:"column:permission_id"`
}

func (RolePermission) TableName() string { return "admin_role_permission" }

// SuperRoleCode 超级管理员角色编码，拥有全部权限（鉴权时短路放行）。
const SuperRoleCode = "super"

// ---- 内置权限清单（seed 真源）----
// code 用「模块:操作」；menu 型带 path/icon 供前端菜单渲染，button 型 parent 指向所属菜单。

var builtinPermissions = []Permission{
	// 工作台
	{Code: "dashboard:view", Name: "工作台", Type: "menu", Path: "/dashboard", Icon: "i-mingcute-dashboard-3-line", Sort: 1},
	// 审核
	{Code: "review:view", Name: "审核工作台", Type: "menu", Path: "/review", Icon: "i-mingcute-task-2-line", Sort: 2},
	{Code: "review:approve", Name: "通过/驳回稿件", Type: "button", Parent: "review:view"},
	// 敏感词
	{Code: "sensitive:view", Name: "敏感词库", Type: "menu", Path: "/sensitive-words", Icon: "i-mingcute-shield-line", Sort: 3},
	{Code: "sensitive:edit", Name: "增删敏感词", Type: "button", Parent: "sensitive:view"},
	// 用户
	{Code: "user:view", Name: "用户管理", Type: "menu", Path: "/users", Icon: "i-mingcute-user-3-line", Sort: 4},
	{Code: "user:punish", Name: "封禁/禁言用户", Type: "button", Parent: "user:view"},
	// 系统 - 账号
	{Code: "admin:view", Name: "账号管理", Type: "menu", Path: "/admins", Icon: "i-mingcute-safe-lock-line", Sort: 5},
	{Code: "admin:edit", Name: "增删改账号", Type: "button", Parent: "admin:view"},
	// 系统 - 角色
	{Code: "role:view", Name: "角色管理", Type: "menu", Path: "/roles", Icon: "i-mingcute-group-line", Sort: 6},
	{Code: "role:edit", Name: "增删改角色/分配权限", Type: "button", Parent: "role:view"},
}

// 内置角色定义（code → 权限码集合；super 特殊，拥有全部）。
type builtinRole struct {
	Code   string
	Name   string
	Perms  []string // nil 表示全部（super）
	Remark string
}

var builtinRoles = []builtinRole{
	{Code: SuperRoleCode, Name: "超级管理员", Perms: nil, Remark: "拥有全部权限，含账号/角色/权限管理"},
	{Code: "review_lead", Name: "审核主管", Remark: "审核 + 敏感词库", Perms: []string{
		"dashboard:view", "review:view", "review:approve", "sensitive:view", "sensitive:edit",
	}},
	{Code: "reviewer", Name: "审核员", Remark: "仅稿件审核", Perms: []string{
		"dashboard:view", "review:view", "review:approve",
	}},
	{Code: "moderator", Name: "用户治理", Remark: "用户查询与处罚", Perms: []string{
		"dashboard:view", "user:view", "user:punish",
	}},
	{Code: "operator", Name: "运营", Remark: "运营配置（预留）", Perms: []string{
		"dashboard:view",
	}},
	{Code: "analyst", Name: "数据分析", Remark: "数据大盘只读（预留）", Perms: []string{
		"dashboard:view",
	}},
}

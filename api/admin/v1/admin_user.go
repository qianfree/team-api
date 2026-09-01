package v1

import "github.com/gogf/gf/v2/frame/g"

// AdminUserListReq 管理员列表请求
type AdminUserListReq struct {
	g.Meta   `path:"/users" method:"get" mime:"json" tags:"管理后台-用户管理" summary:"管理员列表"`
	Page     int    `json:"page" d:"1" dc:"页码"`
	PageSize int    `json:"page_size" d:"20" dc:"每页数量"`
	Keyword  string `json:"keyword" dc:"搜索关键词（用户名/邮箱）"`
	Role     string `json:"role" dc:"角色筛选"`
	Status   string `json:"status" dc:"状态筛选"`
}

type AdminUserListRes struct {
	List     []AdminUserItem `json:"list"`
	Total    int             `json:"total"`
	Page     int             `json:"page"`
	PageSize int             `json:"page_size"`
}

type AdminUserItem struct {
	ID          int64  `json:"id"`
	Username    string `json:"username"`
	Email       string `json:"email"`
	DisplayName string `json:"display_name"`
	Role        string `json:"role"`
	Status      string `json:"status"`
	LastLoginAt string `json:"last_login_at"`
	LastLoginIp string `json:"last_login_ip"`
	LockedUntil string `json:"locked_until"`
	CreatedAt   string `json:"created_at"`
	// Roles 是该账号已分配的角色。role=super_admin 时为空数组（超管权限来自账号属性，
	// 不通过角色授予）；非超管且为空数组表示「未分配角色」＝零权限，界面应醒目提示。
	Roles []AdminRoleBrief `json:"roles"`
}

// AdminUserCreateReq 创建管理员请求
type AdminUserCreateReq struct {
	g.Meta   `path:"/users" method:"post" mime:"json" tags:"管理后台-用户管理" summary:"创建管理员"`
	Username string `json:"username" v:"required|length:3,50#请输入用户名|用户名长度为3-50位" dc:"用户名"`
	Password string `json:"password" v:"required|length:8,64#请输入密码|密码长度为8-64位" dc:"密码"`
	Email    string `json:"email" v:"email#邮箱格式不正确" dc:"邮箱"`
	Role     string `json:"role" d:"admin" v:"in:super_admin,admin#角色只能是 super_admin 或 admin" dc:"角色：super_admin / admin"`
	// RoleIDs 在创建时一并分配角色。不传则账号无任何角色（0 权限，只能访问自助接口），
	// 这是安全的默认值，但界面应提示「未分配角色」。role=super_admin 时本字段忽略。
	RoleIDs []int64 `json:"role_ids" dc:"分配的角色ID列表"`
}

type AdminUserCreateRes struct {
	ID int64 `json:"id"`
}

// AdminUserUpdateReq 更新管理员请求
type AdminUserUpdateReq struct {
	g.Meta      `path:"/users/{id}" method:"put" mime:"json" tags:"管理后台-用户管理" summary:"更新管理员"`
	Id          int64   `json:"id" in:"path" v:"required" dc:"管理员ID"`
	DisplayName *string `json:"display_name" dc:"显示名称"`
	Email       *string `json:"email" dc:"邮箱"`
	Role        *string `json:"role" v:"in:super_admin,admin#角色只能是 super_admin 或 admin" dc:"角色"`
}

type AdminUserUpdateRes struct{}

// AdminUserDeleteReq 删除管理员请求
type AdminUserDeleteReq struct {
	g.Meta `path:"/users/{id}" method:"delete" mime:"json" tags:"管理后台-用户管理" summary:"删除管理员"`
	Id     int64 `json:"id" in:"path" v:"required" dc:"管理员ID"`
}

type AdminUserDeleteRes struct{}

// AdminUserUpdateStatusReq 启用/禁用管理员请求
type AdminUserUpdateStatusReq struct {
	g.Meta `path:"/users/{id}/status" method:"put" mime:"json" tags:"管理后台-用户管理" summary:"启用/禁用管理员"`
	Id     int64  `json:"id" in:"path" v:"required" dc:"管理员ID"`
	Status string `json:"status" v:"required|in:active,disabled#请选择状态|状态值无效" dc:"状态：active / disabled"`
}

type AdminUserUpdateStatusRes struct{}

// AdminUserResetPasswordReq 重置管理员密码请求
type AdminUserResetPasswordReq struct {
	g.Meta      `path:"/users/{id}/reset-password" method:"put" mime:"json" tags:"管理后台-用户管理" summary:"重置管理员密码"`
	Id          int64  `json:"id" in:"path" v:"required" dc:"管理员ID"`
	NewPassword string `json:"new_password" v:"required|length:8,64#请输入新密码|密码长度为8-64位" dc:"新密码"`
}

type AdminUserResetPasswordRes struct{}

// AdminUserUnlockReq 解除管理员登录锁定请求
type AdminUserUnlockReq struct {
	g.Meta `path:"/users/{id}/unlock" method:"put" mime:"json" tags:"管理后台-用户管理" summary:"解除管理员登录锁定"`
	Id     int64 `json:"id" in:"path" v:"required" dc:"管理员ID"`
}

type AdminUserUnlockRes struct{}

// AdminUserExportReq 导出用户列表请求
type AdminUserExportReq struct {
	g.Meta  `path:"/users/export" method:"get" mime:"json" tags:"管理后台-用户管理" summary:"导出用户列表"`
	Format  string `json:"format" in:"query" d:"csv" v:"in:csv,xlsx" dc:"导出格式：csv / xlsx"`
	Keyword string `json:"keyword" in:"query" dc:"搜索关键词（用户名/邮箱）"`
	Role    string `json:"role" in:"query" dc:"角色筛选"`
	Status  string `json:"status" in:"query" dc:"状态筛选"`
}

type AdminUserExportRes struct{}

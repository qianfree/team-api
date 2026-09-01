package v1

import "github.com/gogf/gf/v2/frame/g"

// ─────────────────────────────────────────────────────────────────────────────
// 角色管理
//
// 超级管理员不在本接口的管辖范围：它由 sys_admin_users.role 判定并在鉴权时短路，
// 界面上作为一张固定卡片展示「全部权限（不可配置）」。
// ─────────────────────────────────────────────────────────────────────────────

// AdminRoleBrief 角色简要信息（登录响应、用户列表等处展示用）
type AdminRoleBrief struct {
	ID        int64  `json:"id"`
	Code      string `json:"code"`
	Name      string `json:"name"`
	IsEnabled bool   `json:"is_enabled"`
}

// AdminRoleItem 角色列表项
type AdminRoleItem struct {
	ID          int64  `json:"id"`
	Code        string `json:"code"`
	Name        string `json:"name"`
	Description string `json:"description"`
	IsBuiltin   bool   `json:"is_builtin"`
	IsEnabled   bool   `json:"is_enabled"`
	Sort        int    `json:"sort"`
	UserCount   int    `json:"user_count"` // 关联的管理员账号数（删除前的影响面提示）
	PermCount   int    `json:"perm_count"` // 已授予的权限点数量
	CreatedAt   string `json:"created_at"`
}

// AdminRoleListReq 角色列表请求
type AdminRoleListReq struct {
	g.Meta `path:"/roles" method:"get" mime:"json" tags:"管理后台-角色" summary:"角色列表"`
}

// AdminRoleListRes 角色列表响应（角色数量少，不分页）
type AdminRoleListRes struct {
	List []AdminRoleItem `json:"list"`
}

// AdminRoleModuleTier 角色在某个模块上的档位选择
type AdminRoleModuleTier struct {
	Module string `json:"module"` // 模块名，如 tenant
	Tier   string `json:"tier"`   // none / read / operate / full；空串表示不匹配任何档位（自定义）
}

// AdminRoleDetailReq 角色详情请求
type AdminRoleDetailReq struct {
	g.Meta `path:"/roles/{id}" method:"get" mime:"json" tags:"管理后台-角色" summary:"角色详情"`
	ID     int64 `json:"id" in:"path" v:"required" dc:"角色ID"`
}

// AdminRoleDetailRes 角色详情响应
//
// 同时给出权限点全集与档位视图：界面默认按档位渲染（19 行单选），
// 高级模式切到权限点级勾选。二者是同一份数据的两个视图。
type AdminRoleDetailRes struct {
	ID          int64                 `json:"id"`
	Code        string                `json:"code"`
	Name        string                `json:"name"`
	Description string                `json:"description"`
	IsBuiltin   bool                  `json:"is_builtin"`
	IsEnabled   bool                  `json:"is_enabled"`
	Sort        int                   `json:"sort"`
	UserCount   int                   `json:"user_count"`
	Permissions []string              `json:"permissions"`  // 权限点全集
	ModuleTiers []AdminRoleModuleTier `json:"module_tiers"` // 按模块归纳的档位视图
	CreatedAt   string                `json:"created_at"`
}

// AdminRoleCreateReq 新建角色请求
type AdminRoleCreateReq struct {
	g.Meta         `path:"/roles" method:"post" mime:"json" tags:"管理后台-角色" summary:"新建角色"`
	Code           string   `json:"code" v:"required#请输入角色标识" dc:"角色标识（小写字母开头，可含数字与下划线，创建后不可修改）"`
	Name           string   `json:"name" v:"required#请输入角色名称" dc:"角色显示名称"`
	Description    string   `json:"description" dc:"角色说明"`
	Sort           int      `json:"sort" dc:"排序权重"`
	Permissions    []string `json:"permissions" dc:"权限点列表（与 copy_from_role_id 二选一，同时给出时以本字段为准）"`
	CopyFromRoleID int64    `json:"copy_from_role_id" dc:"从现有角色复制权限（建「实习运营」就是复制「运营」再收掉几个模块）"`
}

// AdminRoleCreateRes 新建角色响应
type AdminRoleCreateRes struct {
	ID int64 `json:"id"`
}

// AdminRoleUpdateReq 更新角色请求（code 不可修改）
type AdminRoleUpdateReq struct {
	g.Meta      `path:"/roles/{id}" method:"put" mime:"json" tags:"管理后台-角色" summary:"更新角色"`
	ID          int64     `json:"id" in:"path" v:"required" dc:"角色ID"`
	Name        *string   `json:"name" dc:"角色显示名称"`
	Description *string   `json:"description" dc:"角色说明"`
	Sort        *int      `json:"sort" dc:"排序权重"`
	Permissions *[]string `json:"permissions" dc:"权限点列表（全量覆盖，传 null 表示不改动权限）"`
}

// AdminRoleUpdateRes 更新角色响应
type AdminRoleUpdateRes struct{}

// AdminRoleDeleteReq 删除角色请求
type AdminRoleDeleteReq struct {
	g.Meta `path:"/roles/{id}" method:"delete" mime:"json" tags:"管理后台-角色" summary:"删除角色"`
	ID     int64 `json:"id" in:"path" v:"required" dc:"角色ID"`
}

// AdminRoleDeleteRes 删除角色响应
//
// 预置角色同样可删 —— is_builtin 只标识来源，不构成删除保护。
// 不存在把自己锁死的风险：超级管理员由 sys_admin_users.role 判定，不依赖任何角色记录。
type AdminRoleDeleteRes struct {
	AffectedUsers int `json:"affected_users"` // 因此次删除而失去该角色权限的账号数
}

// AdminRoleStatusUpdateReq 启用/禁用角色请求
type AdminRoleStatusUpdateReq struct {
	g.Meta    `path:"/roles/{id}/status" method:"put" mime:"json" tags:"管理后台-角色" summary:"启用/禁用角色"`
	ID        int64 `json:"id" in:"path" v:"required" dc:"角色ID"`
	IsEnabled bool  `json:"is_enabled" dc:"是否启用：禁用后该角色权限对全部关联用户立即失效"`
}

// AdminRoleStatusUpdateRes 启用/禁用角色响应
type AdminRoleStatusUpdateRes struct{}

// AdminRoleResetReq 恢复预置角色的默认权限请求
type AdminRoleResetReq struct {
	g.Meta `path:"/roles/{id}/reset" method:"post" mime:"json" tags:"管理后台-角色" summary:"恢复预置角色默认权限"`
	ID     int64 `json:"id" in:"path" v:"required" dc:"角色ID"`
}

// AdminRoleResetRes 恢复默认权限响应
type AdminRoleResetRes struct {
	Permissions []string `json:"permissions"`
}

// ─────────────────────────────────────────────────────────────────────────────
// 用户角色分配
// ─────────────────────────────────────────────────────────────────────────────

// AdminUserRoleListReq 查询管理员已分配角色
type AdminUserRoleListReq struct {
	g.Meta `path:"/users/{id}/roles" method:"get" mime:"json" tags:"管理后台-角色" summary:"查询用户角色"`
	Id     int64 `json:"id" in:"path" v:"required" dc:"管理员用户ID"`
}

// AdminUserRoleListRes 用户角色列表响应
type AdminUserRoleListRes struct {
	List []AdminRoleBrief `json:"list"`
}

// AdminUserRoleAssignReq 分配管理员角色请求（全量覆盖）
type AdminUserRoleAssignReq struct {
	g.Meta  `path:"/users/{id}/roles" method:"put" mime:"json" tags:"管理后台-角色" summary:"分配用户角色"`
	Id      int64   `json:"id" in:"path" v:"required" dc:"管理员用户ID"`
	RoleIDs []int64 `json:"role_ids" dc:"角色ID列表（全量覆盖；传空数组表示不分配任何角色，该账号将只剩自助接口权限）"`
}

// AdminUserRoleAssignRes 分配用户角色响应
type AdminUserRoleAssignRes struct{}

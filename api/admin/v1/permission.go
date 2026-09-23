package v1

import "github.com/gogf/gf/v2/frame/g"

// AdminPermissionListReq 获取管理员权限点请求
type AdminPermissionListReq struct {
	g.Meta `path:"/users/{id}/permissions" method:"get" mime:"json" tags:"管理后台-权限管理" summary:"获取管理员权限点"`
	Id     int64 `json:"id" in:"path" v:"required" dc:"管理员ID"`
}

type AdminPermissionListRes struct {
	Permissions []string        `json:"permissions"`
	DataScopes  []DataScopeItem `json:"data_scopes"`
}

type DataScopeItem struct {
	ID         int64  `json:"id"`
	ScopeType  string `json:"scope_type"`
	ScopeValue string `json:"scope_value"`
}

// AdminPermissionUpdateReq 更新管理员权限点请求
type AdminPermissionUpdateReq struct {
	g.Meta      `path:"/users/{id}/permissions" method:"put" mime:"json" tags:"管理后台-权限管理" summary:"更新管理员权限点"`
	Id          int64    `json:"id" in:"path" v:"required" dc:"管理员ID"`
	Permissions []string `json:"permissions" dc:"权限点列表"`
}

type AdminPermissionUpdateRes struct{}

// 数据范围（sys_admin_data_scopes）的写接口已下线。
//
// 该表可以配置「全部 / 租户组 / 指定租户」，但全站没有任何查询在读它做过滤
// （GetDataScopes 零调用方），配出来的范围不会生效。留着一个能存、能显示、
// 却不约束任何数据的开关，比没有更危险：以为限定了范围的账号实际能看全量数据。
// 表与存量数据保留，待数据范围真正落到查询链路上再恢复配置入口。

// AdminAllPermissionsReq 获取所有可用权限点请求
type AdminAllPermissionsReq struct {
	g.Meta `path:"/permissions" method:"get" mime:"json" tags:"管理后台-权限管理" summary:"获取所有可用权限点"`
}

type AdminAllPermissionsRes struct {
	Groups []PermissionGroup `json:"groups"`
	// Modules 是「模块 × 档位」的配置元数据，角色配置界面据此渲染 19 行单选，
	// 而不是把 60 个权限点摊成复选框。Groups 仍保留，供高级模式按权限点勾选。
	Modules []PermissionModuleMeta `json:"modules"`
	// Dangerous 列出需要标红并二次确认的高危权限点（权限点 → 风险说明）
	Dangerous map[string]string `json:"dangerous"`
}

// PermissionModuleMeta 单个模块的档位配置元数据
type PermissionModuleMeta struct {
	Module string       `json:"module"` // 模块名，与 PermissionGroup.Name 对应
	Label  string       `json:"label"`  // 中文标签
	Tiers  []TierOption `json:"tiers"`  // 该模块实际存在差异的档位（等价档位已折叠，界面不渲染无意义选项）
}

// TierOption 档位选项及其展开后的权限点
type TierOption struct {
	Tier        string   `json:"tier"`  // none / read / operate / full
	Label       string   `json:"label"` // 无 / 只读 / 操作 / 完全
	Permissions []string `json:"permissions"`
}

type PermissionGroup struct {
	Name        string   `json:"name"`
	Label       string   `json:"label"`
	Permissions []string `json:"permissions"`
}

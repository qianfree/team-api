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

// AdminDataScopeUpdateReq 更新管理员数据范围请求
type AdminDataScopeUpdateReq struct {
	g.Meta     `path:"/users/{id}/data-scopes" method:"put" mime:"json" tags:"管理后台-权限管理" summary:"更新管理员数据范围"`
	Id         int64            `json:"id" in:"path" v:"required" dc:"管理员ID"`
	DataScopes []DataScopeInput `json:"data_scopes" dc:"数据范围列表"`
}

type DataScopeInput struct {
	ScopeType  string `json:"scope_type" v:"required|in:all,tenant_group,tenant#请选择范围类型|范围类型无效" dc:"范围类型"`
	ScopeValue string `json:"scope_value" dc:"范围值"`
}

type AdminDataScopeUpdateRes struct{}

// AdminAllPermissionsReq 获取所有可用权限点请求
type AdminAllPermissionsReq struct {
	g.Meta `path:"/permissions" method:"get" mime:"json" tags:"管理后台-权限管理" summary:"获取所有可用权限点"`
}

type AdminAllPermissionsRes struct {
	Groups []PermissionGroup `json:"groups"`
}

type PermissionGroup struct {
	Name        string   `json:"name"`
	Label       string   `json:"label"`
	Permissions []string `json:"permissions"`
}

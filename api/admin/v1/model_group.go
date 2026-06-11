package v1

import "github.com/gogf/gf/v2/frame/g"

// ==================== 分组 CRUD ====================

// ModelGroupListReq 模型分组列表请求
type ModelGroupListReq struct {
	g.Meta   `path:"/model-groups" method:"get" mime:"json" tags:"管理后台-模型分组" summary:"模型分组列表"`
	Page     int    `json:"page" d:"1" v:"min:1" dc:"页码"`
	PageSize int    `json:"page_size" d:"20" v:"min:1|max:100" dc:"每页数量"`
	Status   string `json:"status" dc:"状态筛选：active/disabled"`
	Search   string `json:"search" dc:"搜索关键词（分组名称或标识）"`
}

// ModelGroupListRes 模型分组列表响应
type ModelGroupListRes struct {
	List     []ModelGroupItem `json:"list"`
	Total    int              `json:"total"`
	Page     int              `json:"page"`
	PageSize int              `json:"page_size"`
}

// ModelGroupItem 模型分组信息
type ModelGroupItem struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	Code        string `json:"code"`
	Description string `json:"description"`
	Status      string `json:"status"`
	IsDefault   bool   `json:"is_default"`
	ModelCount  int    `json:"model_count"`
	TenantCount int    `json:"tenant_count"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

// ModelGroupCreateReq 创建模型分组请求
type ModelGroupCreateReq struct {
	g.Meta      `path:"/model-groups" method:"post" mime:"json" tags:"管理后台-模型分组" summary:"创建模型分组"`
	Name        string  `json:"name" v:"required|length:1,100#请输入分组名称|分组名称长度1-100" dc:"分组名称"`
	Code        string  `json:"code" v:"required|length:1,50#请输入分组标识|分组标识长度1-50" dc:"分组唯一标识"`
	Description string  `json:"description" dc:"分组描述"`
	IsDefault   *bool   `json:"is_default" dc:"是否为新租户默认模型组"`
	ModelIds    []int64 `json:"model_ids" dc:"初始包含的模型ID列表"`
}

// ModelGroupCreateRes 创建模型分组响应
type ModelGroupCreateRes struct {
	ID int64 `json:"id"`
}

// ModelGroupUpdateReq 更新模型分组请求
type ModelGroupUpdateReq struct {
	g.Meta      `path:"/model-groups/{id}" method:"put" mime:"json" tags:"管理后台-模型分组" summary:"更新模型分组"`
	ID          int64  `json:"id" in:"path" v:"required" dc:"分组ID"`
	Name        string `json:"name" dc:"分组名称"`
	Description string `json:"description" dc:"分组描述"`
	Status      string `json:"status" v:"in:active,disabled" dc:"状态：active/disabled"`
	IsDefault   *bool  `json:"is_default" dc:"是否为新租户默认模型组"`
}

// ModelGroupUpdateRes 更新模型分组响应
type ModelGroupUpdateRes struct{}

// ModelGroupDeleteReq 删除模型分组请求
type ModelGroupDeleteReq struct {
	g.Meta `path:"/model-groups/{id}" method:"delete" mime:"json" tags:"管理后台-模型分组" summary:"删除模型分组"`
	ID     int64 `json:"id" in:"path" v:"required" dc:"分组ID"`
}

// ModelGroupDeleteRes 删除模型分组响应
type ModelGroupDeleteRes struct{}

// ==================== 分组模型管理 ====================

// GroupModelsListReq 查看分组内模型列表请求
type GroupModelsListReq struct {
	g.Meta `path:"/model-groups/{id}/models" method:"get" mime:"json" tags:"管理后台-模型分组" summary:"查看分组内模型列表"`
	ID     int64 `json:"id" in:"path" v:"required" dc:"分组ID"`
}

// GroupModelsListRes 查看分组内模型列表响应
type GroupModelsListRes struct {
	List []GroupModelItem `json:"list"`
}

// GroupModelItem 分组内模型信息
type GroupModelItem struct {
	ModelId   string `json:"model_id"`
	ModelName string `json:"model_name"`
	Category  string `json:"category"`
	Status    string `json:"status"`
}

// GroupModelsSetReq 设置分组内模型请求（全量替换）
type GroupModelsSetReq struct {
	g.Meta   `path:"/model-groups/{id}/models" method:"put" mime:"json" tags:"管理后台-模型分组" summary:"设置分组内模型"`
	ID       int64   `json:"id" in:"path" v:"required" dc:"分组ID"`
	ModelIds []int64 `json:"model_ids" dc:"模型ID列表（空数组表示清空）"`
}

// GroupModelsSetRes 设置分组内模型响应
type GroupModelsSetRes struct{}

// ==================== 租户分组管理 ====================

// TenantGroupsListReq 查看租户关联分组请求
type TenantGroupsListReq struct {
	g.Meta   `path:"/tenants/{tenant_id}/groups" method:"get" mime:"json" tags:"管理后台-模型分组" summary:"查看租户关联分组"`
	TenantID int64 `json:"tenant_id" in:"path" v:"required" dc:"租户ID"`
}

// TenantGroupsListRes 查看租户关联分组响应
type TenantGroupsListRes struct {
	List []TenantGroupItem `json:"list"`
}

// TenantGroupItem 租户关联的分组信息
type TenantGroupItem struct {
	GroupID    int64  `json:"group_id"`
	Name       string `json:"name"`
	Code       string `json:"code"`
	Status     string `json:"status"`
	ModelCount int    `json:"model_count"`
}

// TenantGroupsSetReq 设置租户关联分组请求（全量替换）
type TenantGroupsSetReq struct {
	g.Meta   `path:"/tenants/{tenant_id}/groups" method:"put" mime:"json" tags:"管理后台-模型分组" summary:"设置租户关联分组"`
	TenantID int64   `json:"tenant_id" in:"path" v:"required" dc:"租户ID"`
	GroupIds []int64 `json:"group_ids" dc:"分组ID列表（空数组表示清空）"`
}

// TenantGroupsSetRes 设置租户关联分组响应
type TenantGroupsSetRes struct{}

// ==================== 分组选项列表 ====================

// ModelGroupOptionsReq 分组选项列表请求（下拉选择专用，不分页）
type ModelGroupOptionsReq struct {
	g.Meta `path:"/model-groups/options" method:"get" mime:"json" tags:"管理后台-模型分组" summary:"分组选项列表（不分页）"`
	Status string `json:"status" in:"query" dc:"状态筛选：active/disabled"`
}

// ModelGroupOptionsRes 分组选项列表响应
type ModelGroupOptionsRes struct {
	List []ModelGroupOptionItem `json:"list"`
}

// ModelGroupOptionItem 分组选项项（精简字段）
type ModelGroupOptionItem struct {
	ID         int64  `json:"id"`
	Name       string `json:"name"`
	Code       string `json:"code"`
	IsDefault  bool   `json:"is_default"`
	ModelCount int    `json:"model_count"`
}

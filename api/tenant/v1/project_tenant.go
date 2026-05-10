package v1

import "github.com/gogf/gf/v2/frame/g"

// === 租户项目管理 ===

type TenantProjectListReq struct {
	g.Meta   `path:"/projects" method:"get" mime:"json" tags:"租户控制台-项目" summary:"项目列表"`
	Page     int `json:"page" in:"query" d:"1"`
	PageSize int `json:"page_size" in:"query" d:"20"`
}

type TenantProjectListRes struct {
	List     []map[string]any `json:"list"`
	Total    int              `json:"total"`
	Page     int              `json:"page"`
	PageSize int              `json:"page_size"`
}

type TenantProjectCreateReq struct {
	g.Meta      `path:"/projects" method:"post" mime:"json" tags:"租户控制台-项目" summary:"创建项目"`
	Name        string  `json:"name" v:"required"`
	Description string  `json:"description"`
	Budget      float64 `json:"budget"`
}

type TenantProjectCreateRes struct {
	ID int64 `json:"id"`
}

type TenantProjectUpdateReq struct {
	g.Meta      `path:"/projects/{id}" method:"put" mime:"json" tags:"租户控制台-项目" summary:"更新项目"`
	Id          int64   `json:"id" in:"path" v:"required|min:1"`
	Name        string  `json:"name"`
	Description string  `json:"description"`
	Budget      float64 `json:"budget"`
}

type TenantProjectUpdateRes struct{}

type TenantProjectArchiveReq struct {
	g.Meta `path:"/projects/{id}/archive" method:"post" mime:"json" tags:"租户控制台-项目" summary:"归档项目"`
	Id     int64 `json:"id" in:"path" v:"required|min:1"`
}

type TenantProjectArchiveRes struct{}

type TenantProjectUnarchiveReq struct {
	g.Meta `path:"/projects/{id}/unarchive" method:"post" mime:"json" tags:"租户控制台-项目" summary:"取消归档"`
	Id     int64 `json:"id" in:"path" v:"required|min:1"`
}

type TenantProjectUnarchiveRes struct{}

type TenantProjectGetReq struct {
	g.Meta `path:"/projects/{id}" method:"get" mime:"json" tags:"租户控制台-项目" summary:"项目详情"`
	Id     int64 `json:"id" in:"path" v:"required|min:1"`
}

type TenantProjectGetRes struct {
	Data map[string]any `json:"data"`
}

// === 项目子资源（owner/admin 权限） ===

type TenantProjectApiKeyListReq struct {
	g.Meta   `path:"/projects/{id}/api-keys" method:"get" mime:"json" tags:"租户控制台-项目" summary:"项目密钥列表"`
	Id       int64 `json:"id" in:"path" v:"required|min:1"`
	Page     int   `json:"page" in:"query" d:"1"`
	PageSize int   `json:"page_size" in:"query" d:"20"`
}

type TenantProjectApiKeyListRes struct {
	List     []map[string]any `json:"list"`
	Total    int              `json:"total"`
	Page     int              `json:"page"`
	PageSize int              `json:"page_size"`
}

type TenantProjectApiKeyCreateReq struct {
	g.Meta        `path:"/projects/{id}/api-keys" method:"post" mime:"json" tags:"租户控制台-项目" summary:"项目密钥创建"`
	Id            int64    `json:"id" in:"path" v:"required|min:1"`
	Name          string   `json:"name" v:"required"`
	Scope         string   `json:"scope" d:"full"`
	ExpiresInDays int      `json:"expires_in_days"`
	ModelNames    []string `json:"model_names"`
}

type TenantProjectApiKeyCreateRes struct {
	Data map[string]any `json:"data"`
}

type TenantProjectApiKeyDeleteReq struct {
	g.Meta `path:"/projects/{id}/api-keys/{keyId}" method:"delete" mime:"json" tags:"租户控制台-项目" summary:"项目密钥删除"`
	Id     int64 `json:"id" in:"path" v:"required|min:1"`
	KeyId  int64 `json:"keyId" in:"path" v:"required|min:1"`
}

type TenantProjectApiKeyDeleteRes struct{}

type TenantProjectUsageStatsReq struct {
	g.Meta `path:"/projects/{id}/usage-stats" method:"get" mime:"json" tags:"租户控制台-项目" summary:"项目用量统计"`
	Id     int64 `json:"id" in:"path" v:"required|min:1"`
}

type TenantProjectUsageStatsRes struct {
	Data map[string]any `json:"data"`
}

type TenantProjectUsageLogsReq struct {
	g.Meta   `path:"/projects/{id}/usage-logs" method:"get" mime:"json" tags:"租户控制台-项目" summary:"项目用量日志"`
	Id       int64 `json:"id" in:"path" v:"required|min:1"`
	Page     int   `json:"page" in:"query" d:"1"`
	PageSize int   `json:"page_size" in:"query" d:"20"`
}

type TenantProjectUsageLogsRes struct {
	List     []map[string]any `json:"list"`
	Total    int              `json:"total"`
	Page     int              `json:"page"`
	PageSize int              `json:"page_size"`
}

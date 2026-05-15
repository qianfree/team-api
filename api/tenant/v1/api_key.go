package v1

import (
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
)

// === 租户 API Key 管理 ===

type TenantApiKeyListReq struct {
	g.Meta    `path:"/api-keys" method:"get" mime:"json" tags:"租户控制台-API Key" summary:"API Key列表"`
	KeyType   string `json:"key_type" in:"query"`
	ProjectID int64  `json:"project_id" in:"query"`
	Page      int    `json:"page" in:"query" d:"1"`
	PageSize  int    `json:"page_size" in:"query" d:"20"`
}

type TenantApiKeyListRes struct {
	List     []map[string]any `json:"list"`
	Total    int              `json:"total"`
	Page     int              `json:"page"`
	PageSize int              `json:"page_size"`
}

type TenantApiKeyCreateReq struct {
	g.Meta     `path:"/api-keys" method:"post" mime:"json" tags:"租户控制台-API Key" summary:"创建API Key"`
	Name       string      `json:"name" v:"required"`
	Scope      string      `json:"scope"`
	KeyType    string      `json:"key_type"`
	ProjectID  int64       `json:"project_id"`
	ExpiresAt  *gtime.Time `json:"expires_at"`
	ModelNames []string    `json:"model_names"`
}

type TenantApiKeyCreateRes struct {
	Id        int64  `json:"id"`
	Key       string `json:"key"`
	KeyPrefix string `json:"key_prefix"`
	KeyType   string `json:"key_type"`
	Name      string `json:"name"`
	Scope     string `json:"scope"`
}

type TenantApiKeyDeleteReq struct {
	g.Meta `path:"/api-keys/{id}" method:"delete" mime:"json" tags:"租户控制台-API Key" summary:"删除API Key"`
	Id     int64 `json:"id" in:"path" v:"required|min:1"`
}

type TenantApiKeyDeleteRes struct{}

type TenantApiKeyUpdateReq struct {
	g.Meta               `path:"/api-keys/{id}" method:"put" mime:"json" tags:"租户控制台-API Key" summary:"更新API Key"`
	Id                   int64       `json:"id" in:"path" v:"required|min:1"`
	Name                 string      `json:"name"`
	Scope                string      `json:"scope"`
	Status               string      `json:"status"`
	ExpiresAt            *gtime.Time `json:"expires_at"`
	RateLimitQps         *int        `json:"rate_limit_qps"`
	RateLimitConcurrency *int        `json:"rate_limit_concurrency"`
	TotalQuota           *float64    `json:"total_quota"`
	ModelNames           []string    `json:"model_names"`
}

type TenantApiKeyUpdateRes struct{}

type TenantApiKeyUpdateScopesReq struct {
	g.Meta     `path:"/api-keys/{id}/scopes" method:"put" mime:"json" tags:"租户控制台-API Key" summary:"更新API Key模型范围"`
	Id         int64    `json:"id" in:"path" v:"required|min:1"`
	ModelNames []string `json:"model_names" v:"required"`
}

type TenantApiKeyUpdateScopesRes struct{}

type TenantApiKeyModelScopesReq struct {
	g.Meta `path:"/api-keys/{id}/model-scopes" method:"get" mime:"json" tags:"租户控制台-API Key" summary:"查询API Key模型范围"`
	Id     int64 `json:"id" in:"path" v:"required|min:1"`
}

type TenantApiKeyModelScopesRes struct {
	ModelNames []string `json:"model_names"`
}

// TenantApiKeyExportReq 导出API Key列表请求
type TenantApiKeyExportReq struct {
	g.Meta    `path:"/api-keys/export" method:"get" mime:"json" tags:"租户控制台-API Key" summary:"导出API Key列表"`
	Format    string `json:"format" in:"query" d:"csv" v:"in:csv,xlsx" dc:"导出格式：csv / xlsx"`
	KeyType   string `json:"key_type" in:"query"`
	ProjectID int64  `json:"project_id" in:"query"`
}

type TenantApiKeyExportRes struct{}

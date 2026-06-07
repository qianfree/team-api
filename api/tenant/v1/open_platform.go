package v1

import "github.com/gogf/gf/v2/frame/g"

// ============================================================
// 开放平台应用管理
// ============================================================

// OpenAppListReq 应用列表
type OpenAppListReq struct {
	g.Meta   `path:"/open/apps" method:"get" tags:"租户控制台-开放平台" summary:"应用列表"`
	Page     int    `json:"page" in:"query" d:"1" dc:"页码"`
	PageSize int    `json:"page_size" in:"query" d:"20" dc:"每页数量"`
	Keyword  string `json:"keyword" in:"query" dc:"搜索关键词"`
}

type OpenAppListRes struct {
	List     []OpenAppItem `json:"list"`
	Total    int           `json:"total"`
	Page     int           `json:"page"`
	PageSize int           `json:"page_size"`
}

type OpenAppItem struct {
	ID          int64    `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	AppID       string   `json:"app_id"`
	Permissions []string `json:"permissions"`
	Status      string   `json:"status"`
	IsSandbox   bool     `json:"is_sandbox"`
	RateLimit   int      `json:"rate_limit"`
	LastUsedAt  string   `json:"last_used_at,omitempty"`
	CreatedAt   string   `json:"created_at"`
}

// OpenAppCreateReq 创建应用
type OpenAppCreateReq struct {
	g.Meta      `path:"/open/apps" method:"post" mime:"json" tags:"租户控制台-开放平台" summary:"创建应用"`
	Name        string   `json:"name" v:"required|length:2,100#请输入应用名称|应用名称长度为2-100位" dc:"应用名称"`
	Description string   `json:"description" v:"length:0,500#描述不超过500字" dc:"应用描述"`
	Permissions []string `json:"permissions" v:"required#请选择权限范围" dc:"权限范围"`
	IPWhitelist []string `json:"ip_whitelist" dc:"IP 白名单"`
	RateLimit   int      `json:"rate_limit" d:"60" dc:"每分钟请求上限"`
}

type OpenAppCreateRes struct {
	ID        int64  `json:"id"`
	AppID     string `json:"app_id"`
	AppSecret string `json:"app_secret"` // 仅创建时返回一次
}

// OpenAppUpdateReq 更新应用
type OpenAppUpdateReq struct {
	g.Meta      `path:"/open/apps/{id}" method:"put" mime:"json" tags:"租户控制台-开放平台" summary:"更新应用"`
	Id          int64    `json:"id" in:"path" v:"required#请指定应用ID" dc:"应用ID"`
	Name        *string  `json:"name" dc:"应用名称"`
	Description *string  `json:"description" dc:"应用描述"`
	Permissions []string `json:"permissions" dc:"权限范围"`
	IPWhitelist []string `json:"ip_whitelist" dc:"IP 白名单"`
	RateLimit   *int     `json:"rate_limit" dc:"每分钟请求上限"`
}

type OpenAppUpdateRes struct{}

// OpenAppDeleteReq 删除应用
type OpenAppDeleteReq struct {
	g.Meta `path:"/open/apps/{id}" method:"delete" tags:"租户控制台-开放平台" summary:"删除应用"`
	Id     int64 `json:"id" in:"path" v:"required#请指定应用ID" dc:"应用ID"`
}

type OpenAppDeleteRes struct{}

// OpenAppResetSecretReq 重置密钥
type OpenAppResetSecretReq struct {
	g.Meta `path:"/open/apps/{id}/reset-secret" method:"post" tags:"租户控制台-开放平台" summary:"重置应用密钥"`
	Id     int64 `json:"id" in:"path" v:"required#请指定应用ID" dc:"应用ID"`
}

type OpenAppResetSecretRes struct {
	AppSecret string `json:"app_secret"` // 新密钥，仅返回一次
}

// OpenAppToggleStatusReq 启用/禁用应用
type OpenAppToggleStatusReq struct {
	g.Meta `path:"/open/apps/{id}/status" method:"put" mime:"json" tags:"租户控制台-开放平台" summary:"启用/禁用应用"`
	Id     int64  `json:"id" in:"path" v:"required#请指定应用ID" dc:"应用ID"`
	Status string `json:"status" v:"required|in:active,disabled#请选择状态|状态无效" dc:"状态"`
}

type OpenAppToggleStatusRes struct{}

// ============================================================
// Webhook 配置管理
// ============================================================

// WebhookConfigListReq Webhook 配置列表
type WebhookConfigListReq struct {
	g.Meta `path:"/open/webhooks" method:"get" tags:"租户控制台-开放平台" summary:"Webhook配置列表"`
}

type WebhookConfigListRes struct {
	List []WebhookConfigItem `json:"list"`
}

type WebhookConfigItem struct {
	ID                     int64    `json:"id"`
	Name                   string   `json:"name"`
	URL                    string   `json:"url"`
	Events                 []string `json:"events"`
	IsActive               bool     `json:"is_active"`
	ConsecutiveFailures    int      `json:"consecutive_failures"`
	MaxConsecutiveFailures int      `json:"max_consecutive_failures"`
	LastDeliveryAt         string   `json:"last_delivery_at,omitempty"`
	CreatedAt              string   `json:"created_at"`
}

// WebhookConfigCreateReq 创建 Webhook
type WebhookConfigCreateReq struct {
	g.Meta                 `path:"/open/webhooks" method:"post" mime:"json" tags:"租户控制台-开放平台" summary:"创建Webhook配置"`
	Name                   string   `json:"name" v:"required|length:2,100#请输入名称|名称长度2-100" dc:"配置名称"`
	URL                    string   `json:"url" v:"required|length:10,500#请输入URL|URL长度不正确" dc:"回调URL（HTTPS）"`
	Events                 []string `json:"events" v:"required#请选择事件" dc:"订阅事件列表"`
	MaxConsecutiveFailures int      `json:"max_consecutive_failures" d:"10" dc:"最大连续失败次数"`
}

type WebhookConfigCreateRes struct {
	ID        int64  `json:"id"`
	SecretKey string `json:"secret_key"` // 签名密钥，仅创建时返回
}

// WebhookConfigUpdateReq 更新 Webhook
type WebhookConfigUpdateReq struct {
	g.Meta                 `path:"/open/webhooks/{id}" method:"put" mime:"json" tags:"租户控制台-开放平台" summary:"更新Webhook配置"`
	Id                     int64    `json:"id" in:"path" v:"required#请指定配置ID" dc:"配置ID"`
	Name                   *string  `json:"name" dc:"配置名称"`
	URL                    *string  `json:"url" dc:"回调URL"`
	Events                 []string `json:"events" dc:"订阅事件列表"`
	IsActive               *bool    `json:"is_active" dc:"是否启用"`
	MaxConsecutiveFailures *int     `json:"max_consecutive_failures" dc:"最大连续失败次数"`
}

type WebhookConfigUpdateRes struct{}

// WebhookConfigDeleteReq 删除 Webhook
type WebhookConfigDeleteReq struct {
	g.Meta `path:"/open/webhooks/{id}" method:"delete" tags:"租户控制台-开放平台" summary:"删除Webhook配置"`
	Id     int64 `json:"id" in:"path" v:"required#请指定配置ID" dc:"配置ID"`
}

type WebhookConfigDeleteRes struct{}

// WebhookDeliveryLogsReq 投递日志
type WebhookDeliveryLogsReq struct {
	g.Meta   `path:"/open/webhooks/{id}/logs" method:"get" tags:"租户控制台-开放平台" summary:"投递日志"`
	Id       int64  `json:"id" in:"path" v:"required#请指定配置ID" dc:"配置ID"`
	Page     int    `json:"page" in:"query" d:"1" dc:"页码"`
	PageSize int    `json:"page_size" in:"query" d:"20" dc:"每页数量"`
	Status   string `json:"status" in:"query" dc:"按状态筛选"`
}

type WebhookDeliveryLogsRes struct {
	List     []WebhookDeliveryLogItem `json:"list"`
	Total    int                      `json:"total"`
	Page     int                      `json:"page"`
	PageSize int                      `json:"page_size"`
}

type WebhookDeliveryLogItem struct {
	ID             int64  `json:"id"`
	EventID        int64  `json:"event_id"`
	EventType      string `json:"event_type"`
	Attempt        int    `json:"attempt"`
	ResponseStatus int    `json:"response_status"`
	ResponseTimeMs int    `json:"response_time_ms"`
	ErrorMessage   string `json:"error_message,omitempty"`
	CreatedAt      string `json:"created_at"`
}

// WebhookRetryReq 手动重试
type WebhookRetryReq struct {
	g.Meta  `path:"/open/webhooks/events/{eventId}/retry" method:"post" tags:"租户控制台-开放平台" summary:"手动重试事件"`
	EventId int64 `json:"eventId" in:"path" v:"required#请指定事件ID" dc:"事件ID"`
}

type WebhookRetryRes struct{}

package v1

import "github.com/gogf/gf/v2/frame/g"

// TenantPluginListReq 租户可用插件列表
type TenantPluginListReq struct {
	g.Meta `path:"/plugins" method:"get" mime:"json" tags:"租户控制台-插件" summary:"租户可用插件列表"`
}

type TenantPluginItem struct {
	Name        string `json:"name"`
	Label       string `json:"label"`
	Description string `json:"description"`
	Version     string `json:"version"`
	Category    string `json:"category"`
	Enabled     bool   `json:"enabled"`
}

type TenantPluginListRes struct {
	List []TenantPluginItem `json:"list"`
}

// TenantPluginDetailReq 租户插件详情
type TenantPluginDetailReq struct {
	g.Meta `path:"/plugins/{name}" method:"get" mime:"json" tags:"租户控制台-插件" summary:"租户插件详情"`
	Name   string `json:"name" in:"path" v:"required" dc:"插件标识"`
}

type TenantPluginDetailRes struct {
	Name        string `json:"name"`
	Label       string `json:"label"`
	Description string `json:"description"`
	Version     string `json:"version"`
	Category    string `json:"category"`
	Enabled     bool   `json:"enabled"`
	Config      g.Map  `json:"config"`
}

// TenantPluginConfigUpdateReq 租户级配置覆盖
type TenantPluginConfigUpdateReq struct {
	g.Meta `path:"/plugins/{name}/config" method:"put" mime:"json" tags:"租户控制台-插件" summary:"租户级配置覆盖"`
	Name   string `json:"name"   in:"path" v:"required" dc:"插件标识"`
	Config g.Map  `json:"config" v:"required" dc:"插件配置"`
}

type TenantPluginConfigUpdateRes struct{}

// TenantPluginEnableReq 租户启用插件
type TenantPluginEnableReq struct {
	g.Meta `path:"/plugins/{name}/enable" method:"post" mime:"json" tags:"租户控制台-插件" summary:"租户启用插件"`
	Name   string `json:"name" in:"path" v:"required" dc:"插件标识"`
}

type TenantPluginEnableRes struct{}

// TenantPluginDisableReq 租户禁用插件
type TenantPluginDisableReq struct {
	g.Meta `path:"/plugins/{name}/disable" method:"post" mime:"json" tags:"租户控制台-插件" summary:"租户禁用插件"`
	Name   string `json:"name" in:"path" v:"required" dc:"插件标识"`
}

type TenantPluginDisableRes struct{}

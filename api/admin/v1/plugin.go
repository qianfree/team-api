package v1

import "github.com/gogf/gf/v2/frame/g"

// PluginListReq 插件列表
type PluginListReq struct {
	g.Meta   `path:"/plugins" method:"get" mime:"json" tags:"管理后台-插件管理" summary:"插件列表"`
	Category string `json:"category" in:"query" dc:"按分类筛选"`
	Status   string `json:"status"   in:"query" dc:"按状态筛选"`
}

type PluginItem struct {
	Name        string `json:"name"`
	Label       string `json:"label"`
	Description string `json:"description"`
	Version     string `json:"version"`
	Category    string `json:"category"`
	Author      string `json:"author"`
	Status      string `json:"status"`
	Installed   bool   `json:"installed"`
	Config      g.Map  `json:"config"`
}

type PluginListRes struct {
	List []PluginItem `json:"list"`
}

// PluginDetailReq 插件详情
type PluginDetailReq struct {
	g.Meta `path:"/plugins/{name}" method:"get" mime:"json" tags:"管理后台-插件管理" summary:"插件详情"`
	Name   string `json:"name" in:"path" v:"required" dc:"插件标识"`
}

type PluginDetailRes struct {
	Name         string   `json:"name"`
	Label        string   `json:"label"`
	Description  string   `json:"description"`
	Version      string   `json:"version"`
	Category     string   `json:"category"`
	Author       string   `json:"author"`
	Status       string   `json:"status"`
	Dependencies []string `json:"dependencies"`
	Config       g.Map    `json:"config"`
	ErrorMsg     string   `json:"error_msg,omitempty"`
}

// PluginInstallReq 安装插件
type PluginInstallReq struct {
	g.Meta `path:"/plugins/{name}/install" method:"post" mime:"json" tags:"管理后台-插件管理" summary:"安装插件"`
	Name   string `json:"name" in:"path" v:"required" dc:"插件标识"`
}

type PluginInstallRes struct{}

// PluginEnableReq 启用插件
type PluginEnableReq struct {
	g.Meta `path:"/plugins/{name}/enable" method:"post" mime:"json" tags:"管理后台-插件管理" summary:"启用插件"`
	Name   string `json:"name" in:"path" v:"required" dc:"插件标识"`
}

type PluginEnableRes struct{}

// PluginDisableReq 禁用插件
type PluginDisableReq struct {
	g.Meta `path:"/plugins/{name}/disable" method:"post" mime:"json" tags:"管理后台-插件管理" summary:"禁用插件"`
	Name   string `json:"name" in:"path" v:"required" dc:"插件标识"`
}

type PluginDisableRes struct{}

// PluginUninstallReq 卸载插件
type PluginUninstallReq struct {
	g.Meta   `path:"/plugins/{name}/uninstall" method:"post" mime:"json" tags:"管理后台-插件管理" summary:"卸载插件"`
	Name     string `json:"name"      in:"path"  v:"required" dc:"插件标识"`
	KeepData bool   `json:"keep_data" in:"query" dc:"是否保留数据"`
}

type PluginUninstallRes struct{}

// PluginUpgradeReq 升级插件
type PluginUpgradeReq struct {
	g.Meta `path:"/plugins/{name}/upgrade" method:"post" mime:"json" tags:"管理后台-插件管理" summary:"升级插件"`
	Name   string `json:"name" in:"path" v:"required" dc:"插件标识"`
}

type PluginUpgradeRes struct{}

// PluginConfigUpdateReq 更新插件配置
type PluginConfigUpdateReq struct {
	g.Meta `path:"/plugins/{name}/config" method:"put" mime:"json" tags:"管理后台-插件管理" summary:"更新插件配置"`
	Name   string `json:"name"   in:"path" v:"required" dc:"插件标识"`
	Config g.Map  `json:"config" v:"required" dc:"插件配置"`
}

type PluginConfigUpdateRes struct{}

// PluginConfigSchemaReq 获取插件配置 schema
type PluginConfigSchemaReq struct {
	g.Meta `path:"/plugins/{name}/config-schema" method:"get" mime:"json" tags:"管理后台-插件管理" summary:"获取插件配置 schema"`
	Name   string `json:"name" in:"path" v:"required" dc:"插件标识"`
}

type PluginConfigFieldItem struct {
	Key         string   `json:"key"`
	Label       string   `json:"label"`
	Type        string   `json:"type"`
	Default     any      `json:"default"`
	Options     []string `json:"options,omitempty"`
	Required    bool     `json:"required"`
	Description string   `json:"description"`
}

type PluginConfigSchemaRes struct {
	List []PluginConfigFieldItem `json:"list"`
}

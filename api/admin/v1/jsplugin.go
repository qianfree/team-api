package v1

import (
	"encoding/json"

	"github.com/gogf/gf/v2/frame/g"
)

// JS 插件（异步任务插件）管理 API：上传/版本/激活/启停。
// 权限两权分立（设计方案 §十三-4）：plugin:upload（上传/版本管理）、
// plugin:manage（启停/激活/删除）；默认仅 super_admin 可见授权。

// JSPluginListReq JS 插件列表
type JSPluginListReq struct {
	g.Meta `path:"/js-plugins" method:"get" mime:"json" tags:"管理后台-JS插件" summary:"JS插件列表"`
}

type JSPluginVersionItem struct {
	ID        int64           `json:"id"`
	Version   string          `json:"version"`
	Origin    string          `json:"origin"` // builtin=内置（不可删除）| upload
	Active    bool            `json:"active"`
	Enabled   bool            `json:"enabled"`
	Remark    string          `json:"remark"`
	CreatedAt string          `json:"created_at"`
	Manifest  json.RawMessage `json:"manifest,omitempty"`
}

type JSPluginItem struct {
	Key           string                `json:"key"`
	Name          string                `json:"name"`
	Icon          string                `json:"icon"` // text/text:label/图标名（前端解析）
	HasImage      bool                  `json:"has_image"`
	Description   string                `json:"description"`
	Author        string                `json:"author"`
	Website       string                `json:"website"`
	Origin        string                `json:"origin"` // 激活版本来源：builtin | upload
	ActiveVersion string                `json:"active_version"`
	Enabled       bool                  `json:"enabled"`
	InService     bool                  `json:"in_service"`
	Models        []string              `json:"models"`
	FetchMode     string                `json:"fetch_mode"`
	AllowedHosts  []string              `json:"allowed_hosts"`
	UsageSchema   []JSPluginUsageField  `json:"usage_schema"`
	Manifest      json.RawMessage       `json:"manifest"`
	Versions      []JSPluginVersionItem `json:"versions"`
}

// JSPluginUsageField 计费单位声明（定价公式 u() 速查参考）。
type JSPluginUsageField struct {
	Key         string `json:"key"`
	Type        string `json:"type"` // number（可计费）| enum
	Unit        string `json:"unit,omitempty"`
	Description string `json:"description,omitempty"`
}

type JSPluginListRes struct {
	List []JSPluginItem `json:"list"`
}

// JSPluginUploadReq 上传 JS 插件（plugin.js 源码）
type JSPluginUploadReq struct {
	g.Meta `path:"/js-plugins/upload" method:"post" mime:"json" tags:"管理后台-JS插件" summary:"上传JS插件"`
	Source string `json:"source" v:"required" dc:"plugin.js 源码全文"`
	Icon   string `json:"icon" dc:"品牌图标（SVG/PNG data URI，可选，≤512KiB，SVG 拒绝脚本/外链）"`
	Remark string `json:"remark" dc:"备注"`
}

type JSPluginUploadRes struct {
	Manifest json.RawMessage `json:"manifest" dc:"解析出的插件清单"`
}

// JSPluginDryRunReq 上传前试运行（不落库、不发布 generation、不发真实 HTTP）
type JSPluginDryRunReq struct {
	g.Meta `path:"/js-plugins/dry-run" method:"post" mime:"json" tags:"管理后台-JS插件" summary:"试运行JS插件（上传前自检）"`
	Source string `json:"source" v:"required" dc:"plugin.js 源码全文"`
}

// JSPluginDryRunCheck 试运行单项检查结果
type JSPluginDryRunCheck struct {
	Name   string `json:"name"`   // compile | hooks | conflict | sample-decode | sample-build | lint-fetch-url
	Status string `json:"status"` // pass=通过 | warn=提示 | fail=缺陷
	Detail string `json:"detail"` // 中文说明
}

// JSPluginDryRunRes 试运行报告（manifest 摘要 + 检查项平铺；编译失败时仅 checks）
type JSPluginDryRunRes struct {
	Key       string                `json:"key,omitempty"`
	Name      string                `json:"name,omitempty"`
	Version   string                `json:"version,omitempty"`
	Models    []string              `json:"models,omitempty"`
	FetchMode string                `json:"fetch_mode,omitempty"`
	Routes    []JSPluginRouteItem   `json:"routes,omitempty"` // manifest 声明的原始路由（未加 /pjs/{key} 前缀）
	Checks    []JSPluginDryRunCheck `json:"checks"`
	Pass      bool                  `json:"pass"` // 无 fail 项即 true（warn 不阻断上传）
}

// JSPluginTemplateReq 下载插件作者模板（内置 golden 参考实现源码）
type JSPluginTemplateReq struct {
	g.Meta `path:"/js-plugins/template" method:"get" mime:"json" tags:"管理后台-JS插件" summary:"下载JS插件模板"`
}

type JSPluginTemplateRes struct {
	Source   string `json:"source"`    // 模板源码全文（golden 参考实现）
	FileName string `json:"file_name"` // 建议保存文件名
}

// JSPluginActivateReq 激活指定版本
type JSPluginActivateReq struct {
	g.Meta  `path:"/js-plugins/{key}/versions/{version}/activate" method:"post" mime:"json" tags:"管理后台-JS插件" summary:"激活JS插件版本"`
	Key     string `json:"key"     in:"path" v:"required" dc:"插件标识"`
	Version string `json:"version" in:"path" v:"required" dc:"版本号"`
}

type JSPluginActivateRes struct{}

// JSPluginEnableReq 启用/禁用 JS 插件
type JSPluginEnableReq struct {
	g.Meta  `path:"/js-plugins/{key}/enable" method:"post" mime:"json" tags:"管理后台-JS插件" summary:"启用/禁用JS插件"`
	Key     string `json:"key"     in:"path" v:"required" dc:"插件标识"`
	Enabled bool   `json:"enabled" dc:"true=启用 false=禁用"`
}

type JSPluginEnableRes struct{}

// JSPluginDeleteReq 删除插件版本
type JSPluginDeleteReq struct {
	g.Meta  `path:"/js-plugins/{key}/versions/{version}" method:"delete" mime:"json" tags:"管理后台-JS插件" summary:"删除JS插件版本"`
	Key     string `json:"key"     in:"path" v:"required" dc:"插件标识"`
	Version string `json:"version" in:"path" v:"required" dc:"版本号"`
}

type JSPluginDeleteRes struct{}

// JSPluginSourceReq 查看插件版本源码
type JSPluginSourceReq struct {
	g.Meta  `path:"/js-plugins/{key}/versions/{version}/source" method:"get" mime:"json" tags:"管理后台-JS插件" summary:"查看JS插件源码"`
	Key     string `json:"key"     in:"path" v:"required" dc:"插件标识"`
	Version string `json:"version" in:"path" v:"required" dc:"版本号"`
}

type JSPluginSourceRes struct {
	Key        string `json:"key"`
	Version    string `json:"version"`
	Origin     string `json:"origin"`      // builtin | upload
	Source     string `json:"source"`      // plugin.js 源码全文
	SourceHash string `json:"source_hash"` // sha256，前端截断展示
	Remark     string `json:"remark"`
	CreatedAt  string `json:"created_at"`
}

// JSPluginRoutesReq 插件路由预览（取激活版本 manifest 声明，禁用状态也可见）
type JSPluginRoutesReq struct {
	g.Meta `path:"/js-plugins/{key}/routes" method:"get" mime:"json" tags:"管理后台-JS插件" summary:"JS插件路由预览"`
	Key    string `json:"key" in:"path" v:"required" dc:"插件标识"`
}

type JSPluginRouteItem struct {
	Method string `json:"method"`
	Path   string `json:"path"`
	Type   string `json:"type"`
}

type JSPluginRoutesRes struct {
	List []JSPluginRouteItem `json:"list"`
}

// JSPluginOpsStatsReq JS 插件运营统计（任务量/成功率/营收，按天窗口聚合）
type JSPluginOpsStatsReq struct {
	g.Meta `path:"/js-plugins/ops-stats" method:"get" mime:"json" tags:"管理后台-JS插件" summary:"JS插件运营统计"`
	Days   int `json:"days" d:"30" v:"between:1,365" dc:"统计窗口天数（默认 30，范围 1-365）"`
}

// JSPluginOpsStatsItem 每插件一行的运营统计。营收为本位币（bil_usage_logs 聚合，
// 实扣优先口径：actual_cost 优先、0/NULL 回退 total_cost）。
type JSPluginOpsStatsItem struct {
	PluginKey       string  `json:"plugin_key"`
	Name            string  `json:"name"`
	Enabled         bool    `json:"enabled"`          // 当前是否在服务（存在于运行中 generation）
	TasksTotal      int64   `json:"tasks_total"`      // 窗口内任务总数
	TasksSuccess    int64   `json:"tasks_success"`    // 成功终态数
	TasksFailed     int64   `json:"tasks_failed"`     // 失败终态数
	TasksProcessing int64   `json:"tasks_processing"` // 非终态数（SUBMITTED/IN_PROGRESS 等）
	SuccessRate     float64 `json:"success_rate"`     // success/(success+failed)，分母 0 时为 0（无终态任务），取值 0-1
	Revenue         float64 `json:"revenue"`          // 窗口内营收（本位币）
	LastTaskTime    string  `json:"last_task_time"`   // 窗口内最近一次任务时间，无任务为空串
	Orphan          bool    `json:"orphan"`           // 任务记录存在但激活 manifest 已不存在（插件被删/换血）
}

type JSPluginOpsStatsRes struct {
	List []JSPluginOpsStatsItem `json:"list"`
}

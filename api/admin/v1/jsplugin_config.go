package v1

import "github.com/gogf/gf/v2/frame/g"

// JSPluginConfigReq JS 插件配置读取（schema + 当前值，加密字段掩码）
type JSPluginConfigReq struct {
	g.Meta `path:"/js-plugins/{key}/config-schema" method:"get" mime:"json" tags:"管理后台-JS插件" summary:"JS插件配置schema"`
	Key    string `json:"key" in:"path" v:"required" dc:"插件标识"`
}

type JSPluginConfigField struct {
	Key         string   `json:"key"`
	Label       string   `json:"label"`
	Type        string   `json:"type"`
	Default     any      `json:"default,omitempty"`
	Options     []string `json:"options,omitempty"`
	Required    bool     `json:"required"`
	Description string   `json:"description"`
	Encrypted   bool     `json:"encrypted,omitempty"`
	Group       string   `json:"group,omitempty"`
	HalfWidth   bool     `json:"half_width,omitempty"`
	Value       any      `json:"value"` // 加密字段：空=未配置 / "********"=已配置
}

type JSPluginConfigRes struct {
	List []JSPluginConfigField `json:"list"`
}

// JSPluginConfigUpdateReq 更新 JS 插件配置（加密字段空值/掩码=保持现值）
type JSPluginConfigUpdateReq struct {
	g.Meta `path:"/js-plugins/{key}/config" method:"put" mime:"json" tags:"管理后台-JS插件" summary:"更新JS插件配置"`
	Key    string         `json:"key"    in:"path" v:"required" dc:"插件标识"`
	Config map[string]any `json:"config" v:"required" dc:"配置值"`
}

type JSPluginConfigUpdateRes struct{}

// JSPluginMetricsReq JS 插件运行指标（内存钩子指标 + DB 任务/产物统计）
type JSPluginMetricsReq struct {
	g.Meta `path:"/js-plugins/metrics" method:"get" mime:"json" tags:"管理后台-JS插件" summary:"JS插件运行指标"`
}

type JSPluginMetricsHook struct {
	Calls     uint64  `json:"calls"`
	Errors    uint64  `json:"errors"`
	ErrorRate float64 `json:"error_rate"`
	AvgMs     float64 `json:"avg_ms"`
	LastErr   string  `json:"last_err,omitempty"`
	LastErrAt string  `json:"last_err_at,omitempty"`
}

type JSPluginMetricsItem struct {
	Key             string                          `json:"key"`
	InService       bool                            `json:"in_service"`
	LiveInstances   int                             `json:"live_instances"`
	TotalCalls      uint64                          `json:"total_calls"`
	TotalErrors     uint64                          `json:"total_errors"`
	AvgMs           float64                         `json:"avg_ms"`
	Hooks           map[string]*JSPluginMetricsHook `json:"hooks"`
	TasksTotal      int64                           `json:"tasks_total"`
	TasksSuccess    int64                           `json:"tasks_success"`
	TasksActive     int64                           `json:"tasks_active"`
	TasksFailed     int64                           `json:"tasks_failed"`
	ArtifactsStored int64                           `json:"artifacts_stored"`
	ArtifactsFailed int64                           `json:"artifacts_failed"`
}

type JSPluginMetricsRes struct {
	Since string                `json:"since"` // 指标累计起点（进程启动）
	List  []JSPluginMetricsItem `json:"list"`
}

package v1

import "github.com/gogf/gf/v2/frame/g"

// ChannelListReq 渠道列表请求
type ChannelListReq struct {
	g.Meta   `path:"/channels" method:"get" mime:"json" tags:"管理后台-渠道" summary:"渠道列表"`
	Page     int    `json:"page" d:"1" v:"min:1" dc:"页码"`
	PageSize int    `json:"page_size" d:"20" v:"min:1|max:100" dc:"每页数量"`
	Type     int    `json:"type" dc:"供应商类型筛选"`
	Status   string `json:"status" dc:"状态筛选：active/disabled/testing"`
	Search   string `json:"search" dc:"搜索关键词"`
}

// ChannelListRes 渠道列表响应
type ChannelListRes struct {
	List     []ChannelItem `json:"list"`
	Total    int           `json:"total"`
	Page     int           `json:"page"`
	PageSize int           `json:"page_size"`
}

// ChannelItem 渠道信息
type ChannelItem struct {
	ID                       int64    `json:"id"`
	Name                     string   `json:"name"`
	Type                     int      `json:"type"`
	TypeName                 string   `json:"type_name"`
	BaseURL                  string   `json:"base_url"`
	Status                   string   `json:"status"`
	Priority                 int      `json:"priority"`
	Weight                   int      `json:"weight"`
	TestModel                string   `json:"test_model"`
	Remark                   string   `json:"remark"`
	IsVIP                    bool     `json:"is_vip"`
	UseProxy                 bool     `json:"use_proxy"`
	SharingThreshold         *float64 `json:"sharing_threshold"`
	PreemptionThreshold      *float64 `json:"preemption_threshold"`
	BorrowingCooldownSeconds *int     `json:"borrowing_cooldown_seconds"`
	CreatedAt                string   `json:"created_at"`
	HealthScore              *float64 `json:"health_score"`
}

// ChannelCreateReq 创建渠道请求
type ChannelCreateReq struct {
	g.Meta                   `path:"/channels" method:"post" mime:"json" tags:"管理后台-渠道" summary:"创建渠道"`
	Name                     string  `json:"name" v:"required|length:1,100#请输入渠道名|渠道名长度1-100" dc:"渠道名称"`
	Type                     int     `json:"type" v:"required|min:1#请选择供应商类型" dc:"供应商类型"`
	BaseURL                  string  `json:"base_url" dc:"API 基础地址（留空使用供应商默认地址）"`
	ApiKey                   string  `json:"api_key" v:"required#请输入 API Key" dc:"API Key"`
	Priority                 int     `json:"priority" d:"0" dc:"优先级"`
	Weight                   int     `json:"weight" d:"100" v:"between:0,100" dc:"权重"`
	TestModel                string  `json:"test_model" dc:"测试模型名"`
	Remark                   string  `json:"remark" dc:"备注"`
	IsVIP                    bool    `json:"is_vip" d:"false" dc:"是否VIP专属渠道"`
	UseProxy                 bool    `json:"use_proxy" d:"false" dc:"启用代理"`
	SharingThreshold         float64 `json:"sharing_threshold" d:"0.6" dc:"普通租户借用阈值"`
	PreemptionThreshold      float64 `json:"preemption_threshold" d:"0.8" dc:"VIP抢占阈值"`
	BorrowingCooldownSeconds int     `json:"borrowing_cooldown_seconds" d:"30" dc:"被抢占后冷却时间(秒)"`
}

// ChannelCreateRes 创建渠道响应
type ChannelCreateRes struct {
	ID int64 `json:"id"`
}

// ChannelUpdateReq 更新渠道请求
type ChannelUpdateReq struct {
	g.Meta                   `path:"/channels/{id}" method:"put" mime:"json" tags:"管理后台-渠道" summary:"更新渠道"`
	ID                       int64    `json:"id" in:"path" v:"required" dc:"渠道ID"`
	Name                     string   `json:"name" dc:"渠道名称"`
	BaseURL                  string   `json:"base_url" dc:"API 基础地址"`
	ApiKey                   *string  `json:"api_key" dc:"更新 API Key（留空不更新）"`
	Priority                 int      `json:"priority" dc:"优先级"`
	Weight                   int      `json:"weight" dc:"权重"`
	TestModel                string   `json:"test_model" dc:"测试模型名"`
	Remark                   string   `json:"remark" dc:"备注"`
	Status                   string   `json:"status" v:"in:active,disabled,testing" dc:"状态"`
	IsVIP                    *bool    `json:"is_vip" dc:"是否VIP专属渠道"`
	UseProxy                 *bool    `json:"use_proxy" dc:"启用代理"`
	SharingThreshold         *float64 `json:"sharing_threshold" dc:"普通租户借用阈值"`
	PreemptionThreshold      *float64 `json:"preemption_threshold" dc:"VIP抢占阈值"`
	BorrowingCooldownSeconds *int     `json:"borrowing_cooldown_seconds" dc:"被抢占后冷却时间(秒)"`
}

// ChannelDeleteReq 删除渠道请求
type ChannelDeleteReq struct {
	g.Meta `path:"/channels/{id}" method:"delete" mime:"json" tags:"管理后台-渠道" summary:"删除渠道"`
	ID     int64 `json:"id" in:"path" v:"required" dc:"渠道ID"`
}

// ChannelDetailReq 渠道详情请求
type ChannelDetailReq struct {
	g.Meta `path:"/channels/{id}" method:"get" mime:"json" tags:"管理后台-渠道" summary:"渠道详情"`
	ID     int64 `json:"id" in:"path" v:"required" dc:"渠道ID"`
}

// ChannelDetailRes 渠道详情响应
type ChannelDetailRes struct {
	ID                       int64    `json:"id"`
	Name                     string   `json:"name"`
	Type                     int      `json:"type"`
	TypeName                 string   `json:"type_name"`
	BaseURL                  string   `json:"base_url"`
	Status                   string   `json:"status"`
	Priority                 int      `json:"priority"`
	Weight                   int      `json:"weight"`
	TestModel                string   `json:"test_model"`
	Remark                   string   `json:"remark"`
	IsVIP                    bool     `json:"is_vip"`
	UseProxy                 bool     `json:"use_proxy"`
	SharingThreshold         *float64 `json:"sharing_threshold"`
	PreemptionThreshold      *float64 `json:"preemption_threshold"`
	BorrowingCooldownSeconds *int     `json:"borrowing_cooldown_seconds"`
	CreatedAt                string   `json:"created_at"`
	UpdatedAt                string   `json:"updated_at"`
	HealthScore              *float64 `json:"health_score"`
	KeyType                  string   `json:"key_type"`
	KeyStatus                string   `json:"key_status"`
	KeyName                  string   `json:"key_name"`
	TokenExpiresAt           string   `json:"token_expires_at"`
}

// ChannelKeyCreateReq 添加渠道 Key 请求
type ChannelKeyCreateReq struct {
	g.Meta    `path:"/channels/{channel_id}/keys" method:"post" mime:"json" tags:"管理后台-渠道" summary:"添加渠道 Key"`
	ChannelID int64  `json:"channel_id" in:"path" v:"required" dc:"渠道ID"`
	Name      string `json:"name" dc:"Key 别名"`
	ApiKey    string `json:"api_key" v:"required" dc:"API Key 原值"`
}

// ChannelKeyDeleteReq 删除渠道 Key 请求
type ChannelKeyDeleteReq struct {
	g.Meta    `path:"/channels/{channel_id}/keys/{key_id}" method:"delete" mime:"json" tags:"管理后台-渠道" summary:"删除渠道 Key"`
	ChannelID int64 `json:"channel_id" in:"path" v:"required" dc:"渠道ID"`
	KeyID     int64 `json:"key_id" in:"path" v:"required" dc:"Key ID"`
}

// ChannelKeyDeleteRes 删除渠道 Key 响应
type ChannelKeyDeleteRes struct{}

// ChannelKeyCreateRes 添加渠道 Key 响应
type ChannelKeyCreateRes struct {
	ID int64 `json:"id"`
}

// ChannelAbilityBatchReq 批量设置渠道模型能力
type ChannelAbilityBatchReq struct {
	g.Meta    `path:"/channels/{channel_id}/abilities" method:"put" mime:"json" tags:"管理后台-渠道" summary:"设置渠道模型能力"`
	ChannelID int64         `json:"channel_id" in:"path" v:"required" dc:"渠道ID"`
	Abilities []AbilityItem `json:"abilities" dc:"能力列表"`
}

// AbilityItem 模型能力项
type AbilityItem struct {
	ID            int64  `json:"id"`
	ModelName     string `json:"model_name" v:"required" dc:"平台标准模型名"`
	UpstreamModel string `json:"upstream_model" dc:"上游实际模型名"`
	Enabled       bool   `json:"enabled" d:"true" dc:"是否启用"`
}

// ProviderDefaultURLReq 获取供应商默认 URL
type ProviderDefaultURLReq struct {
	g.Meta `path:"/channels/provider-default-urls" method:"get" mime:"json" tags:"管理后台-渠道" summary:"供应商默认地址"`
}

type ProviderDefaultURLRes struct {
	URLs map[int]string `json:"urls"`
}

// ChannelUpdateRes 更新渠道响应
type ChannelUpdateRes struct{}

// ChannelDeleteRes 删除渠道响应
type ChannelDeleteRes struct{}

// ChannelAbilityBatchRes 批量设置渠道模型能力响应
type ChannelAbilityBatchRes struct{}

// ChannelKeyListReq 渠道 Key 列表请求
type ChannelKeyListReq struct {
	g.Meta    `path:"/channels/{channel_id}/keys" method:"get" mime:"json" tags:"管理后台-渠道" summary:"渠道Key列表"`
	ChannelID int64 `json:"channel_id" in:"path" v:"required" dc:"渠道ID"`
}

type ChannelKeyListRes struct {
	List []ChannelKeyItem `json:"list"`
}

// ChannelKeyItem 渠道 Key 信息
type ChannelKeyItem struct {
	ID             int64  `json:"id"`
	Name           string `json:"name"`
	ApiKey         string `json:"api_key"`
	Status         string `json:"status"`
	KeyType        string `json:"key_type"`
	TokenExpiresAt string `json:"token_expires_at"`
	CreatedAt      string `json:"created_at"`
}

// ChannelAbilitiesGetReq 获取渠道模型能力请求
type ChannelAbilitiesGetReq struct {
	g.Meta    `path:"/channels/{channel_id}/abilities" method:"get" mime:"json" tags:"管理后台-渠道" summary:"获取渠道模型能力"`
	ChannelID int64 `json:"channel_id" in:"path" v:"required" dc:"渠道ID"`
}

type ChannelAbilitiesGetRes struct {
	List []AbilityItem `json:"list"`
}

// ChannelHealthTrendReq 渠道健康趋势请求
type ChannelHealthTrendReq struct {
	g.Meta `path:"/channels/{id}/health_trend" method:"get" mime:"json" tags:"管理后台-渠道" summary:"渠道健康趋势"`
	ID     int64 `json:"id" in:"path" v:"required" dc:"渠道ID"`
	Hours  int   `json:"hours" d:"24" v:"between:1,168" dc:"查询时长(小时)"`
}

// ChannelHealthTrendRes 渠道健康趋势响应
type ChannelHealthTrendRes struct {
	Points []HealthTrendPoint `json:"points"`
}

// HealthTrendPoint 健康趋势数据点
type HealthTrendPoint struct {
	SnapshotAt          string  `json:"snapshot_at"`
	HealthScore         float64 `json:"health_score"`
	SuccessRate         float64 `json:"success_rate"`
	LatencyMs           float64 `json:"latency_ms"`
	StabilityScore      float64 `json:"stability_score"`
	ConsecutiveFailures int     `json:"consecutive_failures"`
}

// ChannelExportReq 导出渠道列表请求
type ChannelExportReq struct {
	g.Meta `path:"/channels/export" method:"get" mime:"json" tags:"管理后台-渠道" summary:"导出渠道列表"`
	Format string `json:"format" in:"query" d:"csv" v:"in:csv,xlsx" dc:"导出格式：csv / xlsx"`
	Type   int    `json:"type" in:"query" dc:"供应商类型筛选"`
	Status string `json:"status" in:"query" dc:"状态筛选：active/disabled/testing"`
	Search string `json:"search" in:"query" dc:"搜索关键词"`
}

type ChannelExportRes struct{}

// ChannelCloneReq 克隆渠道请求
type ChannelCloneReq struct {
	g.Meta `path:"/channels/{id}/clone" method:"post" mime:"json" tags:"管理后台-渠道" summary:"克隆渠道"`
	ID     int64  `json:"id" in:"path" v:"required" dc:"源渠道ID"`
	Name   string `json:"name" dc:"新渠道名称（留空使用默认）"`
	ApiKey string `json:"api_key" v:"required#请输入新渠道的 API Key" dc:"新渠道的 API Key"`
}

// ChannelCloneRes 克隆渠道响应
type ChannelCloneRes struct {
	ID int64 `json:"id"`
}

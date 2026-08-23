package v1

import "github.com/gogf/gf/v2/frame/g"

// ChannelListReq 渠道列表请求
type ChannelListReq struct {
	g.Meta   `path:"/channels" method:"get" mime:"json" tags:"管理后台-渠道" summary:"渠道列表"`
	Page     int    `json:"page" d:"1" v:"min:1" dc:"页码"`
	PageSize int    `json:"page_size" d:"20" v:"min:1|max:100" dc:"每页数量"`
	Type     int    `json:"type" dc:"供应商类型筛选"`
	Status   string `json:"status" dc:"状态筛选：active/disabled/testing"`
	Search   string `json:"search" dc:"搜索关键词（渠道名称/备注）"`
	ID       int64  `json:"id" dc:"按渠道 ID 精确筛选"`
	Model    string `json:"model" dc:"按支持的模型名筛选（模糊匹配平台模型名/上游模型名）"`
	BaseURL  string `json:"base_url" dc:"按 API 地址筛选（模糊匹配）"`
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
	MaxConcurrency           int      `json:"max_concurrency"`
	Tier                     string   `json:"tier"`
	StrictCapacity           bool     `json:"strict_capacity"`
	TestModel                string   `json:"test_model"`
	Remark                   string   `json:"remark"`
	IsVIP                    bool     `json:"is_vip"`
	UseProxy                 bool     `json:"use_proxy"`
	SharingThreshold         *float64 `json:"sharing_threshold"`
	PreemptionThreshold      *float64 `json:"preemption_threshold"`
	BorrowingCooldownSeconds *int     `json:"borrowing_cooldown_seconds"`
	CreatedAt                string   `json:"created_at"`
	HealthScore              *float64 `json:"health_score"`
	BreakerState             int      `json:"breaker_state"`  // 调度熔断状态：0=正常 1=熔断中 2=半开探活
	BreakerModels            int      `json:"breaker_models"` // 处于熔断/半开的模型数量（渠道×模型级汇总）
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
	MaxConcurrency           int     `json:"max_concurrency" d:"100" v:"min:0" dc:"最大并发请求数（0=按上游 429 水位自动估算）"`
	Tier                     string  `json:"tier" d:"primary" v:"in:primary,secondary,reserve" dc:"调度层级：primary=首选 secondary=备用 reserve=保底"`
	StrictCapacity           bool    `json:"strict_capacity" d:"false" dc:"严格容量：Redis 故障时实例级保守限额（fail-closed），用于高成本渠道"`
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
	Type                     *int     `json:"type" v:"min:1#请选择供应商类型" dc:"供应商类型（留空不修改）"`
	BaseURL                  string   `json:"base_url" dc:"API 基础地址（留空不更新；切换类型且旧地址为该类型默认值时自动跟随新类型默认地址）"`
	ApiKey                   *string  `json:"api_key" dc:"更新 API Key（留空不更新）"`
	Priority                 *int     `json:"priority" dc:"优先级（留空不更新）"`
	Weight                   *int     `json:"weight" dc:"权重（留空不更新）"`
	MaxConcurrency           *int     `json:"max_concurrency" v:"min:0" dc:"最大并发请求数（0=按上游 429 水位自动估算，留空不更新）"`
	TestModel                string   `json:"test_model" dc:"测试模型名"`
	Remark                   string   `json:"remark" dc:"备注"`
	Status                   string   `json:"status" v:"in:active,disabled,testing" dc:"状态"`
	Tier                     string   `json:"tier" v:"in:primary,secondary,reserve" dc:"调度层级（留空不更新）"`
	StrictCapacity           *bool    `json:"strict_capacity" dc:"严格容量（fail-closed）"`
	IsVIP                    *bool    `json:"is_vip" dc:"是否VIP专属渠道"`
	UseProxy                 *bool    `json:"use_proxy" dc:"启用代理"`
	DebugLogEnabled          *bool    `json:"debug_log_enabled" dc:"启用渠道调试日志（记录四段完整报文，排障用，用完及时关闭）"`
	DebugLogTenantID         *int64   `json:"debug_log_tenant_id" dc:"调试目标租户ID过滤（0=不限）"`
	DebugLogUserID           *int64   `json:"debug_log_user_id" dc:"调试目标成员ID过滤（0=不限）"`
	DebugLogApiKeyID         *int64   `json:"debug_log_api_key_id" dc:"调试目标密钥ID过滤（0=不限）"`
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
	MaxConcurrency           int      `json:"max_concurrency"`
	Tier                     string   `json:"tier"`
	StrictCapacity           bool     `json:"strict_capacity"`
	TestModel                string   `json:"test_model"`
	Remark                   string   `json:"remark"`
	IsVIP                    bool     `json:"is_vip"`
	UseProxy                 bool     `json:"use_proxy"`
	DebugLogEnabled          bool     `json:"debug_log_enabled"`
	DebugLogTenantID         int64    `json:"debug_log_tenant_id"`
	DebugLogUserID           int64    `json:"debug_log_user_id"`
	DebugLogApiKeyID         int64    `json:"debug_log_api_key_id"`
	SharingThreshold         *float64 `json:"sharing_threshold"`
	PreemptionThreshold      *float64 `json:"preemption_threshold"`
	BorrowingCooldownSeconds *int     `json:"borrowing_cooldown_seconds"`
	CreatedAt                string   `json:"created_at"`
	UpdatedAt                string   `json:"updated_at"`
	HealthScore              *float64 `json:"health_score"`
	BreakerState             int      `json:"breaker_state"`  // 调度熔断状态：0=正常 1=熔断中 2=半开探活
	BreakerModels            int      `json:"breaker_models"` // 处于熔断/半开的模型数量（渠道×模型级汇总）
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
	ID                int64   `json:"id"`
	ModelName         string  `json:"model_name" v:"required" dc:"平台标准模型名"`
	UpstreamModel     string  `json:"upstream_model" dc:"上游实际模型名"`
	Enabled           bool    `json:"enabled" d:"true" dc:"是否启用"`
	CostRatio         float64 `json:"cost_ratio" d:"1" v:"between:0,100" dc:"成本比例：上游实际价/平台基准价，1.0=等价（参与调度 costFactor）"`
	SupportsResponses bool    `json:"supports_responses" d:"false" dc:"支持 OpenAI Responses 协议（/v1/responses 原生直连，responses 入站软偏好）"`
	ChatViaResponses  bool    `json:"chat_via_responses" d:"false" dc:"上游仅有 Responses 协议（responses-only），chat 入站经桥接发送 /v1/responses"`
	// 模型级健康状态（从 Redis 实时读取）
	HealthScore  *float64 `json:"health_score" dc:"健康分 0-100，null=无数据"`
	SuccEWMA     *float64 `json:"succ_ewma" dc:"成功率 EWMA 0-1"`
	LatencyEWMA  *float64 `json:"latency_ewma" dc:"延迟 EWMA (ms)"`
	BreakerState *int     `json:"breaker_state" dc:"熔断状态：0=正常 1=熔断 2=半开，null=无熔断器"`
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
	ID                 int64  `json:"id"`
	Name               string `json:"name"`
	ApiKey             string `json:"api_key"`
	Status             string `json:"status"`
	KeyType            string `json:"key_type"`
	TokenExpiresAt     string `json:"token_expires_at"`
	LastError          string `json:"last_error" dc:"最后一次错误信息（凭证类错误带 [凭证错误 时间] 前缀）"`
	CooldownRemainingS int    `json:"cooldown_remaining_s" dc:"凭证冷却剩余秒数（401/403 后调度器冷却该 Key），0 = 未在冷却"`
	CreatedAt          string `json:"created_at"`
}

// ChannelAbilitiesGetReq 获取渠道模型能力请求
type ChannelAbilitiesGetReq struct {
	g.Meta    `path:"/channels/{channel_id}/abilities" method:"get" mime:"json" tags:"管理后台-渠道" summary:"获取渠道模型能力"`
	ChannelID int64 `json:"channel_id" in:"path" v:"required" dc:"渠道ID"`
}

type ChannelAbilitiesGetRes struct {
	List []AbilityItem `json:"list"`
	// 全部 active Key 均在凭证冷却中：该渠道当前不可被调度（调度器 pickKey 找不到可用凭证），
	// 但凭证类错误不进健康 EWMA/熔断体系，各模型健康分/熔断仍显示正常——前端需叠加此标记展示
	CredentialAllCooled bool `json:"credential_all_cooled" dc:"全部 active Key 凭证冷却中（渠道当前不可调度）"`
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

// HealthTrendPoint 健康趋势数据点（健康度、延迟取整展示，不含小数）
type HealthTrendPoint struct {
	SnapshotAt          string  `json:"snapshot_at"`
	HealthScore         int64   `json:"health_score"`
	SuccessRate         float64 `json:"success_rate"`
	LatencyMs           int64   `json:"latency_ms"`
	StabilityScore      float64 `json:"stability_score"`
	ConsecutiveFailures int     `json:"consecutive_failures"`
}

// ChannelExportReq 导出渠道列表请求
type ChannelExportReq struct {
	g.Meta  `path:"/channels/export" method:"get" mime:"json" tags:"管理后台-渠道" summary:"导出渠道列表"`
	Format  string `json:"format" in:"query" d:"csv" v:"in:csv,xlsx" dc:"导出格式：csv / xlsx"`
	Type    int    `json:"type" in:"query" dc:"供应商类型筛选"`
	Status  string `json:"status" in:"query" dc:"状态筛选：active/disabled/testing"`
	Search  string `json:"search" in:"query" dc:"搜索关键词（渠道名称/备注）"`
	ID      int64  `json:"id" in:"query" dc:"按渠道 ID 精确筛选"`
	Model   string `json:"model" in:"query" dc:"按支持的模型名筛选（模糊匹配平台模型名/上游模型名）"`
	BaseURL string `json:"base_url" in:"query" dc:"按 API 地址筛选（模糊匹配）"`
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

// ChannelResetHealthReq 重置渠道健康度请求
type ChannelResetHealthReq struct {
	g.Meta    `path:"/channels/{id}/reset-health" method:"post" mime:"json" tags:"管理后台-渠道" summary:"重置渠道健康度"`
	ID        int64  `json:"id" in:"path" v:"required" dc:"渠道ID"`
	ModelName string `json:"model_name" dc:"可选：指定模型名则只重置该模型的健康度，留空则重置整个渠道（含所有模型）"`
}

// ChannelResetHealthRes 重置渠道健康度响应
type ChannelResetHealthRes struct{}

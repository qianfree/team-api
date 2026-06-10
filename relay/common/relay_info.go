package common

import (
	"context"
	"net/http"
	"time"

	"github.com/qianfree/team-api/relay/constant"
)

// ChannelMeta 渠道元信息，由调度器填充
type ChannelMeta struct {
	ChannelID         int64
	ChannelType       int // ProviderType 常量
	ChannelName       string
	BaseURL           string
	ApiKey            string // 解密后的上游 API Key
	UpstreamModelName string // 上游实际模型名（可能与用户请求不同）
	IsModelMapped     bool   // 是否经过模型名映射
	Settings          ChannelSettings
}

const (
	DefaultTimeoutSeconds       = 60
	ImagesGenerationTimeoutSecs = 600
)

// GetTimeoutSeconds 返回请求超时秒数。
// 渠道自定义优先，未配置时根据请求模式返回默认值（图片生成 600s，其余 60s）。
func (s ChannelSettings) GetTimeoutSeconds(relayMode int) int {
	if s.TimeoutSeconds > 0 {
		return s.TimeoutSeconds
	}
	if constant.RelayMode(relayMode) == constant.RelayModeImagesGenerations {
		return ImagesGenerationTimeoutSecs
	}
	return DefaultTimeoutSeconds
}

// ChannelSettings 渠道配置（来自 chn_channels.settings JSONB）
type ChannelSettings struct {
	TimeoutSeconds              int            `json:"timeout_seconds"`                          // 请求超时秒数，默认 60
	RetryCount                  int            `json:"retry_count"`                              // 重试次数，默认 1
	ParamOverride               map[string]any `json:"param_override,omitempty"`                 // 请求体改写规则
	HeaderOverride              map[string]any `json:"header_override,omitempty"`                // Header 改写规则
	ChatCompletionsViaResponses bool           `json:"chat_completions_via_responses,omitempty"` // Chat Completions → Responses API 桥接
	PassThroughBodyEnabled      bool           `json:"pass_through_body_enabled,omitempty"`      // 直连转发：跳过协议转换，原始请求体直接转发上游

	// System Prompt 注入
	SystemPrompt         string `json:"system_prompt,omitempty"`          // 渠道级系统提示词
	SystemPromptOverride bool   `json:"system_prompt_override,omitempty"` // true=替换已有 system message，false=追加

	// 状态码重映射（JSON 格式如 {"429": 500, "403": 500}）
	StatusCodeMapping string `json:"status_code_mapping,omitempty"`

	// 字段清理（安全与成本控制）
	AllowServiceTier      bool `json:"allow_service_tier,omitempty"`      // 允许 service_tier 字段（默认移除，避免额外费用）
	AllowInferenceGeo     bool `json:"allow_inference_geo,omitempty"`     // 允许 inference_geo 字段（Claude 数据驻留）
	AllowSpeed            bool `json:"allow_speed,omitempty"`             // 允许 speed 字段（Claude 推理速度模式）
	DisableStore          bool `json:"disable_store,omitempty"`           // 禁用 store 字段（隐私保护）
	AllowSafetyIdentifier bool `json:"allow_safety_identifier,omitempty"` // 允许 safety_identifier 字段（默认移除）

	// UseProxy 启用代理，使用系统配置的代理地址转发请求
	UseProxy bool `json:"use_proxy,omitempty"`
}

// RelayInfo 代理请求上下文，贯穿整个 relay 请求链路
type RelayInfo struct {
	Context context.Context

	// 认证信息（由 API Key 中间件设置）
	TenantID  int64
	UserID    int64
	ApiKeyID  int64
	ProjectID int64 // 通过 API Key 关联的项目 ID

	// 请求元信息
	RequestID       string
	RelayMode       int // RelayMode 常量
	IsStream        bool
	OriginModelName string // 用户请求的模型名
	RequestURLPath  string
	RequestHeaders  http.Header
	StartTime       time.Time

	// 渠道信息（由调度器设置）
	ChannelMeta *ChannelMeta

	// 重试状态
	RetryIndex int
	LastError  error

	// 响应追踪
	StreamStatus      *StreamStatus
	FirstResponseTime time.Time

	// 入站格式：openai / claude / gemini / responses
	// 决定适配器是否需要做格式转换
	InboundFormat constant.RelayFormat

	// ClientFormat 客户端原始请求格式（在格式转换前保存）
	ClientFormat constant.RelayFormat

	// RuntimeHeadersOverride 由 ParamOverride 中的 set_header/delete_header 操作
	// 动态产生的 header 覆盖，优先级高于 ChannelSettings.HeaderOverride。
	RuntimeHeadersOverride map[string]string

	// UseResponsesAPI 桥接标志：客户端发送 Chat Completions，但请求应通过 Responses API 发送到上游
	UseResponsesAPI bool

	// Thinking 后缀路由（从模型名解析，供适配器消费）
	ThinkingEnabled  bool   // 是否有 -thinking 后缀
	ThinkingDisabled bool   // 是否有 -nothinking 后缀
	ReasoningEffort  string // effort 级别：low/medium/high/xhigh/max/minimal
	BaseModelName    string // 去除 thinking/effort 后缀的基础模型名

	// WebSocket 连接（仅 Realtime 模式使用）
	ClientConn interface{} // *websocket.Conn — 使用 interface{} 避免 relay 层直接依赖 gorilla/websocket
	TargetConn interface{} // *websocket.Conn — 上游 WebSocket 连接
}

// GetOriginalClientFormat 返回客户端原始请求格式
func (info *RelayInfo) GetOriginalClientFormat() constant.RelayFormat {
	if info.ClientFormat != "" {
		return info.ClientFormat
	}
	return info.InboundFormat
}

// SetFirstResponseTime 记录首次响应时间
func (info *RelayInfo) SetFirstResponseTime() {
	if info.FirstResponseTime.IsZero() {
		info.FirstResponseTime = time.Now()
	}
}

// LatencyMs 返回首字节延迟（毫秒）
func (info *RelayInfo) LatencyMs() float64 {
	if info.FirstResponseTime.IsZero() {
		return 0
	}
	return float64(info.FirstResponseTime.Sub(info.StartTime).Milliseconds())
}

// TotalLatencyMs 返回总延迟（毫秒）
func (info *RelayInfo) TotalLatencyMs() float64 {
	return float64(time.Since(info.StartTime).Milliseconds())
}

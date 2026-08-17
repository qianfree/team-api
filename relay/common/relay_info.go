package common

import (
	"context"
	"net/http"
	"time"

	"github.com/qianfree/team-api/relay/constant"
	"github.com/qianfree/team-api/relay/dto"
	"github.com/qianfree/team-api/relaykit/relayconvert/convmeta"
	"github.com/qianfree/team-api/relaykit/types"
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

	// 协议能力（chn_abilities 渠道×模型级）：
	// SupportsResponses：模型支持 /v1/responses，responses 入站原样直连（不做 chat 转换）；
	// ChatViaResponses：responses-only 上游，chat 入站经桥接转换后发送 /v1/responses。
	// ChatViaResponses 隐含上游原生说 Responses 协议（UpstreamSpeaksResponses）。
	SupportsResponses bool
	ChatViaResponses  bool
}

// UpstreamSpeaksResponses 上游（针对当前模型）是否原生支持 Responses 协议：
// supports_responses 或 chat_via_responses 任一为真即成立——后者定义上就是
// "上游只有 /v1/responses"，responses 入站在其上同样应直连而非转 chat。
func (m *ChannelMeta) UpstreamSpeaksResponses() bool {
	return m != nil && (m.SupportsResponses || m.ChatViaResponses)
}

const (
	// DefaultTimeoutSeconds 非流式请求的默认总超时（http.Client.Timeout）。
	// 推理模型（GLM-4.5/4.6 thinking、Claude thinking 等）非流式调用需跑完整段生成才回响应头，
	// 60s 会误杀；放宽到 180s。个别仍超时的模型建议在渠道级 settings.timeout_seconds 配 >180（自动切 longRun 传输层）。
	DefaultTimeoutSeconds       = 180
	ImagesGenerationTimeoutSecs = 600
)

// GetTimeoutSeconds 返回请求超时秒数。
// 图片生成模式强制最低 600s（即使渠道配置了更短的自定义超时），其余模式渠道自定义优先。
func (s ChannelSettings) GetTimeoutSeconds(relayMode int) int {
	if constant.RelayMode(relayMode) == constant.RelayModeImagesGenerations {
		if s.TimeoutSeconds > ImagesGenerationTimeoutSecs {
			return s.TimeoutSeconds
		}
		return ImagesGenerationTimeoutSecs
	}
	if s.TimeoutSeconds > 0 {
		return s.TimeoutSeconds
	}
	return DefaultTimeoutSeconds
}

// ChannelSettings 渠道配置（来自 chn_channels.settings JSONB）
type ChannelSettings struct {
	TimeoutSeconds         int            `json:"timeout_seconds"`                     // 请求超时秒数，默认 60
	RetryCount             int            `json:"retry_count"`                         // 重试次数，默认 1
	ParamOverride          map[string]any `json:"param_override,omitempty"`            // 请求体改写规则
	HeaderOverride         map[string]any `json:"header_override,omitempty"`           // Header 改写规则
	PassThroughBodyEnabled bool           `json:"pass_through_body_enabled,omitempty"` // 直连转发：跳过协议转换，原始请求体直接转发上游

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

	// ResponsesRequest responses 入站请求快照（ConvertResponsesToOpenAI 解析后 stash），
	// 供 chat 上游响应合成回 Responses 格式时 echo temperature/top_p/max_output_tokens/instructions。
	// 直连（原生 Responses 上游）路径不填充，合成时保持默认值。
	ResponsesRequest *dto.OpenAIResponsesRequest

	// Thinking 后缀路由（从模型名解析，供适配器消费）
	ThinkingEnabled  bool   // 是否有 -thinking 后缀
	ThinkingDisabled bool   // 是否有 -nothinking 后缀
	ReasoningEffort  string // effort 级别：low/medium/high/xhigh/max/minimal
	BaseModelName    string // 去除 thinking/effort 后缀的基础模型名

	// WebSocket 连接（仅 Realtime 模式使用）
	ClientConn interface{} // *websocket.Conn — 使用 interface{} 避免 relay 层直接依赖 gorilla/websocket
	TargetConn interface{} // *websocket.Conn — 上游 WebSocket 连接

	// relaykit Meta 接口所需字段
	estimatePromptTokens int
	claudeConvertInfo    *convmeta.ClaudeConvertInfo
	sendResponseCount    int
	conversionChain      []types.RelayFormat
	convOptions          *convmeta.Options
}

// ResponsesRequestSnapshot 提供 Responses 入站请求快照（relaykit Claude→Responses
// 响应转换器经 responsesEchoProvider 接口读取，用于响应对象 echo 请求参数）。
func (r *RelayInfo) ResponsesRequestSnapshot() *dto.OpenAIResponsesRequest {
	if r == nil {
		return nil
	}
	return r.ResponsesRequest
}

// ModelNameMapped 提供渠道模型映射标志（relaykit 转换器经可选能力接口读取，
// 决定响应模型名的选取口径）。方法名避开与 ChannelMeta.IsModelMapped 字段撞名。
func (r *RelayInfo) ModelNameMapped() bool {
	if r == nil || r.ChannelMeta == nil {
		return false
	}
	return r.ChannelMeta.IsModelMapped
}

// GetRequestID 提供请求 ID（relaykit 转换器经可选能力接口读取，用于合成响应 ID）。
func (r *RelayInfo) GetRequestID() string {
	if r == nil {
		return ""
	}
	return r.RequestID
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

// ============================================================================
// convmeta.Meta 接口实现（供 relaykit 转换器使用）
// ============================================================================

var _ convmeta.Meta = (*RelayInfo)(nil)

func (info *RelayInfo) GetOriginModelName() string {
	if info == nil {
		return ""
	}
	return info.OriginModelName
}

func (info *RelayInfo) GetUpstreamModelName() string {
	if info == nil || info.ChannelMeta == nil {
		return ""
	}
	return info.ChannelMeta.UpstreamModelName
}

func (info *RelayInfo) HasChannelMeta() bool {
	return info != nil && info.ChannelMeta != nil
}

func (info *RelayInfo) GetChannelID() int {
	if info == nil || info.ChannelMeta == nil {
		return 0
	}
	return int(info.ChannelMeta.ChannelID)
}

func (info *RelayInfo) GetChannelType() int {
	if info == nil || info.ChannelMeta == nil {
		return 0
	}
	return info.ChannelMeta.ChannelType
}

func (info *RelayInfo) GetIsStream() bool {
	if info == nil {
		return false
	}
	return info.IsStream
}

func (info *RelayInfo) GetReasoningEffort() string {
	if info == nil {
		return ""
	}
	return info.ReasoningEffort
}

func (info *RelayInfo) SetReasoningEffort(effort string) {
	if info == nil {
		return
	}
	info.ReasoningEffort = effort
}

func (info *RelayInfo) GetEstimatePromptTokens() int {
	if info == nil {
		return 0
	}
	return info.estimatePromptTokens
}

// SetEstimatePromptTokens 记录请求侧估算的输入 token 数（与预扣估价同源），
// 供流中断结算时在上游 usage 缺失的情况下按估算值对输入正常计费。
func (info *RelayInfo) SetEstimatePromptTokens(tokens int) {
	if info == nil {
		return
	}
	info.estimatePromptTokens = tokens
}

func (info *RelayInfo) EnsureClaudeConvertInfo() *convmeta.ClaudeConvertInfo {
	if info == nil {
		return &convmeta.ClaudeConvertInfo{LastMessagesType: convmeta.LastMessageTypeNone}
	}
	if info.claudeConvertInfo == nil {
		info.claudeConvertInfo = &convmeta.ClaudeConvertInfo{LastMessagesType: convmeta.LastMessageTypeNone}
	}
	return info.claudeConvertInfo
}

func (info *RelayInfo) GetSendResponseCount() int {
	if info == nil {
		return 0
	}
	return info.sendResponseCount
}

func (info *RelayInfo) IncrSendResponseCount() {
	if info == nil {
		return
	}
	info.sendResponseCount++
}

func (info *RelayInfo) AppendRequestConversion(format types.RelayFormat) {
	if info == nil || format == "" {
		return
	}
	if n := len(info.conversionChain); n > 0 && info.conversionChain[n-1] == format {
		return
	}
	info.conversionChain = append(info.conversionChain, format)
}

func (info *RelayInfo) ConvOptions() *convmeta.Options {
	if info == nil {
		return &convmeta.Options{}
	}
	if info.convOptions == nil {
		info.convOptions = info.buildConvOptions()
	}
	return info.convOptions
}

// buildConvOptions 从配置系统构建转换选项快照
func (info *RelayInfo) buildConvOptions() *convmeta.Options {
	opts := &convmeta.Options{
		Claude: convmeta.ClaudeOptions{
			ThinkingAdapterEnabled:                true, // TODO: 从配置读取
			ThinkingAdapterBudgetTokensPercentage: 0.5,  // TODO: 从配置读取
			DefaultMaxTokens:                      defaultMaxTokensForClaude,
		},
		Gemini: convmeta.GeminiOptions{
			ThinkingAdapterEnabled:                true, // TODO: 从配置读取
			ThinkingAdapterBudgetTokensPercentage: 0.5,  // TODO: 从配置读取
			FunctionCallThoughtSignatureEnabled:   true, // TODO: 从配置读取
			SupportsImagine:                       supportsImagineModel,
			SafetySetting:                         nil, // TODO: 从配置读取
		},
		OpenRouterDialect:      info.ChannelMeta != nil && info.ChannelMeta.ChannelType == int(constant.ProviderOpenRouter),
		PreserveThinkingSuffix: nil, // TODO: 实现黑名单检查
	}
	return opts
}

// defaultMaxTokensForClaude 返回 Claude 模型的默认 max_tokens
func defaultMaxTokensForClaude(modelName string) int {
	// 根据模型返回合理的默认值
	// Claude 3.5 Sonnet 和 Opus 支持更大的输出
	switch {
	case contains(modelName, "claude-3-5-sonnet"):
		return 8192
	case contains(modelName, "claude-3-opus"):
		return 4096
	case contains(modelName, "claude-3-sonnet"):
		return 4096
	case contains(modelName, "claude-3-haiku"):
		return 4096
	default:
		return 4096
	}
}

// supportsImagineModel 检查 Gemini 模型是否支持图像生成
func supportsImagineModel(modelName string) bool {
	return contains(modelName, "imagine")
}

// contains 检查字符串是否包含子串（简单辅助函数）
func contains(s, substr string) bool {
	return len(s) >= len(substr) && indexOf(s, substr) >= 0
}

func indexOf(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

package common

import (
	"context"
	"errors"
	"time"
)

// ErrChannelUnavailable 没有可用渠道
var ErrChannelUnavailable = errors.New("no available channel")

// ErrModelNotFound 模型不存在
var ErrModelNotFound = errors.New("model not found")

// ErrStreamInterrupted 流式响应被客户端中断
var ErrStreamInterrupted = errors.New("stream interrupted by client disconnect")

// ErrTenantModelNotEnabled 租户未启用该模型
var ErrTenantModelNotEnabled = errors.New("model not enabled for this tenant")

// DeprecationInfo 模型弃用信息
type DeprecationInfo struct {
	Deprecated       bool
	SunsetDate       string // 格式: "2006-01-02"，空表示未设置
	ReplacementModel string
}

// DataProvider 是 relay 层访问业务数据的接口。
// 实现在 internal/logic/relay/provider.go 中，使用 GoFrame ORM。
// relay/ 层通过此接口获取数据，不直接依赖 GoFrame。
type DataProvider interface {
	// ValidateApiKey 验证 API Key 并返回认证信息。
	// rawKey 是完整的 API Key 原值（如 sk-a1b2c3d4e5f6...）。
	ValidateApiKey(ctx context.Context, rawKey string) (*ApiKeyInfo, error)

	// GetChannelForModel 为指定模型选择最佳渠道。
	// tenantID 和 userID 用于亲和性计算。
	// excludeChannelIDs 是本次请求已尝试过且失败的渠道，用于重试时排除。
	GetChannelForModel(ctx context.Context, tenantID, userID int64, modelName string, excludeChannelIDs []int64) (*ChannelSelection, error)

	// GetModelMapping 获取模型映射信息。
	// 返回标准模型名和分类（chat/embedding/image 等）。
	GetModelMapping(ctx context.Context, modelName string) (standardName string, category string, err error)

	// RecordUsage 异步记录 API 调用用量日志。
	RecordUsage(ctx context.Context, record *UsageRecord)

	// RecordAudit 异步记录请求审计日志。
	RecordAudit(ctx context.Context, record *AuditRecord)

	// UpdateTaskAudit 更新异步任务的审计记录（任务完成时调用）。
	// 通过 task_id 查找提交阶段写入的审计记录，补充最终结果。
	UpdateTaskAudit(ctx context.Context, record *AuditRecord)

	// UpdateChannelHealth 更新渠道健康度（请求成功/失败后调用）。
	UpdateChannelHealth(ctx context.Context, channelID int64, success bool, latencyMs float64)

	// IncrementConsecutiveFailure 递增渠道连续失败计数。
	IncrementConsecutiveFailure(ctx context.Context, channelID int64)

	// ResetConsecutiveFailure 重置渠道连续失败计数为 0。
	ResetConsecutiveFailure(ctx context.Context, channelID int64)

	// GetAvailableModels 获取指定租户可用的模型列表。
	// apiKeyID > 0 时进一步按 API Key 的模型范围过滤。
	GetAvailableModels(ctx context.Context, tenantID int64, apiKeyID int64) ([]ModelInfo, error)

	// GetModelDetail 获取单个模型的详细信息。
	// tenantID 用于权限校验（检查租户是否有权使用该模型）。
	GetModelDetail(ctx context.Context, tenantID int64, modelName string) (*ModelDetail, error)

	// CheckTenantModelAccess 检查租户是否有权使用指定模型。
	// 返回是否启用和渠道范围（nil/空表示不限渠道）。
	CheckTenantModelAccess(ctx context.Context, tenantID int64, modelName string) (enabled bool, channelScope []int64, err error)

	// GetModelDeprecationInfo 获取模型弃用信息。
	// 返回 nil 表示模型未弃用。
	GetModelDeprecationInfo(ctx context.Context, modelName string) (*DeprecationInfo, error)

	// InvalidateModelCache 清除指定模型的缓存。
	InvalidateModelCache(modelName string)

	// CheckMemberModelAccess 检查成员是否有权使用指定模型。
	// 空 scope（无记录）表示不限制。
	CheckMemberModelAccess(ctx context.Context, tenantID, userID int64, modelName string) (bool, error)

	// CheckApiKeyModelAccess 检查 API Key 是否有权使用指定模型。
	// 无 scope 记录表示不限制（向后兼容）。
	CheckApiKeyModelAccess(ctx context.Context, apiKeyID int64, modelName string) (bool, error)
}

// ApiKeyInfo API Key 验证结果
type ApiKeyInfo struct {
	ID        int64
	TenantID  int64
	UserID    int64
	ProjectID int64  // 项目密钥关联的项目 ID，个人密钥为 0
	Scope     string // full / chat_only / embeddings_only / images_only / read_only / custom
	Status    string // active / disabled / expired
}

// ChannelSelection 渠道选择结果
type ChannelSelection struct {
	ChannelID         int64
	ChannelType       int
	ChannelName       string
	BaseURL           string
	ApiKey            string // 解密后的上游 API Key
	UpstreamModelName string
	IsModelMapped     bool
	Settings          ChannelSettings
}

// UsageRecord API 调用用量记录
type UsageRecord struct {
	TenantID         int64
	UserID           int64
	ApiKeyID         int64
	ProjectID        int64 // 通过 API Key 关联的项目 ID
	ChannelID        int64
	ModelName        string
	RelayMode        int
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
	CachedTokens     int // 缓存读取 token 数
	AudioTokens      int // 音频 token 数（输入+输出）
	ImageTokens      int // 图像 token 数
	ReasoningTokens  int // 推理 token 数
	LatencyMs        float64
	IsStream         bool
	Success          bool
	RequestID        string
	Status           string // success / error / timeout / cancelled
	ErrorMessage     string

	// Cache token 细分
	CacheCreationTokens   int // 缓存创建 token（合并）
	CacheCreation5mTokens int // Claude 5分钟缓存创建
	CacheCreation1hTokens int // Claude 1小时缓存创建
	CacheReadTokens       int // 缓存读取 token

	// 音频 token 分离
	AudioInputTokens  int
	AudioOutputTokens int

	// 其他 token
	ImageOutputTokens int // 图像输出 token（DALL-E）

	// 请求元数据
	RequestedModel   string // 用户请求的模型名
	UpstreamModel    string // 上游实际模型名
	RequestType      int    // 1=sync, 2=stream, 3=async, 4=websocket
	UserAgent        string
	ClientIP         string
	FirstTokenMs     int
	ServiceTier      string
	ReasoningEffort  string
	InboundEndpoint  string // 客户端请求路径
	UpstreamEndpoint string // 上游实际请求路径

	// 渠道详情
	ChannelName string
	ChannelType int

	// 计费元数据
	BillingMode    string
	BillingSource  string
	RateMultiplier float64
	RetryIndex     int

	// 流式追踪
	StreamEndReason string

	// 图像生成
	ImageCount int
	ImageSize  string

	// 费用数据（由 relay handler 从结算结果填充）
	InputCost         float64
	OutputCost        float64
	CacheCreationCost float64
	CacheReadCost     float64
	TotalCost         float64
	ActualCost        float64
	Currency          string
	PreDeductAmount   float64
	RefundAmount      float64
	SupplementAmount  float64

	// 计费快照
	BillingSnapshot string // JSON 字符串
	BillingSummary  string // 人类可读中文文本

	// 异步任务关联
	TaskID string // 异步任务公开ID（task_xxxxx），普通请求为空

}

// AuditRecord 请求审计日志记录
type AuditRecord struct {
	TenantID        int64
	UserID          int64
	ApiKeyID        int64
	ProjectID       int64 // 通过 API Key 关联的项目 ID
	RequestID       string
	Method          string
	Path            string
	QueryParams     string
	StatusCode      int
	ClientIP        string
	UserAgent       string
	RequestBody     string
	ResponseBody    string
	LatencyMs       int
	FirstTokenMs    int
	IsStream        bool
	RequestHeaders  map[string]string // 请求头（仅审计级别为 all 时记录）
	ResponseHeaders map[string]string // 响应头（仅审计级别为 all 时记录）
	ForwardingTrace *ForwardingTrace  // 转发路径追踪（仅管理员可见）

	// 异步任务完成字段（仅 UpdateTaskAudit 时填充）
	TaskID              string            // 异步任务公开ID，关联 tsk_model_tasks.public_task_id
	TaskStatus          string            // SUCCESS / FAILURE
	TaskResult          string            // 上游最终响应体
	TaskUpstreamHeaders map[string]string // 上游最终轮询的响应头
	TaskCompletedAt     *time.Time        // 任务到达终态的时间
}

// ModelInfo 模型信息（用于 /v1/models 端点）
type ModelInfo struct {
	ModelId          string          `json:"model_id"`
	ModelName        string          `json:"model_name"`
	Category         string          `json:"category"`
	Status           string          `json:"status"`
	MaxContextTokens int             `json:"max_context_tokens"`
	MaxOutputTokens  int             `json:"max_output_tokens"`
	Capabilities     map[string]bool `json:"capabilities"`
}

// ModelDetail 模型详细信息（用于 /v1/models/{model_id} 端点）
type ModelDetail struct {
	ID               string          `json:"id"`
	Object           string          `json:"object"`
	Created          int64           `json:"created"`
	OwnedBy          string          `json:"owned_by"`
	ModelName        string          `json:"model_name,omitempty"`
	Category         string          `json:"category,omitempty"`
	Status           string          `json:"status,omitempty"`
	MaxContextTokens int             `json:"max_context_tokens,omitempty"`
	MaxOutputTokens  int             `json:"max_output_tokens,omitempty"`
	Description      string          `json:"description,omitempty"`
	Capabilities     map[string]bool `json:"capabilities,omitempty"`
}

// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// BilUsageLogsDao is the data access object for the table bil_usage_logs.
type BilUsageLogsDao struct {
	table    string              // table is the underlying table name of the DAO.
	group    string              // group is the database configuration group name of the current DAO.
	columns  BilUsageLogsColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler  // handlers for customized model modification.
}

// BilUsageLogsColumns defines and stores column names for the table bil_usage_logs.
type BilUsageLogsColumns struct {
	Id                    string // 主键ID
	TenantId              string // 租户ID
	UserId                string // 用户ID
	ApiKeyId              string // 使用的 API Key ID
	ChannelId             string // 使用的渠道ID
	ModelName             string // 调用的模型名
	RequestId             string // 请求唯一ID
	RelayMode             string // 代理模式：chat_completions / embeddings / images_generations 等
	InputTokens           string // 输入 token 数
	OutputTokens          string // 输出 token 数
	TotalCost             string // 本次调用费用
	Currency              string // 货币（USD）
	LatencyMs             string // 请求延迟（毫秒）
	Status                string // 状态：success（成功）/ error（错误）/ timeout（超时）/ cancelled（取消）
	ErrorMessage          string // 错误信息（成功时为 NULL）
	ClientIp              string // 客户端 IP
	CreatedAt             string // 创建时间
	UpdatedAt             string // 更新时间
	CacheCreationTokens   string // 缓存创建 token 数 (Claude)
	CacheReadTokens       string // 缓存读取 token 数 (Claude/OpenAI)
	InputCost             string // 输入 token 费用
	OutputCost            string // 输出 token 费用
	CacheCreationCost     string // 缓存创建费用
	CacheReadCost         string // 缓存读取费用
	ActualCost            string // 实际扣除费用（含折扣后）
	RequestedModel        string // 用户请求的模型名
	UpstreamModel         string // 上游实际模型名（模型映射后）
	RequestType           string // 请求类型: 1=sync, 2=stream, 3=async, 4=websocket
	UserAgent             string // 客户端 User-Agent
	FirstTokenMs          string // 首 token 延迟（毫秒）
	ServiceTier           string // 服务等级 (default/flex等)
	ReasoningEffort       string // 推理力度 (low/medium/high)
	ChannelName           string // 渠道名称
	ChannelType           string // 渠道类型 (ProviderType)
	BillingMode           string // 计费模式 (token/per_request/tiered)
	BillingSource         string // 定价来源 (base/tenant/custom)
	RateMultiplier        string // 费率倍率/折扣
	PreDeductAmount       string // 预扣金额
	RefundAmount          string // 退款金额
	SupplementAmount      string // 补扣金额
	ImageCount            string // 生成图片数量
	ImageSize             string // 图片尺寸
	StreamEndReason       string // 流结束原因 (done/timeout/client_gone/error/panic)
	RetryIndex            string // 重试次数（0=首次成功）
	BillingSummary        string // 计费快照文本（人类可读的计费过程描述）
	CacheCreation5MTokens string // Claude 5分钟缓存创建 token 数
	CacheCreation1HTokens string // Claude 1小时缓存创建 token 数
	AudioInputTokens      string // 音频输入 token 数
	AudioOutputTokens     string // 音频输出 token 数
	ImageOutputTokens     string // 图像输出 token 数（DALL-E 等）
	ReasoningTokens       string // 推理 token 数（O1/o3 等）
	AccountCost           string // 上游账户成本（用于利润分析）
	InboundEndpoint       string // 客户端请求路径（如 /v1/chat/completions）
	UpstreamEndpoint      string // 上游实际请求路径
	BillingSnapshot       string // 完整计费计算过程快照（JSONB）
	ProjectId             string // 关联项目ID（通过API Key关联，NULL表示个人密钥无项目）
	TaskId                string // 异步任务公开ID（task_xxxxx），关联 tsk_model_tasks.public_task_id，普通请求为空
}

// bilUsageLogsColumns holds the columns for the table bil_usage_logs.
var bilUsageLogsColumns = BilUsageLogsColumns{
	Id:                    "id",
	TenantId:              "tenant_id",
	UserId:                "user_id",
	ApiKeyId:              "api_key_id",
	ChannelId:             "channel_id",
	ModelName:             "model_name",
	RequestId:             "request_id",
	RelayMode:             "relay_mode",
	InputTokens:           "input_tokens",
	OutputTokens:          "output_tokens",
	TotalCost:             "total_cost",
	Currency:              "currency",
	LatencyMs:             "latency_ms",
	Status:                "status",
	ErrorMessage:          "error_message",
	ClientIp:              "client_ip",
	CreatedAt:             "created_at",
	UpdatedAt:             "updated_at",
	CacheCreationTokens:   "cache_creation_tokens",
	CacheReadTokens:       "cache_read_tokens",
	InputCost:             "input_cost",
	OutputCost:            "output_cost",
	CacheCreationCost:     "cache_creation_cost",
	CacheReadCost:         "cache_read_cost",
	ActualCost:            "actual_cost",
	RequestedModel:        "requested_model",
	UpstreamModel:         "upstream_model",
	RequestType:           "request_type",
	UserAgent:             "user_agent",
	FirstTokenMs:          "first_token_ms",
	ServiceTier:           "service_tier",
	ReasoningEffort:       "reasoning_effort",
	ChannelName:           "channel_name",
	ChannelType:           "channel_type",
	BillingMode:           "billing_mode",
	BillingSource:         "billing_source",
	RateMultiplier:        "rate_multiplier",
	PreDeductAmount:       "pre_deduct_amount",
	RefundAmount:          "refund_amount",
	SupplementAmount:      "supplement_amount",
	ImageCount:            "image_count",
	ImageSize:             "image_size",
	StreamEndReason:       "stream_end_reason",
	RetryIndex:            "retry_index",
	BillingSummary:        "billing_summary",
	CacheCreation5MTokens: "cache_creation_5m_tokens",
	CacheCreation1HTokens: "cache_creation_1h_tokens",
	AudioInputTokens:      "audio_input_tokens",
	AudioOutputTokens:     "audio_output_tokens",
	ImageOutputTokens:     "image_output_tokens",
	ReasoningTokens:       "reasoning_tokens",
	AccountCost:           "account_cost",
	InboundEndpoint:       "inbound_endpoint",
	UpstreamEndpoint:      "upstream_endpoint",
	BillingSnapshot:       "billing_snapshot",
	ProjectId:             "project_id",
	TaskId:                "task_id",
}

// NewBilUsageLogsDao creates and returns a new DAO object for table data access.
func NewBilUsageLogsDao(handlers ...gdb.ModelHandler) *BilUsageLogsDao {
	return &BilUsageLogsDao{
		group:    "default",
		table:    "bil_usage_logs",
		columns:  bilUsageLogsColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *BilUsageLogsDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *BilUsageLogsDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *BilUsageLogsDao) Columns() BilUsageLogsColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *BilUsageLogsDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *BilUsageLogsDao) Ctx(ctx context.Context) *gdb.Model {
	model := dao.DB().Model(dao.table)
	for _, handler := range dao.handlers {
		model = handler(model)
	}
	return model.Safe().Ctx(ctx)
}

// Transaction wraps the transaction logic using function f.
// It rolls back the transaction and returns the error if function f returns a non-nil error.
// It commits the transaction and returns nil if function f returns nil.
//
// Note: Do not commit or roll back the transaction in function f,
// as it is automatically handled by this function.
func (dao *BilUsageLogsDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}

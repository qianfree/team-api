// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"github.com/gogf/gf/v2/os/gtime"
	"github.com/shopspring/decimal"
)

// BilUsageLogs is the golang structure for table bil_usage_logs.
type BilUsageLogs struct {
	Id                    int64           `json:"id"                       orm:"id"                       description:"主键ID"`                                                          // 主键ID
	TenantId              int64           `json:"tenant_id"                orm:"tenant_id"                description:"租户ID"`                                                          // 租户ID
	UserId                int64           `json:"user_id"                  orm:"user_id"                  description:"用户ID"`                                                          // 用户ID
	ApiKeyId              int64           `json:"api_key_id"               orm:"api_key_id"               description:"使用的 API Key ID"`                                                // 使用的 API Key ID
	ChannelId             int64           `json:"channel_id"               orm:"channel_id"               description:"使用的渠道ID"`                                                       // 使用的渠道ID
	ModelName             string          `json:"model_name"               orm:"model_name"               description:"调用的模型名"`                                                        // 调用的模型名
	RequestId             string          `json:"request_id"               orm:"request_id"               description:"请求唯一ID"`                                                        // 请求唯一ID
	RelayMode             string          `json:"relay_mode"               orm:"relay_mode"               description:"代理模式：chat_completions / embeddings / images_generations 等"`     // 代理模式：chat_completions / embeddings / images_generations 等
	InputTokens           int             `json:"input_tokens"             orm:"input_tokens"             description:"输入 token 数"`                                                    // 输入 token 数
	OutputTokens          int             `json:"output_tokens"            orm:"output_tokens"            description:"输出 token 数"`                                                    // 输出 token 数
	TotalCost             decimal.Decimal `json:"total_cost"               orm:"total_cost"               description:"本次调用费用"`                                                        // 本次调用费用
	Currency              string          `json:"currency"                 orm:"currency"                 description:"货币（USD）"`                                                       // 货币（USD）
	LatencyMs             int             `json:"latency_ms"               orm:"latency_ms"               description:"请求延迟（毫秒）"`                                                      // 请求延迟（毫秒）
	Status                string          `json:"status"                   orm:"status"                   description:"状态：success（成功）/ error（错误）/ timeout（超时）/ cancelled（取消）"`         // 状态：success（成功）/ error（错误）/ timeout（超时）/ cancelled（取消）
	ErrorMessage          string          `json:"error_message"            orm:"error_message"            description:"错误信息（成功时为 NULL）"`                                               // 错误信息（成功时为 NULL）
	ClientIp              string          `json:"client_ip"                orm:"client_ip"                description:"客户端 IP"`                                                        // 客户端 IP
	CreatedAt             *gtime.Time     `json:"created_at"               orm:"created_at"               description:"创建时间"`                                                          // 创建时间
	UpdatedAt             *gtime.Time     `json:"updated_at"               orm:"updated_at"               description:"更新时间"`                                                          // 更新时间
	CacheCreationTokens   int             `json:"cache_creation_tokens"    orm:"cache_creation_tokens"    description:"缓存创建 token 数 (Claude)"`                                         // 缓存创建 token 数 (Claude)
	CacheReadTokens       int             `json:"cache_read_tokens"        orm:"cache_read_tokens"        description:"缓存读取 token 数 (Claude/OpenAI)"`                                  // 缓存读取 token 数 (Claude/OpenAI)
	InputCost             decimal.Decimal `json:"input_cost"               orm:"input_cost"               description:"输入 token 费用"`                                                   // 输入 token 费用
	OutputCost            decimal.Decimal `json:"output_cost"              orm:"output_cost"              description:"输出 token 费用"`                                                   // 输出 token 费用
	CacheCreationCost     decimal.Decimal `json:"cache_creation_cost"      orm:"cache_creation_cost"      description:"缓存创建费用"`                                                        // 缓存创建费用
	CacheReadCost         decimal.Decimal `json:"cache_read_cost"          orm:"cache_read_cost"          description:"缓存读取费用"`                                                        // 缓存读取费用
	ActualCost            decimal.Decimal `json:"actual_cost"              orm:"actual_cost"              description:"实际扣除费用（含折扣后）"`                                                  // 实际扣除费用（含折扣后）
	RequestedModel        string          `json:"requested_model"          orm:"requested_model"          description:"用户请求的模型名"`                                                      // 用户请求的模型名
	UpstreamModel         string          `json:"upstream_model"           orm:"upstream_model"           description:"上游实际模型名（模型映射后）"`                                                // 上游实际模型名（模型映射后）
	RequestType           int             `json:"request_type"             orm:"request_type"             description:"请求类型: 1=sync, 2=stream, 3=async, 4=websocket"`                  // 请求类型: 1=sync, 2=stream, 3=async, 4=websocket
	UserAgent             string          `json:"user_agent"               orm:"user_agent"               description:"客户端 User-Agent"`                                                // 客户端 User-Agent
	FirstTokenMs          int             `json:"first_token_ms"           orm:"first_token_ms"           description:"首 token 延迟（毫秒）"`                                                // 首 token 延迟（毫秒）
	ServiceTier           string          `json:"service_tier"             orm:"service_tier"             description:"服务等级 (default/flex等)"`                                          // 服务等级 (default/flex等)
	ReasoningEffort       string          `json:"reasoning_effort"         orm:"reasoning_effort"         description:"推理力度 (low/medium/high)"`                                        // 推理力度 (low/medium/high)
	ChannelName           string          `json:"channel_name"             orm:"channel_name"             description:"渠道名称"`                                                          // 渠道名称
	ChannelType           int             `json:"channel_type"             orm:"channel_type"             description:"渠道类型 (ProviderType)"`                                           // 渠道类型 (ProviderType)
	BillingMode           string          `json:"billing_mode"             orm:"billing_mode"             description:"计费模式 (token/per_request/tiered)"`                               // 计费模式 (token/per_request/tiered)
	BillingSource         string          `json:"billing_source"           orm:"billing_source"           description:"定价来源 (base/tenant/custom)"`                                     // 定价来源 (base/tenant/custom)
	RateMultiplier        decimal.Decimal `json:"rate_multiplier"          orm:"rate_multiplier"          description:"费率倍率/折扣"`                                                       // 费率倍率/折扣
	PreDeductAmount       decimal.Decimal `json:"pre_deduct_amount"        orm:"pre_deduct_amount"        description:"预扣金额"`                                                          // 预扣金额
	RefundAmount          decimal.Decimal `json:"refund_amount"            orm:"refund_amount"            description:"退款金额"`                                                          // 退款金额
	SupplementAmount      decimal.Decimal `json:"supplement_amount"        orm:"supplement_amount"        description:"补扣金额"`                                                          // 补扣金额
	ImageCount            int             `json:"image_count"              orm:"image_count"              description:"生成图片数量"`                                                        // 生成图片数量
	ImageSize             string          `json:"image_size"               orm:"image_size"               description:"图片尺寸"`                                                          // 图片尺寸
	StreamEndReason       string          `json:"stream_end_reason"        orm:"stream_end_reason"        description:"流结束原因 (done/timeout/client_gone/error/panic)"`                  // 流结束原因 (done/timeout/client_gone/error/panic)
	RetryIndex            int             `json:"retry_index"              orm:"retry_index"              description:"重试次数（0=首次成功）"`                                                  // 重试次数（0=首次成功）
	BillingSummary        string          `json:"billing_summary"          orm:"billing_summary"          description:"计费快照文本（人类可读的计费过程描述）"`                                           // 计费快照文本（人类可读的计费过程描述）
	CacheCreation5MTokens int             `json:"cache_creation_5m_tokens" orm:"cache_creation_5m_tokens" description:"Claude 5分钟缓存创建 token 数"`                                        // Claude 5分钟缓存创建 token 数
	CacheCreation1HTokens int             `json:"cache_creation_1h_tokens" orm:"cache_creation_1h_tokens" description:"Claude 1小时缓存创建 token 数"`                                        // Claude 1小时缓存创建 token 数
	AudioInputTokens      int             `json:"audio_input_tokens"       orm:"audio_input_tokens"       description:"音频输入 token 数"`                                                  // 音频输入 token 数
	AudioOutputTokens     int             `json:"audio_output_tokens"      orm:"audio_output_tokens"      description:"音频输出 token 数"`                                                  // 音频输出 token 数
	ImageOutputTokens     int             `json:"image_output_tokens"      orm:"image_output_tokens"      description:"图像输出 token 数（DALL-E 等）"`                                        // 图像输出 token 数（DALL-E 等）
	ReasoningTokens       int             `json:"reasoning_tokens"         orm:"reasoning_tokens"         description:"推理 token 数（O1/o3 等）"`                                           // 推理 token 数（O1/o3 等）
	AccountCost           decimal.Decimal `json:"account_cost"             orm:"account_cost"             description:"上游账户成本（用于利润分析）"`                                                // 上游账户成本（用于利润分析）
	InboundEndpoint       string          `json:"inbound_endpoint"         orm:"inbound_endpoint"         description:"客户端请求路径（如 /v1/chat/completions）"`                               // 客户端请求路径（如 /v1/chat/completions）
	UpstreamEndpoint      string          `json:"upstream_endpoint"        orm:"upstream_endpoint"        description:"上游实际请求路径"`                                                      // 上游实际请求路径
	BillingSnapshot       string          `json:"billing_snapshot"         orm:"billing_snapshot"         description:"完整计费计算过程快照（JSONB）"`                                             // 完整计费计算过程快照（JSONB）
	ProjectId             int64           `json:"project_id"               orm:"project_id"               description:"关联项目ID（通过API Key关联，NULL表示个人密钥无项目）"`                             // 关联项目ID（通过API Key关联，NULL表示个人密钥无项目）
	TaskId                string          `json:"task_id"                  orm:"task_id"                  description:"异步任务公开ID（task_xxxxx），关联 tsk_model_tasks.public_task_id，普通请求为空"` // 异步任务公开ID（task_xxxxx），关联 tsk_model_tasks.public_task_id，普通请求为空
}

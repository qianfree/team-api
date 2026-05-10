// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
)

// BilUsageLogs is the golang structure of table bil_usage_logs for DAO operations like Where/Data.
type BilUsageLogs struct {
	g.Meta                `orm:"table:bil_usage_logs, do:true"`
	Id                    any         // 主键ID
	TenantId              any         // 租户ID
	UserId                any         // 用户ID
	ApiKeyId              any         // 使用的 API Key ID
	ChannelId             any         // 使用的渠道ID
	ModelName             any         // 调用的模型名
	RequestId             any         // 请求唯一ID
	RelayMode             any         // 代理模式：chat_completions / embeddings / images_generations 等
	InputTokens           any         // 输入 token 数
	OutputTokens          any         // 输出 token 数
	TotalCost             any         // 本次调用费用
	Currency              any         // 货币（USD）
	LatencyMs             any         // 请求延迟（毫秒）
	Status                any         // 状态：success（成功）/ error（错误）/ timeout（超时）/ cancelled（取消）
	ErrorMessage          any         // 错误信息（成功时为 NULL）
	ClientIp              any         // 客户端 IP
	CreatedAt             *gtime.Time // 创建时间
	UpdatedAt             *gtime.Time // 更新时间
	CacheCreationTokens   any         // 缓存创建 token 数 (Claude)
	CacheReadTokens       any         // 缓存读取 token 数 (Claude/OpenAI)
	InputCost             any         // 输入 token 费用
	OutputCost            any         // 输出 token 费用
	CacheCreationCost     any         // 缓存创建费用
	CacheReadCost         any         // 缓存读取费用
	ActualCost            any         // 实际扣除费用（含折扣后）
	RequestedModel        any         // 用户请求的模型名
	UpstreamModel         any         // 上游实际模型名（模型映射后）
	RequestType           any         // 请求类型: 1=sync, 2=stream, 3=websocket
	UserAgent             any         // 客户端 User-Agent
	FirstTokenMs          any         // 首 token 延迟（毫秒）
	ServiceTier           any         // 服务等级 (default/flex等)
	ReasoningEffort       any         // 推理力度 (low/medium/high)
	ChannelName           any         // 渠道名称
	ChannelType           any         // 渠道类型 (ProviderType)
	BillingMode           any         // 计费模式 (token/per_request/tiered)
	BillingSource         any         // 定价来源 (base/tenant/custom)
	RateMultiplier        any         // 费率倍率/折扣
	PreDeductAmount       any         // 预扣金额
	RefundAmount          any         // 退款金额
	SupplementAmount      any         // 补扣金额
	ImageCount            any         // 生成图片数量
	ImageSize             any         // 图片尺寸
	StreamEndReason       any         // 流结束原因 (done/timeout/client_gone/error/panic)
	RetryIndex            any         // 重试次数（0=首次成功）
	BillingSummary        any         // 计费快照文本（人类可读的计费过程描述）
	CacheCreation5MTokens any         // Claude 5分钟缓存创建 token 数
	CacheCreation1HTokens any         // Claude 1小时缓存创建 token 数
	AudioInputTokens      any         // 音频输入 token 数
	AudioOutputTokens     any         // 音频输出 token 数
	ImageOutputTokens     any         // 图像输出 token 数（DALL-E 等）
	ReasoningTokens       any         // 推理 token 数（O1/o3 等）
	AccountCost           any         // 上游账户成本（用于利润分析）
	InboundEndpoint       any         // 客户端请求路径（如 /v1/chat/completions）
	UpstreamEndpoint      any         // 上游实际请求路径
	BillingSnapshot       any         // 完整计费计算过程快照（JSONB）
	ProjectId             any         // 关联项目ID（通过API Key关联，NULL表示个人密钥无项目）
}

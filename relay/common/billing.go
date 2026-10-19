package common

import (
	"context"
	"errors"
	"math"
	"time"
)

// ErrModelPricingNotConfigured 模型未配置定价（fail-closed 哨兵错误）。
// billing 层预扣估价发现零价定价时返回，handler 层用 errors.Is 识别后
// 返回明确的请求错误，而非误导性的"余额不足"。
var ErrModelPricingNotConfigured = errors.New("model pricing not configured")

// MaxImageCount 图片协议顶层 n（一次生成张数）上限，与 OpenAI images API 的 1~10 一致。
// n 直接放大按次计费（单价 × n），超限值在入口直接 400 拒绝，防止异常大值冻结天价预扣。
const MaxImageCount = 10

// ErrInvalidImageCount 图片协议顶层 n 非法（非整数或超出 1~MaxImageCount）。
var ErrInvalidImageCount = errors.New("n must be an integer between 1 and 10")

// NormalizeImageCount 校验并归一化图片协议顶层 n：必须为 1~MaxImageCount 的整数
// （3.0 与 3 等价）。返回按次计费放大倍数：n>1 时为 n，n=1 时为 0（表示不放大）。
// 同步端点、同步图片异步化、阿里 taskchannel 三条入口共用，保证拒绝口径一致。
func NormalizeImageCount(n float64) (int, error) {
	if n < 1 || n > MaxImageCount || n != math.Trunc(n) {
		return 0, ErrInvalidImageCount
	}
	if n == 1 {
		return 0, nil
	}
	return int(n), nil
}

// BillingProvider 计费接口
// 实现在 internal/logic/billing/ 中，通过接口解耦 relay 层和 GoFrame
type BillingProvider interface {
	// PreDeduct 预扣费用
	// 检查余额 → 冻结预扣金额 → 返回是否成功
	// count 为按次计费的放大倍数（图片协议顶层 n，一次生成 n 张按 n 次计）；<=1 表示不放大
	PreDeduct(ctx context.Context, tenantID int64, modelName string, count, inputTokens, maxTokens int, isStream bool, requestID string) (preDeductAmount float64, err error)

	// Settle 结算费用（成功请求）。billAt 为请求受理时刻（时段定价按该时刻评估）
	Settle(ctx context.Context, tenantID, userID, apiKeyID, channelID int64, modelName, requestID, relayMode string, usage *Usage, preDeductAmount float64, projectID int64, billAt time.Time) (*SettlementResult, error)

	// SettleWithUsage 完整 Usage 结算（含 cache token + 计费快照）
	SettleWithUsage(ctx context.Context, tenantID, userID, apiKeyID, channelID int64,
		modelName, requestID, relayMode string,
		usage *Usage, preDeductAmount float64, relayInfo *RelayInfo) *SettlementResult

	// SettleFailed 失败请求结算（退还预扣）
	SettleFailed(ctx context.Context, tenantID int64, requestID string, preDeductAmount float64) error

	// SettleStreamInterrupted 流式中断结算。billAt 为请求受理时刻（时段定价按该时刻评估）。
	// imageCount 为按次计费张数（图片协议顶层 n）：中断结算须与预扣同口径放大，
	// 否则预扣按 n 张冻结、结算按 1 张计费，多退 (n-1) 张差额；非图片/无张数传 0
	SettleStreamInterrupted(ctx context.Context, tenantID, userID, apiKeyID, channelID int64, modelName, requestID, relayMode string, usage *Usage, preDeductAmount float64, projectID int64, imageCount int, billAt time.Time) (*SettlementResult, error)

	// CheckRateLimit QPS 限流检查
	CheckRateLimit(ctx context.Context, tenantID, userID, apiKeyID int64, keyQPS int) (allowed bool, limitLevel string, limit int, remaining int, resetAt int64)

	// AcquireConcurrent 并发限制检查（含租户级 + 模型级）
	AcquireConcurrent(ctx context.Context, tenantID, userID, apiKeyID int64, modelName string) bool

	// ReleaseConcurrent 释放并发限制（含租户级 + 模型级）
	ReleaseConcurrent(ctx context.Context, tenantID, userID, apiKeyID int64, modelName string)

	// AcquireApiKeyConcurrent 获取 API Key 级并发许可
	AcquireApiKeyConcurrent(ctx context.Context, apiKeyID int64, limit int) bool

	// ReleaseApiKeyConcurrent 释放 API Key 级并发许可
	ReleaseApiKeyConcurrent(ctx context.Context, apiKeyID int64)

	// CheckScope 检查 API Key scope
	CheckScope(scope string, relayMode string) bool

	// CheckIPWhitelist 检查 IP 白名单
	CheckIPWhitelist(whitelist string, clientIP string) bool

	// CheckMemberQuota 检查成员额度是否足够
	CheckMemberQuota(ctx context.Context, tenantID, userID int64, preDeductAmount float64) error

	// CheckApiKeyQuota 检查 API Key 额度是否足够
	CheckApiKeyQuota(ctx context.Context, apiKeyID int64, preDeductAmount float64) error

	// IncrMemberQuotaUsed 结算后累加成员已用额度
	IncrMemberQuotaUsed(ctx context.Context, tenantID, userID int64, amount float64)

	// IncrApiKeyQuotaUsed 结算后累加 API Key 已用额度
	IncrApiKeyQuotaUsed(ctx context.Context, apiKeyID int64, amount float64)
}

// RateLimitInfo 限流信息（用于设置响应头）
type RateLimitInfo struct {
	Limit     int
	Remaining int
	ResetAt   int64
}

// SettlementResult 结算结果（从 billing 层返回）
type SettlementResult struct {
	PreDeductAmount   float64
	BaseCost          float64
	ActualCost        float64
	RefundAmount      float64
	SupplementAmount  float64
	BillingRecordID   int64
	BillingSnapshot   string
	BillingSummary    string
	InputCost         float64
	OutputCost        float64
	CacheCreationCost float64
	CacheReadCost     float64
	TotalCost         float64
	BillingMode       string
	BillingSource     string
	RateMultiplier    float64
	DuplicateSkip     bool // 幂等跳过：本次为重复结算（未扣款未写账单），调用方不得再累加成员/Key 额度
}

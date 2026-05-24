package billing

import (
	"encoding/json"
	"fmt"

	rcommon "github.com/qianfree/team-api/relay/common"
)

// BillingSnapshot 计费快照（写入 JSONB）
type BillingSnapshot struct {
	Pricing     BillingSnapshotPricing      `json:"pricing"`
	Multipliers BillingSnapshotMultipliers  `json:"multipliers"`
	CachePrices *BillingSnapshotCachePrices `json:"cache_prices,omitempty"`
	TokenCosts  map[string]TokenCostDetail  `json:"token_costs"`
	Settlement  BillingSnapshotSettlement   `json:"settlement"`
	RequestMeta BillingSnapshotRequestMeta  `json:"request_meta"`
}

// BillingSnapshotPricing 价格来源信息
type BillingSnapshotPricing struct {
	BaseInputPrice       float64 `json:"base_input_price"`
	BaseOutputPrice      float64 `json:"base_output_price"`
	EffectiveInputPrice  float64 `json:"effective_input_price"`
	EffectiveOutputPrice float64 `json:"effective_output_price"`
	BillingMode          string  `json:"billing_mode"`
	BillingSource        string  `json:"billing_source"`
}

// BillingSnapshotMultipliers 倍率信息
type BillingSnapshotMultipliers struct {
	ModelMultiplier  float64 `json:"model_multiplier"`
	TenantMultiplier float64 `json:"tenant_multiplier"`
	DiscountRatio    float64 `json:"discount_ratio"`
	RateMultiplier   float64 `json:"rate_multiplier"`
}

// BillingSnapshotCachePrices 缓存价格信息
type BillingSnapshotCachePrices struct {
	CacheReadPrice     float64 `json:"cache_read_price"`
	CacheCreationPrice float64 `json:"cache_creation_price"`
}

// TokenCostDetail 单类 token 的费用明细
type TokenCostDetail struct {
	Tokens    int     `json:"tokens"`
	UnitPrice float64 `json:"unit_price"`
	Cost      float64 `json:"cost"`
}

// BillingSnapshotSettlement 结算信息
type BillingSnapshotSettlement struct {
	PreDeductAmount  float64 `json:"pre_deduct_amount"`
	ActualCost       float64 `json:"actual_cost"`
	RefundAmount     float64 `json:"refund_amount"`
	SupplementAmount float64 `json:"supplement_amount"`
	PlanID           int64   `json:"plan_id,omitempty"`
	PlanDeduction    float64 `json:"plan_deduction,omitempty"`
	WalletDeduction  float64 `json:"wallet_deduction,omitempty"`
}

// BillingSnapshotRequestMeta 请求元信息
type BillingSnapshotRequestMeta struct {
	RequestedModel string `json:"requested_model,omitempty"`
	UpstreamModel  string `json:"upstream_model,omitempty"`
	IsModelMapped  bool   `json:"is_model_mapped"`
	IsStream       bool   `json:"is_stream"`
	FirstTokenMs   int    `json:"first_token_ms,omitempty"`
}

// GenerateBillingSnapshot 生成完整计费快照
func GenerateBillingSnapshot(
	pricing *PricingResult,
	breakdown *CostBreakdown,
	usage *rcommon.Usage,
	settlement *SettlementResult,
	info *rcommon.RelayInfo,
) *BillingSnapshot {
	if pricing == nil || breakdown == nil {
		return nil
	}

	snapshot := &BillingSnapshot{
		Pricing: BillingSnapshotPricing{
			BaseInputPrice:       pricing.BaseInputPrice,
			BaseOutputPrice:      pricing.BaseOutputPrice,
			EffectiveInputPrice:  pricing.InputPrice,
			EffectiveOutputPrice: pricing.OutputPrice,
			BillingMode:          pricing.BillingMode,
			BillingSource:        pricing.BillingSource,
		},
		Multipliers: BillingSnapshotMultipliers{
			ModelMultiplier:  pricing.ModelMultiplier,
			TenantMultiplier: pricing.TenantMultiplier,
			DiscountRatio:    pricing.DiscountRatio,
			RateMultiplier:   pricing.TenantMultiplier,
		},
		TokenCosts: buildTokenCosts(pricing, breakdown),
	}

	// Cache 比率（仅当有 cache token 时填充）
	if breakdown.CacheReadTokens > 0 || breakdown.CacheCreationTokens > 0 {
		snapshot.CachePrices = &BillingSnapshotCachePrices{
			CacheReadPrice:     pricing.CacheReadPrice,
			CacheCreationPrice: pricing.CacheCreationPrice,
		}
	}

	// 结算信息
	if settlement != nil {
		snapshot.Settlement = BillingSnapshotSettlement{
			PreDeductAmount:  settlement.PreDeductAmount,
			ActualCost:       settlement.ActualCost,
			RefundAmount:     settlement.RefundAmount,
			SupplementAmount: settlement.SupplementAmount,
			PlanID:           settlement.PlanID,
			PlanDeduction:    settlement.PlanDeduction,
			WalletDeduction:  settlement.WalletDeduction,
		}
	}

	// 请求元信息
	if info != nil {
		requestMeta := BillingSnapshotRequestMeta{
			RequestedModel: info.OriginModelName,
			IsStream:       info.IsStream,
		}
		if info.ChannelMeta != nil {
			requestMeta.UpstreamModel = info.ChannelMeta.UpstreamModelName
			requestMeta.IsModelMapped = info.ChannelMeta.IsModelMapped
		}
		if !info.FirstResponseTime.IsZero() {
			requestMeta.FirstTokenMs = int(info.FirstResponseTime.Sub(info.StartTime).Milliseconds())
		}
		snapshot.RequestMeta = requestMeta
	}

	return snapshot
}

// buildTokenCosts 构建各类 token 的费用明细
func buildTokenCosts(pricing *PricingResult, breakdown *CostBreakdown) map[string]TokenCostDetail {
	costs := make(map[string]TokenCostDetail)

	costs["input"] = TokenCostDetail{
		Tokens:    breakdown.InputTokens,
		UnitPrice: pricing.InputPrice,
		Cost:      breakdown.InputCost,
	}
	costs["output"] = TokenCostDetail{
		Tokens:    breakdown.OutputTokens,
		UnitPrice: pricing.OutputPrice,
		Cost:      breakdown.OutputCost,
	}

	if breakdown.CacheReadTokens > 0 {
		// direct cache price
		costs["cache_read"] = TokenCostDetail{
			Tokens:    breakdown.CacheReadTokens,
			UnitPrice: pricing.CacheReadPrice,
			Cost:      breakdown.CacheReadCost,
		}
	}

	if breakdown.CacheCreationTokens > 0 {
		// direct cache creation price
		costs["cache_creation"] = TokenCostDetail{
			Tokens:    breakdown.CacheCreationTokens,
			UnitPrice: pricing.CacheCreationPrice,
			Cost:      breakdown.CacheCreationCost,
		}
	}

	return costs
}

// GenerateBillingSummary 生成人类可读的计费摘要文本（中文）
func GenerateBillingSummary(snapshot *BillingSnapshot) string {
	if snapshot == nil {
		return ""
	}

	// 价格来源中文映射
	sourceMap := map[string]string{
		"base":          "基础定价",
		"tenant_custom": "租户独立价",
		"plan":          "套餐价",
	}
	source := sourceMap[snapshot.Pricing.BillingSource]
	if source == "" {
		source = snapshot.Pricing.BillingSource
	}

	// 计费模式中文映射
	modeMap := map[string]string{
		"token":       "按量计费",
		"per_request": "按次计费",
		"tiered":      "阶梯计费",
	}
	mode := modeMap[snapshot.Pricing.BillingMode]
	if mode == "" {
		mode = snapshot.Pricing.BillingMode
	}

	// 模型名
	modelName := snapshot.RequestMeta.RequestedModel
	if modelName == "" {
		modelName = "-"
	}

	lines := make([]string, 0, 15)
	lines = append(lines, fmt.Sprintf("模型: %s | 计费模式: %s | 价格来源: %s", modelName, mode, source))
	lines = append(lines, "---")

	// 按次计费特殊处理
	if snapshot.Pricing.BillingMode == "per_request" {
		lines = append(lines, fmt.Sprintf("按次单价: $%.6f", snapshot.Pricing.EffectiveInputPrice))
	} else {
		// 各类 token 费用明细
		if tc, ok := snapshot.TokenCosts["input"]; ok && tc.Tokens > 0 {
			lines = append(lines, fmt.Sprintf("输入: %s tokens × $%s/1M = $%s",
				formatInt(tc.Tokens), formatPrice(tc.UnitPrice), formatPrice(tc.Cost)))
		}
		if tc, ok := snapshot.TokenCosts["output"]; ok && tc.Tokens > 0 {
			lines = append(lines, fmt.Sprintf("输出: %s tokens × $%s/1M = $%s",
				formatInt(tc.Tokens), formatPrice(tc.UnitPrice), formatPrice(tc.Cost)))
		}
		if tc, ok := snapshot.TokenCosts["cache_read"]; ok && tc.Tokens > 0 {
			lines = append(lines, fmt.Sprintf("缓存读取: %s tokens × $%s/1M = $%s",
				formatInt(tc.Tokens), formatPrice(tc.UnitPrice), formatPrice(tc.Cost)))
		}
		if tc, ok := snapshot.TokenCosts["cache_creation"]; ok && tc.Tokens > 0 {
			lines = append(lines, fmt.Sprintf("缓存创建: %s tokens × $%s/1M = $%s",
				formatInt(tc.Tokens), formatPrice(tc.UnitPrice), formatPrice(tc.Cost)))
		}

		// 小计 × 倍率
		subtotal := snapshot.Settlement.ActualCost
		if snapshot.Multipliers.TenantMultiplier != 1.0 && snapshot.Multipliers.TenantMultiplier != 0 {
			preMultiplier := subtotal / snapshot.Multipliers.TenantMultiplier
			lines = append(lines, fmt.Sprintf("小计: $%s × 租户倍率(%.2f) = $%s",
				formatPrice(preMultiplier), snapshot.Multipliers.TenantMultiplier, formatPrice(subtotal)))
		}
	}

	// 结算信息
	lines = append(lines, "---")
	s := snapshot.Settlement
	if s.PreDeductAmount > 0 {
		if s.RefundAmount > 0 {
			lines = append(lines, fmt.Sprintf("预扣: $%s → 实际: $%s → 退还: $%s",
				formatPrice(s.PreDeductAmount), formatPrice(s.ActualCost), formatPrice(s.RefundAmount)))
		} else if s.SupplementAmount > 0 {
			lines = append(lines, fmt.Sprintf("预扣: $%s → 实际: $%s → 补扣: $%s",
				formatPrice(s.PreDeductAmount), formatPrice(s.ActualCost), formatPrice(s.SupplementAmount)))
		} else {
			lines = append(lines, fmt.Sprintf("预扣: $%s → 实际: $%s（无差额）",
				formatPrice(s.PreDeductAmount), formatPrice(s.ActualCost)))
		}
	} else if s.ActualCost > 0 {
		lines = append(lines, fmt.Sprintf("实际费用: $%s", formatPrice(s.ActualCost)))
	}

	result := ""
	for i, line := range lines {
		if i > 0 {
			result += "\n"
		}
		result += line
	}
	return result
}

// SnapshotToJSON 将快照序列化为 JSON 字符串。
// nil 时返回 "null"（合法 JSON），确保 JSONB 列不会收到空字符串。
func SnapshotToJSON(snapshot *BillingSnapshot) string {
	if snapshot == nil {
		return "null"
	}
	data, err := json.Marshal(snapshot)
	if err != nil {
		return "null"
	}
	return string(data)
}

// formatPrice 格式化价格（去除尾部多余零）
func formatPrice(v float64) string {
	return formatCost(v)
}

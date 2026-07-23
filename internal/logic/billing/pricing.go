package billing

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"time"

	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/shopspring/decimal"

	"github.com/qianfree/team-api/internal/dao"
	lcommon "github.com/qianfree/team-api/internal/logic/common"
	rcommon "github.com/qianfree/team-api/relay/common"
)

// modelPriceCache 模型价格缓存（TTL 600s）
var modelPriceCache = lcommon.NewCache("model_price", 600*time.Second)

// ModelPrice 模型计费价格（含快照）
type ModelPrice struct {
	InputPrice      float64 // 每 1M input token 价格 (USD)
	OutputPrice     float64 // 每 1M output token 价格 (USD)
	BillingMode     string  // token / per_request / tiered
	PerRequestPrice float64 // 按次单价
	DiscountRatio   float64 // 折扣比例（优先于 TenantMultiplier）
	Currency        string  // USD
}

// PricingResult 定价计算结果
type PricingResult struct {
	InputPrice       float64 // 最终输入单价（每 1M token）
	OutputPrice      float64 // 最终输出单价（每 1M token）
	BaseInputPrice   float64 // 基础模型输入单价（应用倍率前）
	BaseOutputPrice  float64 // 基础模型输出单价（应用倍率前）
	BillingMode      string  // token / per_request / tiered
	BillingSource    string  // base / tenant_custom / plan
	PerRequestPrice  float64 // 按次单价
	DiscountRatio    float64 // 折扣比例
	InputMultiplier  float64 // 输入价格倍率（兼容旧快照）
	OutputMultiplier float64 // 输出价格倍率（兼容旧快照）
	TenantMultiplier float64 // 租户倍率
	ModelMultiplier  float64 // 模型倍率
	Currency         string

	// Cache 直接定价
	CacheReadPrice     float64 // 缓存读取每 1M token 价格
	CacheCreationPrice float64 // 缓存创建每 1M token 价格

	// 租户自定义阶梯定价（JSONB 解析后的原始数据，供 CalculateCost 使用）
	CustomTiers []pricingTierRow
}

// ClearTenantPriceCache 清除租户的所有模型价格缓存
func ClearTenantPriceCache(ctx context.Context, tenantID int64) {
	var models []struct {
		ModelId string `json:"model_id"`
	}
	dao.MdlTenantModels.Ctx(ctx).
		As("tm").
		LeftJoin("mdl_models m ON tm.model_id = m.id").
		Where("tm.tenant_id", tenantID).
		Fields("m.model_id").
		Scan(&models)

	for _, m := range models {
		cacheKey := fmt.Sprintf("%d:%s", tenantID, m.ModelId)
		modelPriceCache.Delete(ctx, cacheKey)
	}
}

// GetModelPrice 获取模型价格
// 优先级：租户独立价 > 套餐价 > 模型基础价 > 硬编码默认
func GetModelPrice(ctx context.Context, tenantID int64, modelName string) (*PricingResult, error) {
	cacheKey := fmt.Sprintf("%d:%s", tenantID, modelName)
	var cached PricingResult
	if modelPriceCache.GetJSON(ctx, cacheKey, &cached) {
		return &cached, nil
	}

	// 1. 查模型基础信息
	type modelRow struct {
		ID      int64  `json:"id"`
		ModelId string `json:"model_id"`
		Status  string `json:"status"`
	}

	var model *modelRow
	err := dao.MdlModels.Ctx(ctx).
		Where("model_id", modelName).
		Where("status", "active").
		Fields("id, model_id, status").
		Scan(&model)
	if err != nil {
		return nil, gerror.Wrapf(err, "query model price")
	}
	if model == nil {
		return nil, gerror.Newf("model not found: %s", modelName)
	}

	// 2. 从 mdl_pricing 获取基础定价
	type pricingRow struct {
		BillingMode        string   `json:"billing_mode"`
		InputPrice         float64  `json:"input_price"`
		OutputPrice        float64  `json:"output_price"`
		PerRequestPrice    *float64 `json:"per_request_price"`
		CacheReadPrice     float64  `json:"cache_read_price"`
		CacheCreationPrice float64  `json:"cache_creation_price"`
	}

	var pricing *pricingRow
	err = dao.MdlPricing.Ctx(ctx).
		Where("model_id", model.ID).
		Where("min_tokens", 0).
		Scan(&pricing)
	if err != nil {
		return nil, gerror.Wrapf(err, "query model pricing")
	}

	billingMode := "token"
	inputPrice := 0.0
	outputPrice := 0.0
	baseInputPrice := 0.0
	baseOutputPrice := 0.0
	var perRequestPrice float64
	cacheReadPrice := 0.0
	cacheCreationPrice := 0.0

	if pricing != nil {
		if pricing.BillingMode != "" {
			billingMode = pricing.BillingMode
		}
		inputPrice = pricing.InputPrice
		outputPrice = pricing.OutputPrice
		baseInputPrice = pricing.InputPrice
		baseOutputPrice = pricing.OutputPrice
		if pricing.PerRequestPrice != nil {
			perRequestPrice = *pricing.PerRequestPrice
		}
		cacheReadPrice = pricing.CacheReadPrice
		cacheCreationPrice = pricing.CacheCreationPrice
	}

	// 3. 查租户独立价格（mdl_tenant_models）
	type tenantModelRow struct {
		CustomInputPrice         *float64 `json:"custom_input_price"`
		CustomOutputPrice        *float64 `json:"custom_output_price"`
		CustomCacheReadPrice     *float64 `json:"custom_cache_read_price"`
		CustomCacheCreationPrice *float64 `json:"custom_cache_creation_price"`
		CustomPricingTiers       string   `json:"custom_pricing_tiers"`
		Multiplier               *float64 `json:"multiplier"`
		DiscountRatio            *float64 `json:"discount_ratio"`
		BillingMode              *string  `json:"billing_mode"`
		PerRequestPrice          *float64 `json:"per_request_price"`
		Enabled                  bool     `json:"enabled"`
	}

	var tm *tenantModelRow
	err = dao.MdlTenantModels.Ctx(ctx).
		Where("tenant_id", tenantID).
		Where("model_id", model.ID).
		Fields("custom_input_price, custom_output_price, custom_cache_read_price, custom_cache_creation_price, custom_pricing_tiers, multiplier, discount_ratio, billing_mode, per_request_price, enabled").
		Scan(&tm)
	if err != nil {
		return nil, gerror.Wrapf(err, "query tenant model price")
	}

	tenantMultiplier := 1.0
	discountRatio := 1.0
	billingSource := "base"
	var customTiers []pricingTierRow

	if tm != nil && tm.Enabled {
		billingSource = "tenant_custom"
		if tm.BillingMode != nil && *tm.BillingMode != "" {
			billingMode = *tm.BillingMode
		}

		// 租户独立价优先
		if tm.CustomInputPrice != nil && *tm.CustomInputPrice > 0 {
			inputPrice = *tm.CustomInputPrice
		}
		if tm.CustomOutputPrice != nil && *tm.CustomOutputPrice > 0 {
			outputPrice = *tm.CustomOutputPrice
		}

		// 租户覆盖缓存定价
		if tm.CustomCacheReadPrice != nil && *tm.CustomCacheReadPrice > 0 {
			cacheReadPrice = *tm.CustomCacheReadPrice
		}
		if tm.CustomCacheCreationPrice != nil && *tm.CustomCacheCreationPrice > 0 {
			cacheCreationPrice = *tm.CustomCacheCreationPrice
		}

		// 租户自定义阶梯定价
		if tm.CustomPricingTiers != "" && tm.CustomPricingTiers != "null" && tm.CustomPricingTiers != "[]" {
			_ = json.Unmarshal([]byte(tm.CustomPricingTiers), &customTiers)
		}

		// discount_ratio 优先于 multiplier
		if tm.DiscountRatio != nil && *tm.DiscountRatio > 0 {
			discountRatio = *tm.DiscountRatio
			tenantMultiplier = *tm.DiscountRatio
		} else if tm.Multiplier != nil && *tm.Multiplier > 0 {
			tenantMultiplier = *tm.Multiplier
			discountRatio = *tm.Multiplier
		}

		// 租户覆盖按次单价
		if tm.PerRequestPrice != nil && *tm.PerRequestPrice > 0 {
			perRequestPrice = *tm.PerRequestPrice
		}
	}

	// 3.5 级别折扣 fallback：当租户×模型维度未设置倍率时，使用租户级别的 price_multiplier
	if tenantMultiplier == 1.0 {
		levelMultiplier := GetLevelPriceMultiplier(ctx, tenantID)
		if levelMultiplier > 0 && levelMultiplier < 1.0 {
			tenantMultiplier = levelMultiplier
			discountRatio = levelMultiplier
		}
	}

	// 4. 套餐价（待实现）
	// 前置条件：需新增 pln_plan_model_pricing 表存储每个套餐的每个模型定价，
	// 或为 pln_plans 增加 billing_discount_ratio 全局折扣字段。
	// 查询链路：pln_tenant_plans → pln_plans → pln_plan_model_pricing
	// 定价优先级：租户独立价 > 套餐价 > 模型基础价 > 硬编码默认

	result := &PricingResult{
		InputPrice:         inputPrice,
		OutputPrice:        outputPrice,
		BaseInputPrice:     baseInputPrice,
		BaseOutputPrice:    baseOutputPrice,
		BillingMode:        billingMode,
		BillingSource:      billingSource,
		PerRequestPrice:    perRequestPrice,
		DiscountRatio:      discountRatio,
		TenantMultiplier:   tenantMultiplier,
		ModelMultiplier:    1.0,
		Currency:           "USD",
		CacheReadPrice:     cacheReadPrice,
		CacheCreationPrice: cacheCreationPrice,
		CustomTiers:        customTiers,
	}

	modelPriceCache.Set(ctx, cacheKey, result)
	return result, nil
}

// CalculateCost 计算实际费用（含阶梯定价）
// inputTokens / outputTokens 为实际使用的 token 数
func CalculateCost(ctx context.Context, tenantID int64, modelName string, inputTokens, outputTokens int) (*CostBreakdown, error) {
	pricing, err := GetModelPrice(ctx, tenantID, modelName)
	if err != nil {
		return nil, err
	}

	return computeCost(pricing, inputTokens, outputTokens, nil), nil
}

// computeCost 纯计算：根据定价结果和 token 用量计算费用明细。
// 提取为独立函数以便单元测试，不依赖数据库或缓存。
func computeCost(pricing *PricingResult, inputTokens, outputTokens int, usage *rcommon.Usage) *CostBreakdown {
	inputTokens, outputTokens, cacheReadTokens, cacheCreationTokens := resolveTokenCounts(pricing, inputTokens, outputTokens, usage)

	// 按次计费：直接返回单价
	if pricing.BillingMode == "per_request" {
		return &CostBreakdown{
			BaseCost:            pricing.PerRequestPrice,
			TotalCost:           pricing.PerRequestPrice,
			InputTokens:         inputTokens,
			OutputTokens:        outputTokens,
			BillingMode:         pricing.BillingMode,
			PerRequestPrice:     pricing.PerRequestPrice,
			DiscountRatio:       pricing.DiscountRatio,
			TenantMultiplier:    pricing.TenantMultiplier,
			Currency:            pricing.Currency,
			CacheCreationTokens: cacheCreationTokens,
			CacheReadTokens:     cacheReadTokens,
		}
	}

	// 基础输入费用
	baseInputCost := computeInputCost(pricing, inputTokens)

	// 输出费用
	outputCost := computeOutputCost(pricing, outputTokens)

	// A8：token 成本链式计算（÷1e6 × 单价 × 租户倍率 + 各项求和）改用 decimal 精确运算，
	// 最终四舍五入到 10 位（NUMERIC(20,10)）再返回 float64，消除 float64 累计误差。
	million := decimal.NewFromInt(1_000_000)
	mul := dec(pricing.TenantMultiplier)

	baseInputCostD := dec(baseInputCost)
	outputCostD := dec(outputCost)
	cacheReadCostD := decimal.NewFromInt(int64(cacheReadTokens)).Div(million).Mul(dec(pricing.CacheReadPrice))
	cacheCreationCostD := decimal.NewFromInt(int64(cacheCreationTokens)).Div(million).Mul(dec(pricing.CacheCreationPrice))

	// 总费用 = (基础输入 + 输出 + cache各项) × 租户倍率
	subtotalD := baseInputCostD.Add(outputCostD).Add(cacheReadCostD).Add(cacheCreationCostD)
	totalCostD := subtotalD.Mul(mul)

	return &CostBreakdown{
		BaseCost:            roundMoney(subtotalD),
		InputCost:           roundMoney(baseInputCostD.Mul(mul)),
		OutputCost:          roundMoney(outputCostD.Mul(mul)),
		TotalCost:           roundMoney(totalCostD),
		InputTokens:         inputTokens,
		OutputTokens:        outputTokens,
		BillingMode:         pricing.BillingMode,
		PerRequestPrice:     pricing.PerRequestPrice,
		DiscountRatio:       pricing.DiscountRatio,
		TenantMultiplier:    pricing.TenantMultiplier,
		Currency:            pricing.Currency,
		CacheCreationTokens: cacheCreationTokens,
		CacheReadTokens:     cacheReadTokens,
		CacheCreationCost:   roundMoney(cacheCreationCostD.Mul(mul)),
		CacheReadCost:       roundMoney(cacheReadCostD.Mul(mul)),
	}
}

// resolveTokenCounts 根据 usage 信息解析最终的 token 计数。
// 处理 cacheIncludedInPrompt 逻辑：如果 PromptTokens 包含 cache tokens，则扣减以避免重复计费。
func resolveTokenCounts(pricing *PricingResult, inputTokens, outputTokens int, usage *rcommon.Usage) (baseInput, output, cacheRead, cacheCreation int) {
	baseInput = inputTokens
	output = outputTokens

	if usage != nil {
		if usage.PromptTokensDetails != nil {
			cacheRead = usage.PromptTokensDetails.CachedTokens
			cacheCreation = usage.PromptTokensDetails.CachedCreationTokens
		}
		if usage.CacheIncludedInPrompt {
			baseInput = inputTokens - cacheRead - cacheCreation
			if baseInput < 0 {
				baseInput = 0
			}
		}
	}
	return
}

// computeInputCost 计算输入费用（token 或 tiered 模式）
func computeInputCost(pricing *PricingResult, tokens int) float64 {
	if pricing.BillingMode == "tiered" && len(pricing.CustomTiers) > 0 {
		return calculateTieredCostFromTiers(pricing.CustomTiers, tokens, true)
	}
	return float64(tokens) / 1_000_000.0 * pricing.InputPrice
}

// computeOutputCost 计算输出费用（token 或 tiered 模式）
func computeOutputCost(pricing *PricingResult, tokens int) float64 {
	if pricing.BillingMode == "tiered" && len(pricing.CustomTiers) > 0 {
		return calculateTieredCostFromTiers(pricing.CustomTiers, tokens, false)
	}
	return float64(tokens) / 1_000_000.0 * pricing.OutputPrice
}

// CalculateCostWithUsage 计算实际费用（含 cache token 计费）
// 传入完整的 Usage 结构，支持 cache_creation / cache_read 等 token 的费用计算
func CalculateCostWithUsage(ctx context.Context, tenantID int64, modelName string, usage *rcommon.Usage) (*CostBreakdown, error) {
	if usage == nil {
		return nil, gerror.New("usage is nil")
	}

	pricing, err := GetModelPrice(ctx, tenantID, modelName)
	if err != nil {
		return nil, err
	}

	return computeCost(pricing, usage.PromptTokens, usage.CompletionTokens, usage), nil
}

// CostBreakdown 费用明细
type CostBreakdown struct {
	BaseCost         float64 // 基础费用（应用租户折扣前）
	InputCost        float64
	OutputCost       float64
	TotalCost        float64 // 含折扣后的总费用
	InputTokens      int
	OutputTokens     int
	BillingMode      string
	PerRequestPrice  float64
	DiscountRatio    float64
	InputMultiplier  float64
	OutputMultiplier float64
	TenantMultiplier float64
	Currency         string

	// Cache token 费用
	CacheCreationTokens int
	CacheReadTokens     int
	CacheCreationCost   float64
	CacheReadCost       float64
}

// EstimatePreDeductAmount 估算预扣金额
// 非流式：输入 + max_tokens；流式：输入 + 预估上限（模型 max_output_tokens 的 80%）
// 上限 $1.00
func EstimatePreDeductAmount(ctx context.Context, tenantID int64, modelName string, inputTokens, requestedMaxTokens int, isStream bool) (float64, error) {
	pricing, err := GetModelPrice(ctx, tenantID, modelName)
	if err != nil {
		return 0.01, nil
	}

	// 按次计费：直接用单价
	if pricing.BillingMode == "per_request" {
		if pricing.PerRequestPrice > 1.0 {
			return 1.0, nil
		}
		return pricing.PerRequestPrice, nil
	}

	// Token 计费：估算
	type modelRow struct {
		MaxOutputTokens int `json:"max_output_tokens"`
	}
	var model *modelRow
	err = dao.MdlModels.Ctx(ctx).
		Where("model_id", modelName).
		Fields("max_output_tokens").
		Scan(&model)
	if err != nil {
		return 0.01, nil
	}

	maxOutput := 4096
	if model != nil && model.MaxOutputTokens > 0 {
		maxOutput = model.MaxOutputTokens
	}

	estimatedOutput := requestedMaxTokens
	if estimatedOutput <= 0 || isStream {
		estimatedOutput = int(float64(maxOutput) * 0.8)
		if estimatedOutput <= 0 {
			estimatedOutput = 4096
		}
	}

	breakdown, err := CalculateCost(ctx, tenantID, modelName, inputTokens, estimatedOutput)
	if err != nil {
		return 0.01, nil
	}

	if breakdown.TotalCost > 1.0 {
		return 1.0, nil
	}
	if breakdown.TotalCost < 0.001 {
		return 0.001, nil
	}

	return math.Ceil(breakdown.TotalCost*1000000) / 1000000, nil
}

// pricingTierRow 定价阶梯行（绝对价格）
type pricingTierRow struct {
	MinTokens   int64   `json:"min_tokens"`
	MaxTokens   *int64  `json:"max_tokens"`
	InputPrice  float64 `json:"input_price"`
	OutputPrice float64 `json:"output_price"`
}

// calculateTieredCostFromPricing 从 mdl_pricing 读取阶梯绝对价格计算费用
func calculateTieredCostFromPricing(ctx context.Context, modelName string, tokens int, isInput bool) (float64, float64) {
	if tokens <= 0 {
		return 0, 1.0
	}

	// 查模型ID
	type modelIDRow struct {
		ID int64 `json:"id"`
	}
	var mid modelIDRow
	err := dao.MdlModels.Ctx(ctx).
		Where("model_id", modelName).
		Fields("id").
		Scan(&mid)
	if err != nil || mid.ID == 0 {
		return 0, 1.0
	}

	// 查阶梯定价（绝对价格）
	var tiers []pricingTierRow
	err = dao.MdlPricing.Ctx(ctx).
		Where("model_id", mid.ID).
		Where("billing_mode", "tiered").
		OrderAsc("min_tokens").
		Fields("min_tokens, max_tokens, input_price, output_price").
		Scan(&tiers)
	if err != nil || len(tiers) == 0 {
		return 0, 1.0
	}

	return calculateTieredCostFromTiers(tiers, tokens, isInput), 1.0
}

// calculateTieredCostFromTiers 从给定的阶梯数组计算费用（租户自定义阶梯或基础阶梯共用）
func calculateTieredCostFromTiers(tiers []pricingTierRow, tokens int, isInput bool) float64 {
	if tokens <= 0 || len(tiers) == 0 {
		return 0
	}

	// 修复阶梯定价循环累加：用 decimal 精确计算避免每次 ÷1M × price 的误差累积
	million := decimal.NewFromInt(1_000_000)
	totalCostD := decimal.Zero
	remaining := int64(tokens)

	for _, tier := range tiers {
		if remaining <= 0 {
			break
		}

		price := tier.InputPrice
		if !isInput {
			price = tier.OutputPrice
		}

		if tier.MaxTokens == nil {
			// 最后一档：消耗所有剩余 token
			totalCostD = totalCostD.Add(
				decimal.NewFromInt(remaining).Div(million).Mul(dec(price)),
			)
			remaining = 0
		} else {
			available := *tier.MaxTokens - tier.MinTokens
			if available <= 0 {
				continue
			}
			useTokens := remaining
			if useTokens > available {
				useTokens = available
			}
			totalCostD = totalCostD.Add(
				decimal.NewFromInt(useTokens).Div(million).Mul(dec(price)),
			)
			remaining -= useTokens
		}
	}

	return roundMoney(totalCostD)
}

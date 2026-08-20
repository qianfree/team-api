package billing

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"time"

	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"
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
	CacheReadPrice       float64 // 缓存读取每 1M token 价格
	CacheCreationPrice   float64 // 缓存创建每 1M token 价格（5m TTL 基础价）
	CacheCreation5mPrice float64 // 5 分钟缓存创建价格（1.25× 基础价）
	CacheCreation1hPrice float64 // 1 小时缓存创建价格（2× 基础价）

	// 模型最大输出 token 数（随定价一起缓存，供预扣估算使用，
	// 避免 EstimatePreDeductAmount 每请求单独查一次 mdl_models）。
	// 0 表示未设置（含旧缓存条目），使用方需自带默认值兜底。
	MaxOutputTokens int

	// 租户自定义阶梯定价（JSONB 解析后的原始数据，供 CalculateCost 使用）
	// 租户未自定义时回落为平台阶梯（mdl_pricing 的 min_tokens 档位行）
	CustomTiers []pricingTierRow `json:"CustomTiers"`

	// 时段定价配置（mdl_pricing 锚点行 time_segments，随定价缓存存储）
	TimeSegments []TimeSegment `json:"time_segments,omitempty"`
	// 时段乘数（读取时按定价时刻评估；缓存条目中的值是写入时刻的结果，仅作参考）
	TimeMultiplier float64 `json:"time_multiplier"`
	// 命中的时段名（写入计费快照，供账单解释；未命中为空）
	TimeRuleName string `json:"time_rule_name"`
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

// ClearModelPriceCache 清除指定模型在所有租户下的价格缓存。
// 管理后台修改模型基础定价（SetModelPricing / ImportModels 更新 / DeleteModel）后必须调用，
// 否则各租户缓存中的旧价格最多残留 600s，出现「已设价但调用仍报未配置定价/按旧价计费」。
// 缓存键格式为 {tenantID}:{modelName}，此处按模型名匹配所有租户的条目。
func ClearModelPriceCache(ctx context.Context, modelName string) {
	modelPriceCache.DeleteByPattern(ctx, fmt.Sprintf("*:%s", modelName))
}

// GetModelPrice 获取模型价格（时段乘数按当前时刻评估）。
// 预扣等「调用时刻≈受理时刻」的路径使用本函数。
func GetModelPrice(ctx context.Context, tenantID int64, modelName string) (*PricingResult, error) {
	return GetModelPriceAt(ctx, tenantID, modelName, time.Time{})
}

// GetModelPriceAt 获取模型价格，时段乘数按 billAt（定价时刻）评估。
// 优先级：租户独立价 > 套餐价 > 模型基础价 > 硬编码默认。
// 定价时刻语义：结算/异步任务路径必须传「请求（任务）受理时刻」，保证 07:59 发出的请求
// 即使 08:05 结算也按 07:59 的时段价计费，预扣与结算口径一致；billAt 零值按当前时刻。
func GetModelPriceAt(ctx context.Context, tenantID int64, modelName string, billAt time.Time) (*PricingResult, error) {
	if billAt.IsZero() {
		billAt = time.Now()
	}
	cacheKey := fmt.Sprintf("%d:%s", tenantID, modelName)
	var cached PricingResult
	if modelPriceCache.GetJSON(ctx, cacheKey, &cached) {
		// 缓存条目中的时段乘数是写入时刻的评估结果，必须按 billAt 重评估
		// （GetJSON 的 target 为调用方独有副本，L1 回填存的也是反序列化副本，
		// 就地改写不会波及缓存内对象、无数据竞争）
		return reapplyTimeMultiplier(ctx, &cached, billAt), nil
	}

	// 1. 查模型基础信息（max_output_tokens 一并取出，随定价缓存供预扣估算复用）
	type modelRow struct {
		ID              int64  `json:"id"`
		ModelId         string `json:"model_id"`
		Status          string `json:"status"`
		MaxOutputTokens int    `json:"max_output_tokens"`
	}

	var model *modelRow
	err := dao.MdlModels.Ctx(ctx).
		Where("model_id", modelName).
		Where("status", "active").
		Fields("id, model_id, status, max_output_tokens").
		Scan(&model)
	if err != nil {
		return nil, gerror.Wrapf(err, "query model price")
	}
	if model == nil {
		return nil, gerror.Newf("model not found: %s", modelName)
	}

	// 2. 从 mdl_pricing 获取定价（全行读取：锚点行 min_tokens=0 是默认价与时段配置载体，
	// 其余行是平台阶梯档位。此前只读锚点行导致平台阶梯不参与计费，退化为第一档平价）
	type pricingRow struct {
		MinTokens          int64    `json:"min_tokens"`
		MaxTokens          *int64   `json:"max_tokens"`
		BillingMode        string   `json:"billing_mode"`
		InputPrice         float64  `json:"input_price"`
		OutputPrice        float64  `json:"output_price"`
		PerRequestPrice    *float64 `json:"per_request_price"`
		CacheReadPrice     float64  `json:"cache_read_price"`
		CacheCreationPrice float64  `json:"cache_creation_price"`
		TimeSegments       string   `json:"time_segments"`
	}

	var pricingRows []pricingRow
	err = dao.MdlPricing.Ctx(ctx).
		Where("model_id", model.ID).
		OrderAsc("min_tokens").
		Scan(&pricingRows)
	if err != nil {
		return nil, gerror.Wrapf(err, "query model pricing")
	}

	// 锚点行（默认价）与平台阶梯（含锚点行作为第一档，与租户端 buildTiers 口径一致）
	var pricing *pricingRow
	var platformTiers []pricingTierRow
	for i := range pricingRows {
		row := &pricingRows[i]
		if row.MinTokens == 0 {
			pricing = row
		}
		platformTiers = append(platformTiers, pricingTierRow{
			MinTokens:   row.MinTokens,
			MaxTokens:   row.MaxTokens,
			InputPrice:  row.InputPrice,
			OutputPrice: row.OutputPrice,
		})
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

	// 2.5 平台阶梯兜底：租户未自定义阶梯时使用平台阶梯（mdl_pricing 档位行）。
	// 计费模式非 tiered 时 computeCost 不会读取 CustomTiers，无副作用
	if len(customTiers) == 0 {
		customTiers = platformTiers
	}

	// 2.6 时段定价（锚点行 JSONB；解析失败按默认价处理并告警，不阻断计费）
	var timeSegments []TimeSegment
	if pricing != nil && pricing.TimeSegments != "" && pricing.TimeSegments != "null" {
		if err := json.Unmarshal([]byte(pricing.TimeSegments), &timeSegments); err != nil {
			g.Log().Warningf(ctx, "billing: 模型 %s 时段定价配置解析失败，按默认价处理: %v", modelName, err)
			timeSegments = nil
		}
	}

	// 3.5 级别折扣 fallback：当租户×模型维度未设置倍率时，使用租户级别的 price_multiplier
	if tenantMultiplier == 1.0 {
		levelMultiplier := GetLevelPriceMultiplier(ctx, tenantID)
		levelMultiplierFloat := InexactFloat64(levelMultiplier)
		if levelMultiplierFloat > 0 && levelMultiplierFloat < 1.0 {
			tenantMultiplier = levelMultiplierFloat
			discountRatio = levelMultiplierFloat
		}
	}

	// 4. 套餐价（待实现）
	// 前置条件：需新增 pln_plan_model_pricing 表存储每个套餐的每个模型定价，
	// 或为 pln_plans 增加 billing_discount_ratio 全局折扣字段。
	// 查询链路：pln_tenant_plans → pln_plans → pln_plan_model_pricing
	// 定价优先级：租户独立价 > 套餐价 > 模型基础价 > 硬编码默认

	// 4.5 模型倍率（预留，当前恒为 1.0 且不参与费用计算）
	// 设计文档规定最终价格 = 基础价格 × 模型乘数 × 租户乘数，但模型乘数这一环尚未启用：
	//   1) 无数据源：mdl_models 暂无 multiplier 字段，无法读取；
	//   2) computeCost 实际费用计算只乘 TenantMultiplier，不纳入 ModelMultiplier。
	// 因此当前实际生效的公式是「基础价格 × 租户乘数」，bil_records.model_multiplier 快照恒为 1.0。
	// 若要启用模型乘数，需三步：mdl_models 增设 multiplier 字段 → 此处读取 → computeCost 接入乘法。
	modelMultiplier := 1.0

	result := &PricingResult{
		InputPrice:           inputPrice,
		OutputPrice:          outputPrice,
		BaseInputPrice:       baseInputPrice,
		BaseOutputPrice:      baseOutputPrice,
		BillingMode:          billingMode,
		BillingSource:        billingSource,
		PerRequestPrice:      perRequestPrice,
		DiscountRatio:        discountRatio,
		TenantMultiplier:     tenantMultiplier,
		ModelMultiplier:      modelMultiplier,
		Currency:             "USD",
		CacheReadPrice:       cacheReadPrice,
		CacheCreationPrice:   cacheCreationPrice,
		CacheCreation5mPrice: cacheCreationPrice,       // 5m TTL = cache_creation_price 基础价（对应官方 1.25× 输入价）
		CacheCreation1hPrice: cacheCreationPrice * 1.6, // 1h TTL = 1.6× 基础价（因为 2.0÷1.25=1.6，对应官方 2× 输入价）
		MaxOutputTokens:      model.MaxOutputTokens,
		CustomTiers:          customTiers,
		TimeSegments:         timeSegments,
	}

	// 时段乘数按定价时刻评估后随缓存存储（缓存命中路径会按 billAt 重评估，存储值仅参考）
	if len(timeSegments) > 0 {
		loc := pricingTimeLocation(ctx)
		result.TimeMultiplier, result.TimeRuleName = resolveTimeMultiplier(timeSegments, billAt, loc)
	} else {
		result.TimeMultiplier = 1.0
	}

	modelPriceCache.Set(ctx, cacheKey, result)
	return result, nil
}

// reapplyTimeMultiplier 缓存命中路径：按 billAt 重评估时段乘数后就地改写并返回。
// 入参必须是本调用新建/反序列化的独有副本，不得传入共享指针：
// GetJSON 保证 target 归调用方所有（L1 回填存反序列化副本），
// DB 加载路径的 result 在 Set 之后不再改写。
func reapplyTimeMultiplier(ctx context.Context, p *PricingResult, billAt time.Time) *PricingResult {
	if len(p.TimeSegments) == 0 {
		p.TimeMultiplier = 1.0
		p.TimeRuleName = ""
		return p
	}
	loc := pricingTimeLocation(ctx)
	p.TimeMultiplier, p.TimeRuleName = resolveTimeMultiplier(p.TimeSegments, billAt, loc)
	return p
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
	baseInputTokens, outputTokens, cacheReadTokens, cacheCreation5mTokens, cacheCreation1hTokens := resolveTokenCounts(pricing, inputTokens, outputTokens, usage)

	// 按次计费：单价 × 租户乘数 × 时段乘数。
	// 此前未乘租户乘数（按次模型对租户折扣免疫，疑似遗漏），本次与 token/tiered 口径对齐
	if pricing.BillingMode == "per_request" {
		mulD := NewFromFloat(pricing.TenantMultiplier).Mul(NewFromFloat(effectiveTimeMultiplier(pricing)))
		return &CostBreakdown{
			BaseCost:            pricing.PerRequestPrice,
			TotalCost:           InexactFloat64(RoundMoney(NewFromFloat(pricing.PerRequestPrice).Mul(mulD))),
			InputTokens:         baseInputTokens,
			OutputTokens:        outputTokens,
			BillingMode:         pricing.BillingMode,
			PerRequestPrice:     pricing.PerRequestPrice,
			DiscountRatio:       pricing.DiscountRatio,
			TenantMultiplier:    pricing.TenantMultiplier,
			Currency:            pricing.Currency,
			CacheCreationTokens: cacheCreation5mTokens + cacheCreation1hTokens,
			CacheReadTokens:     cacheReadTokens,
		}
	}

	// 基础输入费用（已改为 decimal）
	baseInputCostD := computeInputCost(pricing, baseInputTokens)

	// 输出费用（已改为 decimal）
	outputCostD := computeOutputCost(pricing, outputTokens)

	// A8：token 成本链式计算（÷1e6 × 单价 × 租户倍率 × 时段乘数 + 各项求和）改用 decimal 精确运算，
	// 最终四舍五入到 10 位（NUMERIC(20,10)）再返回 float64，消除 float64 累计误差。
	million := decimal.NewFromInt(1_000_000)
	mul := NewFromFloat(pricing.TenantMultiplier).Mul(NewFromFloat(effectiveTimeMultiplier(pricing)))

	cacheReadCostD := decimal.NewFromInt(int64(cacheReadTokens)).Div(million).Mul(NewFromFloat(pricing.CacheReadPrice))

	// 缓存创建按 TTL 分别计价：5m 按 1.25×，1h 按 2×
	cacheCreation5mCostD := decimal.NewFromInt(int64(cacheCreation5mTokens)).Div(million).Mul(NewFromFloat(pricing.CacheCreation5mPrice))
	cacheCreation1hCostD := decimal.NewFromInt(int64(cacheCreation1hTokens)).Div(million).Mul(NewFromFloat(pricing.CacheCreation1hPrice))
	cacheCreationCostD := cacheCreation5mCostD.Add(cacheCreation1hCostD)

	// 总费用 = (基础输入 + 输出 + cache各项) × 租户倍率
	subtotalD := baseInputCostD.Add(outputCostD).Add(cacheReadCostD).Add(cacheCreationCostD)
	totalCostD := subtotalD.Mul(mul)

	return &CostBreakdown{
		BaseCost:            InexactFloat64(RoundMoney(subtotalD)),
		InputCost:           InexactFloat64(RoundMoney(baseInputCostD.Mul(mul))),
		OutputCost:          InexactFloat64(RoundMoney(outputCostD.Mul(mul))),
		TotalCost:           InexactFloat64(RoundMoney(totalCostD)),
		InputTokens:         baseInputTokens,
		OutputTokens:        outputTokens,
		BillingMode:         pricing.BillingMode,
		PerRequestPrice:     pricing.PerRequestPrice,
		DiscountRatio:       pricing.DiscountRatio,
		TenantMultiplier:    pricing.TenantMultiplier,
		Currency:            pricing.Currency,
		CacheCreationTokens: cacheCreation5mTokens + cacheCreation1hTokens,
		CacheReadTokens:     cacheReadTokens,
		CacheCreationCost:   InexactFloat64(RoundMoney(cacheCreationCostD.Mul(mul))),
		CacheReadCost:       InexactFloat64(RoundMoney(cacheReadCostD.Mul(mul))),
	}
}

// resolveTokenCounts 根据 usage 信息解析最终的 token 计数。
// 处理 cacheIncludedInPrompt 逻辑：如果 PromptTokens 包含 cache tokens，则扣减以避免重复计费。
// 返回：baseInput, output, cacheRead, cacheCreation5m, cacheCreation1h
func resolveTokenCounts(pricing *PricingResult, inputTokens, outputTokens int, usage *rcommon.Usage) (baseInput, output, cacheRead, cacheCreation5m, cacheCreation1h int) {
	baseInput = inputTokens
	output = outputTokens

	if usage != nil {
		if usage.PromptTokensDetails != nil {
			cacheRead = usage.PromptTokensDetails.CachedTokens
			// 优先使用细分的 5m/1h token（Claude 新协议）
			if usage.PromptTokensDetails.CachedCreation5mTokens > 0 || usage.PromptTokensDetails.CachedCreation1hTokens > 0 {
				cacheCreation5m = usage.PromptTokensDetails.CachedCreation5mTokens
				cacheCreation1h = usage.PromptTokensDetails.CachedCreation1hTokens
			} else if usage.PromptTokensDetails.CachedCreationTokens > 0 {
				// 旧协议无 TTL 细分：全部按 5m 兜底（保守计费）
				cacheCreation5m = usage.PromptTokensDetails.CachedCreationTokens
			}
		}
		if usage.CacheIncludedInPrompt {
			baseInput = inputTokens - cacheRead - cacheCreation5m - cacheCreation1h
			if baseInput < 0 {
				baseInput = 0
			}
		}
	}
	return
}

// computeInputCost 计算输入费用（token 或 tiered 模式）
// 返回 decimal.Decimal 避免 float64 链式运算误差
func computeInputCost(pricing *PricingResult, tokens int) decimal.Decimal {
	if pricing.BillingMode == "tiered" && len(pricing.CustomTiers) > 0 {
		// 阶梯定价仍返回 float64，需要转换
		return NewFromFloat(calculateTieredCostFromTiers(pricing.CustomTiers, tokens, true))
	}
	// token / 1M × 单价，全程 decimal 精确运算
	million := decimal.NewFromInt(1_000_000)
	return decimal.NewFromInt(int64(tokens)).Div(million).Mul(NewFromFloat(pricing.InputPrice))
}

// computeOutputCost 计算输出费用（token 或 tiered 模式）
// 返回 decimal.Decimal 避免 float64 链式运算误差
func computeOutputCost(pricing *PricingResult, tokens int) decimal.Decimal {
	if pricing.BillingMode == "tiered" && len(pricing.CustomTiers) > 0 {
		// 阶梯定价仍返回 float64，需要转换
		return NewFromFloat(calculateTieredCostFromTiers(pricing.CustomTiers, tokens, false))
	}
	// token / 1M × 单价，全程 decimal 精确运算
	million := decimal.NewFromInt(1_000_000)
	return decimal.NewFromInt(int64(tokens)).Div(million).Mul(NewFromFloat(pricing.OutputPrice))
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
// 输出 token 估算：用户指定 max_tokens 时按其值（截断到模型上限）；未指定时按模型 max_output_tokens 的 80%。
// 按估算全额冻结（无上限封顶），下限 $0.001；未配置定价的模型 fail-closed 返回错误拒绝请求。
func EstimatePreDeductAmount(ctx context.Context, tenantID int64, modelName string, inputTokens, requestedMaxTokens int, isStream bool) (float64, error) {
	_ = isStream // 预留参数：估算逻辑不再区分流式/非流式，接口签名保持兼容
	pricing, err := GetModelPrice(ctx, tenantID, modelName)
	if err != nil {
		return 0, gerror.Wrapf(err, "estimate pre-deduct: get model price")
	}

	// fail-closed：未配置定价的模型直接拒绝，防止零价计费变成免费放行
	if err := validatePricingConfigured(pricing, modelName); err != nil {
		return 0, err
	}

	// 按次计费：单价 × 租户乘数 × 时段乘数（与 computeCost 结算口径一致）
	if pricing.BillingMode == "per_request" {
		mult := NewFromFloat(pricing.TenantMultiplier).Mul(NewFromFloat(effectiveTimeMultiplier(pricing)))
		return InexactFloat64(NewFromFloat(pricing.PerRequestPrice).Mul(mult)), nil
	}

	// Token 计费：估算输出上限。
	// max_output_tokens 已随 GetModelPrice 的 600s 定价缓存一起取出，
	// 不再单独查 mdl_models（原实现每请求一条无缓存 SQL，高并发下是纯浪费）。
	// 为 0（模型未设置，或旧缓存条目缺此字段）时按 4096 兜底。
	maxOutput := 4096
	if pricing.MaxOutputTokens > 0 {
		maxOutput = pricing.MaxOutputTokens
	}

	estimatedOutput := requestedMaxTokens
	if estimatedOutput <= 0 {
		estimatedOutput = int(float64(maxOutput) * 0.8)
		if estimatedOutput <= 0 {
			estimatedOutput = 4096
		}
	} else if estimatedOutput > maxOutput {
		// 用户传入超模型上限的 max_tokens：按模型上限截断，避免过度冻结
		estimatedOutput = maxOutput
	}

	breakdown, err := CalculateCost(ctx, tenantID, modelName, inputTokens, estimatedOutput)
	if err != nil {
		return 0, gerror.Wrapf(err, "estimate pre-deduct: calculate cost")
	}

	if breakdown.TotalCost < 0.001 {
		return 0.001, nil
	}

	return math.Ceil(breakdown.TotalCost*1000000) / 1000000, nil
}

// validatePricingConfigured 校验模型定价有效性（fail-closed）。
// 未配置定价的模型不得放行：零价会让预扣/结算全部为 0，等同免费使用。
// 允许只配 OutputPrice（输入免费）或只配含正价的自定义/平台阶梯（CustomTiers）的合法场景。
func validatePricingConfigured(pricing *PricingResult, modelName string) error {
	switch pricing.BillingMode {
	case "per_request":
		if pricing.PerRequestPrice <= 0 {
			return gerror.Wrapf(rcommon.ErrModelPricingNotConfigured, "model=%s (per_request price not set)", modelName)
		}
	default: // token / tiered
		if pricing.InputPrice <= 0 && pricing.OutputPrice <= 0 && !tiersHavePrice(pricing.CustomTiers) {
			return gerror.Wrapf(rcommon.ErrModelPricingNotConfigured, "model=%s", modelName)
		}
	}
	return nil
}

// tiersHavePrice 阶梯数组中是否存在任一档正价（输入或输出）。
// 平台阶梯兜底后 CustomTiers 可能恒非空，全 0 价阶梯不得视为「已配置定价」放行
func tiersHavePrice(tiers []pricingTierRow) bool {
	for _, tier := range tiers {
		if tier.InputPrice > 0 || tier.OutputPrice > 0 {
			return true
		}
	}
	return false
}

// pricingTierRow 定价阶梯行（绝对价格）
type pricingTierRow struct {
	MinTokens   int64   `json:"min_tokens"`
	MaxTokens   *int64  `json:"max_tokens"`
	InputPrice  float64 `json:"input_price"`
	OutputPrice float64 `json:"output_price"`
}

// effectiveTimeMultiplier 时段乘数归一化：0/负值视为 1.0。
// 兜底两类场景：升级部署后 Redis L2 缓存中的旧定价条目（无该字段）、手工构造的零值 PricingResult。
// 计费系统 fail-safe 原则：时段字段缺失不得把费用清零。
func effectiveTimeMultiplier(p *PricingResult) float64 {
	if p.TimeMultiplier <= 0 {
		return 1.0
	}
	return p.TimeMultiplier
}

// calculateTieredCostFromTiers 从给定的阶梯数组计算费用（租户自定义阶梯或平台阶梯共用；
// 平台阶梯随 GetModelPrice 加载进 PricingResult.CustomTiers，走 600s 定价缓存）
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
				decimal.NewFromInt(remaining).Div(million).Mul(NewFromFloat(price)),
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
				decimal.NewFromInt(useTokens).Div(million).Mul(NewFromFloat(price)),
			)
			remaining -= useTokens
		}
	}

	return InexactFloat64(RoundMoney(totalCostD))
}

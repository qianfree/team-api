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
	// 租户未自定义时回落为平台阶梯（pricing JSONB 的 tiers 数组）
	CustomTiers []pricingTierRow `json:"CustomTiers"`

	// 时段定价配置（pricing JSONB 顶层 time_segments，随定价缓存存储）
	TimeSegments []TimeSegment `json:"time_segments,omitempty"`
	// 时段乘数（读取时按定价时刻评估；缓存条目中的值是写入时刻的结果，仅作参考）
	TimeMultiplier float64 `json:"time_multiplier"`
	// 命中的时段名（写入计费快照，供账单解释；未命中为空）
	TimeRuleName string `json:"time_rule_name"`

	// 按秒计费矩阵（billing_mode=per_second，mdl_pricing.pricing JSONB 的 prices 键）：
	// 分辨率/规格 → 每秒单价（本位币）。"*" 为兜底价，规格未命中时使用。
	// 旧缓存条目无此字段（zero value = 空 map），token/per_request 模式不受影响。
	PerSecondPrices map[string]float64 `json:"per_second_prices,omitempty"`

	// 参数倍率规则（pricing JSONB 顶层 param_multipliers，横切所有模式）：
	// 按归一化任务体参数匹配，命中规则连乘（EvalParamMultipliers）。
	// 旧缓存条目无此字段（zero value），不受影响。
	ParamMultipliers []ParamRule `json:"param_multipliers,omitempty"`

	// 计费方案（pricing JSONB 顶层 scheme）：空 = generic 通用引擎；
	// 特殊方案的估算分发见 estimateTaskCost / LookupScheme。
	// 旧缓存条目无此字段（空串），不受影响
	Scheme string `json:"scheme,omitempty"`
	// 方案私有配置（不透明容器，方案自行解析）。旧缓存条目无此字段（nil）
	SchemeConfig json.RawMessage `json:"scheme_config,omitempty"`
}

// perSecondWildcard 矩阵兜底价键：规格未命中时使用
const perSecondWildcard = "*"

// BillingModeSpecial 特殊计费模式（与 per_second 平级）：仅由特殊计费方案使用，
// pricing JSONB 顶层必须同时声明 scheme 键（写侧双向配对校验）。计费分发只看
// Scheme 不看此值，它用于展示层区分「按秒计费」与「特殊方案组合计费」，
// 不出现在通用计费模式下拉中（只由方案专属编辑器提交）。
const BillingModeSpecial = "special"

// maxTaskDurationSeconds per_second 计费的时长上限（秒）。
// 时长来自用户请求（spec.duration 经 ratios 流入，metadata 路径绕过请求层校验），
// 钳制防「天价 duration 刷预扣漏洞/恶意配错」；正常视频模型上限远低于此值。
const maxTaskDurationSeconds = 120.0

// defaultTaskDurationSeconds per_second 计费缺省时长（秒）：请求未携带时长信号时的估算基准
const defaultTaskDurationSeconds = 5.0

// LookupPerSecondPrice 按秒计费矩阵查价：spec 未命中依次回退 "*" 兜底价、矩阵最低价。
// 返回 0 表示矩阵为空/全零（调用方按未配价兜底处理）。
func LookupPerSecondPrice(prices map[string]float64, spec string) float64 {
	if len(prices) == 0 {
		return 0
	}
	if p, ok := prices[spec]; ok && p > 0 {
		return p
	}
	if p, ok := prices[perSecondWildcard]; ok && p > 0 {
		return p
	}
	// 无 "*" 且未命中：取矩阵最低价兜底，保证新规格上线未配价不 fail
	min := 0.0
	for _, p := range prices {
		if p > 0 && (min == 0 || p < min) {
			min = p
		}
	}
	return min
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

	// 2. 从 mdl_pricing 获取定价：每模型一行（uk_mdl_pricing_model），pricing JSONB 为唯一真相
	type pricingRow struct {
		BillingMode string `json:"billing_mode"`
		Pricing     string `json:"pricing"` // JSONB 计费详情（按 billing_mode 单模式存储）
	}

	var pricing *pricingRow
	err = dao.MdlPricing.Ctx(ctx).
		Where("model_id", model.ID).
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
	var platformTiers []pricingTierRow
	var perSecondPrices map[string]float64
	var timeSegments []TimeSegment
	var paramMultipliers []ParamRule
	var schemeName string
	var schemeConfig json.RawMessage

	if pricing != nil {
		if pricing.BillingMode != "" {
			billingMode = pricing.BillingMode
		}
		// pricing JSONB 解析（唯一真相；空/解析失败按未配价处理，validatePricingConfigured fail-closed 拦截）
		if pricing.Pricing != "" && pricing.Pricing != "null" {
			blob := &PricingBlob{}
			if err := json.Unmarshal([]byte(pricing.Pricing), blob); err != nil {
				g.Log().Warningf(ctx, "billing: 模型 %s 定价 JSON 解析失败，按未配价处理: %v", modelName, err)
			} else {
				if blob.InputPrice != nil {
					inputPrice = *blob.InputPrice
					baseInputPrice = *blob.InputPrice
				}
				if blob.OutputPrice != nil {
					outputPrice = *blob.OutputPrice
					baseOutputPrice = *blob.OutputPrice
				}
				if blob.CacheReadPrice != nil {
					cacheReadPrice = *blob.CacheReadPrice
				}
				if blob.CacheCreationPrice != nil {
					cacheCreationPrice = *blob.CacheCreationPrice
				}
				if blob.Price != nil {
					perRequestPrice = *blob.Price
				}
				platformTiers = blob.Tiers
				perSecondPrices = blob.Prices
				timeSegments = blob.TimeSegments
				paramMultipliers = blob.ParamMultipliers
				schemeName = blob.Scheme
				schemeConfig = blob.SchemeConfig
			}
		}
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
		// 特殊计费方案模型的 billing_mode 由平台方案决定（special），租户级模式覆盖无效：
		// 方案的输出生成组件消费平台 per_second 矩阵，租户改模式只会造成日志标签与计费口径错位
		if schemeName == "" && tm.BillingMode != nil && *tm.BillingMode != "" {
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

	// 2.5 平台阶梯兜底：租户未自定义阶梯时使用平台阶梯（pricing JSONB tiers 数组）。
	// 计费模式非 tiered 时 computeCost 不会读取 CustomTiers，无副作用
	if len(customTiers) == 0 {
		customTiers = platformTiers
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
		Currency:             Currency(ctx),
		CacheReadPrice:       cacheReadPrice,
		CacheCreationPrice:   cacheCreationPrice,
		CacheCreation5mPrice: cacheCreationPrice,       // 5m TTL = cache_creation_price 基础价（对应官方 1.25× 输入价）
		CacheCreation1hPrice: cacheCreationPrice * 1.6, // 1h TTL = 1.6× 基础价（因为 2.0÷1.25=1.6，对应官方 2× 输入价）
		MaxOutputTokens:      model.MaxOutputTokens,
		CustomTiers:          customTiers,
		TimeSegments:         timeSegments,
		PerSecondPrices:      perSecondPrices,
		ParamMultipliers:     paramMultipliers,
		Scheme:               schemeName,
		SchemeConfig:         schemeConfig,
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
	case BillingModeSpecial, "per_second":
		if LookupPerSecondPrice(pricing.PerSecondPrices, perSecondWildcard) <= 0 {
			return gerror.Wrapf(rcommon.ErrModelPricingNotConfigured, "model=%s (%s prices not set)", modelName, pricing.BillingMode)
		}
	default: // token / tiered
		if pricing.InputPrice <= 0 && pricing.OutputPrice <= 0 && !tiersHavePrice(pricing.CustomTiers) {
			return gerror.Wrapf(rcommon.ErrModelPricingNotConfigured, "model=%s", modelName)
		}
	}
	return nil
}

// PricingItemInput BuildPricingBlob 的输入项（管理端 SetModelPricing / 模型导入共用，
// 与 api/admin/v1.PricingItem 字段一一对应但避免 billing 反向依赖 api 层）
type PricingItemInput struct {
	BillingMode        string
	MinTokens          int64
	MaxTokens          *int64
	InputPrice         float64
	OutputPrice        float64
	PerRequestPrice    *float64
	CacheReadPrice     float64
	CacheCreationPrice float64
	PerSecondPrices    map[string]float64
}

// BuildPricingBlob 从全量替换语义的定价项列表构造 pricing JSONB（按 billing_mode 单模式存储）。
// 校验规则（设计文档 3.4）：金额非负；per_second 矩阵非空且至少一档正价（建议配 "*" 兜底），
// 仅允许单锚点行；tiered 阶梯按 min_tokens 升序整理。
func BuildPricingBlob(items []PricingItemInput) (*PricingBlob, error) {
	if len(items) == 0 {
		return &PricingBlob{}, nil
	}
	mode := items[0].BillingMode
	anchor := items[0]

	switch mode {
	case BillingModeSpecial, "per_second":
		if len(items) > 1 {
			return nil, gerror.Newf("%s 计费模式只允许一行定价（分辨率矩阵）", mode)
		}
		if anchor.MinTokens != 0 {
			return nil, gerror.Newf("%s 计费模式要求 min_tokens=0 的定价行", mode)
		}
		if len(anchor.PerSecondPrices) == 0 {
			return nil, gerror.Newf("%s 计费模式要求配置分辨率单价矩阵（per_second_prices）", mode)
		}
		hasPositive := false
		for spec, price := range anchor.PerSecondPrices {
			if spec == "" {
				return nil, gerror.New("per_second_prices 存在空规格键")
			}
			if price < 0 {
				return nil, gerror.Newf("per_second_prices[%s] 单价为负数", spec)
			}
			if price > 0 {
				hasPositive = true
			}
		}
		if !hasPositive {
			return nil, gerror.New("per_second_prices 至少需要一档正价（建议额外配置 \"*\" 兜底价）")
		}
		return &PricingBlob{Unit: "second", Prices: anchor.PerSecondPrices}, nil

	case "per_request":
		blob := &PricingBlob{}
		if anchor.PerRequestPrice != nil {
			blob.Price = anchor.PerRequestPrice
		}
		return blob, nil

	case "tiered":
		blob := &PricingBlob{
			Tiers:              make([]pricingTierRow, 0, len(items)),
			CacheReadPrice:     positivePtr(anchor.CacheReadPrice),
			CacheCreationPrice: positivePtr(anchor.CacheCreationPrice),
		}
		for _, item := range items {
			blob.Tiers = append(blob.Tiers, pricingTierRow{
				MinTokens:          item.MinTokens,
				MaxTokens:          item.MaxTokens,
				InputPrice:         item.InputPrice,
				OutputPrice:        item.OutputPrice,
				CacheReadPrice:     positivePtr(item.CacheReadPrice),
				CacheCreationPrice: positivePtr(item.CacheCreationPrice),
			})
		}
		return blob, nil

	default: // token
		return &PricingBlob{
			InputPrice:         positivePtr(anchor.InputPrice),
			OutputPrice:        positivePtr(anchor.OutputPrice),
			CacheReadPrice:     positivePtr(anchor.CacheReadPrice),
			CacheCreationPrice: positivePtr(anchor.CacheCreationPrice),
		}, nil
	}
}

// positivePtr 正值取指针（0/负值返回 nil，JSONB 中省略该键由旧列兜底语义对齐）
func positivePtr(v float64) *float64 {
	if v > 0 {
		return &v
	}
	return nil
}

// ParsePricingBlob 解析 mdl_pricing.pricing JSONB（管理端/租户端展示路径复用）。
// 空串/null/解析失败返回 nil，调用方按未配价处理。
func ParsePricingBlob(raw string) *PricingBlob {
	if raw == "" || raw == "null" {
		return nil
	}
	blob := &PricingBlob{}
	if err := json.Unmarshal([]byte(raw), blob); err != nil {
		return nil
	}
	return blob
}

// PricingBlob mdl_pricing.pricing JSONB 的内存形态（按 billing_mode 单模式存储，单一真相）。
// 设计约束（docs/模型定价存储JSON化与按秒计费设计.md）：
//   - 数值一律 float64 number（禁止 decimal 直接 marshal 进 JSONB——会变带引号字符串）；
//   - time_segments 并入顶层（横切所有模式）；
//   - 租户覆盖不逐格配：mdl_tenant_models.multiplier 作用于整个矩阵。
type PricingBlob struct {
	// token 模式
	InputPrice         *float64 `json:"input_price,omitempty"`
	OutputPrice        *float64 `json:"output_price,omitempty"`
	CacheReadPrice     *float64 `json:"cache_read_price,omitempty"`
	CacheCreationPrice *float64 `json:"cache_creation_price,omitempty"`
	// tiered 模式：档位数组（min_tokens 升序，最后一档 max_tokens 可为 null）
	Tiers []pricingTierRow `json:"tiers,omitempty"`
	// per_request 模式
	Price *float64 `json:"price,omitempty"`
	// per_second 模式：规格 → 每秒单价矩阵，"*" 为兜底价
	Unit   string             `json:"unit,omitempty"`
	Prices map[string]float64 `json:"prices,omitempty"`
	// 横切：时段定价
	TimeSegments []TimeSegment `json:"time_segments,omitempty"`
	// 横切：参数倍率（按归一化任务体参数匹配，命中规则连乘；所有计费模式共享）
	ParamMultipliers []ParamRule `json:"param_multipliers,omitempty"`
	// 计费方案：特殊计费模型声明引用的方案名（空 = generic 通用引擎），
	// 前端按此字段分发定价编辑器。导出格式不含该字段（特殊方案定价不随导入导出迁移）
	Scheme string `json:"scheme,omitempty"`
	// 方案私有配置（不透明容器）：schema 由方案实现自定义并自行解析校验，
	// 引擎与通用编辑器不理解其内容，防止主干结构随方案膨胀
	SchemeConfig json.RawMessage `json:"scheme_config,omitempty"`
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

// pricingTierRow 定价阶梯行（绝对价格）。
// 逐档缓存价为可选键（omitempty，旧数据缺键 unmarshal 为 nil，无需迁移）：
// 编辑器逐档缓存价落库于此；计费引擎的缓存 token 计费仍取 blob 顶层锚点缓存价，不读档内字段
type pricingTierRow struct {
	MinTokens          int64    `json:"min_tokens"`
	MaxTokens          *int64   `json:"max_tokens"`
	InputPrice         float64  `json:"input_price"`
	OutputPrice        float64  `json:"output_price"`
	CacheReadPrice     *float64 `json:"cache_read_price,omitempty"`
	CacheCreationPrice *float64 `json:"cache_creation_price,omitempty"`
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

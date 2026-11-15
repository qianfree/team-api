package billing

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/shopspring/decimal"

	"github.com/qianfree/team-api/internal/logic/billing/formula"
	rcommon "github.com/qianfree/team-api/relay/common"
)

// truncateFormulaExpr 公式原文截断（快照留痕用，完整原文以 mdl_pricing.pricing 为准）
func truncateFormulaExpr(src string) string {
	if len(src) > 200 {
		return src[:200] + "..."
	}
	return src
}

// BillingSnapshot 计费快照（写入 JSONB）
type BillingSnapshot struct {
	Pricing     BillingSnapshotPricing      `json:"pricing"`
	Multipliers BillingSnapshotMultipliers  `json:"multipliers"`
	CachePrices *BillingSnapshotCachePrices `json:"cache_prices,omitempty"`
	// TokenCosts token 分项费用（formula 模式不填充：公式没有逐类 token 单价，
	// 分项明细见 Formula.Items）
	TokenCosts  map[string]TokenCostDetail `json:"token_costs,omitempty"`
	Settlement  BillingSnapshotSettlement  `json:"settlement"`
	RequestMeta BillingSnapshotRequestMeta `json:"request_meta"`
	// PerSecond 按秒/方案计费的命中明细（仅任务结算路径、按秒矩阵参与定价时填充）：
	// 记录实际计费秒数、命中档位与单价，配合 multipliers 使
	// 「秒数 × 档位单价 × 乘数链 ≈ 实际费用」可复算；特殊方案含素材组件时该节描述输出生成组件
	PerSecond *BillingSnapshotPerSecond `json:"per_second,omitempty"`
	// Formula 表达式计费命中明细（仅 billing_mode=formula 填充）：命中档位、变量取值、
	// 分项明细（线性计价式）与公式计价，配合 multipliers 使费用可复算。
	// 公式原文（expr/base_expr）仅供管理端展示，文本摘要不输出（租户可见）
	Formula *BillingSnapshotFormula `json:"formula,omitempty"`
	// ToolSurcharges 工具调用附加费明细（联网搜索/云端工具按次收费，任何计费模式可叠加）：
	// 逐工具记录次数/结算时刻单价/费用，使「次数 × 单价 = 费用」可复算。
	// 工具费独立计算，任何乘数（租户/时段/参数倍率）均不作用。
	// 单价为结算时刻全局工具价格表（sys_options tool_call_prices）的解析值，价格后续
	// 调整不影响历史账单解释。仅发生工具计费时填充；旧快照无此键，读取方按空处理
	ToolSurcharges []ToolSurchargeItem `json:"tool_surcharges,omitempty"`
}

// BillingSnapshotFormula 表达式计费命中明细
type BillingSnapshotFormula struct {
	// Expr 求值表达式原文（基础公式 × 请求规则的组合式，超长截断 200 字符，
	// 完整原文见 mdl_pricing.pricing）
	Expr string `json:"expr"`
	// BaseExpr 基础公式原文（不含请求规则；与 Expr 相同时省略）
	BaseExpr string `json:"base_expr,omitempty"`
	// Tiers 命中的档位名（公式用 tier("名称", 计价式) 标记档位时填充，按求值顺序）
	Tiers []string `json:"tiers,omitempty"`
	// Rules 请求规则命中表（header()/param()/时间条件倍率：条件原文、乘数、是否命中）
	Rules []formula.RequestRuleTrace `json:"rules,omitempty"`
	// Vars 求值时的变量取值（仅公式引用的变量；u:key/header:key/param:path
	// 为探针实际取值，header/param 值可能为字符串）。与 Factor 出自同一次求值
	Vars map[string]any `json:"vars,omitempty"`
	// Factor 公式结果（每 1M 口径系数），BaseCost = Factor / 1e6
	Factor float64 `json:"factor"`
	// BaseCost 公式计价（本位币，已含请求规则倍率，未乘租户/时段/附加乘数）
	BaseCost float64 `json:"base_cost"`
	// Multiplier 公式计价之外的乘数链（租户 × 时段 × 附加乘数），Cost = BaseCost × Multiplier
	Multiplier float64 `json:"multiplier"`
	// Cost 公式部分费用（不含工具附加费与最低消费调整）
	Cost float64 `json:"cost"`
	// Items 分项明细（命中档位或整体公式为线性计价式时填充）：
	// Σ Items.Cost = Cost（逐项四舍五入误差除外）；非线性公式为空，账单展示变量取值
	Items []FormulaCostItem `json:"items,omitempty"`
	// RuleMultiplier 命中请求规则乘数之积（Items 非空且 ≠1 时填充，已并入 Items.Multiplier）
	RuleMultiplier float64 `json:"rule_multiplier,omitempty"`
	// Error 求值失败原因（fail-safe 按预扣额结算时记录；正常为空）
	Error string `json:"error,omitempty"`
}

// FormulaCostItem 表达式计费分项：数量 × 单价 × 乘数 = 费用
type FormulaCostItem struct {
	// Var 数量来源：公式变量名 / u:key / 空串（按次固定金额）
	Var   string `json:"var"`
	Label string `json:"label"`
	// Unit 数量单位（tokens/张/秒/次；u:key 计量为空）
	Unit     string  `json:"unit,omitempty"`
	Quantity float64 `json:"quantity"`
	// UnitPrice 单价（PerMillion=true 时为每 1M token 单价，否则为每单位单价）
	UnitPrice  float64 `json:"unit_price"`
	PerMillion bool    `json:"per_million,omitempty"`
	// Multiplier 该项已乘的综合乘数（请求规则 × 租户 × 时段 × 附加乘数）
	Multiplier float64 `json:"multiplier"`
	Cost       float64 `json:"cost"`
}

// BillingSnapshotPerSecond 按秒计费命中明细
type BillingSnapshotPerSecond struct {
	// Resolution 命中的矩阵键（官方 usage 实际分辨率优先，回退提交时请求档位；
	// 空 = 矩阵未按档位配置，经通配/最低价兜底查价）
	Resolution string `json:"resolution,omitempty"`
	// Seconds 计费秒数（官方 usage 实际秒数优先——智能时长模式下可能与请求时长不同，回退请求时长）
	Seconds float64 `json:"seconds"`
	// UnitPrice 命中的每秒单价（本位币）
	UnitPrice float64 `json:"unit_price"`
}

// BillingSnapshotPricing 价格来源信息
type BillingSnapshotPricing struct {
	BaseInputPrice       float64 `json:"base_input_price"`
	BaseOutputPrice      float64 `json:"base_output_price"`
	EffectiveInputPrice  float64 `json:"effective_input_price"`
	EffectiveOutputPrice float64 `json:"effective_output_price"`
	BillingMode          string  `json:"billing_mode"`
	BillingSource        string  `json:"billing_source"`
	// Scheme 命中的特殊计费方案名（pricing JSONB 顶层 scheme 键）；空 = 通用引擎。
	// 供账单事后追溯该笔走的计费方案，摘要据此追加「计费方案」行
	Scheme string `json:"scheme,omitempty"`
	// PerSecondPrices 按秒单价矩阵（仅 per_second / special 模式填充，其余模式 omitted）：
	// special 的矩阵是输出生成组件的定价依据，快照携带供账单解释
	PerSecondPrices map[string]float64 `json:"per_second_prices,omitempty"`
	// PerRequestPrice 按次单价（仅 per_request 模式填充）：摘要的「按次单价」行取此值。
	// 此前误用 EffectiveInputPrice（token 输入价，纯按次模型恒为 0，展示「按次单价: $0」）
	PerRequestPrice float64 `json:"per_request_price,omitempty"`
	// PerRequestCount 按次计费张数（图片协议顶层 n，>1 时费用 = 单价 × 张数）。
	// 摘要据此展示「张数: ×n」；旧快照无此字段，读取方按 1 处理
	PerRequestCount int `json:"per_request_count,omitempty"`
}

// BillingSnapshotMultipliers 倍率信息
type BillingSnapshotMultipliers struct {
	ModelMultiplier  float64 `json:"model_multiplier"`
	TenantMultiplier float64 `json:"tenant_multiplier"`
	DiscountRatio    float64 `json:"discount_ratio"`
	RateMultiplier   float64 `json:"rate_multiplier"`
	TimeMultiplier   float64 `json:"time_multiplier"` // 时段乘数（未启用时段定价时为 1）
	TimeRule         string  `json:"time_rule"`       // 命中的时段名（供账单解释；未命中为空）
	// RatioMultipliers 计费上下文实际应用的附加乘数（任务路径：video_input 折扣、quality、
	// duration_multiplier、param_multiplier 等；与实际乘法链一致，乘数=1 也如实列出）。
	// 缺失该清单时「数量 × 单价 × 租户/时段倍率 = 费用」在附加乘数 ≠ 1 时无法复算。
	// 同步对话路径无 ratios，omitempty 省略；旧快照无此字段，读取方按空处理
	RatioMultipliers map[string]float64 `json:"ratio_multipliers,omitempty"`
	// ParamMatched 参数倍率命中的规则说明（param_matched，| 分隔），解释 param_multiplier 来源
	ParamMatched string `json:"param_matched,omitempty"`
}

// BillingSnapshotCachePrices 缓存价格信息
type BillingSnapshotCachePrices struct {
	CacheReadPrice float64 `json:"cache_read_price"`
	// CacheCreationPrice 5m TTL 基准价（cache_creation_price，Claude 5m 与 OpenAI cache_write 共用）
	CacheCreationPrice float64 `json:"cache_creation_price"`
	// CacheCreation1hPrice 实际生效的 1h TTL 价（显式配置或基准 ×1.6 推导），
	// 快照携带推导值使「1h tokens/1M × 单价 = 费用」可复算；旧快照无此键，读取方按 0 处理
	CacheCreation1hPrice float64 `json:"cache_creation_1h_price,omitempty"`
}

// TokenCostDetail 单类 token 的费用明细
type TokenCostDetail struct {
	Tokens    int     `json:"tokens"`
	UnitPrice float64 `json:"unit_price"`
	// Multiplier 该分项已乘的综合倍率（租户倍率 × 时段乘数）。
	// Cost 是已乘倍率的实际费用而 UnitPrice 是未乘倍率的原价，二者口径不同：
	// 缺少该字段时「tokens/1M × unit_price = cost」表面不成立（折上折观感），
	// 写入后完整算式 tokens/1M × unit_price × multiplier = cost 可自洽复算。
	// 恰为 1（无折扣）时 omitempty 省略；旧快照无此字段，读取方按 1 处理。
	Multiplier float64 `json:"multiplier,omitempty"`
	Cost       float64 `json:"cost"`
}

// BillingSnapshotSettlement 结算信息
type BillingSnapshotSettlement struct {
	PreDeductAmount  float64 `json:"pre_deduct_amount"`
	ActualCost       float64 `json:"actual_cost"`
	RefundAmount     float64 `json:"refund_amount"`
	SupplementAmount float64 `json:"supplement_amount"`
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
			Scheme:               pricing.Scheme,
			PerSecondPrices:      pricing.PerSecondPrices,
			PerRequestPrice:      pricing.PerRequestPrice,
			PerRequestCount:      breakdown.PerRequestCount,
		},
		Multipliers: BillingSnapshotMultipliers{
			ModelMultiplier:  pricing.ModelMultiplier,
			TenantMultiplier: pricing.TenantMultiplier,
			DiscountRatio:    pricing.DiscountRatio,
			RateMultiplier:   pricing.TenantMultiplier,
			TimeMultiplier:   effectiveTimeMultiplier(pricing),
			TimeRule:         pricing.TimeRuleName,
		},
	}
	if pricing.BillingMode != BillingModeFormula {
		snapshot.TokenCosts = buildTokenCosts(pricing, breakdown)
	}

	// 任务计费要素（仅任务结算路径填充）：附加乘数清单 + 按秒命中明细随快照留痕
	if breakdown.TaskFacts != nil {
		snapshot.Multipliers.RatioMultipliers = breakdown.TaskFacts.AppliedRatios
		snapshot.Multipliers.ParamMatched = breakdown.TaskFacts.ParamMatched
		if breakdown.TaskFacts.PerSecond != nil {
			snapshot.PerSecond = &BillingSnapshotPerSecond{
				Resolution: breakdown.TaskFacts.PerSecond.Resolution,
				Seconds:    breakdown.TaskFacts.PerSecond.Seconds,
				UnitPrice:  breakdown.TaskFacts.PerSecond.UnitPrice,
			}
		}
	}

	// 工具调用附加费明细（仅发生工具计费时填充）：固定单价 × 次数，
	// 「次数 × 单价 = cost」自洽复算（不乘任何倍率）
	if len(breakdown.ToolItems) > 0 {
		snapshot.ToolSurcharges = breakdown.ToolItems
	}

	// Cache 比率（仅当有 cache token 时填充）
	if breakdown.CacheReadTokens > 0 || breakdown.CacheCreationTokens > 0 {
		snapshot.CachePrices = &BillingSnapshotCachePrices{
			CacheReadPrice:       pricing.CacheReadPrice,
			CacheCreationPrice:   pricing.CacheCreationPrice,
			CacheCreation1hPrice: pricing.CacheCreation1hPrice,
		}
	}

	if pricing.BillingMode == BillingModeFormula {
		snapshot.Formula = buildFormulaSnapshot(pricing, breakdown)
	}

	// 结算信息
	if settlement != nil {
		snapshot.Settlement = BillingSnapshotSettlement{
			PreDeductAmount:  settlement.PreDeductAmount,
			ActualCost:       settlement.ActualCost,
			RefundAmount:     settlement.RefundAmount,
			SupplementAmount: settlement.SupplementAmount,
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

// buildFormulaSnapshot 表达式计费明细：命中档位/变量取值/分项全部取自求值结果
// （breakdown.FormulaDetail，与系数同一次求值），不事后重建变量环境——任务路径的
// seconds/张数/u() 计量与同步路径的 token 口径不同，重建会导致变量与系数矛盾
func buildFormulaSnapshot(pricing *PricingResult, breakdown *CostBreakdown) *BillingSnapshotFormula {
	snap := &BillingSnapshotFormula{
		Expr:     truncateFormulaExpr(pricing.Formula),
		BaseExpr: truncateFormulaExpr(pricing.FormulaBase),
		Rules:    breakdown.RequestRules,
		Factor:   breakdown.FormulaFactor,
		Error:    breakdown.FormulaError,
	}
	if pricing.FormulaBase == pricing.Formula {
		snap.BaseExpr = "" // 无请求规则时 Expr 即基础公式，避免冗余
	}
	if snap.Error != "" {
		return snap
	}

	million := decimal.NewFromInt(1_000_000)
	mulD := combinedMultiplier(pricing, breakdown)
	baseD := NewFromFloat(breakdown.FormulaFactor).Div(million)
	snap.BaseCost = InexactFloat64(RoundMoney(baseD))
	snap.Multiplier = InexactFloat64(mulD)
	snap.Cost = InexactFloat64(RoundMoney(baseD.Mul(mulD)))

	detail := breakdown.FormulaDetail
	if detail == nil {
		return snap
	}
	snap.Tiers = detail.Tiers
	snap.Vars = detail.Vars
	if detail.Items == nil {
		return snap
	}
	ruleMul := detail.RuleMultiplier
	if ruleMul <= 0 {
		ruleMul = 1
	}
	if ruleMul != 1 {
		snap.RuleMultiplier = ruleMul
	}
	itemMulD := NewFromFloat(ruleMul).Mul(mulD)
	itemMul := InexactFloat64(itemMulD)
	items := make([]FormulaCostItem, 0, len(detail.Items))
	for _, it := range detail.Items {
		perMillion := formula.IsTokenVar(it.Var)
		unitPrice := it.Coef
		if !perMillion {
			// 非 token 计量（张/秒/次/u() 计量）按每单位单价展示：系数 / 1e6
			unitPrice = InexactFloat64(NewFromFloat(it.Coef).Div(million))
		}
		cost := NewFromFloat(it.Quantity).Mul(NewFromFloat(it.Coef)).Div(million).Mul(itemMulD)
		items = append(items, FormulaCostItem{
			Var:        it.Var,
			Label:      formula.VarLabel(it.Var),
			Unit:       formula.VarUnit(it.Var),
			Quantity:   it.Quantity,
			UnitPrice:  unitPrice,
			PerMillion: perMillion,
			Multiplier: itemMul,
			Cost:       InexactFloat64(RoundMoney(cost)),
		})
	}
	snap.Items = items
	return snap
}

// combinedMultiplier 计价之外的综合乘数链：租户 × 时段 × 附加乘数（任务路径 ratios）。
// 附加乘数按键名排序后连乘：map 迭代序不确定，排序保证快照数值跨次一致；
// decimal 相乘避免 0.85×1.2 类 IEEE 尾差进入快照与展示算式
func combinedMultiplier(pricing *PricingResult, breakdown *CostBreakdown) decimal.Decimal {
	mulD := NewFromFloat(pricing.TenantMultiplier).Mul(NewFromFloat(effectiveTimeMultiplier(pricing)))
	if breakdown.TaskFacts != nil && len(breakdown.TaskFacts.AppliedRatios) > 0 {
		keys := make([]string, 0, len(breakdown.TaskFacts.AppliedRatios))
		for k := range breakdown.TaskFacts.AppliedRatios {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			mulD = mulD.Mul(NewFromFloat(breakdown.TaskFacts.AppliedRatios[k]))
		}
	}
	return mulD
}

// buildTokenCosts 构建各类 token 的费用明细。
// Cost 分项为已乘综合倍率的实际费用（与 computeCost 的 InputCost/OutputCost 口径一致），
// Multiplier 随行写入使「tokens/1M × unit_price × multiplier = cost」可自洽复算。
// 任务路径的费用含 ratios 附加乘数（video_input 折扣等，经 RecalculateByTokens），
// 乘数链必须把附加乘数一并纳入，否则算式两边在附加乘数 ≠ 1 时不相等。
func buildTokenCosts(pricing *PricingResult, breakdown *CostBreakdown) map[string]TokenCostDetail {
	mul := InexactFloat64(combinedMultiplier(pricing, breakdown))

	detail := func(tokens int, unitPrice, cost float64) TokenCostDetail {
		return TokenCostDetail{Tokens: tokens, UnitPrice: unitPrice, Multiplier: mul, Cost: cost}
	}

	costs := map[string]TokenCostDetail{
		"input":  detail(breakdown.InputTokens, pricing.InputPrice, breakdown.InputCost),
		"output": detail(breakdown.OutputTokens, pricing.OutputPrice, breakdown.OutputCost),
	}

	if breakdown.CacheReadTokens > 0 {
		// direct cache price
		costs["cache_read"] = detail(breakdown.CacheReadTokens, pricing.CacheReadPrice, breakdown.CacheReadCost)
	}

	if breakdown.CacheCreationTokens > 0 {
		// direct cache creation price（含 OpenAI cache_write 并入的写入 token，合并桶）
		costs["cache_creation"] = detail(breakdown.CacheCreationTokens, pricing.CacheCreationPrice, breakdown.CacheCreationCost)
		// TTL 分桶明细（观测细分；OpenAI cache_write 与无细分报文全落 5m 桶，1h 桶仅
		// Claude ephemeral_1h 触发。前端日志页已预置这两个键的展示标签）
		if breakdown.CacheCreation5mTokens > 0 {
			costs["cache_creation_5m"] = detail(breakdown.CacheCreation5mTokens, pricing.CacheCreation5mPrice, breakdown.CacheCreation5mCost)
		}
		if breakdown.CacheCreation1hTokens > 0 {
			costs["cache_creation_1h"] = detail(breakdown.CacheCreation1hTokens, pricing.CacheCreation1hPrice, breakdown.CacheCreation1hCost)
		}
	}

	return costs
}

// GenerateBillingSummary 生成人类可读的计费摘要文本（中文）。
// 金额符号取系统本位币（billing_currency），摘要为写入时快照，货币在初始化后不可更改。
func GenerateBillingSummary(ctx context.Context, snapshot *BillingSnapshot) string {
	if snapshot == nil {
		return ""
	}

	// 货币符号跟随本位币；money 统一拼接符号 + 价格
	sym := CurrencySymbol(ctx)
	money := func(v float64) string { return sym + formatPrice(v) }

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
		"token":            "按量计费",
		"per_request":      "按次计费",
		"tiered":           "阶梯计费",
		"per_second":       "按秒计费",
		BillingModeSpecial: "特殊计费",
		BillingModeFormula: "表达式计费",
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
	// 特殊计费方案行：方案名直出（custom:minimax-material 等），便于账单事后追溯；
	// 不做后端中文映射表——新增方案不应要求改动本文件（与 scheme 注册表设计一致）
	if snapshot.Pricing.Scheme != "" {
		lines = append(lines, fmt.Sprintf("计费方案: %s", snapshot.Pricing.Scheme))
	}
	lines = append(lines, "---")

	isFormula := snapshot.Pricing.BillingMode == BillingModeFormula && snapshot.Formula != nil
	if isFormula {
		// 表达式计费：命中档位 + 分项明细（线性计价式）或变量取值 + 公式计价（非线性）。
		// 摘要租户可见，不输出公式原文（原文仅在快照 formula.expr 供管理端查看）
		lines = append(lines, formulaSummaryLines(snapshot.Formula, money)...)
	} else if snapshot.Pricing.BillingMode == "per_request" {
		// 按次单价取 PerRequestPrice（此前误用 EffectiveInputPrice = token 输入价，
		// 纯按次模型恒为 0，摘要展示「按次单价: $0.000000」）
		lines = append(lines, fmt.Sprintf("按次单价: %s", money(snapshot.Pricing.PerRequestPrice)))
		// 图片协议一次生成 n 张时按张数放大（张数>1 才展示，旧快照按 1 处理）
		if snapshot.Pricing.PerRequestCount > 1 {
			lines = append(lines, fmt.Sprintf("张数: ×%d", snapshot.Pricing.PerRequestCount))
		}
	} else {
		// 各类 token 费用明细：tokens × 原价/1M × 倍率 = 实际费用。
		// Cost 分项已含折扣（computeCost 分项 × mul），倍率必须显式进入算式，
		// 否则「原价单价 = 折后费用」表面不成立；无折扣时省略倍率段。
		// 任务路径的行倍率含 ratios 附加乘数（buildTokenCosts 已并入乘数链）。
		// 缓存创建：新快照有 5m/1h 分桶键时按桶展示（合并桶行省略，防止合计重复
		// 累加）；旧快照只有合并桶，照旧行展示
		tokenRows := []struct {
			label string
			key   string
		}{
			{"输入", "input"},
			{"输出", "output"},
			{"缓存读取", "cache_read"},
		}
		if _, has5m := snapshot.TokenCosts["cache_creation_5m"]; has5m {
			tokenRows = append(tokenRows,
				struct{ label, key string }{"缓存创建(5分钟)", "cache_creation_5m"},
				struct{ label, key string }{"缓存创建(1小时)", "cache_creation_1h"},
			)
		} else {
			tokenRows = append(tokenRows, struct{ label, key string }{"缓存创建", "cache_creation"})
		}
		var costParts []string
		for _, row := range tokenRows {
			tc, ok := snapshot.TokenCosts[row.key]
			if !ok || tc.Tokens <= 0 {
				continue
			}
			multPart := ""
			if tc.Multiplier > 0 && tc.Multiplier != 1.0 {
				multPart = fmt.Sprintf(" × %s", formatMultiplier(tc.Multiplier))
			}
			lines = append(lines, fmt.Sprintf("%s: %s tokens × %s/1M%s = %s",
				row.label, formatInt(tc.Tokens), money(tc.UnitPrice), multPart, money(tc.Cost)))
			if tc.Cost > 0 {
				costParts = append(costParts, money(tc.Cost))
			}
		}
		if len(costParts) > 1 {
			lines = append(lines, fmt.Sprintf("合计: (%s) = %s",
				joinWithPlus(costParts), money(snapshot.Settlement.ActualCost)))
		}
	}

	// 工具调用附加费行（任何计费模式可叠加，仅发生工具计费时快照携带）：
	// 逐工具「名称: N 次 × 单价/次 = 费用」。工具费独立计算，任何乘数（租户/时段/参数倍率）
	// 均不作用——算式两端严格相等，无需倍率段。单价为结算时刻全局工具价格表的解析值
	// （快照记录解析值而非配置引用，历史账单不随价格表变动漂移）
	toolNameMap := map[string]string{
		"web_search":                    "联网搜索",
		"web_search_preview":            "联网搜索(preview)",
		"google_search":                 "Google检索",
		"google_search_grounded_prompt": "Google检索(按请求)",
		"file_search":                   "文件检索",
		"web_fetch":                     "网页抓取",
		"code_execution":                "代码执行",
	}
	for _, item := range snapshot.ToolSurcharges {
		label := toolNameMap[item.Name]
		if label == "" {
			label = item.Name
		} else {
			label = fmt.Sprintf("%s(%s)", label, item.Name)
		}
		lines = append(lines, fmt.Sprintf("%s: %s 次 × %s/次 = %s",
			label, formatInt(item.Count), money(item.UnitPrice), money(item.Cost)))
	}

	// 按秒计费命中明细行（仅任务结算路径且按秒矩阵参与定价时快照携带）：
	// 展示实际计费秒数与命中档位单价。秒数以官方素材计量为准（智能时长模式下与请求时长不同），
	// 是按秒计费的核心计价依据；费用 = 秒数 × 单价 × 下方乘数链（特殊方案另有素材组件，见计费方案行）
	if snapshot.PerSecond != nil {
		specPart := ""
		if snapshot.PerSecond.Resolution != "" {
			specPart = fmt.Sprintf("（档位 %s）", snapshot.PerSecond.Resolution)
		}
		lines = append(lines, fmt.Sprintf("按秒计费: %s 秒 × %s/秒%s",
			formatMultiplier(snapshot.PerSecond.Seconds), money(snapshot.PerSecond.UnitPrice), specPart))
	}

	// 表达式计费合计行：放在工具附加费之后，使「公式分项 + 工具费 = 实际费用」一行可核对
	if isFormula {
		lines = append(lines, formulaTotalLines(snapshot, money)...)
	}

	// 倍率说明行（所有计费模式通用）：租户/时段/附加乘数逐项列出，任一 ≠ 1 或存在附加乘数时展示。
	// 分项费用已含全部乘数，合计 = 各分项之和 = 实际费用；此行解释乘数构成，
	// 缺失附加乘数（video_input 折扣、param_multiplier 等）时算式无法复算。
	// 不再展示「(分项之和) × 倍率 = 实际」——旧格式的分项是折后值，再乘倍率
	// 呈现折上折观感，算式两边对不上（分项之和本就等于实际费用）
	effTenant := snapshot.Multipliers.TenantMultiplier
	effTime := snapshot.Multipliers.TimeMultiplier
	if effTime <= 0 {
		effTime = 1.0
	}
	// 附加乘数按键名排序：map 迭代序不确定，排序保证摘要文本跨次一致（快照可重放）
	ratioKeys := make([]string, 0, len(snapshot.Multipliers.RatioMultipliers))
	for k := range snapshot.Multipliers.RatioMultipliers {
		ratioKeys = append(ratioKeys, k)
	}
	sort.Strings(ratioKeys)
	// 表达式计费分项的行乘数含请求规则倍率，倍率构成里一并列出
	formulaRuleMul := 0.0
	if isFormula && len(snapshot.Formula.Items) > 0 && snapshot.Formula.RuleMultiplier > 0 && snapshot.Formula.RuleMultiplier != 1 {
		formulaRuleMul = snapshot.Formula.RuleMultiplier
	}
	if (effTenant > 0 && effTenant != 1.0) || effTime != 1.0 || len(ratioKeys) > 0 || formulaRuleMul > 0 {
		var parts []string
		if formulaRuleMul > 0 {
			parts = append(parts, fmt.Sprintf("请求规则(%s)", formatMultiplier(formulaRuleMul)))
		}
		if effTenant > 0 && effTenant != 1.0 {
			parts = append(parts, fmt.Sprintf("租户倍率(%s)", formatMultiplier(effTenant)))
		}
		if effTime != 1.0 {
			parts = append(parts, fmt.Sprintf("时段乘数(%s)", formatMultiplier(effTime)))
		}
		for _, k := range ratioKeys {
			factor := fmt.Sprintf("%s(%s)", k, formatMultiplier(snapshot.Multipliers.RatioMultipliers[k]))
			if k == ratioKeyParamMultiplier && snapshot.Multipliers.ParamMatched != "" {
				factor += fmt.Sprintf("（%s）", snapshot.Multipliers.ParamMatched)
			}
			parts = append(parts, factor)
		}
		lines = append(lines, "已应用倍率: "+strings.Join(parts, " × "))
	}

	// 时段定价行：放在 token/per_request 分支之后统一展示，保证账单可解释（按次计费同样适用时段乘数）
	if snapshot.Multipliers.TimeRule != "" && snapshot.Multipliers.TimeMultiplier > 0 && snapshot.Multipliers.TimeMultiplier != 1.0 {
		lines = append(lines, fmt.Sprintf("时段: %s ×%.2f", snapshot.Multipliers.TimeRule, snapshot.Multipliers.TimeMultiplier))
	}

	// 结算信息
	lines = append(lines, "---")
	s := snapshot.Settlement
	if s.PreDeductAmount > 0 {
		if s.RefundAmount > 0 {
			lines = append(lines, fmt.Sprintf("预扣: %s → 实际: %s → 退还: %s",
				money(s.PreDeductAmount), money(s.ActualCost), money(s.RefundAmount)))
		} else if s.SupplementAmount > 0 {
			lines = append(lines, fmt.Sprintf("预扣: %s → 实际: %s → 补扣: %s",
				money(s.PreDeductAmount), money(s.ActualCost), money(s.SupplementAmount)))
		} else {
			lines = append(lines, fmt.Sprintf("预扣: %s → 实际: %s（无差额）",
				money(s.PreDeductAmount), money(s.ActualCost)))
		}
	} else if s.ActualCost > 0 {
		lines = append(lines, fmt.Sprintf("实际费用: %s", money(s.ActualCost)))
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

// formulaSummaryLines 表达式计费明细行（摘要租户可见，不含公式原文与求值错误细节）：
//   - 命中档位（tier() 标记时）
//   - 线性计价式：逐项「数量 × 单价 × 乘数 = 费用」
//   - 非线性计价式：变量取值 + 公式计价 × 乘数 = 公式费用
//   - 命中的请求规则（解释行乘数中的规则倍率；未命中规则不展示）
func formulaSummaryLines(f *BillingSnapshotFormula, money func(float64) string) []string {
	if f.Error != "" {
		return []string{"表达式计费: 公式求值失败，按预扣额结算"}
	}
	var lines []string
	if len(f.Tiers) > 0 {
		lines = append(lines, "命中档位: "+strings.Join(f.Tiers, "、"))
	}
	if len(f.Items) > 0 {
		for _, it := range f.Items {
			multPart := ""
			if it.Multiplier > 0 && it.Multiplier != 1.0 {
				multPart = fmt.Sprintf(" × %s", formatMultiplier(it.Multiplier))
			}
			if it.PerMillion {
				lines = append(lines, fmt.Sprintf("%s: %s tokens × %s/1M%s = %s",
					it.Label, formatQuantity(it.Quantity), money(it.UnitPrice), multPart, money(it.Cost)))
				continue
			}
			qty, per := formatQuantity(it.Quantity), ""
			if it.Unit != "" {
				qty += " " + it.Unit
				per = "/" + it.Unit
			}
			lines = append(lines, fmt.Sprintf("%s: %s × %s%s%s = %s",
				it.Label, qty, money(it.UnitPrice), per, multPart, money(it.Cost)))
		}
	} else {
		if vars := formatFormulaVars(f.Vars); vars != "" {
			lines = append(lines, "计费变量: "+vars)
		}
		lines = append(lines, fmt.Sprintf("公式计价: %s", money(f.BaseCost)))
		if f.Multiplier > 0 && f.Multiplier != 1.0 {
			lines = append(lines, fmt.Sprintf("公式费用: %s × %s = %s",
				money(f.BaseCost), formatMultiplier(f.Multiplier), money(f.Cost)))
		}
	}
	for i, rule := range f.Rules {
		if rule.Matched {
			lines = append(lines, fmt.Sprintf("请求规则: %s → 命中 ×%s", ruleSummaryText(rule, i), formatMultiplier(rule.Multiplier)))
		}
	}
	return lines
}

// ruleSummaryText 请求规则的租户可见文案：优先中文描述（编译期从条件生成）；
// 无法描述的手写条件不回退表达式原文（原文仅管理端快照可见），只标注序号
func ruleSummaryText(rule formula.RequestRuleTrace, idx int) string {
	if rule.Desc != "" {
		return rule.Desc
	}
	return fmt.Sprintf("第 %d 条规则", idx+1)
}

// formulaTotalLines 表达式计费合计：公式分项（或公式费用）+ 工具附加费 = 实际费用；
// 实际费用高于合计时为任务最低消费钳制（wrapSchemeCost），单独说明差额来源
func formulaTotalLines(snapshot *BillingSnapshot, money func(float64) string) []string {
	f := snapshot.Formula
	if f.Error != "" {
		return nil
	}
	var parts []string
	sum := decimal.Zero
	if len(f.Items) > 0 {
		for _, it := range f.Items {
			parts = append(parts, money(it.Cost))
			sum = sum.Add(NewFromFloat(it.Cost))
		}
	} else {
		parts = append(parts, money(f.Cost))
		sum = sum.Add(NewFromFloat(f.Cost))
	}
	for _, item := range snapshot.ToolSurcharges {
		parts = append(parts, money(item.Cost))
		sum = sum.Add(NewFromFloat(item.Cost))
	}
	var lines []string
	sumF := InexactFloat64(RoundMoney(sum))
	if len(parts) > 1 {
		lines = append(lines, fmt.Sprintf("合计: (%s) = %s", joinWithPlus(parts), money(sumF)))
	}
	if actual := snapshot.Settlement.ActualCost; actual > 0 && actual-sumF > 1e-7 {
		lines = append(lines, fmt.Sprintf("最低消费: 合计 %s 不足最低消费，按 %s 计费", money(sumF), money(actual)))
	}
	return lines
}

// formatFormulaVars 变量取值展示（键排序保证文本稳定）：已知变量「标签 数值」，
// 请求探测值（u:/header:/param:）「标签=值」
func formatFormulaVars(vars map[string]any) string {
	if len(vars) == 0 {
		return ""
	}
	keys := make([]string, 0, len(vars))
	for k := range vars {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		var val string
		switch v := vars[k].(type) {
		case float64:
			val = formatQuantity(v)
		case nil:
			val = "(空)"
		default:
			val = fmt.Sprint(v)
			if val == "" {
				val = "(空)"
			}
		}
		if strings.Contains(k, ":") {
			parts = append(parts, formula.VarLabel(k)+"="+val)
		} else {
			parts = append(parts, formula.VarLabel(k)+" "+val)
		}
	}
	return strings.Join(parts, " · ")
}

// formatQuantity 数量展示：整数直出，小数去尾零
func formatQuantity(v float64) string {
	if v == float64(int64(v)) && v < 1e15 && v > -1e15 {
		return formatInt(int(v))
	}
	return formatMultiplier(v)
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

// joinWithPlus 用 " + " 连接字符串数组
func joinWithPlus(parts []string) string {
	result := ""
	for i, part := range parts {
		if i > 0 {
			result += " + "
		}
		result += part
	}
	return result
}

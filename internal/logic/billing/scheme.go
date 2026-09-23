package billing

import (
	"encoding/json"
	"sync"

	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/shopspring/decimal"

	rcommon "github.com/qianfree/team-api/relay/common"
)

// ============================================================
// 计费方案（BillingScheme）扩展点
// ============================================================
// 背景：部分模型的计费规则无法归约为「单价 × 数量」的结构化配置（条件切价、
// 组合计费、依赖上游私有字段等）。为此把「任务预扣估算」的定价解释权抽象为
// 可注册的方案接口：模型在 pricing JSONB 顶层 "scheme" 键声明引用，未声明走
// generic（现有通用引擎），计费行为与抽象前完全一致。
//
// 设计约束（新增方案必须遵守，否则账目口径漂移）：
//  1. 方案只负责组件费用计算：EstimateTaskCost 返回已乘租户乘数 × 时段乘数的
//     金额；ratios 附加乘数（param_multiplier 等）、minCost 钳制与 RoundMoney
//     由通用层统一应用——全站结算口径、快照与对账不因方案而分叉；
//  2. 方案私有配置存 pricing JSONB 顶层 "scheme_config"（不透明容器），由方案
//     自行解析校验，不得向 PricingBlob 增加方案私有字段（主干文件不随方案膨胀，
//     二开新增方案不改任何既有文件）；
//  3. 优先消费归一化计费上下文（ratios，adaptor 经 EstimateBilling 上报）；
//     taskBody 为归一化任务体，仅确实需要原始请求体的方案使用（nil = 不可用）；
//  4. 自定义方案建议 "custom:" 前缀命名，避免与内置方案撞名。
//
// 注册模式与 relay/channel 的供应商适配器一致：方案实现放独立文件/目录，
// init() 中 RegisterScheme 自注册，新增方案不改本文件与 task_billing.go。

// SchemeGeneric 通用计费方案名（pricing JSONB 未声明 scheme 时的默认值）
const SchemeGeneric = "generic"

// BillingScheme 计费方案接口：特殊计费模型的任务预扣估算与结算重算扩展点。
// 接口签名视为公共契约（主干与二开方案的边界），保持稳定；修改需评估所有已注册方案。
type BillingScheme interface {
	// Name 方案名（唯一，与 pricing JSONB scheme 键对应）
	Name() string

	// EstimateTaskCost 任务预扣估算（纯函数，不触 DB/缓存，便于单测）。
	// 返回已含租户乘数 × 时段乘数、未应用 ratios 附加乘数的费用；
	// 附加乘数、minCost 钳制与 RoundMoney 由 estimateTaskCost 骨架统一应用。
	EstimateTaskCost(pricing *PricingResult, ratios map[string]any, taskBody []byte) decimal.Decimal

	// SettleTaskCost 任务成功结算口径（纯函数）：按上游 usage 的素材计量计算最终费用。
	// 素材计量是「预扣时未知、生成后官方返回」的计费依据（如输入视频时长、图片张数）；
	// usage 为 nil 表示上游未提供，方案应回退 EstimateTaskCost 的预扣口径（多退少补兜底）。
	// 返回值口径与 EstimateTaskCost 一致（含租户/时段乘数，不含 ratios 附加乘数）。
	SettleTaskCost(pricing *PricingResult, ratios map[string]any, usage *rcommon.TaskMaterialUsage) decimal.Decimal

	// ValidateSchemeConfig 校验方案私有配置（pricing JSONB scheme_config），
	// 管理端保存定价时 fail-fast 调用；generic 方案不接受非空配置。
	ValidateSchemeConfig(cfg json.RawMessage) error
}

// schemeRegistry 方案注册表（init 自注册，同 relay/channel 适配器模式）
var schemeRegistry sync.Map // name → BillingScheme

// RegisterScheme 注册计费方案（方案实现的 init 中调用）。
// 空名/重名直接 panic：注册发生在进程启动期，快死优于带病运行。
func RegisterScheme(s BillingScheme) {
	if s == nil || s.Name() == "" {
		panic("billing: register nil or empty-name scheme")
	}
	if _, loaded := schemeRegistry.LoadOrStore(s.Name(), s); loaded {
		panic("billing: duplicate scheme registration: " + s.Name())
	}
}

// LookupScheme 按方案名查注册表：空名/未注册一律回落 generic。
// 未注册兜底是纯函数层的防御（写路径已由 SchemeRegistered 拦截、预扣入口
// fail-closed 拒绝，正常流量到不了这里），保证估算函数不因配置异常而 panic。
func LookupScheme(name string) BillingScheme {
	if name != "" {
		if s, ok := schemeRegistry.Load(name); ok {
			return s.(BillingScheme)
		}
	}
	if s, ok := schemeRegistry.Load(SchemeGeneric); ok {
		return s.(BillingScheme)
	}
	return GenericScheme{}
}

// SchemeRegistered 方案是否已注册。管理端保存定价前校验：未注册的方案名
// 不得写入配置（防止配置引用不存在的实现，运行期才暴露成计费异常）。
func SchemeRegistered(name string) bool {
	_, ok := schemeRegistry.Load(name)
	return ok
}

func init() {
	RegisterScheme(GenericScheme{})
}

// GenericScheme 通用计费方案：token / per_request / per_second / tiered 的
// 现有通用引擎（自 estimateTaskCost 分支逻辑原样搬迁，行为不变）。
type GenericScheme struct{}

func (GenericScheme) Name() string { return SchemeGeneric }

func (GenericScheme) ValidateSchemeConfig(cfg json.RawMessage) error {
	if len(cfg) > 0 && string(cfg) != "null" {
		return gerror.New("通用计费方案不接受 scheme_config（仅特殊方案支持私有配置）")
	}
	return nil
}

// SettleTaskCost 通用引擎的结算口径：
//   - per_second（按秒计费）：上游返回实际输出秒数（usage.OutputSeconds > 0）时按
//     「实际秒数 × 矩阵单价 × 租户乘数 × 时段乘数」结算，多退少补——查价键优先
//     usage.Resolution（实际输出分辨率，如阿里 wan3.0 usage.SR），回退 ratios 的
//     spec.resolution（提交时请求档位）。实际时长与请求时长不一致（智能时长模式、
//     上游按内容微调）时以官方计量为准；usage 缺失回退预扣口径。
//   - 其余模式（token / per_request / tiered）无素材计量语义：忽略 usage，回退预扣口径
//     （与 estimateTaskCost 的估算公式一致，结算保持「预扣即终价，token 重算另走 RecalculateByTokens」）。
func (g GenericScheme) SettleTaskCost(pricing *PricingResult, ratios map[string]any, usage *rcommon.TaskMaterialUsage) decimal.Decimal {
	if pricing.BillingMode == "per_second" && usage != nil && usage.OutputSeconds > 0 {
		duration := usage.OutputSeconds
		// 结算秒数钳制上限与预扣一致，防御异常大值刷扣
		if duration > maxTaskDurationSeconds {
			duration = maxTaskDurationSeconds
		}
		spec, _ := ratioString(ratios, "spec.resolution")
		if usage.Resolution != "" {
			spec = usage.Resolution
		}
		if price := LookupPerSecondPrice(pricing.PerSecondPrices, spec); price > 0 {
			return NewFromFloat(price).
				Mul(NewFromFloat(duration)).
				Mul(NewFromFloat(pricing.TenantMultiplier)).
				Mul(NewFromFloat(effectiveTimeMultiplier(pricing)))
		}
		// 矩阵为空/全零：落到下方回退（占位/预扣口径），与预扣分支的兜底行为一致
	}
	return g.EstimateTaskCost(pricing, ratios, nil)
}

// wrapSchemeCost 方案费用统一收口：应用 ratios 附加乘数 → minCost 钳制 → RoundMoney。
// 预扣估算（estimateTaskCost）与结算重算（RecalculateByMaterials）共用，
// 保证两个时点对同一方案的费用处理口径完全一致。
func wrapSchemeCost(pricing *PricingResult, ratios map[string]any, cost decimal.Decimal) decimal.Decimal {
	// 应用附加比率（video_input 折扣等）：只乘 float 乘数值，
	// 跳过 duration/resolution（已在时长类分支消费）与 spec.*（规格事实值非乘数）
	cost = applyRatioMultipliers(cost, ratios, "duration", "resolution")

	minCost := NewFromFloat(0.01)
	if cost.LessThan(minCost) {
		cost = minCost
	}
	return RoundMoney(cost)
}

// EstimateTaskCost 通用引擎的任务预扣估算。计费口径按任务类型分流：
//   - per_second（按秒计费，视频生成）：矩阵查价（spec.resolution 未命中回退 "*"）×
//     spec.duration 秒 × 租户乘数 × 时段乘数；
//   - per_request（按次计费，图片/音乐等）：直接取按次单价；
//   - 时长类任务（视频生成，ratios 携带 duration/resolution 信号）：按
//     10000 tokens/s × duration × resolution 预估 token 再乘输出单价（存量 token 伪装路径）；
//   - 其余无时长信号的任务（如未显式配成 per_request 的图片模型）：退回按次单价，
//     不再套用视频 token 估算——图片没有时长/分辨率，套 10000×5×2.25 会凭空估出
//     11.25 万 token 的天价预扣（$30/1M 输出价即得 $3.375），且与结算的「0 token」自相矛盾。
func (GenericScheme) EstimateTaskCost(pricing *PricingResult, ratios map[string]any, _ []byte) decimal.Decimal {
	var costD decimal.Decimal
	switch {
	case pricing.BillingMode == "per_second":
		// 按秒计费：矩阵查价 × 时长 × 租户乘数 × 时段乘数
		duration := defaultTaskDurationSeconds
		if d, ok := ratioFloat(ratios, "spec.duration"); ok && d > 0 {
			duration = d
		}
		// 时长来自用户请求，钳制上限防天价预扣（正常视频模型远低于该上限）
		if duration > maxTaskDurationSeconds {
			duration = maxTaskDurationSeconds
		}
		spec, _ := ratioString(ratios, "spec.resolution")
		price := LookupPerSecondPrice(pricing.PerSecondPrices, spec)
		if price <= 0 {
			// 矩阵全零/为空：按未配价占位预扣，结算多退少补
			return NewFromFloat(imagePlaceholderPreDeduct)
		}
		costD = NewFromFloat(price).
			Mul(NewFromFloat(duration)).
			Mul(NewFromFloat(pricing.TenantMultiplier)).
			Mul(NewFromFloat(effectiveTimeMultiplier(pricing)))
	case pricing.BillingMode == "per_request":
		// 按次计费：单价 × 租户乘数 × 时段乘数。与 computeCost / EstimatePreDeductAmount 的
		// 按次口径对齐——按次任务结算无 token 重算信号，预扣即终价，
		// 预扣漏乘租户/时段折扣会让折扣租户按原价多扣
		costD = NewFromFloat(pricing.PerRequestPrice).
			Mul(NewFromFloat(pricing.TenantMultiplier)).
			Mul(NewFromFloat(effectiveTimeMultiplier(pricing)))
	case pricing.OutputPrice > 0 && hasDurationSignal(ratios):
		duration := 5.0 // 默认 5 秒
		if d, ok := ratioFloat(ratios, "duration"); ok && d > 0 {
			duration = d
		}
		resolutionMul := 2.25 // 默认 720p
		if r, ok := ratioFloat(ratios, "resolution"); ok && r > 0 {
			resolutionMul = r
		}

		// 预估 tokens ≈ base_tokens_per_second × duration × resolution_multiplier
		// 火山方舟视频生成约 10000 tokens/s (480p 基准)，用于预扣估算
		// 视频预扣三连乘全程 decimal 避免链式误差
		baseTokensPerSec := decimal.NewFromInt(10000)
		durationD := NewFromFloat(duration)
		resolutionMulD := NewFromFloat(resolutionMul)
		million := decimal.NewFromInt(1_000_000)

		costD = baseTokensPerSec.Mul(durationD).Mul(resolutionMulD).
			Div(million).
			Mul(NewFromFloat(pricing.OutputPrice)).
			Mul(NewFromFloat(pricing.TenantMultiplier))
	default:
		// 无时长信号（图片等扁平计费任务）：优先按次单价（同样乘租户/时段乘数，
		// 预扣即终价的口径与 per_request 分支一致）；未配按次价时用占位预扣，
		// 绝不走视频 token 估算。结算阶段再按上游真实 token 用量多退少补
		// （见 sync_image_worker.settleSyncImageSuccess）。
		if pricing.PerRequestPrice > 0 {
			costD = NewFromFloat(pricing.PerRequestPrice).
				Mul(NewFromFloat(pricing.TenantMultiplier)).
				Mul(NewFromFloat(effectiveTimeMultiplier(pricing)))
		} else {
			costD = NewFromFloat(imagePlaceholderPreDeduct)
		}
	}
	return costD
}

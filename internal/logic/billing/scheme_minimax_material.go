package billing

import (
	"encoding/json"

	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/shopspring/decimal"

	rcommon "github.com/qianfree/team-api/relay/common"
)

// ============================================================
// MiniMax-H3 素材计费方案（custom:minimax-material）
// ============================================================
// 适用：MiniMax-H3 / H3-Max 等按「输入素材 + 输出生成」组合计费的视频模型。
// 计费公式（金额为本位币，统一乘 租户乘数 × 时段乘数）：
//
//	总费用 = 输出生成费 + 输入图片费 + 输入视频费（输入音频免费，不配置即不计费）
//	  输出生成费 = 输出秒数 × pricing JSONB per_second 矩阵[生成分辨率]
//	  输入图片费 = max(0, 图片张数 - free_count) × price_per_unit
//	  输入视频费 = 输入视频秒数 × per_second 矩阵[生成分辨率]
//
// 输入视频不单独定价：官方规则为「输入视频按时长、以输出分辨率对应单价计费」
// （与生成视频同价），单价复用输出生成矩阵按生成分辨率查取，scheme_config 无
// input_video 节（存量配置残留该节在保存时被校验拒绝，提示移除）。
//
// 预扣 vs 结算：
//   - 图片张数提交时可知（adaptor 上报 ratios 的 spec.input_image_count）→ 预扣即精确；
//   - 输入视频常以 URL 提交、时长未知 → 提交含输入视频（spec.has_input_video）时预扣按
//     生成长度上限 15s 冻结，结算以官方 usage（input_seconds）为准多退少补；
//   - 输出秒数预扣按请求时长信号估，结算以 usage.output_seconds 为准。
// 结算时 usage 某字段缺失（上游未返回）则回退 ratios 中提交时刻的对应 spec.* 事实值，
// 官方完全不返回素材计量时整体回退预扣口径（多退少补兜底）。
//
// scheme_config（pricing JSONB 不透明容器，金额本位币；可不配置 = 图片免费，
// 输出与输入视频均按 per_second 矩阵计费）：
//
//	{
//	  "image": {"free_count": 5, "price_per_unit": 0.20}
//	}

// SchemeMiniMaxMaterial MiniMax 素材计费方案名（模型 pricing JSONB scheme 键引用）
const SchemeMiniMaxMaterial = "custom:minimax-material"

// inputVideoMaxSeconds 输入视频计费时长上限（秒）。官方 usage 可信度高，
// 钳制仅防御异常大值刷扣；输出侧沿用 maxTaskDurationSeconds。
const inputVideoMaxSeconds = 600.0

// inputVideoEstimateSeconds 输入视频预扣估算时长（秒）：H3 生成本身上限 15s，
// 输入视频计费时长不会超过生成时长，提交含输入视频时按上限冻结，
// 避免 URL 素材时长未知导致预扣少冻结。
const inputVideoEstimateSeconds = 15.0

// minimaxMaterialConfig 方案私有配置（输入视频按生成分辨率单价计费，无独立配置）
type minimaxMaterialConfig struct {
	Image *minimaxImagePrice `json:"image,omitempty"`
}

// minimaxImagePrice 输入图片单价：请求级免费额度 + 超出部分单价
type minimaxImagePrice struct {
	FreeCount    int     `json:"free_count"`     // 免费张数（0 = 无免费额度）
	PricePerUnit float64 `json:"price_per_unit"` // 超出部分单价（本位币/张）
}

// minimaxMaterialScheme MiniMax 素材计费方案（无状态单例，配置从 pricing.SchemeConfig 读取）
type minimaxMaterialScheme struct{}

func init() {
	RegisterScheme(minimaxMaterialScheme{})
}

func (minimaxMaterialScheme) Name() string { return SchemeMiniMaxMaterial }

// parseConfig 解析方案私有配置（解析失败按未配置处理：仅输出组件计费）
func (minimaxMaterialScheme) parseConfig(pricing *PricingResult) *minimaxMaterialConfig {
	cfg := &minimaxMaterialConfig{}
	if pricing == nil || len(pricing.SchemeConfig) == 0 {
		return cfg
	}
	if err := json.Unmarshal(pricing.SchemeConfig, cfg); err != nil {
		return &minimaxMaterialConfig{}
	}
	return cfg
}

// ValidateSchemeConfig 校验 scheme_config（管理端保存 fail-fast）：
// 配置可空（图片免费、仅按秒矩阵计费输出与输入视频的合法形态）；配置时校验图片
// 单价与免费额度非负；input_video 节已废弃（输入视频改按生成分辨率单价计费），
// 残留即拒绝，提示管理员移除后重存，避免旧单价静默失效造成计费口径突变。
func (minimaxMaterialScheme) ValidateSchemeConfig(cfg json.RawMessage) error {
	if len(cfg) == 0 || string(cfg) == "null" {
		return nil
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(cfg, &raw); err != nil {
		return gerror.Wrap(err, "scheme_config 解析失败")
	}
	if _, ok := raw["input_video"]; ok {
		return gerror.New("输入视频不再单独定价（按生成分辨率单价计费），请移除 scheme_config 中的 input_video 配置")
	}
	var c minimaxMaterialConfig
	if err := json.Unmarshal(cfg, &c); err != nil {
		return gerror.Wrap(err, "scheme_config 解析失败")
	}
	if c.Image != nil {
		if c.Image.PricePerUnit < 0 {
			return gerror.New("输入图片单价不能为负")
		}
		if c.Image.FreeCount < 0 {
			return gerror.New("输入图片免费张数不能为负")
		}
	}
	return nil
}

// EstimateTaskCost 预扣估算：按秒组件（输出生成 + 输入视频，共用生成分辨率单价）+
// 提交可知的图片组件（张数精确）。
func (s minimaxMaterialScheme) EstimateTaskCost(pricing *PricingResult, ratios map[string]any, _ []byte) decimal.Decimal {
	cfg := s.parseConfig(pricing)
	spec, _ := ratioString(ratios, "spec.resolution")
	total := decimal.Zero

	// 按秒组件（与通用按秒引擎共用查价与兜底链），矩阵未配置时输出与输入视频均不计费
	if price := LookupPerSecondPrice(pricing.PerSecondPrices, spec); price > 0 {
		duration := defaultTaskDurationSeconds
		if d, ok := ratioFloat(ratios, "spec.duration"); ok && d > 0 {
			duration = d
		}
		if duration > maxTaskDurationSeconds {
			duration = maxTaskDurationSeconds
		}
		// 输出生成：时长 × 单价
		total = total.Add(NewFromFloat(price).Mul(NewFromFloat(duration)))
		// 输入视频：估算时长（确知时长 > 含视频按生成长度上限 15s 估 > 0）× 同一单价，
		// 结算以官方 usage 补差
		total = total.Add(inputVideoCost(price, estimateInputVideoSeconds(ratios)))
	}

	// 输入图片：请求级免费额度扣减
	total = total.Add(s.imageCost(cfg, ratioFloatOr(ratios, "spec.input_image_count", 0)))

	return total.Mul(NewFromFloat(pricing.TenantMultiplier)).
		Mul(NewFromFloat(effectiveTimeMultiplier(pricing)))
}

// SettleTaskCost 结算口径：素材计量以官方 usage 为准，缺失字段回退 ratios 的提交时事实值，
// 官方完全不返回素材计量时整体回退预扣口径。
func (s minimaxMaterialScheme) SettleTaskCost(pricing *PricingResult, ratios map[string]any, usage *rcommon.TaskMaterialUsage) decimal.Decimal {
	if usage == nil || (usage.InputVideoSeconds <= 0 && usage.InputImageCount <= 0 && usage.OutputSeconds <= 0) {
		return s.EstimateTaskCost(pricing, ratios, nil)
	}

	cfg := s.parseConfig(pricing)
	spec, _ := ratioString(ratios, "spec.resolution")
	total := decimal.Zero

	// 按秒组件单价：生成分辨率矩阵查价（输出与输入视频同价），矩阵未配置时为 0
	price := LookupPerSecondPrice(pricing.PerSecondPrices, spec)

	// 输出生成组件：优先官方 output_seconds，缺失回退请求时长信号（与预扣同源）
	if price > 0 {
		outputSecs := usage.OutputSeconds
		if outputSecs <= 0 {
			outputSecs = ratioFloatOr(ratios, "spec.duration", defaultTaskDurationSeconds)
		}
		if outputSecs > maxTaskDurationSeconds {
			outputSecs = maxTaskDurationSeconds
		}
		total = total.Add(NewFromFloat(price).Mul(NewFromFloat(outputSecs)))
	}

	// 输入图片：官方 input_image_count，缺失回退提交时上报的张数
	imageCount := float64(usage.InputImageCount)
	if usage.InputImageCount <= 0 {
		imageCount = ratioFloatOr(ratios, "spec.input_image_count", 0)
	}
	total = total.Add(s.imageCost(cfg, imageCount))

	// 输入视频：官方 input_seconds，缺失回退预扣口径（确知时长 > 含视频按 15s 上限估 > 0）
	inputSecs := usage.InputVideoSeconds
	if inputSecs <= 0 {
		inputSecs = estimateInputVideoSeconds(ratios)
	}
	total = total.Add(inputVideoCost(price, inputSecs))

	return total.Mul(NewFromFloat(pricing.TenantMultiplier)).
		Mul(NewFromFloat(effectiveTimeMultiplier(pricing)))
}

// imageCost 输入图片费用：max(0, 张数 - 免费额度) × 单价
func (minimaxMaterialScheme) imageCost(cfg *minimaxMaterialConfig, count float64) decimal.Decimal {
	if cfg == nil || cfg.Image == nil || cfg.Image.PricePerUnit <= 0 || count <= float64(cfg.Image.FreeCount) {
		return decimal.Zero
	}
	charged := count - float64(cfg.Image.FreeCount)
	return NewFromFloat(charged).Mul(NewFromFloat(cfg.Image.PricePerUnit))
}

// inputVideoCost 输入视频费用：时长（钳制上限）× 生成分辨率单价。
// 官方规则：输入视频按时长、以输出分辨率对应单价计费（与生成视频同价），
// 不单独定价、不区分输入视频自身分辨率（官方 usage 无该信息）。
func inputVideoCost(price, seconds float64) decimal.Decimal {
	if price <= 0 || seconds <= 0 {
		return decimal.Zero
	}
	if seconds > inputVideoMaxSeconds {
		seconds = inputVideoMaxSeconds
	}
	return NewFromFloat(seconds).Mul(NewFromFloat(price))
}

// estimateInputVideoSeconds 输入视频预扣估算时长（秒）：
// adaptor 确知时长（spec.input_video_seconds）优先；否则提交含输入视频
// （spec.has_input_video）按生成长度上限 15s 估；无输入视频为 0。
func estimateInputVideoSeconds(ratios map[string]any) float64 {
	if s, ok := ratioFloat(ratios, "spec.input_video_seconds"); ok && s > 0 {
		return s
	}
	if has, _ := ratios["spec.has_input_video"].(bool); has {
		return inputVideoEstimateSeconds
	}
	return 0
}

// ratioFloatOr 取计费上下文 float 值，缺失返回默认值
func ratioFloatOr(ratios map[string]any, key string, def float64) float64 {
	if f, ok := ratioFloat(ratios, key); ok {
		return f
	}
	return def
}

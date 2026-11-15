package formula

import (
	"fmt"
	"strings"
	"time"

	rcommon "github.com/qianfree/team-api/relay/common"
)

// FormulaInput 公式变量环境的输入要素。
// 由调用方从 usage（结算路径，经 resolveTokenCounts 归集）或估算值（预扣路径）构造：
// BaseInputTokens/CacheReadTokens/CacheCreation5m/CacheCreation1h 必须采用
// billing.resolveTokenCounts 的口径（缓存排除已在该层完成），公式层不重复扣缓存
type FormulaInput struct {
	BaseInputTokens   int     // 已扣缓存的输入 token（OpenAI 语义 = prompt - 缓存各项；Claude 语义 = input 原值）
	OutputTokens      int     // 输出 token（含全部模态与推理）
	CacheReadTokens   int     // 缓存读取 token
	CacheCreation5m   int     // 缓存写入 5m 桶（Claude cached_creation 与 OpenAI cache_write 合并取大）
	CacheCreation1h   int     // 缓存写入 1h 桶（Claude ephemeral_1h）
	InputImageTokens  int     // 图片输入 token
	InputAudioTokens  int     // 音频输入 token
	OutputImageTokens int     // 图片输出 token
	OutputAudioTokens int     // 音频输出 token
	ReasoningTokens   int     // 推理输出 token
	ImageCount        int     // 图片张数（<=1 归一为 1）
	Seconds           float64 // 任务计费秒数（时长类任务：预扣=请求时长信号，结算=官方素材计量；同步路径恒 0）
	// Facts usage 旁路计量事实（渠道 adaptor 填充的非 token 计量：音频秒数、
	// 字节数等），公式经 u("key") 取值；key 注册表见 KnownFactKeys。
	// 预扣估算路径无 facts（请求侧不可知），u() 取 0，结算按真实 facts 补差
	Facts map[string]float64
	// OpenAISemantics 标记 prompt_tokens 是否包含全部模态 token（OpenAI 原生口径）。
	// 仅 true 时图片/音频输入 token 才需要从 p 中扣除（三方言渠道口径不明，
	// 宁可少扣不双扣——与 resolveTokenCounts 对缓存写入「合并取大保守计费」同向）
	OpenAISemantics bool
	// Eval 求值上下文：时间函数基准时刻 + 默认时区 + header()/param() 请求探测值。
	// nil → 探针取空值（header() 空、param() nil）、时间函数取当前时刻 UTC
	// （烟测与无请求上下文的兜底路径）。预扣取受理时刻、结算取 billAt、
	// 任务取提交冻结值重放——三口径见 EvalContext 注释
	Eval *EvalContext
}

// CacheCreation1hMultiplier 1h 缓存写入相对 5m 写入价的倍数（官方 1h=2× 输入价、5m=1.25×，
// 2÷1.25=1.6；与 billing 包 token 模式的 1h 价推导一致）。公式引用 cc 未引用 cc1h 时使用
const CacheCreation1hMultiplier = 1.6

// cc1hMergedIntoCC 公式引用 cc 未引用 cc1h 且确有 1h 写入：1h 写入按 cc 单价 ×1.6 并入 cc 求值
func (in FormulaInput) cc1hMergedIntoCC(usedVars map[string]bool) bool {
	return usedVars["cc"] && !usedVars["cc1h"] && in.CacheCreation1h > 0
}

// FromUsage 从 usage 细分构造公式输入。baseInput/cacheRead/cc5m/cc1h 传
// resolveTokenCounts 的输出（口径归一已在计费主链路完成），本函数只补模态细分
func FromUsage(baseInput, output, cacheRead, cc5m, cc1h int, usage *rcommon.Usage) FormulaInput {
	in := FormulaInput{
		BaseInputTokens: baseInput,
		OutputTokens:    output,
		CacheReadTokens: cacheRead,
		CacheCreation5m: cc5m,
		CacheCreation1h: cc1h,
		ImageCount:      1,
		OpenAISemantics: usage != nil && usage.CacheIncludedInPrompt,
	}
	if usage == nil {
		return in
	}
	if usage.PromptTokensDetails != nil {
		in.InputImageTokens = usage.PromptTokensDetails.ImageTokens
		in.InputAudioTokens = usage.PromptTokensDetails.AudioTokens
	}
	if usage.CompletionTokenDetails != nil {
		in.OutputImageTokens = usage.CompletionTokenDetails.ImageTokens
		in.OutputAudioTokens = usage.CompletionTokenDetails.AudioTokens
		in.ReasoningTokens = usage.CompletionTokenDetails.ReasoningTokens
	}
	in.Facts = usage.Facts
	return in
}

// Env 构建求值环境（叠加排除语义）：
//   - p：仅当公式引用 img/ai 且 OpenAI 语义（prompt 含全部模态）时扣除对应输入细分
//   - c：仅当公式引用 rt/img_o/ao 时扣除对应输出细分
//   - ctx：完整输入上下文 = baseInput + 缓存各项（两种协议语义统一后的口径，
//     供三目条件做长上下文档位判断，不受排除影响）
//
// 未被公式引用的细分类别保留在 p/c 中按基础系数计价——管理员写 "p*5+c*10"
// 时缓存/音频/图片 token 自动随 p/c 计费，无需显式穷举：
//   - 缓存：BaseInputTokens 已按 resolveTokenCounts 扣除缓存各项，未引用的 cr/cc/cc1h
//     必须加回 p（否则这部分 token 既不在 p 也不单独计价，等于免费）
//   - cc1h 未引用而 cc 已引用：1h 缓存写入按 cc 单价 ×1.6 计（并入 cc 的求值值，
//     与 token 模式「1h 价 = 5m 基准 ×1.6」推导同口径）
//
// trace 为请求规则命中表（Run 传入以回填 Matched；nil 时 _trace 退化为纯数值函数，
// 供快照展示等不执行求值的场景复用同一构建逻辑）
func (in FormulaInput) Env(usedVars map[string]bool, trace []RequestRuleTrace) map[string]any {
	p := in.BaseInputTokens
	if !usedVars["cr"] {
		p += in.CacheReadTokens
	}
	ccValue := float64(in.CacheCreation5m)
	if !usedVars["cc"] {
		p += in.CacheCreation5m
	}
	if !usedVars["cc1h"] {
		if usedVars["cc"] {
			ccValue += float64(in.CacheCreation1h) * CacheCreation1hMultiplier
		} else {
			p += in.CacheCreation1h
		}
	}
	if in.OpenAISemantics {
		if usedVars["img"] {
			p -= in.InputImageTokens
		}
		if usedVars["ai"] {
			p -= in.InputAudioTokens
		}
		if p < 0 {
			p = 0
		}
	}

	c := in.OutputTokens
	if usedVars["rt"] {
		c -= in.ReasoningTokens
	}
	if usedVars["img_o"] {
		c -= in.OutputImageTokens
	}
	if usedVars["ao"] {
		c -= in.OutputAudioTokens
	}
	if c < 0 {
		c = 0
	}

	count := in.ImageCount
	if count < 1 {
		count = 1
	}

	env := map[string]any{
		"p":           float64(p),
		"c":           float64(c),
		"ctx":         float64(in.BaseInputTokens + in.CacheReadTokens + in.CacheCreation5m + in.CacheCreation1h),
		"cr":          float64(in.CacheReadTokens),
		"cc":          ccValue,
		"cc1h":        float64(in.CacheCreation1h),
		"img":         float64(in.InputImageTokens),
		"img_o":       float64(in.OutputImageTokens),
		"ai":          float64(in.InputAudioTokens),
		"ao":          float64(in.OutputAudioTokens),
		"rt":          float64(in.ReasoningTokens),
		"image_count": float64(count),
		"seconds":     in.Seconds,
	}
	for k, v := range BuiltinFuncs() {
		env[k] = v
	}
	// u("key") 从 usage facts 取旁路计量值（非 token 计量：音频秒数、字节数等）。
	// 缺失 key 返回 0（上游未报该字段 ≠ 公式错误——预扣路径天然无 facts）；
	// key 注册表校验在保存烟测（SmokeTest），运行时不再拒绝
	facts := in.Facts
	env["u"] = func(args ...any) (any, error) {
		if len(args) != 1 {
			return nil, fmt.Errorf("u() 需要 1 个字符串 key 参数")
		}
		key, ok := args[0].(string)
		if !ok {
			return nil, fmt.Errorf("u() 的 key 必须是字符串字面量")
		}
		return facts[key], nil
	}
	in.injectRequestFuncs(env, trace)
	return env
}

// injectRequestFuncs 注入请求感知函数：
//   - header("key")：请求头值（小写归一；缺失空串）
//   - param("path")：请求体 JSON 路径值（点分路径，缺失 nil）
//   - has(v, "s")：子串包含（header 值非精确匹配时用）
//   - hour("tz")/minute/weekday/month/day：Eval.At 在指定时区的钟表时间
//     （tz 省略/非法回退 Eval.Location → UTC；基准取受理/冻结时刻而非 now，
//     预扣与结算跨小时边界不漂移）
//   - _trace/_trace_int/_tier：编译期埋点改写的内部函数（保留字），数值等价 + 命中标记
func (in FormulaInput) injectRequestFuncs(env map[string]any, trace []RequestRuleTrace) {
	eval := in.Eval
	if eval == nil {
		eval = &EvalContext{}
	}
	req := eval.Request
	evalAt := eval.At
	location := eval.Location
	if evalAt.IsZero() {
		evalAt = time.Now()
	}

	env["header"] = func(key string) string {
		return req.GetHeader(key)
	}
	env["param"] = func(path string) any {
		return req.GetParam(path)
	}
	env["has"] = func(source any, substr string) bool {
		if source == nil || substr == "" {
			return false
		}
		return strings.Contains(fmt.Sprint(source), substr)
	}
	timeIn := func(tz string) time.Time {
		tz = strings.TrimSpace(tz)
		if tz == "" {
			if location != nil {
				return evalAt.In(location)
			}
			return evalAt.UTC()
		}
		loc, err := time.LoadLocation(tz)
		if err != nil {
			// 非法时区回退默认（与时段定价对非法配置的容错同向），不中断计费
			if location != nil {
				return evalAt.In(location)
			}
			return evalAt.UTC()
		}
		return evalAt.In(loc)
	}
	// 时区参数可省略（省略 = 平台计费时区）：变参签名，多传的参数忽略首个之外的值
	optTZ := func(tz []string) string {
		if len(tz) == 0 {
			return ""
		}
		return tz[0]
	}
	env["hour"] = func(tz ...string) int { return timeIn(optTZ(tz)).Hour() }
	env["minute"] = func(tz ...string) int { return timeIn(optTZ(tz)).Minute() }
	env["weekday"] = func(tz ...string) int { return int(timeIn(optTZ(tz)).Weekday()) }
	env["month"] = func(tz ...string) int { return int(timeIn(optTZ(tz)).Month()) }
	env["day"] = func(tz ...string) int { return timeIn(optTZ(tz)).Day() }

	env[traceFunction] = func(ruleIndex int, matched bool, multiplier float64) float64 {
		if matched && ruleIndex >= 0 && ruleIndex < len(trace) {
			trace[ruleIndex].Matched = true
			return multiplier
		}
		return 1
	}
	env[traceIntFunction] = func(ruleIndex int, matched bool, multiplier int) int {
		if matched && ruleIndex >= 0 && ruleIndex < len(trace) {
			trace[ruleIndex].Matched = true
			return multiplier
		}
		return 1
	}
	// 档位埋点默认实现：数值透传不记录命中（RunDetail 覆盖为记录命中的闭包）
	env[tierFunction] = func(args ...any) (any, error) {
		if len(args) != 2 {
			return nil, fmt.Errorf("tier() 参数异常")
		}
		return args[1], nil
	}
}

// SnapshotVars 求值环境的变量快照（计费快照留痕用）：已知变量取值 + u:key/header:key/
// param:path 探测值。与 Run 求值环境同源构建（口径一致），供 bil_usage_logs 的
// billing_snapshot.formula.vars 消费；header/param 值可能为 string/any，整体 map[string]any
func SnapshotVars(c *Compiled, in FormulaInput) map[string]any {
	return snapshotVarsFromEnv(c, in, in.Env(c.Vars, nil))
}

// snapshotVarsFromEnv 从已构建的求值环境提取变量快照（RunDetail 复用求值环境，保证同源）
func snapshotVarsFromEnv(c *Compiled, in FormulaInput, env map[string]any) map[string]any {
	out := make(map[string]any, len(c.Vars)+len(c.UKeys)+len(c.ParamPaths)+len(c.HeaderKeys))
	for name := range c.Vars {
		if v, ok := env[name]; ok {
			out[name] = v
		}
	}
	// 1h 写入并入 cc 求值时，变量快照仍展示真实 token 数（cc 求值值是 ×1.6 加权和，直接展示会误导）
	if in.cc1hMergedIntoCC(c.Vars) {
		out["cc"] = float64(in.CacheCreation5m)
		out["cc1h"] = float64(in.CacheCreation1h)
	}
	for _, key := range c.UKeys {
		out["u:"+key] = in.Facts[key]
	}
	var req *RequestInput
	if in.Eval != nil {
		req = in.Eval.Request
	}
	for _, key := range c.HeaderKeys {
		out["header:"+key] = req.GetHeader(key)
	}
	for _, path := range c.ParamPaths {
		out["param:"+path] = req.GetParam(path)
	}
	return out
}

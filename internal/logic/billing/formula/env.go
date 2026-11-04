package formula

import (
	"fmt"

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
// 时 audio/image token 自动随 p/c 计费，无需显式穷举
func (in FormulaInput) Env(usedVars map[string]bool) map[string]any {
	p := in.BaseInputTokens
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
		"cc":          float64(in.CacheCreation5m),
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
	return env
}

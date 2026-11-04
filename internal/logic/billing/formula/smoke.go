package formula

import (
	"fmt"
	"strings"
)

// smokeVectors 烟测预置向量：保存定价时强制全组通过（结果有限非负），
// 覆盖空请求 / 常规 chat / 多模态满配 / 大上下文 / 按张放大五类形态。
// 管理端试算接口（validate-formula）以同组向量展示金额预览
var smokeVectors = []SmokeVector{
	{Name: "空请求", In: FormulaInput{BaseInputTokens: 0, OutputTokens: 0, ImageCount: 1}},
	{Name: "常规对话", In: FormulaInput{
		BaseInputTokens: 10000, OutputTokens: 2000,
		CacheReadTokens: 5000, CacheCreation5m: 2000,
		ReasoningTokens: 500, ImageCount: 1,
	}},
	{Name: "多模态满配", In: FormulaInput{
		BaseInputTokens: 5000, OutputTokens: 1000,
		CacheReadTokens: 8000, CacheCreation5m: 4000, CacheCreation1h: 1000,
		InputImageTokens: 3000, InputAudioTokens: 2000,
		OutputImageTokens: 500, OutputAudioTokens: 1000,
		ReasoningTokens: 800, ImageCount: 1, OpenAISemantics: true,
	}},
	{Name: "长上下文", In: FormulaInput{
		BaseInputTokens: 500000, OutputTokens: 100000,
		CacheReadTokens: 300000, ImageCount: 1,
	}},
	{Name: "多张图片", In: FormulaInput{
		BaseInputTokens: 2000, OutputTokens: 4000,
		ImageCount: 4, OpenAISemantics: true,
	}},
	{Name: "音频计量", In: FormulaInput{
		Facts: map[string]float64{"audio_seconds": 60, "audio_bytes": 960000},
	}},
	{Name: "视频任务", In: FormulaInput{
		Seconds: 5, ImageCount: 1,
	}},
}

// SmokeVector 带名称的烟测向量（试算接口展示用）
type SmokeVector struct {
	Name string
	In   FormulaInput
}

// SmokeVectors 返回预置烟测向量副本（防止调用方篡改包级切片）
func SmokeVectors() []SmokeVector {
	out := make([]SmokeVector, len(smokeVectors))
	copy(out, smokeVectors)
	return out
}

// SmokeTest 烟测：编译 + u() key 注册表校验 + 全组预置向量求值（结果有限非负）。
// 保存定价（BuildPricingBlob formula 分支）与管理端试算共用，防错配公式上线。
// u() 未注册 key 直接拒绝——拼写错误的 facts key 会静默取 0 导致错价，必须在保存时拦截
func SmokeTest(src string) error {
	c, err := Compile(src)
	if err != nil {
		return err
	}
	if err := validateUKeys(c); err != nil {
		return err
	}
	for _, v := range smokeVectors {
		if _, err := Run(c, v.In.Env(c.Vars)); err != nil {
			return fmt.Errorf("样例「%s」%w", v.Name, err)
		}
	}
	return nil
}

// validateUKeys u() facts key 注册表校验（SmokeTest 与 Evaluate 共用，口径一致）
func validateUKeys(c *Compiled) error {
	for _, key := range c.UKeys {
		if key == "" {
			return fmt.Errorf("u() 的 key 不能为空")
		}
		if _, ok := KnownFactKeys[key]; !ok {
			return fmt.Errorf("u(\"%s\") 引用了未注册的 usage 字段（可用：%v），请检查拼写或联系开发注册", key, factKeyNames())
		}
	}
	return nil
}

// factKeyNames 已注册 facts key 列表（报错提示用，按名排序保证文本稳定）
func factKeyNames() []string {
	names := make([]string, 0, len(KnownFactKeys))
	for k := range KnownFactKeys {
		names = append(names, k)
	}
	sortStrings(names)
	return names
}

// EvaluateResult 单组试算结果：样例名 + 变量环境快照 + 价格系数
// （金额 = 系数 / 1e6 × 租户乘数 × 时段乘数，换算由调用方完成）
type EvaluateResult struct {
	Name   string         `json:"name"`
	Env    map[string]any `json:"env"`
	Factor float64        `json:"factor"`
}

// Evaluate 编译并对预置向量试算（管理端「校验并试算」接口用）。
// 自定义样例非空时替换预置向量；返回引用变量列表供前端高亮提示。
// u() key 注册校验与保存烟测同口径（validateUKeys），防「试算通过保存被拒」的口径漂移
func Evaluate(src string, custom []FormulaInput) (*Compiled, []EvaluateResult, error) {
	c, err := Compile(src)
	if err != nil {
		return nil, nil, err
	}
	if err := validateUKeys(c); err != nil {
		return c, nil, err
	}

	vectors := smokeVectors
	if len(custom) > 0 {
		vectors = make([]SmokeVector, len(custom))
		for i, in := range custom {
			vectors[i] = SmokeVector{Name: fmt.Sprintf("样例 %d", i+1), In: in}
		}
	}

	results := make([]EvaluateResult, 0, len(vectors))
	for _, v := range vectors {
		env := v.In.Env(c.Vars)
		factor, err := Run(c, env)
		if err != nil {
			return c, results, fmt.Errorf("样例「%s」%w", v.Name, err)
		}
		// 环境快照剔除函数项（不可 JSON 序列化为可读值）；
		// u() 取值以 u:key 形式并入（试算展示与计费快照同一口径）
		envSnapshot := make(map[string]any, len(knownVars)+len(c.UKeys))
		for k, val := range env {
			if knownVars[k] {
				envSnapshot[k] = val
			}
		}
		for _, key := range c.UKeys {
			envSnapshot["u:"+key] = v.In.Facts[key]
		}
		results = append(results, EvaluateResult{Name: v.Name, Env: envSnapshot, Factor: factor})
	}
	return c, results, nil
}

// VarNames 公式引用的变量名列表（展示用，升序稳定）
func VarNames(vars map[string]bool) []string {
	out := make([]string, 0, len(vars))
	for name := range knownVars {
		if vars[name] {
			out = append(out, name)
		}
	}
	// knownVars 遍历顺序不稳定，手动排序保证接口输出稳定
	sortStrings(out)
	return out
}

func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}

// String 编译产物摘要（日志/快照用）
func (c *Compiled) String() string {
	src := c.Src
	if len(src) > 200 {
		src = src[:200] + "..."
	}
	vars := VarNames(c.Vars)
	return fmt.Sprintf("formula(%s) vars=[%s]", src, strings.Join(vars, ","))
}

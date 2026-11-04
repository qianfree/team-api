package formula

import (
	"math"
	"strings"
	"testing"

	rcommon "github.com/qianfree/team-api/relay/common"
)

// openAIUsage OpenAI 原生口径金样本：prompt_tokens 包含缓存与全部模态
func openAIUsage() *rcommon.Usage {
	return &rcommon.Usage{
		PromptTokens:     30000, // = 文本15000 + 缓存读8000 + 缓存写5000 + 图片1500 + 音频500
		CompletionTokens: 6000,  // = 文本3500 + 推理2000 + 音频500
		PromptTokensDetails: &rcommon.TokenDetails{
			CachedTokens:         8000,
			CachedCreationTokens: 0,
			CacheWriteTokens:     5000,
			ImageTokens:          1500,
			AudioTokens:          500,
			TextTokens:           15000,
		},
		CompletionTokenDetails: &rcommon.TokenDetails{
			TextTokens:      3500,
			ReasoningTokens: 2000,
			AudioTokens:     500,
		},
		CacheIncludedInPrompt: true,
	}
}

// claudeUsage Claude 口径金样本：input_tokens 不含缓存，缓存细分独立上报
func claudeUsage() *rcommon.Usage {
	return &rcommon.Usage{
		PromptTokens:     15000, // 纯文本输入
		CompletionTokens: 6000,
		PromptTokensDetails: &rcommon.TokenDetails{
			CachedTokens:           8000,
			CachedCreation5mTokens: 4000,
			CachedCreation1hTokens: 1000,
		},
		CompletionTokenDetails: &rcommon.TokenDetails{
			ReasoningTokens: 2000,
		},
		CacheIncludedInPrompt: false,
	}
}

// resolveRef 复刻 billing.resolveTokenCounts 的口径（测试侧独立实现，
// 两边口径漂移时金样本会失败——这是有意的守卫设计）
func resolveRef(u *rcommon.Usage) (baseInput, output, cacheRead, cc5m, cc1h int) {
	baseInput = u.PromptTokens
	output = u.CompletionTokens
	cacheRead = u.PromptTokensDetails.CachedTokens
	if u.PromptTokensDetails.CachedCreation5mTokens > 0 || u.PromptTokensDetails.CachedCreation1hTokens > 0 {
		cc5m = u.PromptTokensDetails.CachedCreation5mTokens
		cc1h = u.PromptTokensDetails.CachedCreation1hTokens
	} else {
		cc5m = max(u.PromptTokensDetails.CachedCreationTokens, u.PromptTokensDetails.CacheWriteTokens)
	}
	if u.CacheIncludedInPrompt {
		baseInput = u.PromptTokens - cacheRead - cc5m - cc1h
		if baseInput < 0 {
			baseInput = 0
		}
	}
	return
}

func mustRun(t *testing.T, src string, in FormulaInput) (float64, map[string]any) {
	t.Helper()
	c, err := Compile(src)
	if err != nil {
		t.Fatalf("编译失败 %q: %v", src, err)
	}
	env := in.Env(c.Vars, nil)
	v, _, err := Run(c, in)
	if err != nil {
		t.Fatalf("求值失败 %q: %v", src, err)
	}
	return v, env
}

// TestExclusionSemanticsOpenAI OpenAI 口径排除语义金样本：
// 公式引用细分变量时 p/c 必须扣除对应部分，未引用时保留按基础价计费
func TestExclusionSemanticsOpenAI(t *testing.T) {
	u := openAIUsage()
	baseInput, output, cacheRead, cc5m, cc1h := resolveRef(u)
	// OpenAI: baseInput = 30000 - 8000 - 5000 = 17000（含图片1500+音频500+文本15000）
	if baseInput != 17000 || output != 6000 || cacheRead != 8000 || cc5m != 5000 || cc1h != 0 {
		t.Fatalf("resolveRef 基准值异常: %d %d %d %d %d", baseInput, output, cacheRead, cc5m, cc1h)
	}
	in := FromUsage(baseInput, output, cacheRead, cc5m, cc1h, u)

	// 1) 只用 p：图片/音频输入 token 保留在 p 内（17000）
	_, env := mustRun(t, "p", in)
	if env["p"].(float64) != 17000 {
		t.Fatalf("未引用细分时 p 应保留全部模态 token: %v", env["p"])
	}

	// 2) 引用 img/ai：p 扣除图片与音频 → 15000
	_, env = mustRun(t, "p + img + ai", in)
	if env["p"].(float64) != 15000 {
		t.Fatalf("引用 img/ai 后 p 应扣除模态 token: %v", env["p"])
	}
	if env["img"].(float64) != 1500 || env["ai"].(float64) != 500 {
		t.Fatalf("img/ai 变量值异常: %v %v", env["img"], env["ai"])
	}

	// 3) 引用 rt/ao：c 扣除推理与音频输出 → 3500
	_, env = mustRun(t, "c + rt + ao", in)
	if env["c"].(float64) != 3500 {
		t.Fatalf("引用 rt/ao 后 c 应扣除对应 token: %v", env["c"])
	}
	if env["rt"].(float64) != 2000 || env["ao"].(float64) != 500 {
		t.Fatalf("rt/ao 变量值异常: %v %v", env["rt"], env["ao"])
	}

	// 4) len = 完整输入上下文（baseInput + 缓存各项 = 30000）
	_, env = mustRun(t, "ctx", in)
	if env["ctx"].(float64) != 30000 {
		t.Fatalf("len 应为完整输入上下文 30000: %v", env["ctx"])
	}

	// 5) 缓存变量：cc 为合并桶（cache_write 5000）
	_, env = mustRun(t, "cr + cc + cc1h", in)
	if env["cr"].(float64) != 8000 || env["cc"].(float64) != 5000 || env["cc1h"].(float64) != 0 {
		t.Fatalf("缓存变量异常: %v %v %v", env["cr"], env["cc"], env["cc1h"])
	}

	// 6) 金额口径：p*2.5 → 系数 17000*2.5=42500，金额 = 42500/1e6 = 0.0425 本位币
	v, _ := mustRun(t, "p * 2.5", in)
	if v != 42500 {
		t.Fatalf("系数计算异常: %v", v)
	}

	// 7) 排除后总账守恒：p + img + ai + cr + cc = 30000（无双重计费、无漏计）
	v, _ = mustRun(t, "p + img + ai + cr + cc", in)
	if v != 30000 {
		t.Fatalf("排除语义守恒校验失败: %v != 30000", v)
	}
}

// TestExclusionSemanticsClaude Claude 口径排除语义金样本：
// input 不含缓存（resolveTokenCounts 不扣），cc/cc1h 走 TTL 分桶
func TestExclusionSemanticsClaude(t *testing.T) {
	u := claudeUsage()
	baseInput, output, cacheRead, cc5m, cc1h := resolveRef(u)
	if baseInput != 15000 || cacheRead != 8000 || cc5m != 4000 || cc1h != 1000 {
		t.Fatalf("resolveRef 基准值异常: %d %d %d %d", baseInput, cacheRead, cc5m, cc1h)
	}
	in := FromUsage(baseInput, output, cacheRead, cc5m, cc1h, u)

	// len = 15000 + 8000 + 4000 + 1000 = 28000（统一口径的完整上下文）
	_, env := mustRun(t, "ctx", in)
	if env["ctx"].(float64) != 28000 {
		t.Fatalf("Claude len 应补加缓存: %v", env["ctx"])
	}

	// Claude 语义（OpenAISemantics=false）：即使公式引用 img/ai 也不从 p 扣（细分缺省 0，防三方言口径误扣）
	_, env = mustRun(t, "p + img + ai", in)
	if env["p"].(float64) != 15000 {
		t.Fatalf("Claude 语义 p 不应扣减: %v", env["p"])
	}

	// cc/cc1h TTL 分桶直取
	_, env = mustRun(t, "cc * 3.75 + cc1h * 6", in)
	if env["cc"].(float64) != 4000 || env["cc1h"].(float64) != 1000 {
		t.Fatalf("Claude TTL 分桶异常: %v %v", env["cc"], env["cc1h"])
	}

	// 引用 rt：c 扣除推理 token → 4000
	_, env = mustRun(t, "c + rt", in)
	if env["c"].(float64) != 4000 || env["rt"].(float64) != 2000 {
		t.Fatalf("Claude c/rt 异常: %v %v", env["c"], env["rt"])
	}
}

// TestRunRejectsInvalid 求值兜底：负值 / 非有限数 / 非数值结果必须报错
func TestRunRejectsInvalid(t *testing.T) {
	// p>0 的向量：负值公式在求值时才会暴露（p=0 时 -p 为 -0.0，已归一按免费放行）
	in := FormulaInput{BaseInputTokens: 1000, OutputTokens: 500, ImageCount: 1}

	cases := []struct {
		src  string
		fail bool
	}{
		{"p * 2.5", false},
		{"p * 0 + c * 0", false},             // 0 合法（免费档）
		{"-p * 2.5", true},                   // 负值
		{"ctx > 2000000 ? p * 2 : -1", true}, // ctx=1000 走 false 分支 → -1
		{"p / max(ctx - 2000000, 0)", true},  // 除零 → Inf
		{"p == 0", true},                     // 布尔结果不是数值
	}
	for _, tc := range cases {
		c, err := Compile(tc.src)
		if err != nil {
			if tc.fail {
				continue
			}
			t.Fatalf("编译失败 %q: %v", tc.src, err)
		}
		_, _, err = Run(c, in)
		if tc.fail && err == nil {
			t.Fatalf("公式 %q 应当报错", tc.src)
		}
		if !tc.fail && err != nil {
			t.Fatalf("公式 %q 不应报错: %v", tc.src, err)
		}
	}
}

// TestBuiltinFixed fixed() 单位换算：fixed(0.04) → 系数 40000（每 1M 口径），金额 = 0.04
func TestBuiltinFixed(t *testing.T) {
	in := FormulaInput{BaseInputTokens: 1000, OutputTokens: 1000, ImageCount: 4}
	v, _ := mustRun(t, "fixed(0.04)", in)
	if v != 40000 {
		t.Fatalf("fixed(0.04) 系数异常: %v", v)
	}
	// 按张放大：fixed(0.04) * image_count → 0.16/次
	v, _ = mustRun(t, "fixed(0.04) * image_count", in)
	if v != 160000 {
		t.Fatalf("fixed×张数异常: %v", v)
	}
	// 混合：token 系数 + 按次附加费
	v, _ = mustRun(t, "p * 5 + c * 20 + fixed(0.001)", in)
	expected := 1000*5 + 1000*20 + 1000.0
	if math.Abs(v-expected) > 1e-9 {
		t.Fatalf("混合公式异常: %v != %v", v, expected)
	}
	// 整数字面量参数（expr 折叠为 int）
	v, _ = mustRun(t, "fixed(1)", in)
	if v != 1_000_000 {
		t.Fatalf("fixed(1) 系数异常: %v", v)
	}
}

// TestSmokeTestCases 烟测向量全组通过 + 典型拒绝用例
func TestSmokeTestCases(t *testing.T) {
	valid := []string{
		"p * 5 + c * 20",
		"ctx <= 128000 ? p * 5 + c * 20 : p * 10 + c * 40",
		"p * 5 + cr * 1.25 + cc * 12.5 + cc1h * 25 + c * 50",
		"c * 10 + rt * 30",
		"p * 5 + img * 8 + ai * 10 + ao * 20 + c * 30",
		"fixed(0.04) * image_count",
		"max(ctx - 128000, 0) * 0 + p * 5", // max/min 可用
	}
	for _, src := range valid {
		if err := SmokeTest(src); err != nil {
			t.Fatalf("合法公式 %q 烟测失败: %v", src, err)
		}
	}

	invalid := []string{
		"",                    // 空公式
		"p *",                 // 语法错误
		"-p * 5",              // 负值（空请求向量 p=0 时 0*5=0 合法？不——多模态向量 p>0 时为负）
		"unknown_var * 5",     // 未知变量（运行时报错）
		"p * 5 / (ctx - ctx)", // 除零
	}
	for _, src := range invalid {
		if err := SmokeTest(src); err == nil {
			t.Fatalf("非法公式 %q 应被烟测拒绝", src)
		}
	}
}

// TestVarExtraction 变量引用提取（决定排除语义的范围）
func TestVarExtraction(t *testing.T) {
	cases := []struct {
		src  string
		want []string
	}{
		{"p * 2.5", []string{"p"}},
		{"p*5 + cr*1.25 + cc*12.5 + cc1h*25 + c*50", []string{"c", "cc", "cc1h", "cr", "p"}},
		{"ctx <= 128000 ? p*5 : img*8", []string{"ctx", "img", "p"}},
		{"fixed(0.04) * image_count", []string{"image_count"}}, // fixed 是函数不算变量
		{"c * 10 + rt * 30 + ao * 20", []string{"ao", "c", "rt"}},
	}
	for _, tc := range cases {
		c, err := Compile(tc.src)
		if err != nil {
			t.Fatalf("编译失败 %q: %v", tc.src, err)
		}
		got := strings.Join(VarNames(c.Vars), ",")
		want := strings.Join(tc.want, ",")
		if got != want {
			t.Fatalf("变量提取 %q: got %q want %q", tc.src, got, want)
		}
	}
}

// TestEvaluate 试算接口语义：预置向量 + 环境快照
func TestEvaluate(t *testing.T) {
	c, results, err := Evaluate("p * 2.5 + c * 10", nil)
	if err != nil {
		t.Fatalf("Evaluate 失败: %v", err)
	}
	if len(results) != len(SmokeVectors()) {
		t.Fatalf("预置向量组数异常: %d", len(results))
	}
	// 常规对话向量：p=10000 c=2000 → 10000*2.5 + 2000*10 = 45000
	if results[1].Factor != 45000 {
		t.Fatalf("常规对话向量系数异常: %v", results[1].Factor)
	}
	if c.Vars["p"] != true || c.Vars["c"] != true {
		t.Fatal("变量提取异常")
	}
	// 环境快照不含函数项
	for k := range results[0].Env {
		if !knownVars[k] {
			t.Fatalf("环境快照混入非变量项: %s", k)
		}
	}
}

// TestNilUsage usage 为 nil 的兜底（流中断合成 Usage 场景）
func TestNilUsage(t *testing.T) {
	in := FromUsage(1000, 500, 0, 0, 0, nil)
	v, env := mustRun(t, "p * 2.5 + c * 10", in)
	if v != 1000*2.5+500*10 {
		t.Fatalf("nil usage 求值异常: %v", v)
	}
	if env["ctx"].(float64) != 1000 {
		t.Fatalf("nil usage len 异常: %v", env["ctx"])
	}
}

// TestUFacts u() 旁路计量取值：有值取值、缺失返回 0（fail-soft）
func TestUFacts(t *testing.T) {
	in := FormulaInput{
		BaseInputTokens: 100, OutputTokens: 50, ImageCount: 1,
		Facts: map[string]float64{"audio_seconds": 90, "audio_bytes": 1440000},
	}

	// 按秒计费：90 秒 × 系数 0.4 → 36（每 1M 口径系数，金额 = 36/1e6 × 张数……
	// 音频场景实际语义：金额 = u("audio_seconds") * 0.4 / 1e6，系数 36 偏小——
	// 配置时应写 u("audio_seconds") * 400000（= 每次 0.4 本位币），此处仅验证取值
	v, _ := mustRun(t, `u("audio_seconds") * 400000 + u("audio_bytes") * 0.01`, in)
	if v != 90*400000+1440000*0.01 {
		t.Fatalf("u() 取值异常: %v", v)
	}

	// 缺失 key 返回 0（上游未报字段≠公式错误；预扣路径天然无 facts）
	v, _ = mustRun(t, `u("audio_seconds") * 400000`, FormulaInput{ImageCount: 1})
	if v != 0 {
		t.Fatalf("缺失 facts 应取 0: %v", v)
	}

	// UKeys 提取
	c, err := Compile(`u("audio_seconds") > 60 ? u("audio_bytes") * 0.01 : fixed(0.01)`)
	if err != nil {
		t.Fatalf("编译失败: %v", err)
	}
	if len(c.UKeys) != 2 || c.UKeys[0] != "audio_seconds" || c.UKeys[1] != "audio_bytes" {
		t.Fatalf("UKeys 提取异常: %v", c.UKeys)
	}
}

// TestUKeyValidation u() key 注册表校验（保存烟测与试算同口径拒绝未注册 key）
func TestUKeyValidation(t *testing.T) {
	// 未注册 key：SmokeTest 与 Evaluate 都拒绝
	if err := SmokeTest(`u("audio_second") * 400000`); err == nil {
		t.Fatal("未注册 key（拼写错误 audio_second）应被烟测拒绝")
	}
	if _, _, err := Evaluate(`u("audio_second") * 400000`, nil); err == nil {
		t.Fatal("未注册 key 应被试算拒绝")
	}
	// 已注册 key 通过
	if err := SmokeTest(`u("audio_seconds") * 400000`); err != nil {
		t.Fatalf("已注册 key 烟测失败: %v", err)
	}
	// 动态 key 编译拒绝（无法静态校验）
	if _, err := Compile(`u(p) * 400000`); err == nil {
		t.Fatal("动态 key（u(p)）应被编译拒绝")
	}
}

// TestUFactsSnapshot 试算环境快照携带 u:key 条目
func TestUFactsSnapshot(t *testing.T) {
	_, results, err := Evaluate(`u("audio_seconds") * 400000`, nil)
	if err != nil {
		t.Fatalf("Evaluate 失败: %v", err)
	}
	found := false
	for _, r := range results {
		if r.Name == "音频计量" {
			found = true
			if r.Env["u:audio_seconds"].(float64) != 60 {
				t.Fatalf("音频计量向量的 u:audio_seconds 快照异常: %v", r.Env)
			}
			if r.Factor != 60*400000 {
				t.Fatalf("音频计量向量系数异常: %v", r.Factor)
			}
		}
	}
	if !found {
		t.Fatal("预置向量缺「音频计量」组")
	}
}

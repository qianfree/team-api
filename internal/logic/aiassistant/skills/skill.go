// Package skills AI 助手的领域技能：知识注入（Reference，尽量动态生成自代码事实，
// 防手写文档漂移）+ 本地校验闭环（Validate，复用保存/上传时的同一套校验器，
// 口径与正式入口完全一致）。
package skills

import (
	"sort"
	"strings"
)

// 技能 key 常量（chat 请求的 skills 字段取值）。
const (
	KeyBillingFormula = "billing_formula"
	KeyJSPlugin       = "js_plugin"
)

// Check 单项校验结果（js_plugin 技能的检查项清单）。
type Check struct {
	Name   string `json:"name"`
	Status string `json:"status"` // pass=通过 | warn=提示 | fail=缺陷
	Detail string `json:"detail"`
}

// EvalPreview 一组烟测向量的试算结果（billing_formula 技能）。
// Factor 为「每 1M token 口径的价格系数」，金额 = 系数 / 1e6 × 租户乘数 × 时段乘数。
type EvalPreview struct {
	Name   string         `json:"name"`
	Factor float64        `json:"factor"`
	Env    map[string]any `json:"env,omitempty"` // 该向量下公式引用变量的取值快照
}

// Report 技能校验报告（SSE validate 事件的负载）。
type Report struct {
	Skill   string        `json:"skill"`
	OK      bool          `json:"ok"`
	Checks  []Check       `json:"checks,omitempty"`
	Preview []EvalPreview `json:"preview,omitempty"`
	Error   string        `json:"error,omitempty"` // 编译/求值失败原因（billing_formula）
}

// Skill 领域技能接口。新增技能（override 规则、SQL 思路等）实现本接口并注册。
type Skill interface {
	Key() string                  // 唯一标识（chat 请求 skills 字段取值）
	Label() string                // 中文标签
	Fence() string                // 约定的四反引号围栏语言标记（提示词中告知模型）
	Reference() string            // 注入 system prompt 的领域参考
	Validate(code string) *Report // 本地校验（不调外部服务）
}

var registry = map[string]Skill{
	KeyBillingFormula: BillingFormulaSkill{},
	KeyJSPlugin:       JSPluginSkill{},
}

// Get 按 key 取技能。
func Get(key string) (Skill, bool) {
	s, ok := registry[key]
	return s, ok
}

// Keys 全部已注册技能 key（排序稳定，供提示词与前端技能清单）。
func Keys() []string {
	out := make([]string, 0, len(registry))
	for k := range registry {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// PromptFor 拼装技能注入段：共享的代码输出协议 + 各技能领域参考（按 key 升序）。
// 未识别的 key 静默忽略（调用方已过滤）；全空返回空串。
func PromptFor(keys []string) string {
	valid := make([]Skill, 0, len(keys))
	seen := map[string]bool{}
	sorted := append([]string(nil), keys...)
	sort.Strings(sorted)
	for _, k := range sorted {
		if seen[k] {
			continue
		}
		if s, ok := registry[k]; ok {
			valid = append(valid, s)
			seen[k] = true
		}
	}
	if len(valid) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString(codeProtocolPrompt(valid))
	b.WriteString("\n\n")
	for i, s := range valid {
		if i > 0 {
			b.WriteString("\n\n")
		}
		b.WriteString(s.Reference())
	}
	return b.String()
}

// codeProtocolPrompt 共享的代码输出协议：四反引号围栏 + 完整代码。
// 围栏语言标记按激活技能给出（多技能时模型按任务任选其一）。
func codeProtocolPrompt(active []Skill) string {
	var fences []string
	for _, s := range active {
		fences = append(fences, "````"+s.Fence())
	}
	var b strings.Builder
	b.WriteString("【代码输出协议（必须严格遵守）】\n")
	b.WriteString("1. 本轮产出代码时，最终代码必须完整放在一个四反引号围栏中：开栏行是 ")
	b.WriteString(strings.Join(fences, " 或 "))
	b.WriteString("，闭栏行是连续四个反引号（无其他内容）。\n")
	b.WriteString("2. 围栏内必须是完整可用的最终代码：不要 diff、不要片段、不要「省略其余部分」，修复时重新输出完整代码。\n")
	b.WriteString("3. 围栏内的代码不得出现连续四个及以上的反引号（模板字符串等正常使用一至三个反引号不受影响）；讲解代码片段用普通三反引号围栏。\n")
	b.WriteString("4. 一条回复最多一个四反引号围栏；解释、说明、风险提示都写在围栏之外。")
	return b.String()
}

package skills

import (
	"fmt"
	"sort"
	"strings"

	"github.com/qianfree/team-api/internal/logic/billing/formula"
)

// BillingFormulaSkill 计费表达式生成技能。
// 变量表与 u() key 注册表均运行时从 formula 包生成（KnownVarDocs / KnownFactKeys），
// 含 JS 插件 usageSchema 动态注册的计费键——新增变量/事实键不需要改这里。
type BillingFormulaSkill struct{}

// Key 技能标识。
func (BillingFormulaSkill) Key() string { return KeyBillingFormula }

// Label 中文标签。
func (BillingFormulaSkill) Label() string { return "计费表达式" }

// Fence 围栏语言标记。
func (BillingFormulaSkill) Fence() string { return "formula" }

// Reference 领域参考（动态生成）。
func (BillingFormulaSkill) Reference() string {
	var b strings.Builder
	b.WriteString("【技能：计费表达式生成】\n")
	b.WriteString("为平台的「表达式计费」（billing_mode=formula）编写定价公式。\n\n")

	// 变量表：动态取自 formula.KnownVarDocs（唯一事实源）
	b.WriteString("一、可用变量（token 口径，随请求归集）：\n")
	docs := formula.KnownVarDocs()
	names := make([]string, 0, len(docs))
	for name := range docs {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		b.WriteString(fmt.Sprintf("- %s：%s\n", name, docs[name]))
	}

	// u() facts 注册表：动态取自 KnownFactKeys（含插件动态注册键）
	b.WriteString("\n二、u(\"key\") 旁路计量函数（非 token 计量：秒数/字节数/插件自定义单位）。\n")
	b.WriteString("key 必须是字符串字面量且已注册，可用 key 注册表：\n")
	facts := formula.KnownFactKeys()
	factNames := make([]string, 0, len(facts))
	for name := range facts {
		factNames = append(factNames, name)
	}
	sort.Strings(factNames)
	for _, name := range factNames {
		b.WriteString(fmt.Sprintf("- %s：%s\n", name, facts[name]))
	}
	b.WriteString("引用未注册 key 的公式在保存时会被直接拒绝。\n\n")

	b.WriteString(`三、其他函数：
- header("key")：取请求头值（缺失为空串）；param("path")：取请求体 JSON 点分路径值（缺失为 nil）；has(v, "s")：子串包含判断
- hour("tz") / minute("tz") / weekday("tz") / month("tz") / day("tz")：指定 IANA 时区的钟表时间数字（tz 省略用平台计费时区）
- fixed(x)：按次计费金额，x 为每次请求的本位币金额（如 fixed(0.05) = 每次 0.05）
- min / max / abs / ceil / floor：数值辅助（可变参数）

四、语义与硬规则：
- 公式结果 = 「每 1M token 口径的价格系数」，最终金额 = 系数 / 1e6 × 租户乘数 × 时段乘数。
  因此各 token 变量的系数就是该类 token 每 1M 的本位币单价，如 "p*2+c*6" 表示输入 2/M、输出 6/M。
- 算术、比较、三目、&&/|| 均可用；长上下文阶梯用三目，如 "ctx <= 128000 ? p*5+c*20 : p*10+c*40"。
- 结果必须为非负有限数（负数/NaN/Inf 会被拒绝）。
- 保存时系统会用 7 组预置烟测向量（空请求/常规对话/多模态满配/长上下文/多张图片/音频计量/视频任务）
  全组求值校验，任一向量失败即拒绝。
- 围栏内只放表达式本身（通常一行），不要放解释。
`)
	return b.String()
}

// Validate 复用 formula.Evaluate（与管理端「校验并试算」、保存烟测同一口径）。
func (BillingFormulaSkill) Validate(code string) *Report {
	src := strings.TrimSpace(code)
	if src == "" {
		return &Report{Skill: KeyBillingFormula, OK: false, Error: "表达式为空"}
	}
	_, results, err := formula.Evaluate(src, nil)
	if err != nil {
		return &Report{Skill: KeyBillingFormula, OK: false, Error: err.Error()}
	}
	preview := make([]EvalPreview, 0, len(results))
	for _, r := range results {
		preview = append(preview, EvalPreview{Name: r.Name, Factor: r.Factor, Env: r.Env})
	}
	return &Report{
		Skill:   KeyBillingFormula,
		OK:      true,
		Preview: preview,
		Checks: []Check{{
			Name:   "evaluate",
			Status: "pass",
			Detail: fmt.Sprintf("编译、u() key 注册校验与 %d 组烟测向量求值全部通过", len(results)),
		}},
	}
}

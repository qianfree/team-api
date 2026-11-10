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
	b.WriteString("一、可用变量（token 口径，随请求归集；ctx 相当于其他平台的 len）：\n")
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

	b.WriteString(`三、p/c 自动排除：
p、c 是兜底变量，代表表达式中没有单独计价的全部输入/输出 token。表达式写了某个细分变量（如 cr），
这部分 token 就自动从 p 中扣除，避免重复计费；没写的细分变量仍留在 p/c 中按基础价计。
因此缓存/图片/音频不需要单独定价时，直接省略这些变量即可，不会漏收费。
ctx 不受自动排除影响，始终是完整输入长度，档位条件必须用 ctx 而不是 p（缓存命中会让 p 变小，导致误判档位）。

四、内置函数：
- tier("档位名", 计价式)：标记计价档位，值等于计价式，必须包裹每个叶子分支（单档也要包）。
  账单据此展示命中档位，并把「变量 × 单价」逐项列出明细
- fixed(x)：按次金额，x 为每次请求的本位币金额，如 tier("per_request", fixed(0.01))；
  可乘 image_count（按张）或 seconds（按秒），如 fixed(0.04) * image_count
- min / max / abs / ceil / floor：数值辅助（会让账单无法拆分明细，非必要不用）
- header("key")：请求头值（字符串，缺失为空串，只能做等于/has 匹配，不能和数字比大小）；
  param("path")：请求体 JSON 点分路径值（缺失为 nil）。和数字比大小时必须写成 (param("x") ?? 0) >= 10，
  否则请求不带该参数时求值报错、保存校验不通过；has(v, "s")：子串包含（函数，不是运算符）
- hour(tz) / minute(tz) / weekday(tz) / month(tz) / day(tz)：时间函数，tz 为 IANA 时区（如 "Asia/Shanghai"），
  可省略（省略 = 平台计费时区）。weekday 取值 0=周日、1~5=周一至周五、6=周六

五、价格系数：
表达式中的数字就是本位币每 1M token 单价，如 p * 2.5 表示输入 2.5/1M。公式结果 = 每 1M 口径系数，
最终金额 = 系数 / 1e6 × 租户乘数。结果必须为非负有限数；保存时会用 7 组预置烟测向量全组求值校验。

六、两个输入框：基础公式 × 请求规则
管理端编辑器分两个输入框，最终计费 = 基础公式 × 请求规则：
- 基础公式：只写按用量分档的单价（tier 档位链）。
- 请求规则：按时间、请求头、请求参数加价或打折，每条严格写成「(条件 ? 倍数 : 1)」并带外层括号，
  多条用 * 连乘，如 (条件A ? 2 : 1) * (条件B ? 0.8 : 1)。三目优先级最低，不加括号直接相乘会把
  后一条规则吞进前一条的 else 分支，两条同时命中时只生效一条。账单会显示「请求规则: 条件 → 命中 ×倍数」。

七、表达式示例：
简单计价：
tier("base", p * 2.5 + c * 15)

含缓存：
tier("base", p * 2.5 + c * 15 + cr * 0.25 + cc * 3.125)

两档（用 ctx 作条件）：
ctx <= 200000
  ? tier("standard", p * 3 + c * 15 + cr * 0.3 + cc * 3.75)
  : tier("long_context", p * 6 + c * 22.5 + cr * 0.6 + cc * 7.5)

三档：
ctx <= 32000
  ? tier("short", p * 1.2 + c * 6 + cr * 0.24 + cc * 1.5)
  : (ctx <= 128000
    ? tier("mid", p * 2.4 + c * 12 + cr * 0.48 + cc * 3)
    : tier("long", p * 3 + c * 15 + cr * 0.6 + cc * 3.75))

多模态：
tier("base", p * 0.43 + c * 3.06 + img * 0.78 + ai * 3.81 + ao * 15.11)

图片按张 / 视频按秒：
tier("per_image", fixed(0.04) * image_count)
tier("per_second", fixed(0.4) * seconds)

八、规则：
1. 每个叶子分支必须用 tier("名称", 计价式) 包裹，单档也要写成 tier("base", ...)。
2. 档位名用简短英文小写，如 base / standard / long_context / short / mid / long / per_image / per_second。
3. 档位条件用 ctx（不用 p），支持 < <= > >=。
4. 多档用嵌套三目并换行缩进：条件1 ? tier(...) : (条件2 ? tier(...) : tier(...))。
5. 每个档位直接写出该档的最终单价（如超长上下文 1.5 倍就写 p * 1.5 + c * 6），
   不要写成 (p + c * 4) * 1.5，也不要把同一价格表在多个档位间重复后再乘系数。
6. 缓存/图片/音频不需要单独定价时省略对应变量（token 自动计入 p/c）。
   cc1h 默认不写：只写 cc 时，1 小时缓存写入自动按 cc 单价 × 1.6 计；仅当用户明确给出 1 小时写入价时才写 cc1h。
7. 时间、请求头、请求参数相关的加价/折扣一律写进请求规则，严禁写进基础公式的三目分支
   （会让档位数翻倍、价格表重复，账单看不出加价原因）。
   「工作时间」默认指周一至周五 9:00~18:00：weekday() >= 1 && weekday() <= 5 && hour() >= 9 && hour() < 18。

九、输出格式：
- 先写一行「基础公式：」，紧跟一个 formula 围栏只放基础公式；
  有请求规则时再写一行「请求规则：」，紧跟第二个 formula 围栏只放请求规则。没有规则就只输出一个围栏。
- 围栏内只放表达式本身，不要放解释；多档表达式按示例换行缩进。

完整示例：用户要求「输入1，输出4，缓存读0.1，缓存写2，工作时间双倍计费，上下文超过300k费用按1.5倍算」，应输出：
基础公式：
ctx <= 300000
  ? tier("standard", p * 1 + c * 4 + cr * 0.1 + cc * 2)
  : tier("long_context", p * 1.5 + c * 6 + cr * 0.15 + cc * 3)
请求规则：
weekday() >= 1 && weekday() <= 5 && hour() >= 9 && hour() < 18 ? 2 : 1
（实际回复中两个表达式各自放进 formula 围栏）
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

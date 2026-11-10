package formula

import (
	"fmt"
	"math"
	"strings"

	"github.com/expr-lang/expr/ast"
)

// tierFunction 档位埋点内部函数名（保留字，存储的表达式中禁止直接使用）
const tierFunction = "_tier"

// TierDef 编译期提取的命名档位：tier("名称", 计价式) 的名称与线性计价项。
// Terms 为 nil 表示该档位计价式不是线性形态（含三目/min/max/除法等），
// 账单只能展示变量取值与公式结果，无法拆成「数量 × 单价」明细
type TierDef struct {
	Name  string
	Terms []Term
}

// Term 线性计价项：数量来源 × 系数（每 1M 口径，与公式结果同单位）。
// Var 为已知变量名、"u:key"（usage facts）或空串（常数项 = 每次请求固定金额）
type Term struct {
	Var  string
	Coef float64
}

// Item 单次求值的线性分项明细：Value = Quantity × Coef（系数口径，未乘请求规则倍率）
type Item struct {
	Var      string
	Quantity float64
	Coef     float64
	Value    float64
}

// Result 单次求值的完整结果（计费快照留痕用）。
// Vars 与 Factor 出自同一次求值的同一份输入，保证账单变量与金额同源
type Result struct {
	Factor float64
	Rules  []RequestRuleTrace
	// Tiers 命中的档位名（按求值顺序，去重）
	Tiers []string
	// Vars 公式引用变量的取值（含 u:key/header:key/param:path 探测值）
	Vars map[string]any
	// Items 线性分项明细；nil = 不可拆分（非线性计价式，或拆分结果与公式结果不一致）
	Items []Item
	// RuleMultiplier 命中请求规则乘数之积（Items 非空时有效：Σ Item.Value × RuleMultiplier = Factor）
	RuleMultiplier float64
}

// tierPatcher 编译期把 tier("名称", 计价式) 改写为 _tier(idx, 计价式)：数值结果不变，
// 求值时回填命中档位。名称必须是非空字符串字面量（账单展示用，动态名称无法静态校验）
type tierPatcher struct {
	tiers []TierDef
	err   error
}

func (p *tierPatcher) Visit(node *ast.Node) {
	if p.err != nil {
		return
	}
	call, ok := (*node).(*ast.CallNode)
	if !ok {
		return
	}
	id, ok := call.Callee.(*ast.IdentifierNode)
	if !ok || id.Value != "tier" {
		return
	}
	if len(call.Arguments) != 2 {
		p.err = fmt.Errorf(`tier() 需要 2 个参数：tier("档位名", 计价式)`)
		return
	}
	name, ok := call.Arguments[0].(*ast.StringNode)
	if !ok || strings.TrimSpace(name.Value) == "" {
		p.err = fmt.Errorf("tier() 的档位名必须是非空字符串字面量")
		return
	}
	def := TierDef{Name: strings.TrimSpace(name.Value)}
	if terms, linear := linearTerms(call.Arguments[1]); linear {
		def.Terms = mergeTerms(terms)
	}
	idx := len(p.tiers)
	p.tiers = append(p.tiers, def)
	ast.Patch(node, &ast.CallNode{
		Callee:    &ast.IdentifierNode{Value: tierFunction},
		Arguments: []ast.Node{&ast.IntegerNode{Value: idx}, call.Arguments[1]},
	})
}

// linearTerms 识别线性计价式：若干「数量 × 常数系数」与常数项之和。
// 数量 = 已知变量 / u("key")；常数系数 = 数字字面量 / fixed(数字字面量)（及其乘积）。
// 其余形态（三目、函数、除法、减法、取负、变量相乘）一律判为非线性
func linearTerms(node ast.Node) ([]Term, bool) {
	switch n := node.(type) {
	case *ast.IntegerNode:
		return []Term{{Coef: float64(n.Value)}}, true
	case *ast.FloatNode:
		return []Term{{Coef: n.Value}}, true
	case *ast.IdentifierNode:
		if knownVars[n.Value] {
			return []Term{{Var: n.Value, Coef: 1}}, true
		}
	case *ast.CallNode:
		id, ok := n.Callee.(*ast.IdentifierNode)
		if !ok || len(n.Arguments) != 1 {
			return nil, false
		}
		switch id.Value {
		case "fixed":
			// fixed(x) 运行时返回 x×1e6，常数项同口径
			if v, ok := numberLiteral(n.Arguments[0]); ok {
				return []Term{{Coef: v * 1_000_000}}, true
			}
		case "u":
			if str, ok := n.Arguments[0].(*ast.StringNode); ok {
				return []Term{{Var: "u:" + str.Value, Coef: 1}}, true
			}
		}
	case *ast.BinaryNode:
		switch n.Operator {
		case "+":
			left, ok := linearTerms(n.Left)
			if !ok {
				return nil, false
			}
			right, ok := linearTerms(n.Right)
			if !ok {
				return nil, false
			}
			return append(left, right...), true
		case "*":
			if k, ok := constantCoef(n.Left); ok {
				if right, ok := linearTerms(n.Right); ok {
					return scaleTerms(right, k), true
				}
			}
			if k, ok := constantCoef(n.Right); ok {
				if left, ok := linearTerms(n.Left); ok {
					return scaleTerms(left, k), true
				}
			}
		}
	}
	return nil, false
}

// constantCoef 子树是纯常数（只含数字字面量与 fixed(字面量) 的线性组合）时返回其值
func constantCoef(node ast.Node) (float64, bool) {
	terms, ok := linearTerms(node)
	if !ok {
		return 0, false
	}
	sum := 0.0
	for _, t := range terms {
		if t.Var != "" {
			return 0, false
		}
		sum += t.Coef
	}
	return sum, true
}

func numberLiteral(node ast.Node) (float64, bool) {
	switch v := node.(type) {
	case *ast.IntegerNode:
		return float64(v.Value), true
	case *ast.FloatNode:
		return v.Value, true
	}
	return 0, false
}

func scaleTerms(terms []Term, k float64) []Term {
	out := make([]Term, len(terms))
	for i, t := range terms {
		out[i] = Term{Var: t.Var, Coef: t.Coef * k}
	}
	return out
}

// mergeTerms 同一数量来源的系数合并（保持首次出现顺序），如 "p*2 + p*3" → p*5
func mergeTerms(terms []Term) []Term {
	out := make([]Term, 0, len(terms))
	index := map[string]int{}
	for _, t := range terms {
		if i, ok := index[t.Var]; ok {
			out[i].Coef += t.Coef
			continue
		}
		index[t.Var] = len(out)
		out = append(out, t)
	}
	return out
}

// rootTerms 无 tier() 的公式整体按线性形态识别：先剥离外层乘上的请求规则因子
// （EffectiveFormula 组合出的 "(base) * (rules)"），规则倍率由求值后的命中表还原
func rootTerms(root ast.Node) []Term {
	if terms, ok := linearTerms(stripRuleFactors(root)); ok {
		return mergeTerms(terms)
	}
	return nil
}

func stripRuleFactors(node ast.Node) ast.Node {
	if b, ok := node.(*ast.BinaryNode); ok && b.Operator == "*" {
		if isRuleOnly(b.Right) {
			return stripRuleFactors(b.Left)
		}
		if isRuleOnly(b.Left) {
			return stripRuleFactors(b.Right)
		}
	}
	return node
}

// isRuleOnly 子树只由请求规则埋点（_trace/_trace_int）相乘构成
func isRuleOnly(node ast.Node) bool {
	switch n := node.(type) {
	case *ast.CallNode:
		id, ok := n.Callee.(*ast.IdentifierNode)
		return ok && (id.Value == traceFunction || id.Value == traceIntFunction)
	case *ast.BinaryNode:
		return n.Operator == "*" && isRuleOnly(n.Left) && isRuleOnly(n.Right)
	}
	return false
}

// decompose 按命中档位（或整体线性形态）展开分项明细，并以公式结果反向校验：
// Σ 数量×系数 × 命中规则乘数之积 必须等于公式结果，否则判为不可拆分（宁可降级展示，
// 不输出与实际金额对不上的明细——规则嵌在计价式内部、多档位叠加等形态由此兜住）
func decompose(c *Compiled, hit []int, env map[string]any, in FormulaInput, rules []RequestRuleTrace, factor float64) ([]Item, float64) {
	var terms []Term
	if len(c.Tiers) > 0 {
		if len(hit) == 0 {
			return nil, 0
		}
		for _, idx := range hit {
			if c.Tiers[idx].Terms == nil {
				return nil, 0
			}
			terms = append(terms, c.Tiers[idx].Terms...)
		}
		terms = mergeTerms(terms)
	} else {
		terms = c.RootTerms
	}
	if terms == nil {
		return nil, 0
	}

	ruleMul := 1.0
	for _, r := range rules {
		if r.Matched {
			ruleMul *= r.Multiplier
		}
	}

	items := make([]Item, 0, len(terms)+1)
	sum := 0.0
	for _, t := range terms {
		// 1h 缓存写入按 cc 单价 ×1.6 并入 cc 求值：明细拆回两行（5m 原价 + 1h ×1.6 单价），
		// 数量保持真实 token 数
		if t.Var == "cc" && in.cc1hMergedIntoCC(c.Vars) {
			for _, it := range []Item{
				{Var: "cc", Quantity: float64(in.CacheCreation5m), Coef: t.Coef},
				{Var: "cc1h", Quantity: float64(in.CacheCreation1h), Coef: t.Coef * CacheCreation1hMultiplier},
			} {
				it.Value = it.Quantity * it.Coef
				sum += it.Value
				if it.Quantity != 0 {
					items = append(items, it)
				}
			}
			continue
		}
		qty := 1.0
		switch {
		case t.Var == "":
		case strings.HasPrefix(t.Var, "u:"):
			qty = in.Facts[strings.TrimPrefix(t.Var, "u:")]
		default:
			v, ok := toFloat64(env[t.Var])
			if !ok {
				return nil, 0
			}
			qty = v
		}
		value := qty * t.Coef
		sum += value
		// 数量为 0 的项不贡献费用，不进入账单明细
		if qty == 0 {
			continue
		}
		items = append(items, Item{Var: t.Var, Quantity: qty, Coef: t.Coef, Value: value})
	}
	if math.Abs(sum*ruleMul-factor) > 1e-9*math.Max(1, math.Abs(factor)) {
		return nil, 0
	}
	return items, ruleMul
}

// varLabels 变量账单展示名（短标签；完整语义见 knownVarDocs）
var varLabels = map[string]string{
	"p":           "输入",
	"c":           "输出",
	"ctx":         "上下文",
	"cr":          "缓存读取",
	"cc":          "缓存创建(5分钟)",
	"cc1h":        "缓存创建(1小时)",
	"img":         "图片输入",
	"img_o":       "图片输出",
	"ai":          "音频输入",
	"ao":          "音频输出",
	"rt":          "推理输出",
	"image_count": "图片张数",
	"seconds":     "计费秒数",
}

// tokenVars token 计量变量：账单按「N tokens × 单价/1M」展示
var tokenVars = map[string]bool{
	"p": true, "c": true, "ctx": true, "cr": true, "cc": true, "cc1h": true,
	"img": true, "img_o": true, "ai": true, "ao": true, "rt": true,
}

// VarLabel 变量的账单展示名：已知变量取中文短标签；u:/header:/param: 探测值带来源前缀；
// 空串为常数项（按次固定金额）
func VarLabel(name string) string {
	if label, ok := varLabels[name]; ok {
		return label
	}
	switch {
	case name == "":
		return "按次"
	case strings.HasPrefix(name, "u:"):
		return "计量 " + strings.TrimPrefix(name, "u:")
	case strings.HasPrefix(name, "header:"):
		return "请求头 " + strings.TrimPrefix(name, "header:")
	case strings.HasPrefix(name, "param:"):
		return "参数 " + strings.TrimPrefix(name, "param:")
	}
	return name
}

// IsTokenVar 变量是否为 token 计量（单价按每 1M 展示）
func IsTokenVar(name string) bool {
	return tokenVars[name]
}

// VarUnit 变量数量的计量单位（账单展示用；u:key 单位由插件/渠道定义，返回空串）
func VarUnit(name string) string {
	switch {
	case tokenVars[name]:
		return "tokens"
	case name == "image_count":
		return "张"
	case name == "seconds":
		return "秒"
	case name == "":
		return "次"
	}
	return ""
}

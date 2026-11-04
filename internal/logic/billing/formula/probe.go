package formula

import (
	"fmt"

	"github.com/expr-lang/expr/ast"
	"github.com/expr-lang/expr/vm"
)

// 请求规则埋点内部函数名（保留字，存储的表达式中禁止直接使用）
const (
	traceFunction    = "_trace"
	traceIntFunction = "_trace_int"
)

// requestProbes 请求探针函数名集合：条件引用其中至少一个的三目才参与规则埋点
var requestProbes = map[string]bool{
	"param":   true,
	"header":  true,
	"hour":    true,
	"minute":  true,
	"weekday": true,
	"month":   true,
	"day":     true,
}

// requestRulePatcher 编译期给请求倍率三目加 trace 副作用（参考 new-api billingexpr
// 的 requestRulePatcher）：形状严格为「<探针条件> ? <数字字面量> : 1」的节点原地
// 改写为 _trace(ruleIndex, 条件, 乘数)。数值结果完全等价（命中返回乘数、否则 1），
// 仅顺带在 trace 表中标记该规则命中——「未命中」与「被短路跳过」都保持未命中。
// 其他形状的三目（如 cond ? 2 : 1.5、回退非 1）正常求值但不埋点不提取
type requestRulePatcher struct {
	rules                []RequestRuleTrace
	restrictedIdentifier string
}

func (p *requestRulePatcher) Visit(node *ast.Node) {
	if identifier, ok := (*node).(*ast.IdentifierNode); ok {
		switch identifier.Value {
		case traceFunction, traceIntFunction:
			p.restrictedIdentifier = identifier.Value
		}
		return
	}

	conditional, ok := (*node).(*ast.ConditionalNode)
	if !ok || !conditional.Ternary || !usesRequestProbe(conditional.Cond) {
		return
	}
	multiplier, ok := requestRuleNumber(conditional.Exp1)
	fallback, fallbackOK := requestRuleNumber(conditional.Exp2)
	if !ok || !fallbackOK || fallback != 1 {
		return
	}

	ruleIndex := len(p.rules)
	p.rules = append(p.rules, RequestRuleTrace{
		Cond:       conditional.Cond.String(),
		Multiplier: multiplier,
	})

	// 整型乘数走 _trace_int 保持整数语义（如参与 % 运算的因子），其余统一 float64
	traceFn := traceFunction
	var multiplierNode ast.Node = &ast.FloatNode{Value: multiplier}
	if _, mIsInt := conditional.Exp1.(*ast.IntegerNode); mIsInt {
		if _, fIsInt := conditional.Exp2.(*ast.IntegerNode); fIsInt {
			traceFn = traceIntFunction
			multiplierNode = conditional.Exp1
		}
	}

	ast.Patch(node, &ast.CallNode{
		Callee: &ast.IdentifierNode{Value: traceFn},
		Arguments: []ast.Node{
			&ast.IntegerNode{Value: ruleIndex},
			conditional.Cond,
			multiplierNode,
		},
	})
}

// requestRuleNumber 规则乘数/回退分支必须是数字字面量
func requestRuleNumber(node ast.Node) (float64, bool) {
	switch value := node.(type) {
	case *ast.IntegerNode:
		return float64(value.Value), true
	case *ast.FloatNode:
		return value.Value, true
	default:
		return 0, false
	}
}

// usesRequestProbe 条件子树是否引用至少一个请求探针函数
func usesRequestProbe(node ast.Node) bool {
	return ast.Find(node, func(node ast.Node) bool {
		identifier, ok := node.(*ast.IdentifierNode)
		return ok && requestProbes[identifier.Value]
	}) != nil
}

// extractProbes AST 提取 param()/header() 引用的字面路径/键集合（探测集合）。
// 与 u() 同口径：参数必须是字符串字面量（动态参数无法静态提取探测集合供任务
// 路径冻结，编译即拒绝），重复引用去重保持首次出现顺序
func extractProbes(program *vm.Program) (paramPaths, headerKeys []string, err error) {
	c := &probeCollector{
		paramSeen: map[string]bool{},
		headSeen:  map[string]bool{},
	}
	root := program.Node()
	ast.Walk(&root, c)
	return c.paramPaths, c.headerKeys, c.err
}

type probeCollector struct {
	paramPaths []string
	headerKeys []string
	paramSeen  map[string]bool
	headSeen   map[string]bool
	err        error
}

func (c *probeCollector) Visit(node *ast.Node) {
	if c.err != nil {
		return
	}
	call, ok := (*node).(*ast.CallNode)
	if !ok {
		return
	}
	id, ok := call.Callee.(*ast.IdentifierNode)
	if !ok {
		return
	}
	switch id.Value {
	case "param":
		key, err := literalKey(call, "param")
		if err != nil {
			c.err = err
			return
		}
		if !c.paramSeen[key] {
			c.paramSeen[key] = true
			c.paramPaths = append(c.paramPaths, key)
		}
	case "header":
		key, err := literalKey(call, "header")
		if err != nil {
			c.err = err
			return
		}
		if !c.headSeen[key] {
			c.headSeen[key] = true
			c.headerKeys = append(c.headerKeys, key)
		}
	}
}

// literalKey 校验 param()/header() 的单字面量参数并返回原值（header 键在
// resolve 时统一小写归一，此处保留原文供快照展示）
func literalKey(call *ast.CallNode, name string) (string, error) {
	if len(call.Arguments) != 1 {
		return "", fmt.Errorf("%s() 需要 1 个字符串字面量参数", name)
	}
	str, ok := call.Arguments[0].(*ast.StringNode)
	if !ok {
		return "", fmt.Errorf("%s() 的参数必须是字符串字面量（动态参数无法静态提取探测集合）", name)
	}
	if str.Value == "" {
		return "", fmt.Errorf("%s() 的参数不能为空", name)
	}
	return str.Value, nil
}

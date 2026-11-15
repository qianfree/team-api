package formula

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/expr-lang/expr/ast"
)

// 请求规则条件的中文描述（账单展示用，租户可见，不暴露表达式原文）。
// 编译期从条件 AST 生成，覆盖管理端可视化编辑器能生成的全部形态与常见手写形态：
// 星期（单值/区间/枚举）、时间段（小时或分钟精度、跨午夜）、请求参数/请求头的
// 等于/不等于/包含/存在/数值比较。无法描述的条件返回空串，展示方回退通用文案。

var weekdayNames = []string{"周日", "周一", "周二", "周三", "周四", "周五", "周六"}

var cmpNames = map[string]string{
	"==": "等于", "!=": "不等于", ">": "大于", ">=": "大于等于", "<": "小于", "<=": "小于等于",
}

// conjState 一组 && 条件的描述累积：星期/时间上下界合并后统一成区间描述
type conjState struct {
	wLo, wHi int // 星期区间（-1 = 未设置）
	wEq      []int
	tLo, tHi int // 时间区间（分钟，-1 = 未设置；下界含、上界不含）
	parts    []string
	tz       string
	tzSet    bool
	ok       bool
}

// describeRuleCond 条件 AST → 中文描述；无法描述返回空串
func describeRuleCond(node ast.Node) string {
	st := &conjState{wLo: -1, wHi: -1, tLo: -1, tHi: -1, ok: true}
	for _, part := range flatten(node, "&&") {
		st.addConjunct(part)
		if !st.ok {
			return ""
		}
	}
	return st.finish()
}

func flatten(node ast.Node, op string) []ast.Node {
	if b, ok := node.(*ast.BinaryNode); ok && b.Operator == op {
		return append(flatten(b.Left, op), flatten(b.Right, op)...)
	}
	return []ast.Node{node}
}

func (st *conjState) useTZ(tz string) {
	if !st.tzSet {
		st.tz, st.tzSet = tz, true
		return
	}
	if st.tz != tz {
		st.ok = false // 同一规则混用多个时区：不做合并描述
	}
}

func (st *conjState) addConjunct(node ast.Node) {
	if b, ok := node.(*ast.BinaryNode); ok && b.Operator == "||" {
		text, ok := st.describeOr(flatten(b, "||"))
		if !ok {
			st.ok = false
			return
		}
		st.parts = append(st.parts, text)
		return
	}
	b, ok := node.(*ast.BinaryNode)
	if ok {
		if fn, tz, ok := timeProbe(b.Left); ok {
			n, isNum := intLiteral(b.Right)
			if !isNum {
				st.ok = false
				return
			}
			st.useTZ(tz)
			switch fn {
			case "weekday":
				st.addWeekday(b.Operator, n)
			case "hour":
				st.addTime(b.Operator, n*60, 60)
			case "minuteOfDay":
				st.addTime(b.Operator, n, 1)
			default:
				st.parts = append(st.parts, describeCalendar(fn, b.Operator, n))
			}
			return
		}
	}
	text, ok := describeProbeAtom(node)
	if !ok {
		st.ok = false
		return
	}
	st.parts = append(st.parts, text)
}

func (st *conjState) addWeekday(op string, n int) {
	switch op {
	case "==":
		st.wEq = append(st.wEq, n)
	case ">=":
		st.wLo = n
	case ">":
		st.wLo = n + 1
	case "<=":
		st.wHi = n
	case "<":
		st.wHi = n - 1
	default:
		st.ok = false
	}
}

// addTime 时间界统一换算为「下界含、上界不含」的分钟数；unit 为该比较量的精度（小时 60 / 分钟 1）
func (st *conjState) addTime(op string, minutes, unit int) {
	switch op {
	case ">=":
		st.tLo = minutes
	case ">":
		st.tLo = minutes + unit
	case "<":
		st.tHi = minutes
	case "<=":
		st.tHi = minutes + unit
	case "==":
		st.tLo, st.tHi = minutes, minutes+unit
	default:
		st.ok = false
	}
}

func (st *conjState) finish() string {
	var head []string
	switch {
	case len(st.wEq) > 0:
		if st.wLo >= 0 || st.wHi >= 0 {
			return ""
		}
		head = append(head, describeDays(st.wEq))
	case st.wLo >= 0 || st.wHi >= 0:
		lo, hi := max(st.wLo, 0), st.wHi
		if hi < 0 || hi > 6 {
			hi = 6
		}
		if lo > hi {
			return ""
		}
		days := make([]int, 0, hi-lo+1)
		for d := lo; d <= hi; d++ {
			days = append(days, d)
		}
		head = append(head, describeDays(days))
	}
	if st.tLo >= 0 || st.tHi >= 0 {
		lo, hi := max(st.tLo, 0), st.tHi
		if hi < 0 {
			hi = 1440
		}
		if lo >= hi {
			return ""
		}
		head = append(head, fmt.Sprintf("%s–%s", clock(lo), clock(hi)))
	}
	parts := append(head, st.parts...)
	if len(parts) == 0 {
		return ""
	}
	text := strings.Join(parts, " 且 ")
	if st.tz != "" {
		text += "（" + st.tz + "）"
	}
	return text
}

// describeOr 「||」组：星期枚举 / 跨午夜时间段；其余逐项描述以「或」连接
func (st *conjState) describeOr(nodes []ast.Node) (string, bool) {
	days := make([]int, 0, len(nodes))
	allWeekday := true
	lo, hi := -1, -1
	timeOK := len(nodes) == 2
	for _, n := range nodes {
		b, ok := n.(*ast.BinaryNode)
		if !ok {
			allWeekday, timeOK = false, false
			break
		}
		fn, tz, ok := timeProbe(b.Left)
		v, isNum := intLiteral(b.Right)
		if !ok || !isNum {
			allWeekday, timeOK = false, false
			break
		}
		st.useTZ(tz)
		if fn == "weekday" && b.Operator == "==" {
			days = append(days, v)
		} else {
			allWeekday = false
		}
		scale := map[string]int{"hour": 60, "minuteOfDay": 1}[fn]
		if scale == 0 {
			timeOK = false
			continue
		}
		switch b.Operator {
		case ">=":
			lo = v * scale
		case "<":
			hi = v * scale
		default:
			timeOK = false
		}
	}
	if allWeekday && len(days) == len(nodes) {
		return describeDays(days), true
	}
	if timeOK && lo >= 0 && hi >= 0 {
		if lo > hi {
			return fmt.Sprintf("%s–次日 %s", clock(lo), clock(hi)), true
		}
	}
	parts := make([]string, 0, len(nodes))
	for _, n := range nodes {
		text := describeRuleCond(n)
		if text == "" {
			return "", false
		}
		parts = append(parts, text)
	}
	return "（" + strings.Join(parts, " 或 ") + "）", true
}

// timeProbe 识别时间探针：weekday/hour/month/day/minute(tz)，以及 hour(tz)*60+minute(tz)（一天内分钟数）
func timeProbe(node ast.Node) (fn string, tz string, ok bool) {
	if call, isCall := node.(*ast.CallNode); isCall {
		id, isID := call.Callee.(*ast.IdentifierNode)
		if !isID {
			return "", "", false
		}
		switch id.Value {
		case "weekday", "hour", "month", "day", "minute":
			tz, ok := tzArgument(call)
			return id.Value, tz, ok
		}
		return "", "", false
	}
	b, isBin := node.(*ast.BinaryNode)
	if !isBin || b.Operator != "+" {
		return "", "", false
	}
	mul, isMul := b.Left.(*ast.BinaryNode)
	if !isMul || mul.Operator != "*" {
		return "", "", false
	}
	hFn, hTZ, hOK := timeProbe(mul.Left)
	sixty, isNum := intLiteral(mul.Right)
	mFn, mTZ, mOK := timeProbe(b.Right)
	if hOK && mOK && isNum && sixty == 60 && hFn == "hour" && mFn == "minute" && hTZ == mTZ {
		return "minuteOfDay", hTZ, true
	}
	return "", "", false
}

func tzArgument(call *ast.CallNode) (string, bool) {
	if len(call.Arguments) == 0 {
		return "", true
	}
	if s, ok := call.Arguments[0].(*ast.StringNode); ok {
		return s.Value, true
	}
	return "", false
}

func intLiteral(node ast.Node) (int, bool) {
	switch v := node.(type) {
	case *ast.IntegerNode:
		return v.Value, true
	case *ast.FloatNode:
		if v.Value == float64(int(v.Value)) {
			return int(v.Value), true
		}
	}
	return 0, false
}

// describeCalendar 月份/日期/分钟等日历量的通用描述（如「月份 等于 12」）
func describeCalendar(fn, op string, n int) string {
	name := map[string]string{"month": "月份", "day": "日期", "minute": "分钟"}[fn]
	if op == "==" {
		switch fn {
		case "month":
			return fmt.Sprintf("%d 月", n)
		case "day":
			return fmt.Sprintf("每月 %d 日", n)
		}
	}
	return fmt.Sprintf("%s %s %d", name, cmpNames[op], n)
}

// describeProbeAtom 请求参数/请求头原子条件
func describeProbeAtom(node ast.Node) (string, bool) {
	switch n := node.(type) {
	case *ast.CallNode:
		// has(param("x")|header("x"), "v")
		id, ok := n.Callee.(*ast.IdentifierNode)
		if !ok || id.Value != "has" || len(n.Arguments) != 2 {
			return "", false
		}
		label, ok := probeLabel(n.Arguments[0])
		val, isStr := n.Arguments[1].(*ast.StringNode)
		if !ok || !isStr {
			return "", false
		}
		return fmt.Sprintf("%s 包含 %s", label, val.Value), true
	case *ast.BinaryNode:
		op, known := cmpNames[n.Operator]
		if !known {
			return "", false
		}
		label, ok := probeLabel(n.Left)
		if !ok {
			return "", false
		}
		switch v := n.Right.(type) {
		case *ast.NilNode:
			if n.Operator == "!=" {
				return label + " 存在", true
			}
			if n.Operator == "==" {
				return label + " 不存在", true
			}
		case *ast.StringNode:
			if v.Value == "" && n.Operator == "!=" {
				return label + " 存在", true
			}
			return fmt.Sprintf("%s %s %s", label, op, v.Value), true
		case *ast.IntegerNode:
			return fmt.Sprintf("%s %s %d", label, op, v.Value), true
		case *ast.FloatNode:
			return fmt.Sprintf("%s %s %s", label, op, strconv.FormatFloat(v.Value, 'f', -1, 64)), true
		case *ast.BoolNode:
			return fmt.Sprintf("%s %s %t", label, op, v.Value), true
		}
	}
	return "", false
}

// probeLabel param("x") / (param("x") ?? 0) / header("x") → 「参数 x」/「请求头 x」
func probeLabel(node ast.Node) (string, bool) {
	if b, ok := node.(*ast.BinaryNode); ok && b.Operator == "??" {
		node = b.Left
	}
	call, ok := node.(*ast.CallNode)
	if !ok || len(call.Arguments) != 1 {
		return "", false
	}
	id, ok := call.Callee.(*ast.IdentifierNode)
	s, isStr := call.Arguments[0].(*ast.StringNode)
	if !ok || !isStr {
		return "", false
	}
	switch id.Value {
	case "param":
		return "参数 " + s.Value, true
	case "header":
		return "请求头 " + s.Value, true
	}
	return "", false
}

// describeDays 星期集合描述：周一在前、周日在后；连续 3 天以上用「至」
func describeDays(days []int) string {
	seen := map[int]bool{}
	order := make([]int, 0, len(days))
	for _, d := range days {
		if d < 0 || d > 6 || seen[d] {
			continue
		}
		seen[d] = true
		if d == 0 {
			d = 7 // 周日排最后
		}
		order = append(order, d)
	}
	sort.Ints(order)
	name := func(d int) string { return weekdayNames[d%7] }
	if len(order) >= 3 && order[len(order)-1]-order[0] == len(order)-1 {
		return name(order[0]) + "至" + name(order[len(order)-1])
	}
	names := make([]string, len(order))
	for i, d := range order {
		names[i] = name(d)
	}
	return strings.Join(names, "、")
}

func clock(minutes int) string {
	return fmt.Sprintf("%02d:%02d", minutes/60, minutes%60)
}

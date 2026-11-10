// Package formula 表达式计费引擎（billing_mode=formula）。
// 公式从 usage 归集的变量环境取值求出
// 「每 1M token 口径的价格系数」，金额 = 系数 / 1e6 × 租户乘数 × 时段乘数。
// 公式只表达模型基础成本，与折扣/时段乘数正交（外挂，不在公式内表达）。
package formula

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math"
	"sync"

	"github.com/expr-lang/expr"
	"github.com/expr-lang/expr/ast"
	"github.com/expr-lang/expr/vm"
)

// knownVarDocs 公式可用变量文档表（key → 中文说明），knownVars 的唯一事实源：
// 说明随变量定义维护在同一处，AI 助手技能参考（KnownVarDocs）据此动态生成，
// 不另立文档副本。新增变量必须同步：
// 1) FormulaInput.Env 注入；2) 前端 FormulaPricingEditor 变量速查；3) smoke 向量覆盖
var knownVarDocs = map[string]string{
	"p":           "纯文本输入 token（公式单独计价的细分类别已扣除）",
	"c":           "输出 token（引用 rt/img_o/ao 时扣除对应细分）",
	"ctx":         "完整输入上下文 token（含缓存，长上下文档位判断用，不扣除）。不可命名 len——与 expr 内置函数冲突",
	"cr":          "缓存读取（命中）token",
	"cc":          "缓存写入 5m token（Claude ephemeral_5m / OpenAI cache_write 合并桶）",
	"cc1h":        "缓存写入 1h token（Claude ephemeral_1h）",
	"img":         "图片输入 token",
	"img_o":       "图片输出 token",
	"ai":          "音频输入 token",
	"ao":          "音频输出 token",
	"rt":          "推理输出 token（reasoning）",
	"image_count": "图片张数（图片协议顶层 n / 任务 ratios count，<=1 归一为 1）",
	"seconds":     "任务计费秒数（时长类任务：预扣=请求时长信号，结算=官方素材计量；同步对话路径恒 0）",
}

// knownVars 公式可用变量集合（由 knownVarDocs 派生，排除语义见 env.go）。
var knownVars = func() map[string]bool {
	m := make(map[string]bool, len(knownVarDocs))
	for k := range knownVarDocs {
		m[k] = true
	}
	return m
}()

// KnownVarDocs 返回变量文档表快照（AI 助手等外部消费方动态生成参考用，防手写文档漂移）。
func KnownVarDocs() map[string]string {
	out := make(map[string]string, len(knownVarDocs))
	for k, v := range knownVarDocs {
		out[k] = v
	}
	return out
}

// knownFactKeys 已注册 usage facts 键表（key → 中文说明）。
// u("key") 从渠道 adaptor 填充的 usage facts（非 token 计量：音频秒数、字节数等）
// 取值；保存公式时烟测强校验字面 key 必须已注册（防拼写错误静默取 0 错价）。
// adaptor 新增 facts 填充点时必须同步在此注册，并更新前端速查表；
// JS 任务插件的 usageSchema number 型键在插件激活时经 RegisterFactKeys 动态注册。
//
// 注意：工具调用计数（web_search/google_search 等按次附加费）走 Usage.ToolCalls
// 旁路（结算层 tool_pricing.go 计价），刻意不注册为 facts——注册后公式 u() 与
// 附加费通道会对同一计数双重计费
var (
	knownFactKeysMu sync.RWMutex
	knownFactKeys   = map[string]string{
		"audio_seconds": "音频时长秒数（STT/翻译 verbose_json.duration，OpenAI 渠道填充）",
		"audio_bytes":   "音频响应字节数（TTS 二进制响应实测大小，OpenAI 渠道填充）",
	}
)

// KnownFactKeys 返回已注册 facts 键表快照（管理端速查表导出/报错提示用）。
func KnownFactKeys() map[string]string {
	knownFactKeysMu.RLock()
	defer knownFactKeysMu.RUnlock()
	out := make(map[string]string, len(knownFactKeys))
	for k, v := range knownFactKeys {
		out[k] = v
	}
	return out
}

// RegisterFactKeys 动态注册 usage facts 键（JS 任务插件 usageSchema 激活时调用）。
// 幂等：重复注册覆盖描述。插件禁用不摘除——key 只影响公式保存期校验与 u() 取 0 兜底，
// 摘除会让历史公式保存失败，保留无害。
func RegisterFactKeys(keys map[string]string) {
	if len(keys) == 0 {
		return
	}
	knownFactKeysMu.Lock()
	defer knownFactKeysMu.Unlock()
	for k, desc := range keys {
		if k != "" {
			knownFactKeys[k] = desc
		}
	}
}

// factKeyRegistered 报告 key 是否已注册（烟测校验用）。
func factKeyRegistered(key string) bool {
	knownFactKeysMu.RLock()
	defer knownFactKeysMu.RUnlock()
	_, ok := knownFactKeys[key]
	return ok
}

// Compiled 编译产物：字节码程序 + AST 静态分析出的变量引用集合。
// Vars 驱动排除语义（Env 构建时只扣除公式真正引用的细分类别）；
// UKeys 是 u() 引用的 facts 字面 key 集合（快照记录与注册表校验用）；
// ParamPaths/HeaderKeys 是 param()/header() 引用的字面路径/键集合（计费层据此
// 从实际请求预解析探测值，任务路径随 ratios 冻结）；Rules 是编译期从
// 「<探针条件> ? <数字字面量> : 1」形状三目提取的请求规则表（_trace 埋点回填命中）
type Compiled struct {
	Program    *vm.Program
	Vars       map[string]bool
	UKeys      []string
	ParamPaths []string
	HeaderKeys []string
	Rules      []RequestRuleTrace
	Src        string
}

// compileCache 编译缓存：key = 公式 SHA-256，FIFO 驱逐，上限 256。
// 公式数量级 = 模型数（数百），缓存命中后求值走字节码，无重复解析开销
var (
	compileCacheMu sync.Mutex
	compileCache   = map[string]*Compiled{}
	compileOrder   []string // FIFO 驱逐顺序
)

const compileCacheMax = 256

// Compile 编译公式（带缓存）。公式语法见 knownVars 与 BuiltinFuncs，
// 算术/比较/三目/&&/|| 均为 expr 原生语法，如 "ctx <= 128000 ? p*5+c*20 : p*10+c*40"
func Compile(src string) (*Compiled, error) {
	if src == "" {
		return nil, fmt.Errorf("计费公式为空")
	}
	key := cacheKey(src)

	compileCacheMu.Lock()
	if c, ok := compileCache[key]; ok {
		compileCacheMu.Unlock()
		return c, nil
	}
	compileCacheMu.Unlock()

	// 请求规则埋点在编译管线内完成（expr.Patch 先于常量折叠执行，短路分支也可静态
	// 检测）；数值结果不变，仅把「条件 ? 乘数 : 1」改写为 _trace(idx, 条件, 乘数)
	patcher := &requestRulePatcher{}
	program, err := expr.Compile(src, expr.Patch(patcher))
	if err != nil {
		return nil, fmt.Errorf("公式编译失败: %w", err)
	}
	if patcher.restrictedIdentifier != "" {
		return nil, fmt.Errorf("公式编译失败: 标识符 %q 为内部埋点保留字，不可使用", patcher.restrictedIdentifier)
	}

	ukeys, err := extractUKeys(program)
	if err != nil {
		return nil, fmt.Errorf("公式编译失败: %w", err)
	}
	paramPaths, headerKeys, err := extractProbes(program)
	if err != nil {
		return nil, fmt.Errorf("公式编译失败: %w", err)
	}

	c := &Compiled{
		Program:    program,
		Vars:       extractVars(program),
		UKeys:      ukeys,
		ParamPaths: paramPaths,
		HeaderKeys: headerKeys,
		Rules:      patcher.rules,
		Src:        src,
	}

	compileCacheMu.Lock()
	if len(compileOrder) >= compileCacheMax {
		// FIFO 驱逐最旧条目（编译成本毫秒级，重编译无感知）
		oldest := compileOrder[0]
		compileOrder = compileOrder[1:]
		delete(compileCache, oldest)
	}
	compileCache[key] = c
	compileOrder = append(compileOrder, key)
	compileCacheMu.Unlock()
	return c, nil
}

// Run 在公式输入上求值，返回「每 1M token 口径的价格系数」与请求规则命中表。
// 变量环境由输入内部构建（含 _trace 埋点闭包，命中规则随求值回填 Matched）；
// 结果必须为有限数且非负（三目可能返回整数类型，统一转 float64）
func Run(c *Compiled, in FormulaInput) (float64, []RequestRuleTrace, error) {
	trace := cloneTraces(c.Rules)
	env := in.Env(c.Vars, trace)
	raw, err := expr.Run(c.Program, env)
	if err != nil {
		return 0, nil, fmt.Errorf("公式求值失败: %w", err)
	}
	v, ok := toFloat64(raw)
	if !ok {
		return 0, nil, fmt.Errorf("公式结果不是数值: %T", raw)
	}
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return 0, nil, fmt.Errorf("公式结果非有限数: %v", v)
	}
	if v < 0 {
		return 0, nil, fmt.Errorf("公式结果为负数: %v", v)
	}
	if v == 0 {
		// 负零归一（"-p" 在 p=0 时产生 -0.0，数学上等于 0，按免费档放行）
		return 0, trace, nil
	}
	return v, trace, nil
}

// BuiltinFuncs 公式内置函数（注入每个求值环境）：
//   - fixed(x)：按次金额。内部返回 x×1e6，把「每次请求金额」换算成
//     与 token 系数相同的每 1M 口径，金额统一 = 结果 / 1e6
//   - min/max/abs/ceil/floor：数值辅助（expr 内置不含这组数学函数）
func BuiltinFuncs() map[string]any {
	return map[string]any{
		"fixed": func(args ...any) (any, error) {
			if len(args) != 1 {
				return nil, fmt.Errorf("fixed() 需要 1 个参数")
			}
			v, ok := toFloat64(args[0])
			if !ok {
				return nil, fmt.Errorf("fixed() 参数不是数值")
			}
			return v * 1_000_000, nil
		},
		"min": numFunc2("min", math.Min),
		"max": numFunc2("max", math.Max),
		"abs": func(args ...any) (any, error) {
			if len(args) != 1 {
				return nil, fmt.Errorf("abs() 需要 1 个参数")
			}
			v, ok := toFloat64(args[0])
			if !ok {
				return nil, fmt.Errorf("abs() 参数不是数值")
			}
			return math.Abs(v), nil
		},
		"ceil":  numFunc1("ceil", math.Ceil),
		"floor": numFunc1("floor", math.Floor),
	}
}

func numFunc1(name string, f func(float64) float64) func(...any) (any, error) {
	return func(args ...any) (any, error) {
		if len(args) != 1 {
			return nil, fmt.Errorf("%s() 需要 1 个参数", name)
		}
		v, ok := toFloat64(args[0])
		if !ok {
			return nil, fmt.Errorf("%s() 参数不是数值", name)
		}
		return f(v), nil
	}
}

func numFunc2(name string, f func(a, b float64) float64) func(...any) (any, error) {
	return func(args ...any) (any, error) {
		if len(args) < 2 {
			return nil, fmt.Errorf("%s() 至少需要 2 个参数", name)
		}
		first, ok := toFloat64(args[0])
		if !ok {
			return nil, fmt.Errorf("%s() 参数不是数值", name)
		}
		result := first
		for _, arg := range args[1:] {
			v, ok := toFloat64(arg)
			if !ok {
				return nil, fmt.Errorf("%s() 参数不是数值", name)
			}
			result = f(result, v)
		}
		return result, nil
	}
}

// toFloat64 宽松数值转换（expr 三目/整数运算可能返回 int 族类型）
func toFloat64(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case float32:
		return float64(n), true
	case int:
		return float64(n), true
	case int8:
		return float64(n), true
	case int16:
		return float64(n), true
	case int32:
		return float64(n), true
	case int64:
		return float64(n), true
	case uint:
		return float64(n), true
	case uint8:
		return float64(n), true
	case uint16:
		return float64(n), true
	case uint32:
		return float64(n), true
	case uint64:
		return float64(n), true
	default:
		return 0, false
	}
}

// extractVars AST 静态分析：收集公式引用的已知变量。
// 函数名（fixed/min 等）不是 knownVars 成员，天然不会被误收
func extractVars(program *vm.Program) map[string]bool {
	vars := map[string]bool{}
	root := program.Node()
	ast.Walk(&root, &varCollector{vars: vars})
	return vars
}

type varCollector struct {
	vars map[string]bool
}

func (c *varCollector) Visit(node *ast.Node) {
	if id, ok := (*node).(*ast.IdentifierNode); ok {
		if knownVars[id.Value] {
			c.vars[id.Value] = true
		}
	}
}

// extractUKeys AST 提取 u() 引用的 facts 字面 key。
// 约束：参数必须是字符串字面量（u("seconds")），动态 key（u(var)）无法静态
// 校验注册表，编译即拒绝；u() 以外的同名调用（不存在）与未引用场景返回空集
func extractUKeys(program *vm.Program) ([]string, error) {
	c := &ukeyCollector{seen: map[string]bool{}}
	root := program.Node()
	ast.Walk(&root, c)
	if c.err != nil {
		return nil, c.err
	}
	return c.keys, nil
}

type ukeyCollector struct {
	keys []string
	seen map[string]bool
	err  error
}

func (c *ukeyCollector) Visit(node *ast.Node) {
	if c.err != nil {
		return
	}
	call, ok := (*node).(*ast.CallNode)
	if !ok {
		return
	}
	id, ok := call.Callee.(*ast.IdentifierNode)
	if !ok || id.Value != "u" {
		return
	}
	if len(call.Arguments) != 1 {
		c.err = fmt.Errorf("u() 需要 1 个字符串字面量参数")
		return
	}
	str, ok := call.Arguments[0].(*ast.StringNode)
	if !ok {
		c.err = fmt.Errorf("u() 的 key 必须是字符串字面量（动态 key 不支持静态校验）")
		return
	}
	if !c.seen[str.Value] {
		c.seen[str.Value] = true
		c.keys = append(c.keys, str.Value)
	}
}

func cacheKey(src string) string {
	sum := sha256.Sum256([]byte(src))
	return hex.EncodeToString(sum[:])
}

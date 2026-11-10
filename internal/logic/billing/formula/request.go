package formula

import (
	"strings"
	"time"
)

// RequestInput 公式请求感知探针（header()/param()）的已解析形态。
// 由计费层（billing 包）在求值前从「编译产物静态提取的字面探测集合 × 实际请求」构建：
// 只含公式真正引用的 header 键与 param 路径（体积小，任务路径随 ratios 冻结持久化，
// 结算重放保证预扣/结算口径一致），formula 包自身不接触原始 body 与 JSON 解析
// （避免 formula→billing 导入环；路径解析复用 param_multipliers 的 lookupPath）
type RequestInput struct {
	Headers map[string]string `json:"headers,omitempty"` // key 小写归一
	Params  map[string]any    `json:"params,omitempty"`  // body 路径 → 值（通配多值取首个）
}

// GetHeader 取请求头值。key 小写归一；缺失返回空串（未传头 ≠ 公式错误）
func (r *RequestInput) GetHeader(key string) string {
	if r == nil {
		return ""
	}
	return r.Headers[strings.ToLower(strings.TrimSpace(key))]
}

// GetParam 取请求体路径值。缺失返回 nil（param() == 比较走 false 分支）
func (r *RequestInput) GetParam(path string) any {
	if r == nil {
		return nil
	}
	return r.Params[path]
}

// ToMap 序列化为 ratios 冻结持久化用的普通 map（billing_context JSONB 往返）
func (r *RequestInput) ToMap() map[string]any {
	if r == nil {
		return nil
	}
	m := map[string]any{}
	if len(r.Headers) > 0 {
		m["headers"] = r.Headers
	}
	if len(r.Params) > 0 {
		m["params"] = r.Params
	}
	return m
}

// RequestInputFromMap 从 ratios 冻结的 map 还原。兼容两种形态：JSONB 反序列化后的
// map[string]any 与进程内 ToMap 直接返回的 map[string]string（单测/同进程重放路径）
func RequestInputFromMap(m map[string]any) *RequestInput {
	if m == nil {
		return nil
	}
	in := &RequestInput{}
	switch hs := m["headers"].(type) {
	case map[string]any:
		if len(hs) > 0 {
			in.Headers = make(map[string]string, len(hs))
			for k, v := range hs {
				if s, ok := v.(string); ok {
					in.Headers[k] = s
				}
			}
		}
	case map[string]string:
		if len(hs) > 0 {
			in.Headers = hs
		}
	}
	if ps, ok := m["params"].(map[string]any); ok && len(ps) > 0 {
		in.Params = ps
	}
	return in
}

// EvalContext 单次求值的上下文要素：时间基准 + 默认时区 + 请求探测值。
// 同步路径预扣取当前时刻、结算取受理时刻 billAt（与时段乘数同哲学，跨小时边界不漂移）；
// 任务路径取提交时冻结的 At/Request 重放。nil 表示无请求上下文（探针取空值、时间取 now）
type EvalContext struct {
	At       time.Time      // 时间函数（hour/minute/weekday/month/day）基准时刻
	Location *time.Location // 省略时区参数时的默认时区（nil → UTC）
	Request  *RequestInput  // header()/param() 探测值
}

// RequestRuleTrace 请求规则命中留痕。编译期从「<探针条件> ? <数字字面量> : 1」形状的
// 三目静态提取（条件与乘数固化），运行时由 _trace 埋点回填 Matched；随计费快照
// 写入 bil_usage_logs 供账单解释「本次请求命中了哪些倍率规则」
type RequestRuleTrace struct {
	Cond string `json:"cond"`
	// Desc 条件的中文描述（如「周一至周五 且 09:00–18:00」），租户可见账单用；
	// 无法描述的手写条件为空，展示方回退通用文案
	Desc       string  `json:"desc,omitempty"`
	Multiplier float64 `json:"multiplier"`
	Matched    bool    `json:"matched"`
}

// cloneTraces 深拷贝规则表（每次 Run 独立回填 Matched，编译缓存中的模板不被污染）
func cloneTraces(rules []RequestRuleTrace) []RequestRuleTrace {
	if len(rules) == 0 {
		return nil
	}
	out := make([]RequestRuleTrace, len(rules))
	copy(out, rules)
	return out
}

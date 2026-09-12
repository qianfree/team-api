package billing

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// 参数倍率（param_multipliers）：按「网关归一化后的任务体」中的请求参数匹配规则，
// 对命中规则连乘倍率。规则配置在 mdl_pricing.pricing JSONB 顶层（横切所有计费模式，与 time_segments 同级）。
//
// 设计约束（docs/模型定价存储JSON化与按秒计费设计.md 3.2）：
//   - 匹配源 = 转换后的通用任务体（{model, prompt, seconds, metadata{...}}），规则跟语义走不跟入口协议走；
//   - 一条规则内扁平条件列表 = 隐式 AND（全部满足才命中），不做嵌套布尔组（已否决）；
//   - 规则间连乘，互斥分支多配几条规则即 OR；
//   - 不做 NOT / 正则 / 范围外运算符；
//   - 数字强转：字符串可转数字则参与比较（"/v1/videos 的 seconds 是字符串"）。

// ParamCondition 单个匹配条件
type ParamCondition struct {
	Path  string `json:"path"`            // 任务体 JSON 路径，支持 [*] 数组通配（任一元素命中即命中），如 metadata.content[*].video_url
	Match string `json:"match"`           // has_value | equals | gt | gte | lt | lte | between
	Value any    `json:"value,omitempty"` // equals/gt/gte/lt/lte 单值；between 为 [min,max] 双元素数组
}

// ParamRule 参数倍率规则：conditions 全部满足时按 multiplier 计入连乘
type ParamRule struct {
	Conditions []ParamCondition `json:"conditions"`
	Multiplier float64          `json:"multiplier"`
	Note       string           `json:"note,omitempty"` // 命中说明，写入计费上下文供对账解释
}

// 求值注入的 ratios 键（随任务 PrivateData.billing_context 持久化，
// 预扣 / per_second 结算 / token 重算三口径经 applyRatioMultipliers 自动同源）
const (
	ratioKeyParamMultiplier = "param_multiplier"
	ratioKeyParamMatched    = "param_matched" // 命中规则 note 列表（| 分隔）；string 值不参与乘法
)

// validParamMatches 合法匹配运算符集合
var validParamMatches = map[string]bool{
	"has_value": true, "equals": true,
	"gt": true, "gte": true, "lt": true, "lte": true, "between": true,
}

// ValidateParamMultipliers 校验规则配置（SetModelPricing / 模型导入 fail-fast 用）
func ValidateParamMultipliers(rules []ParamRule) error {
	for i, rule := range rules {
		if len(rule.Conditions) == 0 {
			return fmt.Errorf("参数倍率规则 %d 至少需要一个条件", i+1)
		}
		if rule.Multiplier <= 0 || rule.Multiplier > 100 {
			return fmt.Errorf("参数倍率规则 %d 倍率需在 (0, 100] 区间", i+1)
		}
		for j, cond := range rule.Conditions {
			if cond.Path == "" {
				return fmt.Errorf("参数倍率规则 %d 条件 %d 路径为空", i+1, j+1)
			}
			if !validParamMatches[cond.Match] {
				return fmt.Errorf("参数倍率规则 %d 条件 %d 匹配方式非法：%s", i+1, j+1, cond.Match)
			}
			switch cond.Match {
			case "between":
				arr, ok := cond.Value.([]any)
				if !ok || len(arr) != 2 {
					return fmt.Errorf("参数倍率规则 %d 条件 %d between 需要 [min,max] 双元素", i+1, j+1)
				}
				if _, ok1 := toNumber(arr[0]); !ok1 {
					return fmt.Errorf("参数倍率规则 %d 条件 %d between 边界非数字", i+1, j+1)
				}
				if _, ok2 := toNumber(arr[1]); !ok2 {
					return fmt.Errorf("参数倍率规则 %d 条件 %d between 边界非数字", i+1, j+1)
				}
			case "has_value":
				// 无需值
			default: // equals / gt / gte / lt / lte 需要比对值
				if cond.Value == nil {
					return fmt.Errorf("参数倍率规则 %d 条件 %d 需要指定比对值", i+1, j+1)
				}
				if cond.Match != "equals" {
					if _, ok := toNumber(cond.Value); !ok {
						return fmt.Errorf("参数倍率规则 %d 条件 %d %s 需要数字值", i+1, j+1, cond.Match)
					}
				}
			}
		}
	}
	return nil
}

// EvalParamMultipliers 对任务体求值参数倍率：命中规则连乘，返回 (总倍率, 命中说明列表)。
// 无规则/无命中返回 (1.0, nil)；解析失败的任务体安全返回未命中（不阻断计费）。
func EvalParamMultipliers(rules []ParamRule, taskBody []byte) (float64, []string) {
	if len(rules) == 0 || len(taskBody) == 0 {
		return 1.0, nil
	}
	var root any
	if err := json.Unmarshal(taskBody, &root); err != nil {
		return 1.0, nil
	}

	total := 1.0
	var matched []string
	for _, rule := range rules {
		if rule.Multiplier <= 0 || !ruleConditionsMatch(rule.Conditions, root) {
			continue
		}
		total *= rule.Multiplier
		if rule.Note != "" {
			matched = append(matched, rule.Note)
		} else {
			matched = append(matched, strings.Join(conditionPaths(rule.Conditions), " & "))
		}
	}
	if len(matched) == 0 {
		return 1.0, nil
	}
	return total, matched
}

// ruleConditionsMatch 隐式 AND：全部条件满足才命中
func ruleConditionsMatch(conds []ParamCondition, root any) bool {
	for _, cond := range conds {
		if !conditionMatch(cond, root) {
			return false
		}
	}
	return true
}

// conditionMatch 单条件匹配：路径提取（[*] 扇出）后按运算符判断，任一候选值满足即命中
func conditionMatch(cond ParamCondition, root any) bool {
	values, found := lookupPath(root, cond.Path)
	if !found {
		return false
	}
	switch cond.Match {
	case "has_value":
		// 路径存在且任一候选值非 null（false/0/"" 均视为"有值"——与"未传"语义不同）
		for _, v := range values {
			if v != nil {
				return true
			}
		}
		return false
	case "equals":
		for _, v := range values {
			if valuesEqual(v, cond.Value) {
				return true
			}
		}
		return false
	case "between":
		arr, ok := cond.Value.([]any)
		if !ok || len(arr) != 2 {
			return false
		}
		lo, ok1 := toNumber(arr[0])
		hi, ok2 := toNumber(arr[1])
		if !ok1 || !ok2 {
			return false
		}
		for _, v := range values {
			if n, ok := toNumber(v); ok && n >= lo && n <= hi {
				return true
			}
		}
		return false
	default: // gt / gte / lt / lte
		target, ok := toNumber(cond.Value)
		if !ok {
			return false
		}
		for _, v := range values {
			n, ok := toNumber(v)
			if !ok {
				continue
			}
			switch cond.Match {
			case "gt":
				if n > target {
					return true
				}
			case "gte":
				if n >= target {
					return true
				}
			case "lt":
				if n < target {
					return true
				}
			case "lte":
				if n <= target {
					return true
				}
			}
		}
		return false
	}
}

// lookupPath 按点分路径提取候选值（[*] 数组通配扇出）。
// found=false 表示路径不存在（键缺失或类型不符）；[*] 命中空数组时 found=false。
func lookupPath(root any, path string) (values []any, found bool) {
	segments := strings.Split(path, ".")
	current := []any{root}
	for _, seg := range segments {
		if seg == "" {
			return nil, false
		}
		wildcard := strings.HasSuffix(seg, "[*]")
		key := seg
		if wildcard {
			key = strings.TrimSuffix(seg, "[*]")
		}
		var next []any
		for _, cur := range current {
			m, ok := cur.(map[string]any)
			if !ok {
				continue
			}
			v, exists := m[key]
			if !exists {
				continue
			}
			if wildcard {
				arr, ok := v.([]any)
				if !ok {
					continue
				}
				next = append(next, arr...)
			} else {
				next = append(next, v)
			}
		}
		if len(next) == 0 {
			return nil, false
		}
		current = next
	}
	return current, true
}

// valuesEqual 等值比较：数字强转后比较（"8" == 8），string/bool 严格相等
func valuesEqual(v, target any) bool {
	if target == nil {
		return false
	}
	// 数字优先：两侧都能转数字则按数值比
	if vn, ok1 := toNumber(v); ok1 {
		if tn, ok2 := toNumber(target); ok2 {
			return vn == tn
		}
	}
	vs, ok1 := v.(string)
	ts, ok2 := target.(string)
	if ok1 && ok2 {
		return vs == ts
	}
	vb, ok1 := v.(bool)
	tb, ok2 := target.(bool)
	if ok1 && ok2 {
		return vb == tb
	}
	return false
}

// toNumber 数字强转：JSON number（float64）/ 可解析数字字符串 → float64；bool/对象/数组不可转
func toNumber(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case string:
		f, err := strconv.ParseFloat(strings.TrimSpace(n), 64)
		if err == nil {
			return f, true
		}
		return 0, false
	default:
		return 0, false
	}
}

// conditionPaths 条件路径列表（规则无 note 时作命中说明兜底）
func conditionPaths(conds []ParamCondition) []string {
	paths := make([]string, 0, len(conds))
	for _, c := range conds {
		paths = append(paths, c.Path)
	}
	return paths
}

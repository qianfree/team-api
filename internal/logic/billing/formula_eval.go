// formula_eval.go 表达式计费的求值上下文构建（同步路径）。
// 请求感知探针（header()/param()）的解析在此完成：编译产物静态提取字面探测集合，
// 按集合从实际请求预解析出小体积探测值 map——任务路径随 ratios 冻结持久化、
// 结算重放保证预扣/结算口径一致；formula 包自身保持纯函数（不接触 HTTP 与 JSON 解析）
package billing

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/qianfree/team-api/internal/logic/billing/formula"
	rcommon "github.com/qianfree/team-api/relay/common"
)

// EffectiveFormula 组合基础公式与请求规则为最终求值表达式：
// rules 为空返回 base 原文；否则 "(base) * (rules)"（乘法语义，与 new-api 存储形态一致）
func EffectiveFormula(base, rules string) string {
	base = strings.TrimSpace(base)
	rules = strings.TrimSpace(rules)
	if base == "" {
		return ""
	}
	if rules == "" {
		return base
	}
	return "(" + base + ") * (" + rules + ")"
}

// ResolveFormulaRequest 按编译产物的字面探测集合从实际请求解析探测值：
//   - HeaderKeys：http.Header 取值（键大小写不敏感）→ 小写归一存储；空值剔除。
//     凭证类请求头（sensitiveHeaderNames）运行时跳过，防止已保存的旧公式泄露密钥
//   - ParamPaths：body 按点分路径（含 [*] 通配）提取，多值取首个；非 JSON/解析失败
//     安全返回空 Params（路径缺失 ≠ 公式错误，与 u() 缺失取 0 同语义）
//
// 无任何探测引用时返回 nil（绝大多数公式不感知请求，零开销）
func ResolveFormulaRequest(c *formula.Compiled, headers http.Header, body []byte) *formula.RequestInput {
	if len(c.HeaderKeys) == 0 && len(c.ParamPaths) == 0 {
		return nil
	}
	req := &formula.RequestInput{Headers: map[string]string{}, Params: map[string]any{}}
	for _, key := range c.HeaderKeys {
		// 运行时二次守卫：跳过凭证类请求头，即使公式在编译期拦截前就已保存（旧数据容错）
		if rcommon.IsSensitiveHeader(key) {
			continue
		}
		if v := strings.TrimSpace(headers.Get(key)); v != "" {
			req.Headers[strings.ToLower(key)] = v
		}
	}
	if len(c.ParamPaths) > 0 && len(body) > 0 {
		var root any
		if err := json.Unmarshal(body, &root); err == nil {
			for _, path := range c.ParamPaths {
				// 复用参数倍率的路径解析（点分 + [*] 通配），两机制路径语法对管理员统一
				if vals, found := lookupPath(root, path); found && len(vals) > 0 {
					req.Params[path] = vals[0]
				}
			}
		}
	}
	return req
}

// NewFormulaEvalContext 同步路径求值上下文：时间函数基准 = at（预扣传当前时刻、
// 结算传受理时刻 billAt，两时点几乎重合且以 billAt 为准，跨小时边界不漂移），
// 默认时区 = 平台计费时区（与时段定价共用 pricing_time_timezone 配置），
// 请求探测值从 headers+body 预解析。headers/body 缺失（如 legacy 路径）时探针取空值
func NewFormulaEvalContext(ctx context.Context, pricing *PricingResult, headers http.Header, body []byte, at time.Time) *formula.EvalContext {
	ec := &formula.EvalContext{
		At:       at,
		Location: pricingTimeLocation(ctx),
	}
	if pricing.BillingMode == BillingModeFormula && pricing.Formula != "" {
		if compiled, err := formula.Compile(pricing.Formula); err == nil {
			ec.Request = ResolveFormulaRequest(compiled, headers, body)
		}
	}
	return ec
}

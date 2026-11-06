package jsplugin

import (
	"context"
	"encoding/json"
	"regexp"
	"strings"
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// taskLoggerKey 任务日志回调的 context 键：桥接层把 tsk_task_logs 写入器挂进 ctx，
// 插件 utils.log 与钩子观测日志随任务详情落库（作者排错的生命线）。
type taskLoggerKey struct{}

// TaskLogger 任务级日志回调：level ∈ info/warning/error。
type TaskLogger func(level, message string)

// WithTaskLogger 把任务日志回调挂入 ctx。
func WithTaskLogger(ctx context.Context, logger TaskLogger) context.Context {
	return context.WithValue(ctx, taskLoggerKey{}, logger)
}

// TaskLoggerFromContext 取出任务日志回调（无则 nil）。
func TaskLoggerFromContext(ctx context.Context) TaskLogger {
	if ctx == nil {
		return nil
	}
	if logger, ok := ctx.Value(taskLoggerKey{}).(TaskLogger); ok {
		return logger
	}
	return nil
}

// summaryLimit 观测日志中入参/返回摘要的截断长度（兼作对外错误消息的截断上限）。
const summaryLimit = 512

// 敏感值脱敏正则：钩子出入参会携带渠道密钥（BuildSubmitContext.apiKey）、插件算好
// 的鉴权头（Authorization）等，落日志/任务日志/对外错误前必须抹值。
var (
	// 引号值形态（键可带引号、分隔可宽松）："apiKey":"sk-x" / apiKey: "sk-x" / token "sk-x"。
	// 组 1=键名、组 2=值；键大小写不敏感，含 api_key 蛇形与 access_token 尾部命中。
	secretQuotedValueRe = regexp.MustCompile(`(?i)"?(api_?key|authorization|token|secret|password|cookie)"?\s*[:=]?\s*"([^"]*)"`)
	// URL query 形态：?api_key=xxx&sig=yyy（utils.fetch 报错消息里含 rawURL）
	querySecretValueRe = regexp.MustCompile(`(?i)(api_?key|token|secret|sig|signature|password)=[^&;"\s]+`)
)

// RedactSecrets 抹除字符串中的敏感值（引号值形态与 URL query 形态），键名保留便于排障。
// 用 ReplaceAllStringFunc 只替换值区段——键的引号/冒号原形态保持不变，
// JSON 日志经脱敏后仍是合法 JSON（模板替换难以区分键是否带引号，会破坏形态）。
func RedactSecrets(s string) string {
	s = secretQuotedValueRe.ReplaceAllStringFunc(s, func(m string) string {
		sub := secretQuotedValueRe.FindStringSubmatchIndex(m)
		if sub == nil || len(sub) < 6 {
			return m
		}
		return m[:sub[4]] + "***" + m[sub[5]:]
	})
	return querySecretValueRe.ReplaceAllString(s, `${1}=***`)
}

// SanitizeHookMessage 钩子错误消息对外净化：脱敏 + 剥控制字符 + 截断。
// 插件异常是面向用户的业务文案通道（上游 vendor message 原样透传），只净化不泛化；
// 但消息可能意外内嵌密钥/签名片段（如签名失败把 secret 拼进 Error），必须抹除。
func SanitizeHookMessage(msg string) string {
	msg = RedactSecrets(msg)
	msg = strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f {
			return -1
		}
		return r
	}, msg)
	if runes := []rune(msg); len(runes) > summaryLimit {
		return string(runes[:summaryLimit]) + "..."
	}
	return msg
}

// RecordHookCall 记录一次钩子调用（耗时与结果摘要）到应用日志与任务日志。
// args/result 摘要与错误消息都先脱敏——两者都可能携带渠道密钥明文。
func RecordHookCall(ctx context.Context, pluginKey, hook string, args, result []byte, dur time.Duration, callErr error) {
	// 试运行（DryRun）的样例调用不进观测：插件尚未入库，指标条目会以不存在的
	// key 混进管理端指标列表；自检结果已在 API 响应中返回，观测日志也无排错价值。
	if isDryRunCall(ctx) {
		return
	}
	fields := "args=" + summarize(args) + " result=" + summarize(result)
	recordHookMetrics(pluginKey, hook, dur, callErr)
	if callErr != nil {
		errMsg := RedactSecrets(callErr.Error())
		g.Log().Warningf(ctx, "[jsplugin:%s] hook %s failed (%s): %s | %s",
			pluginKey, hook, dur.Truncate(time.Millisecond), errMsg, fields)
		if logger := TaskLoggerFromContext(ctx); logger != nil {
			logger("error", "hook "+hook+" 失败: "+errMsg)
		}
		return
	}
	g.Log().Debugf(ctx, "[jsplugin:%s] hook %s ok (%s): %s",
		pluginKey, hook, dur.Truncate(time.Millisecond), fields)
	if logger := TaskLoggerFromContext(ctx); logger != nil {
		logger("info", "hook "+hook+" 完成 ("+dur.Truncate(time.Millisecond).String()+")")
	}
}

// summarize 截断观测摘要（先脱敏后截断——截断在键值对中间劈开会漏出半个密钥）。
func summarize(data []byte) string {
	if len(data) == 0 {
		return "-"
	}
	s := RedactSecrets(string(data))
	if len(s) > summaryLimit {
		return s[:summaryLimit] + "...(truncated)"
	}
	return s
}

// marshalJSON / unmarshalJSON hostfunc 用的薄封装（统一错误口径）。
func marshalJSON(v any) ([]byte, error) { return json.Marshal(v) }

func unmarshalJSON(data []byte, v any) error { return json.Unmarshal(data, v) }

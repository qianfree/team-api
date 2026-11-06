package jsplugin

import (
	"context"
	"encoding/json"
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

// summaryLimit 观测日志中入参/返回摘要的截断长度。
const summaryLimit = 512

// RecordHookCall 记录一次钩子调用（耗时与结果摘要）到应用日志与任务日志。
func RecordHookCall(ctx context.Context, pluginKey, hook string, args, result []byte, dur time.Duration, callErr error) {
	fields := "args=" + summarize(args) + " result=" + summarize(result)
	recordHookMetrics(pluginKey, hook, dur, callErr)
	if callErr != nil {
		g.Log().Warningf(ctx, "[jsplugin:%s] hook %s failed (%s): %s | %s",
			pluginKey, hook, dur.Truncate(time.Millisecond), callErr, fields)
		if logger := TaskLoggerFromContext(ctx); logger != nil {
			logger("error", "hook "+hook+" 失败: "+callErr.Error())
		}
		return
	}
	g.Log().Debugf(ctx, "[jsplugin:%s] hook %s ok (%s): %s",
		pluginKey, hook, dur.Truncate(time.Millisecond), fields)
	if logger := TaskLoggerFromContext(ctx); logger != nil {
		logger("info", "hook "+hook+" 完成 ("+dur.Truncate(time.Millisecond).String()+")")
	}
}

// summarize 截断观测摘要。
func summarize(data []byte) string {
	if len(data) == 0 {
		return "-"
	}
	s := string(data)
	if len(s) > summaryLimit {
		return s[:summaryLimit] + "...(truncated)"
	}
	return s
}

// marshalJSON / unmarshalJSON hostfunc 用的薄封装（统一错误口径）。
func marshalJSON(v any) ([]byte, error) { return json.Marshal(v) }

func unmarshalJSON(data []byte, v any) error { return json.Unmarshal(data, v) }

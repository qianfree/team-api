package convmeta

import "fmt"

// SoftErrorRecorder 是宿主可选实现的能力接口：记录转换过程中的软错误（流继续处理）。
//
// 典型场景是上游 SSE 里夹杂无法按预期格式解析的 data 行：转换器只能跳过，但「跳过了什么」
// 对排障极其关键——此前这类行被静默 continue，线上出现截断/空回答时只能靠 chn_debug_logs
// 逐帧对照。宿主端由 *relaycommon.RelayInfo 实现（落到 StreamStatus 的软错误列表，随
// bil_usage_logs / 调试日志一同持久化）；未实现时静默跳过。
type SoftErrorRecorder interface {
	RecordConvertSoftError(msg string)
}

// RecordSoftError 若 m 实现了 SoftErrorRecorder 则记录一条软错误；否则 no-op。
func RecordSoftError(m Meta, format string, args ...any) {
	rec, ok := m.(SoftErrorRecorder)
	if !ok || rec == nil {
		return
	}
	rec.RecordConvertSoftError(fmt.Sprintf(format, args...))
}

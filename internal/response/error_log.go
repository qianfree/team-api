package response

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"strings"
	"time"

	"github.com/gogf/gf/v2/errors/gerror"

	"github.com/qianfree/team-api/internal/logic/common"
)

// ErrorLogRecord is a lightweight struct for async error log persistence.
type ErrorLogRecord struct {
	RequestId    string
	ErrorCode    int
	ErrorMessage string
	StackTrace   string
	HttpMethod   string
	RequestPath  string
	RequestBody  string
	Source       string
}

// errorLogWriter is the global async writer for sys_error_logs.
var errorLogWriter *common.UsageLogWriter

// InitErrorLogWriter initializes the global error log writer.
func InitErrorLogWriter() {
	errorLogWriter = common.NewUsageLogWriter(common.UsageLogWriterConfig{
		Table:         "sys_error_logs",
		QueueSize:     2048,
		BatchSize:     16,
		FlushInterval: 2 * time.Second,
		Workers:       2,
		Overflow:      common.OverflowDrop,
	})
	errorLogWriter.Start()
}

// CloseErrorLogWriter flushes and closes the global error log writer.
func CloseErrorLogWriter() {
	if errorLogWriter != nil {
		errorLogWriter.Close()
	}
}

// captureStackTrace captures the current goroutine's stack trace.
func captureStackTrace(skip int) string {
	pcs := make([]uintptr, 32)
	n := runtime.Callers(skip+2, pcs)
	if n == 0 {
		return ""
	}
	frames := runtime.CallersFrames(pcs[:n])
	var b strings.Builder
	for {
		frame, more := frames.Next()
		fmt.Fprintf(&b, "%s\n\t%s:%d\n", frame.Function, frame.File, frame.Line)
		if !more {
			break
		}
	}
	return b.String()
}

// truncateString truncates a string to maxLen bytes.
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// writeErrorLog submits an error record to the async writer.
func writeErrorLog(r *ErrorLogRecord) {
	if errorLogWriter == nil {
		return
	}
	errorLogWriter.Submit(r)
}

// WriteErrorLogFromCtx writes an error log from non-HTTP context (cron/background).
func WriteErrorLogFromCtx(ctx context.Context, err error, source string) {
	if errorLogWriter == nil {
		return
	}
	code := 500
	var gerr *gerror.Error
	if errors.As(err, &gerr) {
		code = gerr.Code().Code()
		if code == 0 {
			code = 500
		}
	}
	errorLogWriter.Submit(&ErrorLogRecord{
		ErrorCode:    code,
		ErrorMessage: err.Error(),
		StackTrace:   captureStackTrace(2),
		Source:       source,
		RequestId:    getRequestIDFromCtx(ctx),
	})
}

// WritePanicLog writes a panic record to sys_error_logs (called from Recovery middleware).
func WritePanicLog(requestID, httpMethod, requestPath, errMsg, stack string) {
	if errorLogWriter == nil {
		return
	}
	errorLogWriter.Submit(&ErrorLogRecord{
		RequestId:    requestID,
		ErrorCode:    500,
		ErrorMessage: errMsg,
		StackTrace:   stack,
		HttpMethod:   httpMethod,
		RequestPath:  requestPath,
		Source:       "panic",
	})
}

// getRequestIDFromCtx extracts request ID from GoFrame context.
func getRequestIDFromCtx(ctx context.Context) string {
	if ctx != nil {
		if v := ctx.Value("RequestId"); v != nil {
			if s, ok := v.(string); ok {
				return s
			}
		}
	}
	return ""
}

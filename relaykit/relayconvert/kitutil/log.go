package kitutil

import (
	"fmt"
	"os"
	"sync/atomic"
)

// kit 各包通过这些 hook 上报少见的请求数据形态异常。宿主在启动时将它们重定向进自身的日志系统；
// 独立使用 relaykit 的用户则使用默认的 stderr 输出。

type LogFunc func(message string)

var (
	logInfo        atomic.Pointer[LogFunc]
	logError       atomic.Pointer[LogFunc]
	logSystemError atomic.Pointer[LogFunc]
)

func SetLogging(info LogFunc, errorFn LogFunc) {
	if info != nil {
		logInfo.Store(&info)
	}
	if errorFn != nil {
		logError.Store(&errorFn)
	}
}

// SetSystemErrorLogging 配置转换器内部故障专用的 hook。
func SetSystemErrorLogging(errorFn LogFunc) {
	if errorFn != nil {
		logSystemError.Store(&errorFn)
	}
}

func LogInfo(message string) {
	if fn := logInfo.Load(); fn != nil {
		(*fn)(message)
		return
	}
	fmt.Fprintf(os.Stderr, "[relaykit] %s\n", message)
}

func LogError(message string) {
	if fn := logError.Load(); fn != nil {
		(*fn)(message)
		return
	}
	fmt.Fprintf(os.Stderr, "[relaykit] ERROR %s\n", message)
}

// LogSystemError 通过专用 hook 上报转换器内部故障，
// 与请求数据格式错误的诊断信息区分开来。
func LogSystemError(message string) {
	if fn := logSystemError.Load(); fn != nil {
		(*fn)(message)
		return
	}
	fmt.Fprintf(os.Stderr, "[relaykit] SYSTEM ERROR %s\n", message)
}

// Debug 标识是否启用 kit 的详细诊断信息。宿主在启动时设置一次
// （new-api 将 common.DebugEnabled 镜像同步给它）。
var Debug atomic.Bool

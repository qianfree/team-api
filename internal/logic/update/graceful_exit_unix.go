//go:build !windows

package update

import (
	"syscall"
)

// signalSelfTerm 向自身发送 SIGTERM，触发 GoFrame 的优雅关闭信号处理
func signalSelfTerm() bool {
	return syscall.Kill(syscall.Getpid(), syscall.SIGTERM) == nil
}

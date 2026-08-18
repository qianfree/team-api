//go:build !windows

package update

import (
	"os"
	"syscall"
)

// execSelf 用 syscall.Exec 将当前进程原地替换为 bin 指向的新版本二进制。
// 成功时进程镜像被替换、永不返回（PID 不变，外部进程管理器感知不到重启）；
// 失败返回 false，由调用方决定退出。
func execSelf(bin string) bool {
	return syscall.Exec(bin, os.Args, os.Environ()) == nil
}

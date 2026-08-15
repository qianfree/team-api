package update

import (
	"context"
	"os"
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

const (
	// exitResponseGrace 让 HTTP 响应先写出的等待时间
	exitResponseGrace = 500 * time.Millisecond
	// gracefulExitTimeout 优雅关闭链路的最长等待。需覆盖 cmd.go defer 链中
	// sync-image 任务池的收尾等待（3 分钟），再留余量；超时则强制换壳/退出兜底，
	// 防止个别长连接把退出拖到无限长。
	gracefulExitTimeout = 4 * time.Minute
)

// gracefulExit 更新/回滚完成后的进程退出入口。
//
// 不能直接 os.Exit：那会跳过 cmd.go 中 s.Run() 之后的 defer 链（排空任务池、
// flush 用量/审计异步 Writer、退款在途任务），导致计费数据丢失。
// 正确做法是向自身发送 SIGTERM，走 GoFrame 的信号处理链路：
//
//	SIGTERM → ghttp 关闭监听并等待在途请求 → s.Run() 返回
//	        → cmd.go defer 链执行（LIFO，排空任务池/Writer）
//	        → 末尾 MaybeExecRestart 用 syscall.Exec 原地换壳为新版本
//	          （同 PID 继续运行，不依赖外部进程管理器拉起）
//	        → 若没有待重启版本则自然退出
//
// Windows 无自信号机制（且在线更新仅支持 Linux），退化为直接退出。
func gracefulExit(ctx context.Context, reason string) {
	go func() {
		time.Sleep(exitResponseGrace)
		g.Log().Infof(ctx, "%s: requesting graceful shutdown", reason)

		if !signalSelfTerm() {
			// 平台不支持自信号：无优雅链路可用，直接退出（不阻塞调用方）
			os.Exit(0)
		}

		// 优雅链路正常情况下会走完 cmd.go defer 链末尾的 MaybeExecRestart 换壳；
		// 若超过上限仍未完成（如 SSE 流长时间不结束），defer 链不会执行，
		// 这里直接原地换壳/退出兜底，防止服务停机。
		time.Sleep(gracefulExitTimeout)
		g.Log().Warningf(ctx, "%s: graceful shutdown not finished within %v, restarting directly",
			reason, gracefulExitTimeout)
		restartOrExit(ctx)
	}()
}

// MaybeExecRestart 是 cmd.go defer 链的最后一步（注册在最前、最后执行）。
// 若存在待重启的新版本（在线更新/回滚已完成换壳），用 syscall.Exec 原地换壳：
// 进程 PID 不变，不依赖外部进程管理器拉起；正常退出（无待重启版本）时是空操作，
// 不干预普通关闭流程。
func MaybeExecRestart(ctx context.Context) {
	bin := manager.ConsumeRestartBinary()
	if bin == "" {
		return
	}
	g.Log().Infof(ctx, "Restarting into %s", bin)
	if !execSelf(bin) {
		g.Log().Errorf(ctx, "Failed to exec %s, exiting", bin)
		os.Exit(1)
	}
	// exec 成功时进程镜像已被替换，永不返回
}

// restartOrExit 超时兜底入口：此时 cmd.go defer 链尚未执行，直接原地换壳；
// 无待重启版本则直接退出。
func restartOrExit(ctx context.Context) {
	bin := manager.ConsumeRestartBinary()
	if bin == "" {
		os.Exit(0)
	}
	g.Log().Infof(ctx, "Restarting into %s (timeout fallback)", bin)
	if !execSelf(bin) {
		g.Log().Errorf(ctx, "Failed to exec %s, exiting", bin)
		os.Exit(1)
	}
}

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
	// sync-image 任务池的收尾等待（3 分钟），再留余量；超时则强制退出兜底，
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
//	        → cmd.go defer 链执行（LIFO）→ main 返回，进程自然退出
//
// 由进程管理器（systemd Restart=always / supervisor 等）随后拉起新二进制。
// Windows 无自信号机制（且在线更新仅支持 Linux），退化为直接退出。
func gracefulExit(ctx context.Context, reason string) {
	go func() {
		time.Sleep(exitResponseGrace)
		g.Log().Infof(ctx, "%s: requesting graceful shutdown", reason)

		if !signalSelfTerm() {
			// 平台不支持自信号：无优雅链路可用，直接退出（不阻塞调用方）
			os.Exit(0)
		}

		// 优雅链路正常情况下会让 main 自然返回；若超过上限仍未退出
		// （如 SSE 流长时间不结束），强制退出兜底
		time.Sleep(gracefulExitTimeout)
		g.Log().Warningf(ctx, "%s: graceful shutdown not finished within %v, forcing exit",
			reason, gracefulExitTimeout)
		os.Exit(0)
	}()
}

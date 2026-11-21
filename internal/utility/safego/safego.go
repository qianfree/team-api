// Package safego 提供后台 goroutine 的统一 panic 防护包装。
//
// 规范要求每个自行启动的 goroutine 顶层必须有 defer recover（2026-11 审计 §4.2）：
// 任意后台 goroutine panic 都会直接崩掉整个多租户进程，而不只是该任务失败。
// 此前仓内各处就地手写 recover（cron.go runHandlerSafely、workbench、provider 等），
// 风格不一且大量点位遗漏；统一收敛到本包装，新代码一律使用 safego.Run 启动
// 后台 goroutine，不再手写 recover。
package safego

import (
	"context"
	"runtime/debug"
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// 长驻循环 panic 重启的退避区间：首次 5s，指数递增，封顶 60s，
// 防止确定性 panic（如启动即 nil deref）演变成重启风暴刷爆日志。
const (
	loopRestartBackoffInitial = 5 * time.Second
	loopRestartBackoffMax     = 60 * time.Second
)

// Run 在新 goroutine 中执行 fn，并兜底 panic：进程存活，Error 级日志记录
// panic 值与完整堆栈（对齐 cron.go runHandlerSafely 的口径：堆栈只进日志）。
//
// name 用于日志定位，约定格式「模块: 场景」（如 "billing:sweeper"）。
// ctx 原样传给 fn，不改变取消语义——调用方需要脱离请求生命周期时自行传
// context.WithoutCancel(ctx) 派生的 ctx。
func Run(ctx context.Context, name string, fn func(ctx context.Context)) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				g.Log().Errorf(ctx, "%s panicked: %v\n%s", name, r, debug.Stack())
			}
		}()
		fn(ctx)
	}()
}

// RunLoop 在新 goroutine 中运行长驻循环 fn：fn panic 后按指数退避间隔重启，
// fn 正常返回视为收到停机信号（select 到 stop/ctx.Done 后 return），不重启。
//
// 动机：长驻循环（物化器/订阅器/轮询器/维护循环）被 Run 包住后，失败模式从
// 「进程崩溃 → 编排器重启自愈」变成「首次 panic 即静默永久降级」；RunLoop 补上
// 自愈能力（范式同 usage_log_writer runWorker 的 recover-重启）。
//
// 约束：
//   - fn 必须幂等可重入（重启 = 重新执行整个 fn，内部 ticker/连接会重建）；
//   - 不得在 fn 内做 WaitGroup.Done 等一次性配额操作——wg 计数的循环请改用
//     「循环体内逐条 Guard」的形态（worker 存活，无需重启）；
//   - ctx 须为进程生命周期 ctx（重启等待期间 ctx 取消则放弃重启）。
func RunLoop(ctx context.Context, name string, fn func(ctx context.Context)) {
	go func() {
		backoff := loopRestartBackoffInitial
		for {
			panicked := true
			func() {
				defer func() {
					if r := recover(); r != nil {
						g.Log().Errorf(ctx, "%s panicked: %v（%s 后重启）\n%s", name, r, backoff, debug.Stack())
					} else {
						panicked = false
					}
				}()
				fn(ctx)
			}()
			if !panicked {
				return
			}
			select {
			case <-ctx.Done():
				return
			case <-time.After(backoff):
			}
			if backoff < loopRestartBackoffMax {
				backoff *= 2
			}
		}
	}()
}

// Guard 同步执行 fn 并兜底 panic（不启动 goroutine）。适用于循环体内逐段保护：
// 一条消息/一个 job 的异常只影响当次处理，循环本身存活继续消费。
func Guard(ctx context.Context, name string, fn func()) {
	defer func() {
		if r := recover(); r != nil {
			g.Log().Errorf(ctx, "%s panicked: %v\n%s", name, r, debug.Stack())
		}
	}()
	fn()
}

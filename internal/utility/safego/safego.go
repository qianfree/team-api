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

	"github.com/gogf/gf/v2/frame/g"
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

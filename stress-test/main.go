package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

func main() {
	cfg := ParseConfig()
	cfg.PrintSummary()

	// 确保输出目录存在
	os.MkdirAll(cfg.OutputDir, 0755)

	// 创建指标收集器
	metrics := NewMetrics()

	// 上下文：支持 Ctrl+C 优雅退出 + 超时
	ctx, cancel := context.WithTimeout(context.Background(), cfg.Duration)
	defer cancel()

	// 监听中断信号
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		fmt.Println("\n\n⚠️  收到中断信号，正在停止压测...")
		cancel()
	}()

	// 实时进度显示
	stopDisplay := make(chan struct{})
	go liveDisplay(ctx, metrics, cfg, stopDisplay)

	// 启动 worker
	startTime := time.Now()
	var wg sync.WaitGroup

	activeWorkers := atomic.Int64{}
	totalWorkers := cfg.Concurrency

	// 计算梯度上升时间间隔
	rampInterval := time.Duration(0)
	if cfg.RampUp > 0 && cfg.Concurrency > 1 {
		rampInterval = cfg.RampUp / time.Duration(cfg.Concurrency)
	}

	for i := range totalWorkers {
		// 梯度上升：逐步启动 worker
		if rampInterval > 0 && i > 0 {
			time.Sleep(rampInterval)
		}

		select {
		case <-ctx.Done():
			// 上下文已取消，不再启动新 worker
			goto done
		default:
		}

		wg.Add(1)
		activeWorkers.Add(1)

		go func(workerID int) {
			defer wg.Done()
			defer activeWorkers.Add(-1)

			worker := createWorker(cfg, metrics, cfg.Provider)
			worker(ctx, workerID)
		}(i)
	}

done:
	// 等待所有 worker 完成
	wg.Wait()
	close(stopDisplay)

	elapsed := time.Since(startTime)

	// 打印最终报告
	metrics.PrintReport(cfg, elapsed)

	// 保存 JSON 报告
	if err := metrics.SaveJSON(cfg, elapsed); err != nil {
		fmt.Printf("保存报告失败: %v\n", err)
	}
}

// workerFunc worker 函数签名
type workerFunc func(ctx context.Context, workerID int)

// createWorker 根据场景创建对应的 worker
func createWorker(cfg *Config, metrics *Metrics, scene string) workerFunc {
	switch scene {
	case "chat":
		tester := NewChatTester(cfg, metrics)
		return tester.RunWorker
	case "image":
		tester := NewImageTester(cfg, metrics)
		return tester.RunWorker
	case "video":
		tester := NewVideoTester(cfg, metrics)
		return tester.RunWorker
	case "mixed":
		// 混合模式：按比例分配 worker
		return createMixedWorker(cfg, metrics)
	default:
		fmt.Printf("未知场景: %s，使用 chat\n", scene)
		tester := NewChatTester(cfg, metrics)
		return tester.RunWorker
	}
}

// createMixedWorker 混合模式：chat 60% + image 30% + video 10%
func createMixedWorker(cfg *Config, metrics *Metrics) workerFunc {
	chatTester := NewChatTester(cfg, metrics)
	imageTester := NewImageTester(cfg, metrics)
	videoTester := NewVideoTester(cfg, metrics)

	return func(ctx context.Context, workerID int) {
		// 简单分配：workerID % 10 决定场景
		// 0-5: chat, 6-8: image, 9: video
		sceneType := workerID % 10
		switch {
		case sceneType < 6:
			chatTester.RunWorker(ctx, workerID)
		case sceneType < 9:
			imageTester.RunWorker(ctx, workerID)
		default:
			videoTester.RunWorker(ctx, workerID)
		}
	}
}

// liveDisplay 实时进度显示
func liveDisplay(ctx context.Context, metrics *Metrics, cfg *Config, stop chan struct{}) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-stop:
			return
		case <-ctx.Done():
			return
		case <-ticker.C:
			total := metrics.TotalRequests.Load()
			success := metrics.SuccessRequests.Load()
			failed := metrics.FailedRequests.Load()
			qps := metrics.CurrentQPS()
			elapsed := time.Since(metrics.startTime)

			if elapsed > 0 && total > 0 {
				avgQPS := float64(total) / elapsed.Seconds()
				fmt.Printf("⏱  %s | 请求: %d | 成功: %d | 失败: %d | 实时QPS: %.1f | 平均QPS: %.1f\n",
					elapsed.Round(time.Second),
					total, success, failed,
					qps, avgQPS,
				)
			}

			// 视频任务统计
			tasksSub := metrics.TasksSubmitted.Load()
			if tasksSub > 0 {
				fmt.Printf("   📊 任务: 提交 %d | 完成 %d | 失败 %d\n",
					tasksSub,
					metrics.TasksCompleted.Load(),
					metrics.TasksFailed.Load(),
				)
			}
		}
	}
}

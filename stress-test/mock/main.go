package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
)

func main() {
	cfg := ParseMockConfig()
	cfg.MustValidate()
	cfg.PrintBanner()

	tasks := &sync.Map{}

	mux := http.NewServeMux()

	// Chat completions（OpenAI 兼容）
	mux.HandleFunc("/v1/chat/completions", chatHandler(cfg))

	// Completions（兼容旧版）
	mux.HandleFunc("/v1/completions", chatHandler(cfg))

	// 图片生成
	mux.HandleFunc("/v1/images/generations", imageHandler(cfg))

	// 视频生成 - 提交
	mux.HandleFunc("/v1/video/generations", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" {
			videoSubmitHandler(cfg, tasks)(w, r)
		} else {
			videoFetchHandler(cfg, tasks)(w, r)
		}
	})

	// 视频生成 - 轮询（带 task_id 的路径）
	mux.HandleFunc("/v1/video/generations/", videoFetchHandler(cfg, tasks))

	// 模型列表
	mux.HandleFunc("/v1/models", modelsHandler())
	mux.HandleFunc("/v1/models/", modelsHandler())

	// Mock 自身统计
	mux.HandleFunc("/mock/stats", statsHandler())

	// 兜底
	mux.HandleFunc("/", catchAll(cfg))

	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.Port),
		Handler: mux,
	}

	// 实时统计显示
	go liveStatsDisplay(cfg)

	// 优雅关闭
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		fmt.Println("\n\n🛑 Mock Server 正在停止...")
		server.Close()
	}()

	fmt.Printf("✅ Mock AI Server 已启动\n")
	fmt.Printf("   Chat:   POST http://localhost:%d/v1/chat/completions\n", cfg.Port)
	fmt.Printf("   Image:  POST http://localhost:%d/v1/images/generations\n", cfg.Port)
	fmt.Printf("   Video:  POST http://localhost:%d/v1/video/generations\n", cfg.Port)
	fmt.Printf("   Models: GET  http://localhost:%d/v1/models\n", cfg.Port)
	fmt.Printf("   Stats:  GET  http://localhost:%d/mock/stats\n", cfg.Port)
	fmt.Println()
	fmt.Println("💡 在 team-api 管理后台添加渠道，Base URL 设为:")
	fmt.Printf("   http://localhost:%d\n\n", cfg.Port)
	fmt.Println("按 Ctrl+C 停止")
	fmt.Println("─────────────────────────────────────────────────")

	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("❌ Mock Server 启动失败: %v", err)
	}
}

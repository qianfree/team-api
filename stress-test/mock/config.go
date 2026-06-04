package main

import (
	"flag"
	"fmt"
	"os"
	"time"
)

// MockConfig Mock 服务器配置
type MockConfig struct {
	Port         int           // 监听端口
	Latency      time.Duration // 基础响应延迟（模拟 AI 推理时间）
	StreamSpeed  int           // 流式输出速度（tokens/秒）
	MaxTokens    int           // 每次响应最大 token 数
	ErrorRate    float64       // 错误率（0.0 ~ 1.0）
	TimeoutRate  float64       // 超时率（0.0 ~ 1.0）
	VideoProcess time.Duration // 视频任务模拟处理时长
	Verbose      bool
}

func ParseMockConfig() *MockConfig {
	cfg := &MockConfig{}

	flag.IntVar(&cfg.Port, "port", 19000, "Mock 服务器端口")
	flag.DurationVar(&cfg.Latency, "latency", 500*time.Millisecond, "基础响应延迟（模拟 AI 推理时间）")
	flag.IntVar(&cfg.StreamSpeed, "speed", 40, "流式输出速度（tokens/秒）")
	flag.IntVar(&cfg.MaxTokens, "max-tokens", 256, "每次响应最大 token 数")
	flag.Float64Var(&cfg.ErrorRate, "error-rate", 0.0, "模拟错误率（0.0~1.0）")
	flag.Float64Var(&cfg.TimeoutRate, "timeout-rate", 0.0, "模拟超时率（0.0~1.0）")
	flag.DurationVar(&cfg.VideoProcess, "video-time", 30*time.Second, "视频生成模拟耗时")
	flag.BoolVar(&cfg.Verbose, "v", false, "详细日志")

	flag.Parse()

	return cfg
}

func (c *MockConfig) PrintBanner() {
	fmt.Println("╔══════════════════════════════════════════════════╗")
	fmt.Println("║          Mock AI Server (本地模拟)               ║")
	fmt.Println("╠══════════════════════════════════════════════════╣")
	fmt.Printf("║  端口:         %-36d ║\n", c.Port)
	fmt.Printf("║  基础延迟:     %-36s ║\n", c.Latency)
	fmt.Printf("║  流式速度:     %-36d tokens/s ║\n", c.StreamSpeed)
	fmt.Printf("║  最大 Token:   %-36d ║\n", c.MaxTokens)
	fmt.Printf("║  错误率:       %-36.1f%% ║\n", c.ErrorRate*100)
	fmt.Printf("║  超时率:       %-36.1f%% ║\n", c.TimeoutRate*100)
	fmt.Printf("║  视频耗时:     %-36s ║\n", c.VideoProcess)
	fmt.Println("╠══════════════════════════════════════════════════╣")
	fmt.Println("║  在 team-api 中配置渠道 Base URL 为:            ║")
	fmt.Printf("║  http://localhost:%-30d ║\n", c.Port)
	fmt.Println("╚══════════════════════════════════════════════════╝")
	fmt.Println()
	fmt.Printf("🚀 启动中... http://localhost:%d\n\n", c.Port)
}

func (c *MockConfig) MustValidate() {
	if c.ErrorRate < 0 || c.ErrorRate > 1 {
		fmt.Println("❌ error-rate 必须在 0.0 ~ 1.0 之间")
		os.Exit(1)
	}
	if c.TimeoutRate < 0 || c.TimeoutRate > 1 {
		fmt.Println("❌ timeout-rate 必须在 0.0 ~ 1.0 之间")
		os.Exit(1)
	}
	if c.Port <= 0 || c.Port > 65535 {
		fmt.Println("❌ port 必须在 1 ~ 65535 之间")
		os.Exit(1)
	}
}

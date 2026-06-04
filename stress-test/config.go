package main

import (
	"flag"
	"fmt"
	"os"
	"time"
)

// Config 压测配置
type Config struct {
	// 目标服务
	BaseURL    string // 服务地址，如 http://localhost:8000
	APIKey     string // API Key（Bearer Token）
	APIKeyName string // API Key 名称，用于报告标识

	// 并发参数
	Concurrency int           // 并发客户端数
	Duration    time.Duration // 压测总时长
	MaxRequests int           // 总请求数（0 表示不限，按 Duration 停止）
	RampUp      time.Duration // 并发递增时间（在 RampUp 内逐步拉满 Concurrency）

	// 请求参数
	Model    string // 模型名称
	Provider string // 测试场景: chat / image / video / mixed

	// Chat 场景参数
	ChatPrompt    string // 对话提示词
	ChatMaxTokens int    // 最大生成 token 数
	ChatStream    bool   // 是否流式

	// Image 场景参数
	ImagePrompt string // 图片提示词
	ImageSize   string // 图片尺寸

	// Video 场景参数
	VideoPrompt   string        // 视频提示词
	VideoPollInt  time.Duration // 轮询间隔
	VideoPollWait time.Duration // 提交后首次轮询等待

	// 超时
	Timeout time.Duration // 单请求超时

	// 输出
	OutputDir string // 报告输出目录
	Verbose   bool   // 详细日志
}

func ParseConfig() *Config {
	cfg := &Config{}

	flag.StringVar(&cfg.BaseURL, "url", "http://localhost:18888", "目标服务地址")
	flag.StringVar(&cfg.APIKey, "key", "", "API Key（Bearer Token）")
	flag.StringVar(&cfg.APIKeyName, "key-name", "default", "API Key 名称，用于报告")
	flag.IntVar(&cfg.Concurrency, "c", 10, "并发客户端数")
	flag.DurationVar(&cfg.Duration, "d", 60*time.Second, "压测总时长")
	flag.IntVar(&cfg.MaxRequests, "n", 0, "总请求数（0=不限，按 Duration 停止）")
	flag.DurationVar(&cfg.RampUp, "ramp", 0, "并发递增时间（0=立即全部启动）")
	flag.StringVar(&cfg.Model, "model", "gpt-4o", "模型名称")
	flag.StringVar(&cfg.Provider, "scene", "chat", "测试场景: chat / image / video / mixed")
	flag.StringVar(&cfg.ChatPrompt, "chat-prompt", "请用一句话介绍人工智能的发展历史", "Chat 提示词")
	flag.IntVar(&cfg.ChatMaxTokens, "chat-max-tokens", 256, "Chat 最大生成 token 数")
	flag.BoolVar(&cfg.ChatStream, "stream", true, "是否使用流式响应")
	flag.StringVar(&cfg.ImagePrompt, "image-prompt", "一只可爱的小猫在阳光下打盹", "图片生成提示词")
	flag.StringVar(&cfg.ImageSize, "image-size", "1024x1024", "图片尺寸")
	flag.StringVar(&cfg.VideoPrompt, "video-prompt", "一只猫在草地上奔跑", "视频生成提示词")
	flag.DurationVar(&cfg.VideoPollInt, "video-poll-int", 5*time.Second, "视频任务轮询间隔")
	flag.DurationVar(&cfg.VideoPollWait, "video-poll-wait", 10*time.Second, "视频提交后首次轮询等待")
	flag.DurationVar(&cfg.Timeout, "timeout", 120*time.Second, "单请求超时")
	flag.StringVar(&cfg.OutputDir, "output", "./stress-test/reports", "报告输出目录")
	flag.BoolVar(&cfg.Verbose, "v", false, "详细日志")

	flag.Parse()

	if cfg.APIKey == "" {
		fmt.Println("❌ 必须指定 -key 参数（API Key）")
		flag.Usage()
		os.Exit(1)
	}

	return cfg
}

// PrintSummary 打印压测配置摘要
func (c *Config) PrintSummary() {
	sceneEmoji := map[string]string{
		"chat": "💬", "image": "🖼️", "video": "🎬", "mixed": "🔀",
	}
	emoji := sceneEmoji[c.Provider]

	fmt.Println("╔══════════════════════════════════════════════════╗")
	fmt.Println("║          Team-API 压力测试                      ║")
	fmt.Println("╠══════════════════════════════════════════════════╣")
	fmt.Printf("║  目标服务: %-38s ║\n", c.BaseURL)
	fmt.Printf("║  API Key:  %-38s ║\n", maskKey(c.APIKey))
	fmt.Printf("║  场景:     %-38s ║\n", emoji+" "+c.Provider)
	fmt.Printf("║  模型:     %-38s ║\n", c.Model)
	fmt.Printf("║  并发数:   %-38d ║\n", c.Concurrency)
	if c.MaxRequests > 0 {
		fmt.Printf("║  总请求:   %-38d ║\n", c.MaxRequests)
	} else {
		fmt.Printf("║  持续时间: %-38s ║\n", c.Duration)
	}
	if c.RampUp > 0 {
		fmt.Printf("║  爬坡时间: %-38s ║\n", c.RampUp)
	}
	if c.Provider == "chat" {
		fmt.Printf("║  流式:     %-38v ║\n", c.ChatStream)
		fmt.Printf("║  MaxToken: %-38d ║\n", c.ChatMaxTokens)
	}
	fmt.Println("╚══════════════════════════════════════════════════╝")
	fmt.Println()
}

func maskKey(key string) string {
	if len(key) <= 8 {
		return "****"
	}
	return key[:4] + "****" + key[len(key)-4:]
}

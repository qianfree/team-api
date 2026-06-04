package main

import (
	"encoding/json"
	"fmt"
	"math/rand/v2"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// Statistic Mock 服务器统计
type Statistic struct {
	TotalRequests     atomic.Int64
	ChatRequests      atomic.Int64
	ImageRequests     atomic.Int64
	VideoSubmits      atomic.Int64
	VideoPolls        atomic.Int64
	ModelRequests     atomic.Int64
	SimulatedErrors   atomic.Int64
	SimulatedTimeouts atomic.Int64
}

var stats Statistic

// chatHandler 处理 /v1/chat/completions
func chatHandler(cfg *MockConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		stats.TotalRequests.Add(1)
		stats.ChatRequests.Add(1)

		if cfg.Verbose {
			fmt.Printf("[Chat] %s %s\n", r.Method, r.URL.Path)
		}

		// 解析请求
		var req struct {
			Model     string `json:"model"`
			Stream    bool   `json:"stream"`
			MaxTokens int    `json:"max_tokens"`
			Messages  []struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"messages"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeOpenAIError(w, 400, "invalid_request_error", "Invalid request body")
			return
		}

		// 模拟错误
		if shouldSimulateError(cfg) {
			stats.SimulatedErrors.Add(1)
			writeOpenAIError(w, 500, "server_error", "Simulated upstream error")
			return
		}

		// 模拟超时（直接不响应，让客户端超时）
		if shouldSimulateTimeout(cfg) {
			stats.SimulatedTimeouts.Add(1)
			time.Sleep(300 * time.Second) // 模拟挂起
			return
		}

		// 模拟推理延迟
		time.Sleep(cfg.Latency)

		if req.Stream {
			handleStreamChat(w, r, cfg, &req)
		} else {
			handleNonStreamChat(w, r, cfg, &req)
		}
	}
}

func handleStreamChat(w http.ResponseWriter, _ *http.Request, cfg *MockConfig, req *struct {
	Model     string `json:"model"`
	Stream    bool   `json:"stream"`
	MaxTokens int    `json:"max_tokens"`
	Messages  []struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	} `json:"messages"`
}) {
	maxTokens := cfg.MaxTokens
	if req.MaxTokens > 0 && req.MaxTokens < maxTokens {
		maxTokens = req.MaxTokens
	}

	// SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)

	flusher, ok := w.(http.Flusher)
	if !ok {
		return
	}

	// 生成模拟文本
	content := generateMockContent(maxTokens)
	words := splitToWords(content, cfg.StreamSpeed)

	// 第一个 chunk: role
	firstChunk := map[string]any{
		"id":      "chatcmpl-mock-" + randomID(),
		"object":  "chat.completion.chunk",
		"created": time.Now().Unix(),
		"model":   req.Model,
		"choices": []map[string]any{
			{
				"index": 0,
				"delta": map[string]string{
					"role": "assistant",
				},
				"finish_reason": nil,
			},
		},
	}
	writeSSE(w, flusher, firstChunk)

	// 内容 chunks
	for i, word := range words {
		chunk := map[string]any{
			"id":      "chatcmpl-mock-" + randomID(),
			"object":  "chat.completion.chunk",
			"created": time.Now().Unix(),
			"model":   req.Model,
			"choices": []map[string]any{
				{
					"index": 0,
					"delta": map[string]string{
						"content": word,
					},
					"finish_reason": nil,
				},
			},
		}
		writeSSE(w, flusher, chunk)

		// 按速度控制输出频率
		if i > 0 && cfg.StreamSpeed > 0 {
			interval := time.Second / time.Duration(cfg.StreamSpeed)
			time.Sleep(interval)
		}
	}

	// 最终 chunk: finish_reason + usage
	totalTokens := len(strings.Join(words, ""))
	finalChunk := map[string]any{
		"id":      "chatcmpl-mock-" + randomID(),
		"object":  "chat.completion.chunk",
		"created": time.Now().Unix(),
		"model":   req.Model,
		"choices": []map[string]any{
			{
				"index":         0,
				"delta":         map[string]string{},
				"finish_reason": "stop",
			},
		},
		"usage": map[string]int{
			"prompt_tokens":     len(req.Messages) * 20,
			"completion_tokens": totalTokens,
			"total_tokens":      len(req.Messages)*20 + totalTokens,
		},
	}
	writeSSE(w, flusher, finalChunk)

	// [DONE]
	fmt.Fprintf(w, "data: [DONE]\n\n")
	flusher.Flush()
}

func handleNonStreamChat(w http.ResponseWriter, _ *http.Request, cfg *MockConfig, req *struct {
	Model     string `json:"model"`
	Stream    bool   `json:"stream"`
	MaxTokens int    `json:"max_tokens"`
	Messages  []struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	} `json:"messages"`
}) {
	maxTokens := cfg.MaxTokens
	if req.MaxTokens > 0 && req.MaxTokens < maxTokens {
		maxTokens = req.MaxTokens
	}

	content := generateMockContent(maxTokens)

	resp := map[string]any{
		"id":      "chatcmpl-mock-" + randomID(),
		"object":  "chat.completion",
		"created": time.Now().Unix(),
		"model":   req.Model,
		"choices": []map[string]any{
			{
				"index": 0,
				"message": map[string]string{
					"role":    "assistant",
					"content": content,
				},
				"finish_reason": "stop",
			},
		},
		"usage": map[string]int{
			"prompt_tokens":     len(req.Messages) * 20,
			"completion_tokens": len(content),
			"total_tokens":      len(req.Messages)*20 + len(content),
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// imageHandler 处理 /v1/images/generations
func imageHandler(cfg *MockConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		stats.TotalRequests.Add(1)
		stats.ImageRequests.Add(1)

		if cfg.Verbose {
			fmt.Printf("[Image] %s %s\n", r.Method, r.URL.Path)
		}

		if shouldSimulateError(cfg) {
			stats.SimulatedErrors.Add(1)
			writeOpenAIError(w, 500, "server_error", "Simulated error")
			return
		}

		// 图片生成延迟（通常比 chat 长）
		time.Sleep(cfg.Latency * 2)

		resp := map[string]any{
			"created": time.Now().Unix(),
			"data": []map[string]any{
				{
					"url":            "https://mock-local.example.com/images/mock_" + randomID() + ".png",
					"revised_prompt": "Mock image: a generated image based on your prompt",
				},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}
}

// videoSubmitHandler 处理 /v1/video/generations (POST)
func videoSubmitHandler(cfg *MockConfig, tasks *sync.Map) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		stats.TotalRequests.Add(1)
		stats.VideoSubmits.Add(1)

		if cfg.Verbose {
			fmt.Printf("[Video Submit] %s %s\n", r.Method, r.URL.Path)
		}

		if shouldSimulateError(cfg) {
			stats.SimulatedErrors.Add(1)
			writeOpenAIError(w, 500, "server_error", "Simulated error")
			return
		}

		taskID := "task-mock-" + randomID()
		createdAt := time.Now()

		// 存储任务
		tasks.Store(taskID, &videoTask{
			ID:        taskID,
			Status:    "processing",
			CreatedAt: createdAt,
			ReadyAt:   createdAt.Add(cfg.VideoProcess),
		})

		resp := map[string]any{
			"id":      taskID,
			"object":  "video.generation",
			"status":  "processing",
			"created": createdAt.Unix(),
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}
}

// videoFetchHandler 处理 /v1/video/generations/:id (GET)
func videoFetchHandler(cfg *MockConfig, tasks *sync.Map) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		stats.TotalRequests.Add(1)
		stats.VideoPolls.Add(1)

		// 从路径提取 task_id
		parts := strings.Split(strings.TrimSuffix(r.URL.Path, "/"), "/")
		taskID := parts[len(parts)-1]

		val, ok := tasks.Load(taskID)
		if !ok {
			writeOpenAIError(w, 404, "not_found", "Task not found: "+taskID)
			return
		}

		task := val.(*videoTask)

		now := time.Now()
		var resp map[string]any

		if now.After(task.ReadyAt) {
			// 任务完成
			resp = map[string]any{
				"id":     task.ID,
				"status": "succeeded",
				"result": map[string]any{
					"videos": []map[string]string{
						{
							"url":    "https://mock-local.example.com/videos/" + task.ID + ".mp4",
							"status": "succeeded",
						},
					},
				},
			}
			if cfg.Verbose {
				fmt.Printf("[Video Fetch] ✅ 任务完成: %s\n", taskID)
			}
		} else {
			// 仍在处理
			progress := float64(now.Sub(task.CreatedAt)) / float64(task.ReadyAt.Sub(task.CreatedAt)) * 100
			if progress > 99 {
				progress = 99
			}
			resp = map[string]any{
				"id":       task.ID,
				"status":   "processing",
				"progress": fmt.Sprintf("%.0f%%", progress),
			}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}
}

// modelsHandler 处理 /v1/models (GET)
func modelsHandler() http.HandlerFunc {
	models := []map[string]any{
		{"id": "gpt-4o", "object": "model", "owned_by": "mock"},
		{"id": "gpt-4o-mini", "object": "model", "owned_by": "mock"},
		{"id": "claude-3-5-sonnet", "object": "model", "owned_by": "mock"},
		{"id": "dall-e-3", "object": "model", "owned_by": "mock"},
		{"id": "kling-video", "object": "model", "owned_by": "mock"},
	}

	return func(w http.ResponseWriter, _ *http.Request) {
		stats.TotalRequests.Add(1)
		stats.ModelRequests.Add(1)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"object": "list",
			"data":   models,
		})
	}
}

// statsHandler 处理 /mock/stats (GET) — Mock 服务器自身统计
func statsHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"total_requests":     stats.TotalRequests.Load(),
			"chat_requests":      stats.ChatRequests.Load(),
			"image_requests":     stats.ImageRequests.Load(),
			"video_submits":      stats.VideoSubmits.Load(),
			"video_polls":        stats.VideoPolls.Load(),
			"model_requests":     stats.ModelRequests.Load(),
			"simulated_errors":   stats.SimulatedErrors.Load(),
			"simulated_timeouts": stats.SimulatedTimeouts.Load(),
		})
	}
}

// ─── 辅助函数 ──────────────────────────────────────────

type videoTask struct {
	ID        string
	Status    string
	CreatedAt time.Time
	ReadyAt   time.Time
}

func writeSSE(w http.ResponseWriter, flusher http.Flusher, data any) {
	jsonBytes, _ := json.Marshal(data)
	fmt.Fprintf(w, "data: %s\n\n", jsonBytes)
	flusher.Flush()
}

func writeOpenAIError(w http.ResponseWriter, code int, errType, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]any{
		"error": map[string]string{
			"type":    errType,
			"message": message,
			"code":    strconv.Itoa(code),
		},
	})
}

func shouldSimulateError(cfg *MockConfig) bool {
	return cfg.ErrorRate > 0 && rand.Float64() < cfg.ErrorRate
}

func shouldSimulateTimeout(cfg *MockConfig) bool {
	return cfg.TimeoutRate > 0 && rand.Float64() < cfg.TimeoutRate
}

// generateMockContent 生成模拟 AI 回复文本
func generateMockContent(tokens int) string {
	paragraphs := []string{
		"人工智能（Artificial Intelligence，简称AI）是计算机科学的一个分支，致力于创建能够执行通常需要人类智能的任务的系统。",
		"自20世纪50年代以来，AI经历了多次发展浪潮。早期的研究集中在符号推理和专家系统上，而现代AI则主要基于机器学习和深度学习技术。",
		"近年来，大规模语言模型（LLM）的出现标志着AI发展的一个重要里程碑。这些模型通过在海量文本数据上训练，展现出了令人惊叹的语言理解和生成能力。",
		"深度学习的突破得益于三个关键因素：大数据的可用性、计算能力的大幅提升（尤其是GPU），以及算法的创新（如Transformer架构）。",
		"AI技术的应用场景日益广泛，从自然语言处理、计算机视觉到自动驾驶、医疗诊断等各个领域都在产生深远影响。",
		"展望未来，AI将继续推动技术创新和社会变革。同时，AI伦理、安全性和可控性等问题也日益受到关注，需要社会各界共同努力。",
		"多模态AI模型正在成为新的研究热点，它们能够同时处理文本、图像、音频和视频等多种信息形式，展现出更接近人类认知的能力。",
		"强化学习与大型语言模型的结合为AI代理（AI Agent）的发展开辟了新道路，使得AI系统能够在复杂环境中自主决策和执行任务。",
	}

	// 循环拼接直到达到目标长度
	result := ""
	for len(result) < tokens {
		for _, p := range paragraphs {
			result += p
			if len(result) >= tokens {
				break
			}
		}
	}
	if len(result) > tokens {
		result = result[:tokens]
	}
	return result
}

// splitToWords 将文本按模拟 token 粒度分割
func splitToWords(text string, tokensPerSec int) []string {
	// 模拟中文约 1 字符 ≈ 1 token，每批输出 tokensPerSec/10 个字符
	chunkSize := max(tokensPerSec/10, 1)
	var chunks []string
	reader := strings.NewReader(text)
	buf := make([]rune, chunkSize)
	for {
		n := 0
		for n < chunkSize {
			r, _, err := reader.ReadRune()
			if err != nil {
				break
			}
			buf[n] = r
			n++
		}
		if n == 0 {
			break
		}
		chunks = append(chunks, string(buf[:n]))
	}
	return chunks
}

func randomID() string {
	return fmt.Sprintf("%d%d", time.Now().UnixMilli(), rand.IntN(10000))
}

// liveStatsDisplay 实时显示 Mock 服务器统计
func liveStatsDisplay(cfg *MockConfig) {
	ticker := time.NewTicker(5 * time.Second)
	for range ticker.C {
		total := stats.TotalRequests.Load()
		if total == 0 {
			continue
		}
		fmt.Printf("📊 Mock Server | 总请求: %d | Chat: %d | Image: %d | Video提交: %d | 模拟错误: %d | 模拟超时: %d\n",
			total,
			stats.ChatRequests.Load(),
			stats.ImageRequests.Load(),
			stats.VideoSubmits.Load(),
			stats.SimulatedErrors.Load(),
			stats.SimulatedTimeouts.Load(),
		)
	}
}

// catchAll 兜底路由：记录未匹配的请求
func catchAll(cfg *MockConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if cfg.Verbose {
			fmt.Printf("[404] %s %s\n", r.Method, r.URL.Path)
		}
		// 如果是 OPTIONS（CORS preflight），直接返回
		if r.Method == "OPTIONS" {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "*")
			w.WriteHeader(http.StatusOK)
			return
		}

		// 尝试匹配常见的视频/图片异步路由
		if r.Method == "GET" && strings.Contains(r.URL.Path, "/generations/") {
			// 可能是任务轮询，需要 videoFetchHandler 处理
			// 但这里没有 tasks sync.Map，返回 404
		}

		writeOpenAIError(w, 404, "not_found", "Mock server: route not found "+r.Method+" "+r.URL.Path)
	}
}

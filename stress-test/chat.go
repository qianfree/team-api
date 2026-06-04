package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"time"
)

// ChatTester Chat 对话压测
type ChatTester struct {
	cfg     *Config
	metrics *Metrics
	client  *http.Client
}

func NewChatTester(cfg *Config, metrics *Metrics) *ChatTester {
	return &ChatTester{
		cfg:     cfg,
		metrics: metrics,
		client: &http.Client{
			Timeout: cfg.Timeout,
		},
	}
}

// chatRequest OpenAI 兼容请求体
type chatRequest struct {
	Model     string        `json:"model"`
	Stream    bool          `json:"stream"`
	MaxTokens int           `json:"max_tokens,omitempty"`
	Messages  []chatMessage `json:"messages"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// streamChunk SSE 流式响应块
type streamChunk struct {
	Choices []struct {
		Delta struct {
			Content string `json:"content"`
		} `json:"delta"`
		FinishReason *string `json:"finish_reason"`
	} `json:"choices"`
	Usage *struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}

// nonStreamResponse 非流式响应
type nonStreamResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}

// RunWorker 单个并发 worker
func (ct *ChatTester) RunWorker(ctx context.Context, workerID int) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			ct.sendRequest(ctx, workerID)
		}
	}
}

func (ct *ChatTester) sendRequest(ctx context.Context, workerID int) {
	start := time.Now()

	reqBody := chatRequest{
		Model:     ct.cfg.Model,
		Stream:    ct.cfg.ChatStream,
		MaxTokens: ct.cfg.ChatMaxTokens,
		Messages: []chatMessage{
			{Role: "user", Content: ct.cfg.ChatPrompt},
		},
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		ct.metrics.RecordRequest(time.Since(start), 0, err, 0, 0)
		if ct.cfg.Verbose {
			fmt.Printf("[Worker-%d] JSON 编码失败: %v\n", workerID, err)
		}
		return
	}

	req, err := http.NewRequestWithContext(ctx, "POST", ct.cfg.BaseURL+"/v1/chat/completions", bytes.NewReader(bodyBytes))
	if err != nil {
		ct.metrics.RecordRequest(time.Since(start), 0, err, 0, 0)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+ct.cfg.APIKey)

	resp, err := ct.client.Do(req)
	if err != nil {
		ct.metrics.RecordRequest(time.Since(start), 0, err, 0, 0)
		if ct.cfg.Verbose {
			fmt.Printf("[Worker-%d] 请求失败: %v\n", workerID, err)
		}
		return
	}
	defer resp.Body.Close()

	if ct.cfg.ChatStream {
		ct.handleStreamResponse(resp, start, workerID)
	} else {
		ct.handleNonStreamResponse(resp, start, workerID)
	}
}

func (ct *ChatTester) handleStreamResponse(resp *http.Response, start time.Time, workerID int) {
	var (
		firstTokenTime time.Duration
		tokens         int64
		firstToken     atomic.Bool
		scanner        = bufio.NewScanner(resp.Body)
	)

	for scanner.Scan() {
		line := scanner.Text()

		// SSE 行格式: "data: {...}" 或 "data: [DONE]"
		if !strings.HasPrefix(line, "data: ") {
			continue
		}

		data := strings.TrimPrefix(line, "data: ")

		if data == "[DONE]" {
			break
		}

		// 记录首 token 时间
		if firstToken.CompareAndSwap(false, true) {
			firstTokenTime = time.Since(start)
		}

		var chunk streamChunk
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}

		// 统计 token（从 usage 字段，通常在最后一个 chunk）
		if chunk.Usage != nil {
			tokens = int64(chunk.Usage.CompletionTokens)
		}

		// 粗略估算（无 usage 时按 delta content 字符数估算）
		for _, choice := range chunk.Choices {
			if choice.Delta.Content != "" && tokens == 0 {
				tokens += int64(len(choice.Delta.Content)) / 2 // 粗略: 1 token ≈ 2 字符
			}
		}
	}

	latency := time.Since(start)
	statusCode := resp.StatusCode

	if statusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		ct.metrics.RecordRequest(latency, statusCode,
			fmt.Errorf("HTTP %d: %s", statusCode, truncate(string(body), 200)),
			firstTokenTime, 0)
	} else {
		ct.metrics.RecordRequest(latency, statusCode, nil, firstTokenTime, tokens)
		if ct.cfg.Verbose {
			fmt.Printf("[Worker-%d] ✅ 流式完成 %s (TTFB: %s, tokens: %d)\n",
				workerID, latency.Round(time.Millisecond), firstTokenTime.Round(time.Millisecond), tokens)
		}
	}
}

func (ct *ChatTester) handleNonStreamResponse(resp *http.Response, start time.Time, workerID int) {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		ct.metrics.RecordRequest(time.Since(start), resp.StatusCode, err, 0, 0)
		return
	}

	latency := time.Since(start)

	if resp.StatusCode >= 400 {
		ct.metrics.RecordRequest(latency, resp.StatusCode,
			fmt.Errorf("HTTP %d: %s", resp.StatusCode, truncate(string(body), 200)), 0, 0)
		if ct.cfg.Verbose {
			fmt.Printf("[Worker-%d] ❌ HTTP %d\n", workerID, resp.StatusCode)
		}
		return
	}

	var result nonStreamResponse
	if err := json.Unmarshal(body, &result); err != nil {
		ct.metrics.RecordRequest(latency, resp.StatusCode, err, latency, 0)
		return
	}

	tokens := int64(0)
	if result.Usage.CompletionTokens > 0 {
		tokens = int64(result.Usage.CompletionTokens)
	}

	ct.metrics.RecordRequest(latency, resp.StatusCode, nil, latency, tokens)
	if ct.cfg.Verbose {
		fmt.Printf("[Worker-%d] ✅ 完成 %s (tokens: %d)\n",
			workerID, latency.Round(time.Millisecond), tokens)
	}
}

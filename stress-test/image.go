package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// ImageTester 图片生成压测
type ImageTester struct {
	cfg     *Config
	metrics *Metrics
	client  *http.Client
}

func NewImageTester(cfg *Config, metrics *Metrics) *ImageTester {
	return &ImageTester{
		cfg:     cfg,
		metrics: metrics,
		client: &http.Client{
			Timeout: cfg.Timeout,
		},
	}
}

// imageRequest OpenAI 兼容图片生成请求
type imageRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
	Size   string `json:"size,omitempty"`
	N      int    `json:"n,omitempty"`
}

// imageResponse OpenAI 兼容图片生成响应
type imageResponse struct {
	Created int64 `json:"created"`
	Data    []struct {
		URL           string `json:"url,omitempty"`
		B64JSON       string `json:"b64_json,omitempty"`
		RevisedPrompt string `json:"revised_prompt,omitempty"`
	} `json:"data"`
}

func (it *ImageTester) RunWorker(ctx context.Context, workerID int) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			it.sendRequest(ctx, workerID)
		}
	}
}

func (it *ImageTester) sendRequest(ctx context.Context, workerID int) {
	start := time.Now()

	reqBody := imageRequest{
		Model:  it.cfg.Model,
		Prompt: it.cfg.ImagePrompt,
		Size:   it.cfg.ImageSize,
		N:      1,
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		it.metrics.RecordRequest(time.Since(start), 0, err, 0, 0)
		return
	}

	req, err := http.NewRequestWithContext(ctx, "POST", it.cfg.BaseURL+"/v1/images/generations", bytes.NewReader(bodyBytes))
	if err != nil {
		it.metrics.RecordRequest(time.Since(start), 0, err, 0, 0)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+it.cfg.APIKey)

	resp, err := it.client.Do(req)
	if err != nil {
		it.metrics.RecordRequest(time.Since(start), 0, err, 0, 0)
		if it.cfg.Verbose {
			fmt.Printf("[Worker-%d] 请求失败: %v\n", workerID, err)
		}
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		it.metrics.RecordRequest(time.Since(start), resp.StatusCode, err, 0, 0)
		return
	}

	latency := time.Since(start)

	if resp.StatusCode >= 400 {
		it.metrics.RecordRequest(latency, resp.StatusCode,
			fmt.Errorf("HTTP %d: %s", resp.StatusCode, truncate(string(body), 200)), 0, 0)
		if it.cfg.Verbose {
			fmt.Printf("[Worker-%d] ❌ HTTP %d %s\n", workerID, resp.StatusCode, latency.Round(time.Millisecond))
		}
		return
	}

	var result imageResponse
	if err := json.Unmarshal(body, &result); err != nil {
		it.metrics.RecordRequest(latency, resp.StatusCode, err, latency, 0)
		return
	}

	it.metrics.RecordRequest(latency, resp.StatusCode, nil, latency, 0)
	if it.cfg.Verbose {
		imgCount := len(result.Data)
		fmt.Printf("[Worker-%d] ✅ 图片生成完成 %s (图片数: %d)\n",
			workerID, latency.Round(time.Millisecond), imgCount)
	}
}

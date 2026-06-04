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

// VideoTester 视频生成压测（异步提交 + 轮询模式）
type VideoTester struct {
	cfg     *Config
	metrics *Metrics
	client  *http.Client
}

func NewVideoTester(cfg *Config, metrics *Metrics) *VideoTester {
	return &VideoTester{
		cfg:     cfg,
		metrics: metrics,
		client: &http.Client{
			Timeout: cfg.Timeout,
		},
	}
}

// videoSubmitRequest 视频生成提交请求
type videoSubmitRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
}

// videoSubmitResponse 提交响应
type videoSubmitResponse struct {
	ID     string `json:"id"`
	Object string `json:"object"`
	Status string `json:"status"`
}

// videoFetchResponse 轮询响应
type videoFetchResponse struct {
	ID     string `json:"id"`
	Status string `json:"status"`
	Error  string `json:"error,omitempty"`
	Result struct {
		Videos []struct {
			URL    string `json:"url"`
			Status string `json:"status"`
		} `json:"videos"`
	} `json:"result"`
}

func (vt *VideoTester) RunWorker(ctx context.Context, workerID int) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			vt.submitAndWait(ctx, workerID)
		}
	}
}

func (vt *VideoTester) submitAndWait(ctx context.Context, workerID int) {
	// 阶段 1: 提交任务
	taskID, err := vt.submitTask(ctx, workerID)
	if err != nil {
		// 提交失败已经记录在 submitTask 中
		return
	}

	vt.metrics.TasksSubmitted.Add(1)

	if vt.cfg.Verbose {
		fmt.Printf("[Worker-%d] 📤 任务已提交: %s\n", workerID, taskID)
	}

	// 阶段 2: 等待首次轮询
	select {
	case <-time.After(vt.cfg.VideoPollWait):
	case <-ctx.Done():
		return
	}

	// 阶段 3: 轮询直到完成
	start := time.Now()
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		completed, taskErr := vt.pollTask(ctx, taskID)
		if completed {
			latency := time.Since(start)
			if taskErr != nil {
				vt.metrics.TasksFailed.Add(1)
				vt.metrics.RecordRequest(latency, 200, taskErr, 0, 0)
				if vt.cfg.Verbose {
					fmt.Printf("[Worker-%d] ❌ 任务失败: %s %s\n", workerID, taskID, taskErr)
				}
			} else {
				vt.metrics.TasksCompleted.Add(1)
				vt.metrics.RecordRequest(latency, 200, nil, 0, 0)
				if vt.cfg.Verbose {
					fmt.Printf("[Worker-%d] ✅ 视频生成完成: %s %s\n", workerID, taskID, latency.Round(time.Second))
				}
			}
			return
		}

		select {
		case <-time.After(vt.cfg.VideoPollInt):
		case <-ctx.Done():
			return
		}
	}
}

func (vt *VideoTester) submitTask(ctx context.Context, workerID int) (string, error) {
	start := time.Now()

	reqBody := videoSubmitRequest{
		Model:  vt.cfg.Model,
		Prompt: vt.cfg.VideoPrompt,
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		vt.metrics.RecordRequest(time.Since(start), 0, err, 0, 0)
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", vt.cfg.BaseURL+"/v1/video/generations", bytes.NewReader(bodyBytes))
	if err != nil {
		vt.metrics.RecordRequest(time.Since(start), 0, err, 0, 0)
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+vt.cfg.APIKey)

	resp, err := vt.client.Do(req)
	if err != nil {
		vt.metrics.RecordRequest(time.Since(start), 0, err, 0, 0)
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		vt.metrics.RecordRequest(time.Since(start), resp.StatusCode, err, 0, 0)
		return "", err
	}

	if resp.StatusCode >= 400 {
		err := fmt.Errorf("提交失败 HTTP %d: %s", resp.StatusCode, truncate(string(body), 200))
		vt.metrics.RecordRequest(time.Since(start), resp.StatusCode, err, 0, 0)
		if vt.cfg.Verbose {
			fmt.Printf("[Worker-%d] ❌ %s\n", workerID, err)
		}
		return "", err
	}

	var result videoSubmitResponse
	if err := json.Unmarshal(body, &result); err != nil {
		vt.metrics.RecordRequest(time.Since(start), resp.StatusCode, err, 0, 0)
		return "", err
	}

	return result.ID, nil
}

func (vt *VideoTester) pollTask(ctx context.Context, taskID string) (completed bool, err error) {
	url := fmt.Sprintf("%s/v1/video/generations/%s", vt.cfg.BaseURL, taskID)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return false, err
	}
	req.Header.Set("Authorization", "Bearer "+vt.cfg.APIKey)

	resp, err := vt.client.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return false, err
	}

	var result videoFetchResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return false, err
	}

	switch result.Status {
	case "succeeded", "success", "completed":
		return true, nil
	case "failed", "error":
		return true, fmt.Errorf("任务失败: %s", result.Error)
	case "processing", "running", "pending", "queued", "in_progress":
		return false, nil
	default:
		return false, nil
	}
}

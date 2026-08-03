package dto

import "encoding/json"

// TaskSubmitRequest 异步任务提交请求（客户端 → 平台）
type TaskSubmitRequest struct {
	Model    string          `json:"model"`
	Prompt   string          `json:"prompt,omitempty"`
	Metadata json.RawMessage `json:"metadata,omitempty"` // 额外参数（resolution, duration, ratio 等）
	Size     string          `json:"size,omitempty"`     // 视频分辨率（已废弃，使用 metadata.resolution）
	Length   int             `json:"length,omitempty"`   // 视频时长（已废弃，使用 metadata.duration）
}

// TaskSubmitResponse 异步任务提交响应（平台 → 客户端）
type TaskSubmitResponse struct {
	ID        string `json:"id"`
	Status    string `json:"status"`
	Model     string `json:"model"`
	CreatedAt int64  `json:"created_at"`
}

// TaskFetchResponse 异步任务查询响应
type TaskFetchResponse struct {
	ID          string `json:"id"`
	Status      string `json:"status"`
	Progress    string `json:"progress,omitempty"`
	Model       string `json:"model"`
	URL         string `json:"url,omitempty"`
	Error       string `json:"error,omitempty"`
	CreatedAt   int64  `json:"created_at"`
	CompletedAt int64  `json:"completed_at,omitempty"`
}

// SunoSubmitRequest Suno 提交请求
type SunoSubmitRequest struct {
	// Suno 的请求参数由 action 决定，这里用通用 map 承载
	// 具体字段在适配器中解析
}

// SunoFetchResponse Suno 批量查询响应
type SunoFetchResponse struct {
	Tasks []TaskFetchResponse `json:"tasks"`
}

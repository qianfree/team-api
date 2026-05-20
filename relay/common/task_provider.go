package common

import (
	"context"
	"encoding/json"
	"time"
)

// TaskDataProvider 异步任务数据持久化接口
type TaskDataProvider interface {
	// CreateTask 创建异步任务记录
	CreateTask(ctx context.Context, task *AsyncTask) error

	// UpdateTask 更新任务记录
	UpdateTask(ctx context.Context, task *AsyncTask) error

	// UpdateTaskCAS CAS 状态更新（防重复结算）
	UpdateTaskCAS(ctx context.Context, task *AsyncTask, oldStatus string) error

	// GetTaskByPublicID 根据公开任务 ID 查询任务
	GetTaskByPublicID(ctx context.Context, publicTaskID string) (*AsyncTask, error)

	// GetTaskByPublicIDAndUser 根据公开任务 ID + 用户 ID 查询（客户端查询）
	GetTaskByPublicIDAndUser(ctx context.Context, publicTaskID string, userID int64) (*AsyncTask, error)

	// GetNonTerminalTasks 获取所有非终态任务（轮询用）
	GetNonTerminalTasks(ctx context.Context, limit int) ([]*AsyncTask, error)

	// GetTimedOutTasks 获取超时未完成任务
	GetTimedOutTasks(ctx context.Context, cutoffUnix int64, limit int) ([]*AsyncTask, error)

	// GetUnsettledTasks 获取终态但未结算的任务（结算重试）
	GetUnsettledTasks(ctx context.Context, limit int) ([]*AsyncTask, error)

	// GetChannelByID 获取渠道基本信息
	GetChannelByID(ctx context.Context, channelID int64) (*ChannelBasicInfo, error)
}

// AsyncTask 异步任务数据结构
type AsyncTask struct {
	ID           int64  `json:"id"`
	PublicTaskID string `json:"public_task_id"` // task_xxxxx
	RequestID    string `json:"request_id"`     // 原始请求 ID（req_xxxxx）
	Platform     string `json:"platform"`       // sora, kling, suno
	Action       string `json:"action"`         // generate, music, lyrics
	Status       string `json:"status"`
	Progress     string `json:"progress"`
	FailReason   string `json:"fail_reason,omitempty"`

	TenantID  int64 `json:"tenant_id"`
	UserID    int64 `json:"user_id"`
	ApiKeyID  int64 `json:"api_key_id"`
	ChannelID int64 `json:"channel_id"`

	ModelName     string `json:"model_name"`
	UpstreamModel string `json:"upstream_model,omitempty"`

	PreDeductAmount float64 `json:"pre_deduct_amount"`
	ActualCost      float64 `json:"actual_cost"`
	BillingSettled  bool    `json:"billing_settled"`

	PromptTokens     int `json:"prompt_tokens,omitempty"`
	CompletionTokens int `json:"completion_tokens,omitempty"`
	TotalTokens      int `json:"total_tokens,omitempty"`

	ResultURL   string          `json:"result_url,omitempty"`
	Data        json.RawMessage `json:"data,omitempty"`         // 上游原始响应
	PrivateData json.RawMessage `json:"private_data,omitempty"` // 上游任务 ID、API Key 等敏感数据

	SubmitTime *time.Time `json:"submit_time,omitempty"`
	StartTime  *time.Time `json:"start_time,omitempty"`
	FinishTime *time.Time `json:"finish_time,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
}

// ChannelBasicInfo 渠道基本信息（轮询时查询）
type ChannelBasicInfo struct {
	ID       int64
	Type     int // ProviderType
	Name     string
	BaseURL  string
	ApiKey   string
	Settings json.RawMessage
}

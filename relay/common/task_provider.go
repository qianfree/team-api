package common

import (
	"context"
	"encoding/json"
	"io"
	"time"

	"github.com/shopspring/decimal"
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
	GetTaskByPublicIDAndUser(ctx context.Context, publicTaskID string, userID int64, tenantID int64) (*AsyncTask, error)

	// SoftDeleteTask 软删除任务（DELETE /v1/videos/{id}）：置 deleted 标记，
	// 计费与审计记录保留，后续客户端查询按不存在处理
	SoftDeleteTask(ctx context.Context, task *AsyncTask) error

	// GetNonTerminalTasks 获取所有非终态任务（轮询用）
	GetNonTerminalTasks(ctx context.Context, limit int) ([]*AsyncTask, error)

	// GetTimedOutTasks 获取超时未完成任务
	GetTimedOutTasks(ctx context.Context, cutoffUnix int64, limit int) ([]*AsyncTask, error)

	// GetUnsettledTasks 获取终态但未结算的任务（结算重试）
	GetUnsettledTasks(ctx context.Context, limit int) ([]*AsyncTask, error)

	// GetChannelByID 获取渠道基本信息
	GetChannelByID(ctx context.Context, channelID int64) (*ChannelBasicInfo, error)
}

// StoredArtifact 任务产物内容读取句柄（调用方负责 Close）。
type StoredArtifact struct {
	Body        io.ReadCloser
	ContentType string
	Size        int64 // 0 = 未知
}

// TaskContentOpener TaskDataProvider 的可选能力：打开任务产物内容（key 为空 = 主产物）。
// 已转存的从平台对象存储读取，未转存的按产物声明回源代理；/v1/videos/{id}/content
// 与统一内容链接 /v1/tasks/{id}/content 共用。found=false 表示任务无登记产物
// （调用方回落 ResultURL）。
type TaskContentOpener interface {
	OpenTaskContent(ctx context.Context, task *AsyncTask, key string) (artifact *StoredArtifact, found bool, err error)
}

// TaskDelivery 成功任务的对外交付视图。
// Pending=true：上游已完成但产物仍在转存平台对象存储途中——对外须呈现进行中、不给链接
// （客户端拿到完成态时链接必然可用）。ContentURL：就绪时的主产物统一内容链接（可空）。
type TaskDelivery struct {
	Pending    bool
	ContentURL string
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

	PreDeductAmount decimal.Decimal `json:"pre_deduct_amount"`
	ActualCost      decimal.Decimal `json:"actual_cost"`
	BillingSettled  bool            `json:"billing_settled"`

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

	// 软删除标记（DELETE /v1/videos/{id}）：true 后客户端查询/下载按 404 处理
	Deleted   bool       `json:"deleted,omitempty"`
	DeletedAt *time.Time `json:"deleted_at,omitempty"`
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

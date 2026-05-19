package common

import (
	"context"
	"io"
	"net/http"
)

// TaskAdaptor 异步任务适配器接口
// 与同步 Adaptor 分离，因为异步任务有提交/轮询/解析等特殊方法
type TaskAdaptor interface {
	// Init 初始化适配器
	Init(info *RelayInfo)

	// ValidateRequest 校验请求参数
	ValidateRequest(ctx context.Context, info *RelayInfo, body []byte) *TaskError

	// EstimateBilling 估算任务费用（提交前）
	// 返回计费比率 map，如 {"duration_ratio": 1.5, "resolution_ratio": 2.0}
	EstimateBilling(ctx context.Context, info *RelayInfo, body []byte) map[string]float64

	// AdjustBillingOnSubmit 提交后根据上游确认参数调整计费
	AdjustBillingOnSubmit(info *RelayInfo, taskData []byte) map[string]float64

	// BuildRequestURL 构建上游请求 URL
	BuildRequestURL(info *RelayInfo) (string, error)

	// BuildRequestHeader 构建上游请求 Header
	BuildRequestHeader(header http.Header, info *RelayInfo) error

	// BuildRequestBody 构建上游请求体
	BuildRequestBody(ctx context.Context, info *RelayInfo, body []byte) (io.Reader, error)

	// DoRequest 发送请求到上游
	DoRequest(ctx context.Context, info *RelayInfo, requestBody io.Reader) (*http.Response, error)

	// DoResponse 解析上游提交响应，返回上游任务 ID 和任务数据
	DoResponse(ctx context.Context, resp *http.Response, info *RelayInfo) (upstreamTaskID string, taskData []byte, taskErr *TaskError)

	// FetchTask 查询上游任务状态
	FetchTask(baseURL, apiKey string, taskData []byte) (*http.Response, error)

	// ParseTaskResult 解析上游任务查询结果
	ParseTaskResult(body []byte) (*TaskInfo, error)

	// GetModelList 返回适配器支持的模型列表
	GetModelList() []string

	// GetChannelName 返回适配器名称
	GetChannelName() string
}

// TaskInfo 异步任务查询结果
type TaskInfo struct {
	Status     TaskStatusEnum
	Progress   string // "10%", "50%", "100%"
	FailReason string
	ResultURL  string    // 成功时的结果资源 URL
	Data       []byte    // 上游原始响应
	SubTasks   []SubTask // 子任务结果（如多图/多视频场景）
	ActualCost float64   // 上游返回的实际费用（0 表示未提供，使用预扣金额）

	// Token 用量（部分异步任务上游会返回，如火山引擎视频模型）
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
}

// TaskStatusEnum 任务状态枚举
type TaskStatusEnum string

const (
	TaskStatusNotStart   TaskStatusEnum = "NOT_START"
	TaskStatusSubmitted  TaskStatusEnum = "SUBMITTED"
	TaskStatusQueued     TaskStatusEnum = "QUEUED"
	TaskStatusInProgress TaskStatusEnum = "IN_PROGRESS"
	TaskStatusSuccess    TaskStatusEnum = "SUCCESS"
	TaskStatusFailure    TaskStatusEnum = "FAILURE"
)

// IsTerminal 判断是否为终态
func (s TaskStatusEnum) IsTerminal() bool {
	return s == TaskStatusSuccess || s == TaskStatusFailure
}

// SubTask 子任务结果
type SubTask struct {
	Index     int
	Status    TaskStatusEnum
	ResultURL string
}

// TaskError 异步任务错误
type TaskError struct {
	StatusCode int    // HTTP 状态码
	Message    string // 错误消息
	ErrCode    string // 错误码
}

func (e *TaskError) Error() string {
	return e.Message
}

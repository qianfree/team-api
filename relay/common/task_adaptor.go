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

	// EstimateBilling 估算任务费用（提交前）。
	// 返回计费上下文 map，两类键：
	//   - float64 乘数值：如 {"video_input": 0.6, "quality": 3.5}，引擎按序连乘；
	//   - string 规格事实值：如 {"spec.duration": 8, "spec.resolution": "720p"}，
	//     per_second 计费模式据此查定价矩阵（spec.duration 为 float64 秒数，spec.resolution 为规格原值）。
	// 旧键 duration/resolution 保留 token 伪装估算路径使用，语义随各 adaptor 存量口径，勿混用。
	EstimateBilling(ctx context.Context, info *RelayInfo, body []byte) map[string]any

	// AdjustBillingOnSubmit 提交后根据上游确认参数调整计费
	AdjustBillingOnSubmit(info *RelayInfo, taskData []byte) map[string]any

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

	// FetchTask 查询上游任务状态。ctx 供超时控制与渠道调试日志捕获器传递
	//（轮询侧经 WithDebugAttempt 注入，未开启调试时透传无额外开销）
	FetchTask(ctx context.Context, baseURL, apiKey string, taskData []byte) (*http.Response, error)

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

	// 素材计量（部分任务上游 usage 返回，如 MiniMax H3 的 input_seconds/input_image_count）：
	// 「预扣时未知、生成后官方返回」的计费依据，素材计费方案（BillingScheme.SettleTaskCost）的结算输入。
	// nil = 上游未提供
	MaterialUsage *TaskMaterialUsage

	// Token 用量（部分异步任务上游会返回，如火山引擎视频模型）
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
}

// TaskMaterialUsage 上游任务 usage 中的素材计量。
// 与 TaskInfo.TotalTokens 同级的结算信号：链接素材（视频/音频）的时长在提交时不可知，
// 以生成成功后官方返回为准；各字段零值表示该计量上游未返回（方案自行回退）。
type TaskMaterialUsage struct {
	InputVideoSeconds float64 // 输入视频时长（秒）
	InputImageCount   int     // 输入图片张数
	InputAudioSeconds float64 // 输入音频时长（秒）
	OutputSeconds     float64 // 输出（生成）时长（秒）
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

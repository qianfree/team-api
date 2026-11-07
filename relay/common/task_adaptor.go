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

// UpstreamRequestIDExtractor 供应商私有的「上游调用追踪 ID 提取」可选能力。
// 为什么需要：部分异步上游（如阿里 DashScope）不在响应头返回请求 ID，而是放在
// 提交响应体顶层（request_id 字段），通用管线无法用统一键提取。实现方从提交
// 响应体解析自家私有的追踪 ID，与 output.task_id（任务句柄）互补——提工单或
// 在上游日志定位一次提交调用时用它。未实现的上游走响应头提取或为空。
type UpstreamRequestIDExtractor interface {
	// ExtractUpstreamRequestID 从上游提交响应体提取调用追踪 ID，未返回时为空串
	ExtractUpstreamRequestID(body []byte) string
}

// BatchTaskPoller 批量轮询能力（fetchMode=batch 的适配器可选实现，如 JS 任务插件）：
// 一次上游往返携带同渠道同插件下全部未终态任务，避免逐任务打满上游配额。
// 结算/落库与逐任务路径共用 applyTaskPollResult，口径不因轮询模式分叉。
type BatchTaskPoller interface {
	// FetchModeFor 按任务平台声明轮询模式："batch" 启用批量通道，其余值按逐任务轮询。
	// 以 platform 为参是因为轮询循环拿到的适配器是工厂新实例（无状态）——按实例
	// 自身状态判定会恒得默认值，批量通道永远无法激活；同渠道不同插件可异模式。
	FetchModeFor(platform string) string
	// FetchBatchTasks 一次查询整批任务（同渠道同插件；platform 一致的分组由轮询循环保证）
	FetchBatchTasks(ctx context.Context, baseURL, apiKey string, tasks []BatchTaskRef) (*http.Response, error)
	// ParseBatchTasks 解析批量响应，按上游任务 ID 映射结果；
	// 本轮未回报的任务不出现在 map 中（保持现状等下一轮）
	ParseBatchTasks(body []byte) (map[string]*TaskInfo, error)
}

// BatchTaskRef 批量轮询的任务引用。
type BatchTaskRef struct {
	Platform string // 任务平台（js:{key}；批量分组键）
	TaskID   string // 上游任务 ID
	TaskData []byte // 任务最新上游快照（可空）
	State    []byte // 插件状态（可空；JS 插件 v1 未持久化 state）
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

	// Artifacts 终态产物声明（JS 任务插件 parseQueryResponse 上报）：key + taskData
	// 内的取值路径（上游 URL 或 base64 字段）。字节永不进 JS，转存由宿主产物流水线
	// 完成（internal/logic/task/artifact）；空 = 无产物需要转存
	Artifacts []TaskArtifact
}

// TaskArtifact 产物声明（转存流水线的输入，纯数据、无 JS 依赖）。
type TaskArtifact struct {
	Key        string // 任务内唯一，如 "video"
	MimeType   string // inline 必填；url 型为空时按内容推断
	SourceType string // url | inline
	SourcePath string // taskData 内取值路径，如 "data.videos[0].url"
	SourceAuth string // 需鉴权下载的凭据形态（"channel-bearer"）；空 = 裸下载
}

// TaskMaterialUsage 上游任务 usage 中的素材计量。
// 与 TaskInfo.TotalTokens 同级的结算信号：链接素材（视频/音频）的时长在提交时不可知，
// 以生成成功后官方返回为准；各字段零值表示该计量上游未返回（方案自行回退）。
type TaskMaterialUsage struct {
	InputVideoSeconds float64 // 输入视频时长（秒）
	InputImageCount   int     // 输入图片张数
	InputAudioSeconds float64 // 输入音频时长（秒）
	OutputSeconds     float64 // 输出（生成）时长（秒）
	Resolution        string  // 实际输出分辨率规格（如 "720P"，对齐 per_second 矩阵键）；空 = 上游未返回

	// Facts 计费用量事实（JS 任务插件经 parseQueryResponse 的 items[].usage 上报，
	// 键 ∈ usageSchema 的 number 型声明）：结算重算时并入公式 u() 通道。unit=second
	// 的事实应同时映射到 OutputSeconds（它是结算期公式重跑的触发信号）。
	Facts map[string]float64
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

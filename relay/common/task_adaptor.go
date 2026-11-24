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

// StrictBillingEstimator 严格计费估算能力（可选）：估算无法完成时返回错误，而非静默回落
// 默认价。提交管线优先于 EstimateBilling 调用——返回 TaskError 即拒绝提交（预扣尚未发生，
// 无资金副作用）。估算静默失败会让公式计费的 u() 全取 0、预扣落到下限，真实费用到结算
// 才补扣；未实现本接口的适配器沿用 EstimateBilling 的宽松语义。
type StrictBillingEstimator interface {
	// EstimateBillingStrict 估算计费上下文，返回值语义同 EstimateBilling；估算失败返回 TaskError
	EstimateBillingStrict(ctx context.Context, info *RelayInfo, body []byte) (map[string]any, *TaskError)
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
	State    []byte // 插件状态（可空；任务私有数据 plugin_state，JS 插件跨轮透传容器）
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

	// UsageSummary 终态用量摘要（面向客户端查询端点的展示视图）：各渠道官方 usage 的归一
	// 并集（时长/张数/分辨率/tokens），由各适配器在 ParseTaskResult 中声明式填充。
	// 纯展示、不参与计费——结算仍走 ActualCost/MaterialUsage/tokens 链路，两者互不影响。
	// nil = 该渠道未上报用量
	UsageSummary *TaskUsageSummary

	// Token 用量（部分异步任务上游会返回，如火山引擎视频模型）
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int

	// Artifacts 终态产物声明（JS 任务插件 parseQueryResponse 上报）：key + taskData
	// 内的取值路径（上游 URL 或 base64 字段）。字节永不进 JS，转存由宿主产物流水线
	// 完成（internal/logic/task/artifact）；空 = 无产物需要转存
	Artifacts []TaskArtifact

	// PluginState JS 任务插件本轮返回的状态（parseQueryResponse 的 items[].state）：
	// 非空时轮询链覆盖任务私有数据中的 plugin_state，下一轮回带给插件；空 = 保持原值
	PluginState []byte
}

// TaskUsageSummary 任务终态用量摘要（查询端点展示用）：各视频渠道官方 usage 的归一并集。
// 「没有的数据不返回」：指针字段仅在上游实际报告时填充（含有意义的 0，如火山输入 token 恒 0），
// 空值省略；分辨率保留各渠道原生词汇（如火山 "720p"、wan3.0/MiniMax "768P"）。
type TaskUsageSummary struct {
	InputVideoSeconds  *float64 `json:"input_video_seconds,omitempty"`  // 输入视频时长（秒）
	OutputVideoSeconds *float64 `json:"output_video_seconds,omitempty"` // 输出（生成）视频时长（秒）
	TotalSeconds       *float64 `json:"total_seconds,omitempty"`        // 总时长（输入+输出，秒）
	InputImageCount    *int     `json:"input_image_count,omitempty"`    // 输入参考图片张数
	InputAudioSeconds  *float64 `json:"input_audio_seconds,omitempty"`  // 输入音频时长（秒）
	Resolution         string   `json:"resolution,omitempty"`           // 实际输出分辨率（渠道原生词汇）
	PromptTokens       *int     `json:"prompt_tokens,omitempty"`        // 输入 token
	CompletionTokens   *int     `json:"completion_tokens,omitempty"`    // 输出 token
	TotalTokens        *int     `json:"total_tokens,omitempty"`         // 总 token
}

// UsageF / UsageI 构造 TaskUsageSummary 指针字段的小工具（适配器填充用）
func UsageF(v float64) *float64 { return &v }
func UsageI(v int) *int         { return &v }

// SynthesizeUsageSummary 从计费链路已有的用量事实（MaterialUsage + tokens）合成展示摘要，
// 供未显式声明 UsageSummary 的适配器兜底（如 JS 插件的 facts 链路）。全空返回 nil。
func SynthesizeUsageSummary(info *TaskInfo) *TaskUsageSummary {
	if info == nil {
		return nil
	}
	s := &TaskUsageSummary{}
	if mu := info.MaterialUsage; mu != nil {
		if mu.InputVideoSeconds > 0 {
			s.InputVideoSeconds = UsageF(mu.InputVideoSeconds)
		}
		if mu.OutputSeconds > 0 {
			s.OutputVideoSeconds = UsageF(mu.OutputSeconds)
		}
		if mu.InputImageCount > 0 {
			s.InputImageCount = UsageI(mu.InputImageCount)
		}
		if mu.InputAudioSeconds > 0 {
			s.InputAudioSeconds = UsageF(mu.InputAudioSeconds)
		}
		s.Resolution = mu.Resolution
	}
	if info.TotalTokens > 0 {
		s.PromptTokens = UsageI(info.PromptTokens)
		s.CompletionTokens = UsageI(info.CompletionTokens)
		s.TotalTokens = UsageI(info.TotalTokens)
	}
	if s.InputVideoSeconds == nil && s.OutputVideoSeconds == nil && s.TotalSeconds == nil &&
		s.InputImageCount == nil && s.InputAudioSeconds == nil && s.Resolution == "" &&
		s.PromptTokens == nil && s.CompletionTokens == nil && s.TotalTokens == nil {
		return nil
	}
	return s
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
	// 键 ∈ 宿主固定用量词汇的 number 型）：结算重算时并入公式 u() 通道。
	// output_seconds 应同时映射到 OutputSeconds（它是结算期公式重跑的触发信号）。
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

package minimax

// MiniMax v2 视频协议的任务对象与响应结构。
// 字段定义对齐官方 VideoTask（docs/modeldocs/minimax/video/04-响应对象.md），
// 由适配器解析上游查询响应、relay/handler 层组装查询端点回放响应共用。

// VideoTaskContent 任务产物内容（视频任务返回 url，Context-IR 任务返回 prompt）
type VideoTaskContent struct {
	URL    string `json:"url,omitempty"`
	Prompt string `json:"prompt,omitempty"`
}

// VideoTaskError 任务错误信息（任务失败时返回）
type VideoTaskError struct {
	Code    string `json:"code,omitempty"`
	Message string `json:"message,omitempty"`
}

// VideoTaskUsage 用量：视频任务返回按秒计量字段，H3-Context-IR 任务返回 Token 字段
type VideoTaskUsage struct {
	TotalSeconds      int `json:"total_seconds,omitempty"`
	InputSeconds      int `json:"input_seconds,omitempty"`
	OutputSeconds     int `json:"output_seconds,omitempty"`
	InputImageCount   int `json:"input_image_count,omitempty"`
	InputAudioSeconds int `json:"input_audio_seconds,omitempty"`
	TotalTokens       int `json:"total_tokens,omitempty"`
	PromptTokens      int `json:"prompt_tokens,omitempty"`
	CompletionTokens  int `json:"completion_tokens,omitempty"`
}

// VideoTask 官方 VideoTask 对象（单任务查询与列表接口共用的任务载体）
type VideoTask struct {
	ID         string            `json:"id"`
	Model      string            `json:"model"`
	Status     string            `json:"status"`
	Error      *VideoTaskError   `json:"error,omitempty"`
	CreatedAt  int64             `json:"created_at"`
	UpdatedAt  int64             `json:"updated_at"`
	Content    *VideoTaskContent `json:"content,omitempty"`
	Resolution string            `json:"resolution,omitempty"`
	Duration   int               `json:"duration,omitempty"`
	Usage      *VideoTaskUsage   `json:"usage,omitempty"`
	Ratio      string            `json:"ratio,omitempty"`
	TaskType   string            `json:"task_type,omitempty"`
	Modality   string            `json:"modality,omitempty"`
}

// TaskQueryResponse v2 单任务查询响应（GetVideoGenerationV2Resp，task 外层包装）
type TaskQueryResponse struct {
	Task *VideoTask `json:"task"`
}

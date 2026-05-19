package ali

// dashScopeRequest DashScope 异步图片生成请求
type dashScopeRequest struct {
	Model      string         `json:"model"`
	Input      dashScopeInput `json:"input"`
	Parameters map[string]any `json:"parameters,omitempty"`
}

// dashScopeInput DashScope 请求输入
type dashScopeInput struct {
	Prompt         string `json:"prompt,omitempty"`
	NegativePrompt string `json:"negative_prompt,omitempty"`
}

// dashScopeSubmitResponse 异步提交响应
type dashScopeSubmitResponse struct {
	Output struct {
		TaskID     string `json:"task_id"`
		TaskStatus string `json:"task_status"`
		Code       string `json:"code,omitempty"`
		Message    string `json:"message,omitempty"`
	} `json:"output"`
	RequestID string `json:"request_id"`
}

// dashScopeTaskResponse 异步任务轮询响应
type dashScopeTaskResponse struct {
	Output struct {
		TaskID     string            `json:"task_id"`
		TaskStatus string            `json:"task_status"`
		Results    []dashScopeResult `json:"results"`
		Code       string            `json:"code,omitempty"`
		Message    string            `json:"message,omitempty"`
	} `json:"output"`
	Usage struct {
		ImageCount int `json:"image_count"`
	} `json:"usage,omitempty"`
	RequestID string `json:"request_id,omitempty"`
}

// dashScopeResult 单个图片结果
type dashScopeResult struct {
	URL string `json:"url"`
}

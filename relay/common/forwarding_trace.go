package common

// ForwardingTrace 请求转发路径追踪（仅管理员可见）
type ForwardingTrace struct {
	EntryPath      string          `json:"entry_path"`
	EntryFormat    string          `json:"entry_format"`
	RequestedModel string          `json:"requested_model"`
	UpstreamModel  string          `json:"upstream_model"`
	ModelMapped    bool            `json:"model_mapped"`
	Hops           []ForwardingHop `json:"hops"`
	TotalAttempts  int             `json:"total_attempts"`
}

// ForwardingHop 单次转发跳转记录
type ForwardingHop struct {
	Attempt       int     `json:"attempt"`
	ChannelID     int64   `json:"channel_id"`
	ChannelName   string  `json:"channel_name"`
	ChannelType   int     `json:"channel_type"`
	Provider      string  `json:"provider"`
	BaseURL       string  `json:"base_url"`
	UpstreamURL   string  `json:"upstream_url"`
	UpstreamModel string  `json:"upstream_model"`
	ModelMapped   bool    `json:"model_mapped"`
	Success       bool    `json:"success"`
	Error         string  `json:"error,omitempty"`
	LatencyMs     float64 `json:"latency_ms"`
}

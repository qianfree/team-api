package common

// ForwardingTrace 请求转发路径追踪（仅管理员可见）
type ForwardingTrace struct {
	EntryPath      string              `json:"entry_path"`
	EntryFormat    string              `json:"entry_format"`
	RequestedModel string              `json:"requested_model"`
	UpstreamModel  string              `json:"upstream_model"`
	ModelMapped    bool                `json:"model_mapped"`
	Hops           []ForwardingHop     `json:"hops"`
	TotalAttempts  int                 `json:"total_attempts"`
	Scheduler      []SchedulerDecision `json:"scheduler,omitempty"` // 新调度引擎决策明细（修订 R5）
}

// SchedulerDecision 新调度引擎单次选择的决策明细（修订 R5：权重分解进 ForwardingTrace）。
type SchedulerDecision struct {
	Attempt         int                `json:"attempt"`
	ChannelID       int64              `json:"channel_id"`
	KeyID           int64              `json:"key_id,omitempty"`
	Reason          string             `json:"reason"` // bind / hrw / overflow / probe / cred_rotate
	Tier            string             `json:"tier,omitempty"`
	SessionSource   string             `json:"session_source,omitempty"`
	Weights         map[string]float64 `json:"weights,omitempty"` // base/tier/health/headroom/cost/ramp/effective
	Candidates      int                `json:"candidates,omitempty"`
	ExcludedBreaker int                `json:"excluded_breaker,omitempty"`
	ExcludedLease   int                `json:"excluded_lease,omitempty"`
	ExcludedRequest int                `json:"excluded_request,omitempty"`
}

// ForwardingHop 单次转发跳转记录
type ForwardingHop struct {
	Attempt         int     `json:"attempt"`
	ChannelID       int64   `json:"channel_id"`
	ChannelName     string  `json:"channel_name"`
	ChannelType     int     `json:"channel_type"`
	Provider        string  `json:"provider"`
	BaseURL         string  `json:"base_url"`
	UpstreamURL     string  `json:"upstream_url"`
	UpstreamModel   string  `json:"upstream_model"`
	ModelMapped     bool    `json:"model_mapped"`
	SelectionReason string  `json:"selection_reason,omitempty"`
	Priority        int     `json:"priority"`
	Weight          int     `json:"weight"`
	HealthScore     float64 `json:"health_score"`
	Success         bool    `json:"success"`
	Error           string  `json:"error,omitempty"`
	LatencyMs       float64 `json:"latency_ms"`
}

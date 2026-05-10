package ws

import "encoding/json"

// 频道常量
const (
	ChannelNotification = "notification"
	ChannelAlert        = "alert"
	ChannelMonitor      = "monitor"
	ChannelTask         = "task"
	ChannelBilling      = "billing"
)

// 客户端 action
const (
	ActionPing = "ping"
)

// 服务端 action
const (
	ActionPong     = "pong"
	ActionCreated  = "created"
	ActionUpdated  = "updated"
	ActionResolved = "resolved"
	ActionProgress = "progress"
	ActionSnapshot = "snapshot"
)

// WsMessage 是所有 WebSocket 消息的统一信封。
// 客户端→服务端：Action 为 ping。
// 服务端→客户端：Action 为 created / updated / progress 等。
type WsMessage struct {
	Channel string          `json:"channel"`
	Action  string          `json:"action"`
	Payload json.RawMessage `json:"payload,omitempty"`
	Seq     int64           `json:"seq,omitempty"`
}

// RedisWsMessage 是 Redis Pub/Sub 传输信封，携带路由信息。
type RedisWsMessage struct {
	UserType string          `json:"user_type"` // "admin" / "tenant"
	Target   string          `json:"target"`    // "user:{id}" / "all" / "tenant_all:{tid}"
	Channel  string          `json:"channel"`
	Action   string          `json:"action"`
	Payload  json.RawMessage `json:"payload"`
}

// NotificationPayload 通知频道 payload
type NotificationPayload struct {
	ID          int64  `json:"id"`
	Type        string `json:"type"`
	Title       string `json:"title"`
	Content     string `json:"content"`
	IsBroadcast bool   `json:"is_broadcast"`
	CreatedAt   string `json:"created_at"`
}

// AlertPayload 告警频道 payload
type AlertPayload struct {
	ID             int64   `json:"id"`
	RuleName       string  `json:"rule_name"`
	MetricType     string  `json:"metric_type"`
	Level          string  `json:"level"`
	Status         string  `json:"status"`
	TriggerValue   float64 `json:"trigger_value"`
	Threshold      float64 `json:"threshold"`
	TriggerMessage string  `json:"trigger_message"`
	Timestamp      string  `json:"timestamp"`
}

// TaskPayload 任务频道 payload
type TaskPayload struct {
	TaskID     string `json:"task_id"`
	Status     string `json:"status"`
	Progress   int    `json:"progress"`
	FailReason string `json:"fail_reason,omitempty"`
	ResultURL  string `json:"result_url,omitempty"`
	UpdatedAt  string `json:"updated_at"`
}

// BillingPayload 计费频道 payload
type BillingPayload struct {
	EventType     string  `json:"event_type"`
	BalanceBefore float64 `json:"balance_before,omitempty"`
	BalanceAfter  float64 `json:"balance_after,omitempty"`
	Amount        float64 `json:"amount,omitempty"`
	Message       string  `json:"message,omitempty"`
	Timestamp     string  `json:"timestamp"`
}

package plugin

import (
	"context"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/net/ghttp"
)

// ---------------------------------------------------------------------------
// 可选扩展接口（插件按需实现，利用 Go 接口隐式实现特性）
// ---------------------------------------------------------------------------

// Routable 可注册 HTTP 路由的插件实现此接口。
type Routable interface {
	Routes(ctx context.Context, server *ghttp.Server) error
}

// Hookable 订阅系统事件的插件实现此接口。
type Hookable interface {
	Hooks() []HookBinding
}

// Cronable 注册定时任务的插件实现此接口。
type Cronable interface {
	CronJobs() []CronJobDef
}

// Configurable 声明配置项的插件实现此接口。
type Configurable interface {
	ConfigSchema() []ConfigFieldDef
}

// TenantAware 租户级控制的插件实现此接口。
type TenantAware interface {
	OnTenantEnable(ctx context.Context, tenantID int64) error
	OnTenantDisable(ctx context.Context, tenantID int64) error
}

// RelayInterceptor 拦截 AI 代理请求的插件实现此接口。
type RelayInterceptor interface {
	BeforeRelay(ctx context.Context, req *RelayRequest) (*RelayRequest, error)
	AfterRelay(ctx context.Context, resp *RelayResponse) (*RelayResponse, error)
}

// ---------------------------------------------------------------------------
// 支持类型
// ---------------------------------------------------------------------------

// HookBinding 事件绑定。
type HookBinding struct {
	Event    string      // 事件名，如 "relay.before_request"
	Priority int         // 优先级，数字越小越先执行
	Handler  HookHandler // 处理函数
}

// HookHandler 事件处理函数。
type HookHandler func(ctx context.Context, payload HookPayload) (HookResult, error)

// HookPayload 事件载荷。
type HookPayload struct {
	Event string // 事件名
	Data  g.Map  // 事件数据，不同事件结构不同
}

// HookResult 事件处理结果。
type HookResult struct {
	Data    g.Map // 可修改的数据（会合并回主流程）
	Aborted bool  // 是否中断后续处理（仅同步事件有效）
}

// CronJobDef 定时任务定义。
type CronJobDef struct {
	Name      string                    // 任务名（全局唯一）
	CronExpr  string                    // cron 表达式
	Handler   func(ctx context.Context) // 任务处理函数
	Singleton bool                      // 是否单例执行
}

// ConfigFieldDef 配置字段定义。
type ConfigFieldDef struct {
	Key         string      // 配置键，如 "smtp_host"
	Label       string      // 显示名
	Type        string      // 类型：string/int/bool/select/json
	Default     interface{} // 默认值
	Options     []string    // select 类型的选项
	Required    bool        // 是否必填
	Description string      // 说明
}

// RelayRequest Relay 请求信息。
type RelayRequest struct {
	Model     string            `json:"model"`      // 请求模型
	Messages  string            `json:"messages"`   // 消息内容（JSON）
	ChannelID int64             `json:"channel_id"` // 渠道 ID
	TenantID  int64             `json:"tenant_id"`  // 租户 ID
	Headers   map[string]string `json:"headers"`    // 请求头
	Body      string            `json:"body"`       // 原始请求体
}

// RelayResponse Relay 响应信息。
type RelayResponse struct {
	StatusCode int               `json:"status_code"` // HTTP 状态码
	Headers    map[string]string `json:"headers"`     // 响应头
	Body       string            `json:"body"`        // 原始响应体
	Usage      *TokenUsage       `json:"usage"`       // Token 用量
}

// TokenUsage Token 用量。
type TokenUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

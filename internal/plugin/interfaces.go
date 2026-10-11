package plugin

import (
	"context"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/net/ghttp"

	v1 "github.com/qianfree/team-api/api/admin/v1"
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

// PermissionProvider 声明权限点的插件实现此接口。
//
// 不实现此接口的插件路由会被 AdminPermissionGuard 默认拒绝（未映射规则一律 403），
// 其功能只有 super_admin 可用。实现了此接口的插件：
//   - 声明的权限点进入授权校验（可被授予角色/特批给用户）；
//   - 声明的模块档位在插件启用后出现在角色配置界面；
//   - 声明的路由规则参与 guard 的三阶段匹配（与静态规则合并，前缀更长者优先）。
//
// 规则随插件启用登记、禁用/卸载摘除（见 permission.go 的注册表）。
type PermissionProvider interface {
	// PermissionGroups 声明权限点分组（结构与核心 predefinedPermissionGroups 一致），
	// 供权限点有效性校验与角色配置高级模式展示。
	PermissionGroups() []v1.PermissionGroup
	// PermissionModules 声明「模块 × 档位」元数据，供角色配置默认界面渲染。
	// Module 需与 PermissionGroups 的 Name 对应，否则界面上两处对不上。
	PermissionModules() []v1.PermissionModuleMeta
	// PermissionRules 声明插件路由的权限映射。
	PermissionRules() []RoutePermRule
}

// RoutePermRule 插件路由的权限映射规则，字段语义与 middleware 静态规则一致：
// path 精确匹配（与 prefix 互斥）、prefix+suffix 组合、prefix 兜底，
// 三阶段内均为「更长前缀优先」。
type RoutePermRule struct {
	Method string // HTTP 方法（大小写不敏感，登记时归一化为大写），如 "GET"
	Path   string // 精确路径匹配（与 Prefix 互斥），如 "/api/admin/plugins/reqguard/ai/test"
	Prefix string // 路径前缀匹配，如 "/api/admin/plugins/reqguard/"
	Suffix string // 路径后缀匹配（配合 Prefix），如 "/reports"
	Perm   string // 所需权限点，必须在 PermissionGroups 中声明
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
	Name        string                    // 任务名（全局唯一，注册时自动加 plugin_ 前缀）
	DisplayName string                    // 中文展示名，供管理后台列表展示
	CronExpr    string                    // cron 表达式
	Handler     func(ctx context.Context) // 任务处理函数
	Singleton   bool                      // 是否单例执行
}

// ConfigFieldDef 配置字段定义。
type ConfigFieldDef struct {
	Key         string      // 配置键，如 "smtp_host"
	Label       string      // 显示名
	Type        string      // 类型：string/int/bool/select/json/text（text=长文本多行输入）
	Default     interface{} // 默认值
	Options     []string    // select 类型的选项
	Required    bool        // 是否必填
	Description string      // 说明（展示在输入框下方）
	Encrypted   bool        // 敏感字段：存储时 AES-256-GCM 加密，管理界面读取时掩码显示；更新时空值/掩码值表示保持现值
	Group       string      // 分组名：配置界面按组渲染小节；空值不分组（平铺）
	HalfWidth   bool        // 半宽布局：与相邻半宽字段同行；不声明时按类型默认（string/json/text 整行，int/bool/select 半宽）
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

	// 缓存/推理明细（上游未返回时为 0）。PromptTokens 始终为「含缓存总输入」，
	// cache 各项是其子集明细；CacheIncludedInPrompt 标记原始口径（false=Claude 排除口径）
	CacheReadTokens       int  `json:"cache_read_tokens,omitempty"`     // 缓存命中读取
	CacheCreationTokens   int  `json:"cache_creation_tokens,omitempty"` // 写入缓存（Claude cache_creation / OpenAI cache_write）
	ReasoningTokens       int  `json:"reasoning_tokens,omitempty"`      // 推理 token（CompletionTokens 子集）
	CacheIncludedInPrompt bool `json:"-"`                               // 原始响应的 PromptTokens 是否已含缓存（OpenAI/Gemini=true，Claude=false）
}

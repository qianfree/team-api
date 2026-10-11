// Package aireqanalyze AI请求分析插件：基于请求审计日志与会话关联，用独立配置的
// 分析 AI（OpenAI 兼容端点，建议内网自部署）按会话单元分析用户 AI 使用行为的
// 合规风险与数据泄露风险。设计方案见 docs/AI请求分析插件设计.md。
package aireqanalyze

import (
	"context"

	"github.com/gogf/gf/v2/os/glog"

	"github.com/qianfree/team-api/internal/plugin"
)

const (
	// PluginName 插件唯一标识，用于权限点前缀、路由命名空间与 cron 任务名；
	// 表前缀将其连字符归一为下划线（PostgreSQL 标识符不允许连字符）→ plg_ai_req_analyze_
	PluginName = "ai-req-analyze"
	// PluginVersion 当前版本，Upgrade 按此与 sys_plugins.version 的差异做增量迁移
	PluginVersion = "1.0.0"
)

type aiReqAnalyzePlugin struct{}

func init() {
	plugin.Register(&aiReqAnalyzePlugin{})
}

// Info 实现 Plugin 基础接口
func (p *aiReqAnalyzePlugin) Info() plugin.PluginInfo {
	return plugin.PluginInfo{
		Name:        PluginName,
		Label:       "AI请求分析",
		Description: "基于请求审计日志与会话关联，用独立 AI 分析用户 AI 使用行为的合规风险与数据泄露风险",
		Author:      "team-api",
		Version:     PluginVersion,
		Category:    "extension",
	}
}

// Init 实现 Plugin 基础接口。
// 任务执行器在首个任务提交时惰性启动；此处清理上次运行残留的
// pending/running 任务（队列不跨重启，残留任务若无清理会永远挂起），
// 并按配置校正巡查计划（CronJobs 声明的是默认表达式）。
func (p *aiReqAnalyzePlugin) Init(ctx context.Context, app *plugin.App) error {
	markStaleJobsFailed(ctx)
	applyPatrolSchedule(ctx)
	glog.Infof(ctx, "ai-req-analyze plugin initialized")
	return nil
}

// Destroy 实现 Plugin 基础接口：停掉任务执行器并等待在跑任务收尾。
func (p *aiReqAnalyzePlugin) Destroy(ctx context.Context) error {
	stopJobRunner()
	glog.Infof(ctx, "ai-req-analyze plugin destroyed")
	return nil
}

// ConfigSchema 实现 Configurable 接口。
// ai_api_key 声明为 Encrypted：框架在落库前 AES-256-GCM 加密、
// 管理界面读取时掩码显示、空值/掩码提交表示保持现值。
func (p *aiReqAnalyzePlugin) ConfigSchema() []plugin.ConfigFieldDef {
	return []plugin.ConfigFieldDef{
		// ── 分析 AI 连接 ──
		{
			Key:         "ai_base_url",
			Label:       "分析 AI 端点",
			Type:        "string",
			Default:     "",
			Required:    true,
			Group:       "分析 AI 连接",
			HalfWidth:   true,
			Description: "OpenAI 兼容端点地址（填法取决于右侧「端点模式」）。建议内网自部署，防止分析内容二次泄露",
		},
		{
			Key:         "ai_endpoint_mode",
			Label:       "端点模式",
			Type:        "select",
			Default:     "base",
			Options:     []string{"base", "full"},
			Group:       "分析 AI 连接",
			Description: "base：端点填基地址（如 http://ollama:11434/v1，自动拼接 /chat/completions）；full：端点填完整请求地址（非标准路径用）",
		},
		{
			Key:         "ai_api_key",
			Label:       "分析 AI 密钥",
			Type:        "string",
			Default:     "",
			Encrypted:   true,
			Group:       "分析 AI 连接",
			HalfWidth:   true,
			Description: "端点访问密钥，加密存储；内网无鉴权部署（如 Ollama）可留空",
		},
		{
			Key:         "ai_model",
			Label:       "分析模型",
			Type:        "string",
			Default:     "",
			Required:    true,
			Group:       "分析 AI 连接",
			HalfWidth:   true,
			Description: "用于分析的模型名（如 qwen2.5:32b、gpt-4o-mini）",
		},
		{
			Key:         "ai_timeout_ms",
			Label:       "调用超时（毫秒）",
			Type:        "int",
			Default:     60000,
			Group:       "分析 AI 连接",
			Description: "单次分析调用的超时时间",
		},
		{
			Key:         "ai_max_tokens",
			Label:       "单次输出上限",
			Type:        "int",
			Default:     4096,
			Group:       "分析 AI 连接",
			Description: "单次分析调用的 max_tokens",
		},

		// ── 智能体提示词 ──
		{
			Key:     "agent_prompt",
			Label:   "智能体提示词",
			Type:    "text",
			Default: "",
			Group:   "智能体提示词",
			Description: "自定义分析智能体的角色设定与审计维度（如专属行业合规口径），留空使用内置提示词。" +
				"判定质量要求与 JSON 输出格式由系统自动拼接，无需编写",
		},

		// ── 分析行为 ──
		{
			Key:         "units_per_batch",
			Label:       "每批分析单元数",
			Type:        "int",
			Default:     6,
			Group:       "分析行为",
			Description: "每批送入 AI 的会话/独立单元数。单元含多轮对话，比逐请求的批量小一个量级",
		},
		{
			Key:         "snippet_max_len",
			Label:       "单元截断长度（字符）",
			Type:        "int",
			Default:     4000,
			Group:       "分析行为",
			Description: "单个分析单元送入 AI 的最大长度，超出截断",
		},
		{
			Key:         "custom_rules",
			Label:       "自定义合规规则",
			Type:        "string",
			Default:     "",
			Group:       "分析行为",
			Description: "追加到分析系统提示词的额外规则（如行业监管要求），可选",
		},

		// ── 定时巡查 ──
		{
			Key:         "patrol_enabled",
			Label:       "启用定时巡查",
			Type:        "bool",
			Default:     false,
			Group:       "定时巡查",
			Description: "按计划自动对巡查范围内的租户产出分析报告",
		},
		{
			Key:         "patrol_cron",
			Label:       "巡查计划",
			Type:        "string",
			Default:     "0 3 * * *",
			Group:       "定时巡查",
			Description: "cron 表达式，默认每天凌晨 3 点；修改后需禁用→启用插件生效",
		},
		{
			Key:         "patrol_scope",
			Label:       "巡查范围",
			Type:        "json",
			Default:     `{"tenants":[]}`,
			Group:       "定时巡查",
			Description: "指定租户 ID 列表，空数组表示全部活跃租户",
		},
		{
			Key:         "patrol_window_h",
			Label:       "巡查回看窗口（小时）",
			Type:        "int",
			Default:     24,
			Group:       "定时巡查",
			Description: "每次巡查分析的回看时间窗口",
		},

		// ── 数据保留 ──
		{
			Key:         "retention_days",
			Label:       "报告保留天数",
			Type:        "int",
			Default:     90,
			Group:       "数据保留",
			Description: "报告及发现明细的保留期，超期由清理任务删除",
		},
	}
}

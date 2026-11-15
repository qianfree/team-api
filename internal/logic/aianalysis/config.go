package aianalysis

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/util/gconv"

	"github.com/qianfree/team-api/internal/logic/common"
)

// 配置存 sys_options（ai_analysis_* 键，注册表见 common/settings_registry.go），
// 读取走 ConfigService 双层缓存；密钥项 AES-256-GCM 加密存储，经 GetSecret 解密。

// maxAgentPromptLen 自定义智能体提示词长度上限（字符），防表单滥用撑爆提示词。
const maxAgentPromptLen = 16000

// 端点模式：base = ai_analysis_base_url 为基础地址（自动拼接 /chat/completions）；
// full = ai_analysis_base_url 为完整端点地址，原样请求（应对路径非标准的兼容端点）。
const (
	EndpointModeBase = "base"
	EndpointModeFull = "full"
)

// Config 运行配置（类型化视图，密钥已解密）。
type Config struct {
	AIBaseURL      string // OpenAI 兼容端点（含版本路径）；full 模式下为完整请求地址
	AIEndpointMode string // 端点模式：base（拼接 /chat/completions）/ full（原样请求）
	AIAPIKey       string // 端点密钥（明文，仅内存驻留）
	AIModel        string // 分析模型名
	AITimeoutMs    int    // 单次调用超时
	AIMaxTokens    int    // 单次调用 max_tokens
	UnitsPerBatch  int    // 每批分析单元数
	SnippetMaxLen  int    // 单元截断长度（字符）
	AgentPrompt    string // 自定义智能体提示词（空=内置提示词；替换角色与维度定义段）
	CustomRules    string // 自定义合规规则（追加进系统提示词）

	PatrolEnabled bool    // 定时巡查开关
	PatrolCron    string  // 巡查 cron 表达式
	PatrolScope   []int64 // 巡查租户列表（空=全部活跃租户）
	PatrolWindowH int     // 巡查回看窗口（小时）
	RetentionDays int     // 报告保留天数
}

// patrolScopeJSON ai_analysis_patrol_scope 配置的存储形态。
type patrolScopeJSON struct {
	Tenants []int64 `json:"tenants"`
}

// defaultConfig 与设置注册表的 Default 一致的兜底值。
func defaultConfig() *Config {
	return &Config{
		AIEndpointMode: EndpointModeBase,
		AITimeoutMs:    60000,
		AIMaxTokens:    4096,
		UnitsPerBatch:  6,
		SnippetMaxLen:  4000,
		PatrolCron:     "0 3 * * *",
		PatrolWindowH:  24,
		RetentionDays:  90,
	}
}

// LoadConfig 读取配置并应用默认值（缺失键回落注册表默认值，密钥项已解密）。
func LoadConfig(ctx context.Context) *Config {
	cfg := common.Config()
	m := g.Map{
		"ai_analysis_base_url":        cfg.GetString(ctx, "ai_analysis_base_url"),
		"ai_analysis_endpoint_mode":   cfg.GetString(ctx, "ai_analysis_endpoint_mode"),
		"ai_analysis_api_key":         cfg.GetSecret(ctx, "ai_analysis_api_key"),
		"ai_analysis_model":           cfg.GetString(ctx, "ai_analysis_model"),
		"ai_analysis_timeout_ms":      cfg.GetString(ctx, "ai_analysis_timeout_ms"),
		"ai_analysis_max_tokens":      cfg.GetString(ctx, "ai_analysis_max_tokens"),
		"ai_analysis_agent_prompt":    cfg.GetString(ctx, "ai_analysis_agent_prompt"),
		"ai_analysis_custom_rules":    cfg.GetString(ctx, "ai_analysis_custom_rules"),
		"ai_analysis_units_per_batch": cfg.GetString(ctx, "ai_analysis_units_per_batch"),
		"ai_analysis_snippet_max_len": cfg.GetString(ctx, "ai_analysis_snippet_max_len"),
		"ai_analysis_patrol_enabled":  cfg.GetString(ctx, "ai_analysis_patrol_enabled"),
		"ai_analysis_patrol_cron":     cfg.GetString(ctx, "ai_analysis_patrol_cron"),
		"ai_analysis_patrol_scope":    cfg.GetString(ctx, "ai_analysis_patrol_scope"),
		"ai_analysis_patrol_window_h": cfg.GetString(ctx, "ai_analysis_patrol_window_h"),
		"ai_analysis_retention_days":  cfg.GetString(ctx, "ai_analysis_retention_days"),
	}
	return configFromMap(m)
}

// configFromMap 从配置 map 构造类型化配置（纯函数，测试用）。
// 已在库的键覆盖默认值，缺失的键用默认值；类型不匹配按零值处理并由
// 后续 Validate 兜底（配置来源是管理界面表单，不值得为此引入严格校验链）。
func configFromMap(m g.Map) *Config {
	cfg := defaultConfig()
	if len(m) == 0 {
		return cfg
	}
	cfg.AIBaseURL = gconv.String(m["ai_analysis_base_url"])
	// 端点模式仅接受 base/full，其余值（含空）回落 base——错配 full 会导致请求
	// 打到 base_url 根路径，宁可保守拼接
	switch gconv.String(m["ai_analysis_endpoint_mode"]) {
	case EndpointModeFull:
		cfg.AIEndpointMode = EndpointModeFull
	default:
		cfg.AIEndpointMode = EndpointModeBase
	}
	cfg.AIAPIKey = gconv.String(m["ai_analysis_api_key"])
	cfg.AIModel = gconv.String(m["ai_analysis_model"])
	cfg.AITimeoutMs = intWithMin(gconv.Int(m["ai_analysis_timeout_ms"]), cfg.AITimeoutMs, 1000)
	cfg.AIMaxTokens = intWithMin(gconv.Int(m["ai_analysis_max_tokens"]), cfg.AIMaxTokens, 64)
	cfg.UnitsPerBatch = intWithMin(gconv.Int(m["ai_analysis_units_per_batch"]), cfg.UnitsPerBatch, 1)
	cfg.SnippetMaxLen = intWithMin(gconv.Int(m["ai_analysis_snippet_max_len"]), cfg.SnippetMaxLen, 200)
	cfg.CustomRules = gconv.String(m["ai_analysis_custom_rules"])
	// 自定义提示词长度防御：异常超长（如表单被滥用）截断，正常自定义远用不到上限
	cfg.AgentPrompt = truncateRunes(strings.TrimSpace(gconv.String(m["ai_analysis_agent_prompt"])), maxAgentPromptLen)

	cfg.PatrolEnabled = gconv.Bool(m["ai_analysis_patrol_enabled"])
	cfg.PatrolCron = nonEmptyString(gconv.String(m["ai_analysis_patrol_cron"]), cfg.PatrolCron)
	cfg.PatrolWindowH = intWithMin(gconv.Int(m["ai_analysis_patrol_window_h"]), cfg.PatrolWindowH, 1)
	cfg.RetentionDays = intWithMin(gconv.Int(m["ai_analysis_retention_days"]), cfg.RetentionDays, 1)
	if raw := gconv.String(m["ai_analysis_patrol_scope"]); raw != "" {
		var scope patrolScopeJSON
		if err := json.Unmarshal([]byte(raw), &scope); err == nil {
			cfg.PatrolScope = scope.Tenants
		}
	}
	return cfg
}

// Validate 校验发起分析/连通性测试所需的最低配置。
func (c *Config) Validate() error {
	if c.AIBaseURL == "" {
		return gerror.New("未配置分析 AI 端点，请先在「AI请求分析」的配置中填写")
	}
	if c.AIModel == "" {
		return gerror.New("未配置分析模型，请先在「AI请求分析」的配置中填写")
	}
	return nil
}

// intWithMin 配置值小于下限时回落默认值（零值/负数视为未配置）。
func intWithMin(v, def, min int) int {
	if v < min {
		return def
	}
	return v
}

func nonEmptyString(v, def string) string {
	if v == "" {
		return def
	}
	return v
}

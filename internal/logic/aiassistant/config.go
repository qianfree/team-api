// Package aiassistant 管理后台全局 AI 助手：独立配置的 OpenAI 兼容客户端
// （沿用 aianalysis「独立 AI」架构决策：不经渠道调度、不计费、不进租户审计），
// 加上两个领域技能（计费表达式 / JS 插件代码生成）的知识注入与本地校验闭环。
package aiassistant

import (
	"context"
	"strings"

	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/util/gconv"

	"github.com/qianfree/team-api/internal/logic/common"
)

// 配置存 sys_options（ai_assistant_* 键，注册表见 common/settings_registry.go），
// 读取走 ConfigService 双层缓存；密钥项 AES-256-GCM 加密存储，经 GetSecret 解密。

// 端点模式：base = ai_assistant_base_url 为基础地址（自动拼接 /chat/completions）；
// full = ai_assistant_base_url 为完整端点地址，原样请求（应对路径非标准的兼容端点）。
const (
	EndpointModeBase = "base"
	EndpointModeFull = "full"
)

// 输入防御上限（chat 请求校验用）。
const (
	MaxMessageBytes = 32 << 10 // 单条消息 32KB 上限
	MaxMessages     = 200      // 单次请求携带消息数硬上限
	MaxSystemPrompt = 16000    // 自定义系统提示词长度上限（字符）
)

// Config 运行配置（类型化视图，密钥已解密）。
type Config struct {
	Enabled         bool
	BaseURL         string  // OpenAI 兼容端点（含版本路径）；full 模式下为完整请求地址
	EndpointMode    string  // 端点模式：base / full
	APIKey          string  // 端点密钥（明文，仅内存驻留）
	Model           string  // 对话模型名
	SystemPrompt    string  // 自定义系统提示词（空=内置默认）
	MaxTokens       int     // 单次回复 max_tokens
	Temperature     float64 // 采样温度
	TimeoutMs       int     // 单次调用总超时
	HistoryLimit    int     // 每次请求携带的历史消息条数上限（后端截断）
	RateLimitPerMin int     // 每管理员每分钟调用上限（0=不限制）
}

// defaultConfig 与设置注册表的 Default 一致的兜底值。
func defaultConfig() *Config {
	return &Config{
		EndpointMode:    EndpointModeBase,
		MaxTokens:       4096,
		Temperature:     0.7,
		TimeoutMs:       120000,
		HistoryLimit:    20,
		RateLimitPerMin: 30,
	}
}

// LoadConfig 读取配置并应用默认值（缺失键回落注册表默认值，密钥项已解密）。
func LoadConfig(ctx context.Context) *Config {
	cfg := common.Config()
	m := g.Map{
		"ai_assistant_enabled":            cfg.GetString(ctx, "ai_assistant_enabled"),
		"ai_assistant_base_url":           cfg.GetString(ctx, "ai_assistant_base_url"),
		"ai_assistant_endpoint_mode":      cfg.GetString(ctx, "ai_assistant_endpoint_mode"),
		"ai_assistant_api_key":            cfg.GetSecret(ctx, "ai_assistant_api_key"),
		"ai_assistant_model":              cfg.GetString(ctx, "ai_assistant_model"),
		"ai_assistant_system_prompt":      cfg.GetString(ctx, "ai_assistant_system_prompt"),
		"ai_assistant_max_tokens":         cfg.GetString(ctx, "ai_assistant_max_tokens"),
		"ai_assistant_temperature":        cfg.GetString(ctx, "ai_assistant_temperature"),
		"ai_assistant_timeout_ms":         cfg.GetString(ctx, "ai_assistant_timeout_ms"),
		"ai_assistant_history_limit":      cfg.GetString(ctx, "ai_assistant_history_limit"),
		"ai_assistant_rate_limit_per_min": cfg.GetString(ctx, "ai_assistant_rate_limit_per_min"),
	}
	return configFromMap(m)
}

// configFromMap 从配置 map 构造类型化配置（纯函数，测试用）。
// 零值/非法值回落默认值，由 Validate 兜底必填项。
func configFromMap(m g.Map) *Config {
	cfg := defaultConfig()
	if len(m) == 0 {
		return cfg
	}
	cfg.Enabled = gconv.Bool(m["ai_assistant_enabled"])
	cfg.BaseURL = strings.TrimSpace(gconv.String(m["ai_assistant_base_url"]))
	// 端点模式仅接受 base/full，其余值（含空）回落 base——错配 full 会导致请求
	// 打到 base_url 根路径，宁可保守拼接（与 aianalysis 同口径）
	switch gconv.String(m["ai_assistant_endpoint_mode"]) {
	case EndpointModeFull:
		cfg.EndpointMode = EndpointModeFull
	default:
		cfg.EndpointMode = EndpointModeBase
	}
	cfg.APIKey = gconv.String(m["ai_assistant_api_key"])
	cfg.Model = strings.TrimSpace(gconv.String(m["ai_assistant_model"]))
	cfg.SystemPrompt = truncateRunes(strings.TrimSpace(gconv.String(m["ai_assistant_system_prompt"])), MaxSystemPrompt)
	cfg.MaxTokens = intWithMin(gconv.Int(m["ai_assistant_max_tokens"]), cfg.MaxTokens, 64)
	cfg.Temperature = gconv.Float64(m["ai_assistant_temperature"])
	if cfg.Temperature < 0 {
		cfg.Temperature = defaultConfig().Temperature
	}
	cfg.TimeoutMs = intWithMin(gconv.Int(m["ai_assistant_timeout_ms"]), cfg.TimeoutMs, 1000)
	cfg.HistoryLimit = intWithMin(gconv.Int(m["ai_assistant_history_limit"]), cfg.HistoryLimit, 2)
	cfg.RateLimitPerMin = intWithMin(gconv.Int(m["ai_assistant_rate_limit_per_min"]), 0, 0)
	return cfg
}

// Validate 校验对话所需的最低配置。
func (c *Config) Validate() error {
	if c.BaseURL == "" {
		return gerror.New("未配置 AI 助手端点，请先在系统设置「AI 助手」中填写")
	}
	if c.Model == "" {
		return gerror.New("未配置 AI 助手模型，请先在系统设置「AI 助手」中填写")
	}
	return nil
}

// intWithMin 配置值小于下限时回落默认值（负数视为未配置）。
func intWithMin(v, def, min int) int {
	if v < min {
		return def
	}
	return v
}

// truncateRunes 按字符截断（避免中文乱码）。
func truncateRunes(s string, max int) string {
	if max <= 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max])
}

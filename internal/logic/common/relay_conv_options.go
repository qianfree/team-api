package common

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/qianfree/team-api/relaykit/relayconvert/convmeta"
)

// RelayConvOptionsProvider 从 sys_options（经 Config() 双层缓存）组装转换选项快照，
// 供 relay/common.SetConvOptionsProvider 在进程启动时注入（cmd.go 接线）。
// 默认值与 settings_registry 中各键的 Default 一致，配置缺失时行为与硬编码时代相同。
//
// 函数型钩子（DefaultMaxTokens / SupportsImagine）不在此填——由
// RelayInfo.buildConvOptions 按包内默认补齐（模型能力口径不属于运营配置）。
func RelayConvOptionsProvider(ctx context.Context) *convmeta.Options {
	cfg := Config()
	opts := &convmeta.Options{
		Claude: convmeta.ClaudeOptions{
			ThinkingAdapterEnabled:                cfg.GetBool(ctx, "relay_claude_thinking_adapter_enabled"),
			ThinkingAdapterBudgetTokensPercentage: cfg.GetFloat(ctx, "relay_claude_thinking_budget_percentage"),
		},
		Gemini: convmeta.GeminiOptions{
			ThinkingAdapterEnabled:                cfg.GetBool(ctx, "relay_gemini_thinking_adapter_enabled"),
			ThinkingAdapterBudgetTokensPercentage: cfg.GetFloat(ctx, "relay_gemini_thinking_budget_percentage"),
			FunctionCallThoughtSignatureEnabled:   cfg.GetBool(ctx, "relay_gemini_thought_signature_enabled"),
			SafetySetting: parseGeminiSafetySetting(ctx, cfg.GetString(ctx, "relay_gemini_safety_setting")),
		},
		PreserveThinkingSuffix: parsePreserveThinkingSuffix(cfg.GetString(ctx, "relay_preserve_thinking_suffix_models")),
	}
	return opts
}

// parseGeminiSafetySetting 解析 relay_gemini_safety_setting（JSON：类别 → 伤害阈值）。
// 空配置返回 nil（不附带 safetySettings，保持历史行为）；畸形 JSON 记 Warning 亦返回 nil，
// 不让配置错误阻断转换链路。
func parseGeminiSafetySetting(ctx context.Context, raw string) func(category string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	var m map[string]string
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		g.Log().Warningf(ctx, "[Config] relay_gemini_safety_setting 不是合法的 JSON 映射，忽略: %v", err)
		return nil
	}
	if len(m) == 0 {
		return nil
	}
	return func(category string) string { return m[category] }
}

// parsePreserveThinkingSuffix 解析 relay_preserve_thinking_suffix_models（逗号分隔模型名，
// 支持尾部 * 前缀匹配）。空配置返回 nil（对所有模型剥离 thinking 后缀，历史行为）。
func parsePreserveThinkingSuffix(raw string) func(modelName string) bool {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	var exact, prefixes []string
	for _, name := range strings.Split(raw, ",") {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		if p, ok := strings.CutSuffix(name, "*"); ok {
			if p != "" {
				prefixes = append(prefixes, p)
			}
			continue
		}
		exact = append(exact, name)
	}
	if len(exact) == 0 && len(prefixes) == 0 {
		return nil
	}
	return func(modelName string) bool {
		for _, name := range exact {
			if modelName == name {
				return true
			}
		}
		for _, p := range prefixes {
			if strings.HasPrefix(modelName, p) {
				return true
			}
		}
		return false
	}
}

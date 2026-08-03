package shared

import (
	"strings"

	"github.com/qianfree/team-api/relaykit/dto"
	"github.com/qianfree/team-api/relaykit/relayconvert/convmeta"
	"github.com/qianfree/team-api/relaykit/relayconvert/reasoning"
)

// ThinkingInfo 保存从模型名后缀解析得到的 thinking 配置。
type ThinkingInfo struct {
	BaseModel    string // 不带后缀的模型名
	IsThinking   bool   // 带 -thinking 后缀
	IsNoThinking bool   // 带 -nothinking 后缀
	EffortLevel  string // low/medium/high/xhigh/max/minimal/none
}

// ParseThinkingSuffix 从模型名中解析 thinking 相关后缀。
// 处理：-thinking、-nothinking、-low、-medium、-high、-xhigh、-max、-minimal、-none
func ParseThinkingSuffix(modelName string) ThinkingInfo {
	info := ThinkingInfo{
		BaseModel: modelName,
	}

	// 检查 -thinking 后缀
	if strings.HasSuffix(modelName, "-thinking") {
		info.IsThinking = true
		info.BaseModel = strings.TrimSuffix(modelName, "-thinking")
		return info
	}

	// 检查 -nothinking 后缀
	if strings.HasSuffix(modelName, "-nothinking") {
		info.IsNoThinking = true
		info.BaseModel = strings.TrimSuffix(modelName, "-nothinking")
		return info
	}

	// 检查 effort 等级后缀
	baseModel, effort, found := reasoning.TrimEffortSuffix(modelName)
	if found {
		info.BaseModel = baseModel
		info.EffortLevel = effort
	}

	return info
}

// ApplyThinkingToClaude 将 thinking 配置应用到 Claude 请求。
func ApplyThinkingToClaude(req *dto.ClaudeRequest, info ThinkingInfo, opts convmeta.ClaudeOptions) {
	if !opts.ThinkingAdapterEnabled {
		return
	}

	// 显式禁用 thinking
	if info.IsNoThinking {
		return
	}

	// 启用 thinking
	if info.IsThinking || info.EffortLevel != "" {
		req.Thinking = &dto.ClaudeThinking{
			Type: "enabled",
		}

		// 若已配置则设置 budget tokens
		if req.MaxTokens != nil && opts.ThinkingAdapterBudgetTokensPercentage > 0 {
			budgetTokens := int(float64(*req.MaxTokens) * opts.ThinkingAdapterBudgetTokensPercentage)
			if budgetTokens > 0 {
				req.Thinking.BudgetTokens = &budgetTokens
			}
		}
	}
}

// ApplyThinkingToGemini 将 thinking 配置应用到 Gemini 请求。
func ApplyThinkingToGemini(config *dto.GeminiGenerationConfig, info ThinkingInfo, opts convmeta.GeminiOptions) {
	if !opts.ThinkingAdapterEnabled {
		return
	}

	// 显式禁用 thinking
	if info.IsNoThinking {
		return
	}

	// 启用 thinking
	if info.IsThinking || info.EffortLevel != "" {
		thinkingConfig := &dto.GeminiThinkingConfig{
			IncludeThoughts: true,
		}

		// 将 effort 等级映射为 thinking budget
		if config.MaxOutputTokens != nil && opts.ThinkingAdapterBudgetTokensPercentage > 0 {
			thinkingBudget := int(float64(*config.MaxOutputTokens) * opts.ThinkingAdapterBudgetTokensPercentage)
			if thinkingBudget > 0 {
				thinkingConfig.ThoughtBudget = &thinkingBudget
			}
		}

		config.ThinkingConfig = thinkingConfig
	}
}

// ShouldPreserveThinkingSuffix 检查是否应在上游模型名上保留 thinking 后缀。
func ShouldPreserveThinkingSuffix(modelName string, opts *convmeta.Options) bool {
	if opts == nil {
		return false
	}
	return opts.ShouldPreserveThinkingSuffix(modelName)
}

package shared

import (
	"strings"

	"github.com/qianfree/team-api/relaykit/dto"
	"github.com/qianfree/team-api/relaykit/relayconvert/convmeta"
	"github.com/qianfree/team-api/relaykit/relayconvert/reasoning"
)

// ThinkingInfo contains parsed thinking configuration from model name suffix.
type ThinkingInfo struct {
	BaseModel    string // Model name without suffix
	IsThinking   bool   // Has -thinking suffix
	IsNoThinking bool   // Has -nothinking suffix
	EffortLevel  string // low/medium/high/xhigh/max/minimal/none
}

// ParseThinkingSuffix parses thinking-related suffixes from model name.
// Handles: -thinking, -nothinking, -low, -medium, -high, -xhigh, -max, -minimal, -none
func ParseThinkingSuffix(modelName string) ThinkingInfo {
	info := ThinkingInfo{
		BaseModel: modelName,
	}

	// Check for -thinking suffix
	if strings.HasSuffix(modelName, "-thinking") {
		info.IsThinking = true
		info.BaseModel = strings.TrimSuffix(modelName, "-thinking")
		return info
	}

	// Check for -nothinking suffix
	if strings.HasSuffix(modelName, "-nothinking") {
		info.IsNoThinking = true
		info.BaseModel = strings.TrimSuffix(modelName, "-nothinking")
		return info
	}

	// Check for effort level suffixes
	baseModel, effort, found := reasoning.TrimEffortSuffix(modelName)
	if found {
		info.BaseModel = baseModel
		info.EffortLevel = effort
	}

	return info
}

// ApplyThinkingToClaude applies thinking configuration to Claude request.
func ApplyThinkingToClaude(req *dto.ClaudeRequest, info ThinkingInfo, opts convmeta.ClaudeOptions) {
	if !opts.ThinkingAdapterEnabled {
		return
	}

	// Disable thinking explicitly
	if info.IsNoThinking {
		return
	}

	// Enable thinking
	if info.IsThinking || info.EffortLevel != "" {
		req.Thinking = &dto.ClaudeThinking{
			Type: "enabled",
		}

		// Set budget tokens if configured
		if req.MaxTokens != nil && opts.ThinkingAdapterBudgetTokensPercentage > 0 {
			budgetTokens := int(float64(*req.MaxTokens) * opts.ThinkingAdapterBudgetTokensPercentage)
			if budgetTokens > 0 {
				req.Thinking.BudgetTokens = &budgetTokens
			}
		}
	}
}

// ApplyThinkingToGemini applies thinking configuration to Gemini request.
func ApplyThinkingToGemini(config *dto.GeminiGenerationConfig, info ThinkingInfo, opts convmeta.GeminiOptions) {
	if !opts.ThinkingAdapterEnabled {
		return
	}

	// Disable thinking explicitly
	if info.IsNoThinking {
		return
	}

	// Enable thinking
	if info.IsThinking || info.EffortLevel != "" {
		thinkingConfig := &dto.GeminiThinkingConfig{
			IncludeThoughts: true,
		}

		// Map effort level to thinking budget
		if config.MaxOutputTokens != nil && opts.ThinkingAdapterBudgetTokensPercentage > 0 {
			thinkingBudget := int(float64(*config.MaxOutputTokens) * opts.ThinkingAdapterBudgetTokensPercentage)
			if thinkingBudget > 0 {
				thinkingConfig.ThoughtBudget = &thinkingBudget
			}
		}

		config.ThinkingConfig = thinkingConfig
	}
}

// ShouldPreserveThinkingSuffix checks if thinking suffix should be kept on upstream model name.
func ShouldPreserveThinkingSuffix(modelName string, opts *convmeta.Options) bool {
	if opts == nil {
		return false
	}
	return opts.ShouldPreserveThinkingSuffix(modelName)
}

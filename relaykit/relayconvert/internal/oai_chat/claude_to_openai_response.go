package oai_chat

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/qianfree/team-api/relaykit/dto"
	"github.com/qianfree/team-api/relaykit/relayconvert"
	"github.com/qianfree/team-api/relaykit/relayconvert/convmeta"
	"github.com/qianfree/team-api/relaykit/relayconvert/internal/shared"
	"github.com/qianfree/team-api/relaykit/types"
)

// ClaudeToOpenAIResponseConverter 将 Claude Messages API 响应转换为 OpenAI Chat Completions 响应。
type ClaudeToOpenAIResponseConverter struct{}

func (c *ClaudeToOpenAIResponseConverter) ID() string {
	return relayconvert.ConverterClaudeMessagesToOpenAIChat
}

func (c *ClaudeToOpenAIResponseConverter) From() types.RelayFormat {
	return types.RelayFormatClaude
}

func (c *ClaudeToOpenAIResponseConverter) To() types.RelayFormat {
	return types.RelayFormatOpenAI
}

func (c *ClaudeToOpenAIResponseConverter) Quality() relayconvert.ResponseConverterQuality {
	return relayconvert.ResponseConverterQualityGood
}

func (c *ClaudeToOpenAIResponseConverter) ConvertResponse(
	ctx context.Context,
	info convmeta.Meta,
	response any,
) (any, error) {
	claudeResp, ok := response.(*dto.ClaudeResponse)
	if !ok {
		return nil, fmt.Errorf("expected *dto.ClaudeResponse, got %T", response)
	}

	// 确定模型名
	modelName := claudeResp.Model
	if modelName == "" && info != nil {
		modelName = info.GetOriginModelName()
	}

	openaiResp := &dto.ChatCompletionResponse{
		ID:      claudeResp.ID,
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   modelName,
		Choices: []dto.Choice{{
			Index:        0,
			FinishReason: mapClaudeStopReasonToOpenAI(claudeResp.StopReason),
		}},
	}

	// 将 content blocks 转换为 OpenAI message
	message := convertClaudeContentToMessage(claudeResp.Content)
	openaiResp.Choices[0].Message = message

	// 转换 usage。Claude 的 input_tokens 不含缓存（三项并列），OpenAI 的 prompt_tokens
	// 含缓存（cached 是其子集），转换必须做加法：prompt = input + cache_read + cache_creation，
	// 否则缓存场景下客户端按 OpenAI 语义解析会少算输入量（甚至出现 cached > prompt 的矛盾）
	if claudeResp.Usage != nil {
		promptTotal := claudeResp.Usage.InputTokens +
			claudeResp.Usage.CacheReadInputTokens +
			claudeResp.Usage.CacheCreationInputTokens
		openaiResp.Usage = dto.UsageWithDetails{
			PromptTokens:     promptTotal,
			CompletionTokens: claudeResp.Usage.OutputTokens,
			TotalTokens:      promptTotal + claudeResp.Usage.OutputTokens,
		}
		openaiResp.Usage.PromptTokensDetails = claudeCacheTokenDetails(claudeResp.Usage)
	}

	return openaiResp, nil
}

// claudeCacheTokenDetails 将 ClaudeUsage 的缓存字段映射为 OpenAI prompt_tokens_details 细分。
// OpenAI 协议本身无 5m/1h TTL 概念，这两个字段以扩展键（cached_creation_5m/1h_tokens）透出：
// 宿主计费（resolveTokenCounts 的 TTL 分桶计价）从转换产物捕获用量，细分缺失时 1h 写入
// 会被兜底按 5m 基础价计费（少收 37.5%），因此与总量一并透传，客户端不识别扩展键无副作用。
// 无任何缓存用量时返回 nil（与 OpenAI 官方行为一致：prompt_tokens_details 省略）。
func claudeCacheTokenDetails(u *dto.ClaudeUsage) *dto.TokenDetails {
	if u == nil || (u.CacheReadInputTokens <= 0 && u.CacheCreationInputTokens <= 0) {
		return nil
	}
	td := &dto.TokenDetails{
		CachedTokens:         u.CacheReadInputTokens,
		CachedCreationTokens: u.CacheCreationInputTokens,
	}
	if u.CacheCreation != nil {
		td.CachedCreation5mTokens = u.CacheCreation.Ephemeral5mInputTokens
		td.CachedCreation1hTokens = u.CacheCreation.Ephemeral1hInputTokens
	}
	return td
}

// convertClaudeContentToMessage 将 Claude content blocks 转换为 OpenAI message 格式
func convertClaudeContentToMessage(blocks []dto.ClaudeContentBlock) dto.Message {
	var textParts []string
	var thinkingParts []string
	var toolCalls []dto.ToolCall

	for _, block := range blocks {
		switch block.Type {
		case "text":
			if block.Text != nil {
				textParts = append(textParts, *block.Text)
			}
		case "thinking":
			if block.Thinking != nil {
				thinkingParts = append(thinkingParts, *block.Thinking)
			}
		case "redacted_thinking":
			// 已脱敏的 thinking 在 OpenAI 格式中无对应概念，忽略
		case "tool_use":
			toolCalls = append(toolCalls, shared.MapClaudeToolCallsToOpenAI([]dto.ClaudeContentBlock{block})...)
		}
	}

	message := dto.Message{
		Role: "assistant",
	}

	// 设置 content
	if len(textParts) > 0 {
		message.Content = strings.Join(textParts, "\n")
	}

	// 设置 reasoning content（thinking）
	if len(thinkingParts) > 0 {
		thinking := strings.Join(thinkingParts, "")
		message.ReasoningContent = &thinking
	}

	// 设置 tool calls
	if len(toolCalls) > 0 {
		message.ToolCalls = toolCalls
	}

	return message
}

// mapClaudeStopReasonToOpenAI 将 Claude stop_reason 映射为 OpenAI finish_reason
func mapClaudeStopReasonToOpenAI(stopReason string) string {
	switch stopReason {
	case "end_turn":
		return "stop"
	case "max_tokens":
		return "length"
	case "tool_use":
		return "tool_calls"
	case "stop_sequence":
		return "stop"
	default:
		return "stop"
	}
}

// 转换器在宿主应用的包初始化阶段注册，而非在此 internal 实现包中完成。

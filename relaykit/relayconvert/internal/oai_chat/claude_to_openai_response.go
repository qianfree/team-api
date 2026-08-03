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

	// 转换 usage
	if claudeResp.Usage != nil {
		openaiResp.Usage = dto.UsageWithDetails{
			PromptTokens:     claudeResp.Usage.InputTokens,
			CompletionTokens: claudeResp.Usage.OutputTokens,
			TotalTokens:      claudeResp.Usage.InputTokens + claudeResp.Usage.OutputTokens,
		}
	}

	return openaiResp, nil
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

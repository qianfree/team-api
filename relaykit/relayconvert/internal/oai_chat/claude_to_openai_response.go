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

// ClaudeToOpenAIResponseConverter converts Claude Messages API response to OpenAI Chat Completions response.
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

	// Determine model name
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

	// Convert content blocks to OpenAI message
	message := convertClaudeContentToMessage(claudeResp.Content)
	openaiResp.Choices[0].Message = message

	// Convert usage
	if claudeResp.Usage != nil {
		openaiResp.Usage = dto.UsageWithDetails{
			PromptTokens:     claudeResp.Usage.InputTokens,
			CompletionTokens: claudeResp.Usage.OutputTokens,
			TotalTokens:      claudeResp.Usage.InputTokens + claudeResp.Usage.OutputTokens,
		}
	}

	return openaiResp, nil
}

// convertClaudeContentToMessage converts Claude content blocks to OpenAI message format
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
			// Redacted thinking has no equivalent in OpenAI format, ignore
		case "tool_use":
			toolCalls = append(toolCalls, shared.MapClaudeToolCallsToOpenAI([]dto.ClaudeContentBlock{block})...)
		}
	}

	message := dto.Message{
		Role: "assistant",
	}

	// Set content
	if len(textParts) > 0 {
		message.Content = strings.Join(textParts, "\n")
	}

	// Set reasoning content (thinking)
	if len(thinkingParts) > 0 {
		thinking := strings.Join(thinkingParts, "")
		message.ReasoningContent = &thinking
	}

	// Set tool calls
	if len(toolCalls) > 0 {
		message.ToolCalls = toolCalls
	}

	return message
}

// mapClaudeStopReasonToOpenAI maps Claude stop_reason to OpenAI finish_reason
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

// Converter registration happens at package initialization in the host
// application, not here in the internal implementation package.

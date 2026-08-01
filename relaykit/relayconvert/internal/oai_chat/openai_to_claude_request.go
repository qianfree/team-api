package oai_chat

import (
	"context"
	"fmt"
	"strings"

	"github.com/qianfree/team-api/relaykit/dto"
	"github.com/qianfree/team-api/relaykit/relayconvert"
	"github.com/qianfree/team-api/relaykit/relayconvert/convmeta"
	"github.com/qianfree/team-api/relaykit/relayconvert/internal/shared"
	"github.com/qianfree/team-api/relaykit/types"
)

// OpenAIToClaudeRequestConverter converts OpenAI Chat Completions request to Claude Messages API request.
type OpenAIToClaudeRequestConverter struct{}

func (c *OpenAIToClaudeRequestConverter) ID() string {
	return relayconvert.ConverterOpenAIChatToClaudeMessages
}

func (c *OpenAIToClaudeRequestConverter) From() types.RelayFormat {
	return types.RelayFormatOpenAI
}

func (c *OpenAIToClaudeRequestConverter) To() types.RelayFormat {
	return types.RelayFormatClaude
}

func (c *OpenAIToClaudeRequestConverter) Quality() relayconvert.RequestConverterQuality {
	return relayconvert.RequestConverterQualityFair
}

func (c *OpenAIToClaudeRequestConverter) ConvertRequest(
	ctx context.Context,
	info convmeta.Meta,
	request any,
) (any, error) {
	openaiReq, ok := request.(*dto.GeneralOpenAIRequest)
	if !ok {
		return nil, fmt.Errorf("expected *dto.GeneralOpenAIRequest, got %T", request)
	}

	// Determine upstream model name
	upstreamModel := info.GetUpstreamModelName()
	if upstreamModel == "" {
		upstreamModel = info.GetOriginModelName()
	}

	// Parse thinking suffix and strip it from model name if not preserved
	thinkingInfo := shared.ParseThinkingSuffix(upstreamModel)
	opts := convmeta.OptionsOf(info)
	if !opts.ShouldPreserveThinkingSuffix(upstreamModel) {
		upstreamModel = thinkingInfo.BaseModel
	}

	claudeReq := &dto.ClaudeRequest{
		Model:    upstreamModel,
		Messages: make([]dto.ClaudeMessage, 0),
		Stream:   openaiReq.Stream,
	}

	// MaxTokens (required by Claude API)
	if openaiReq.MaxTokens != nil {
		maxTokens := uint(*openaiReq.MaxTokens)
		claudeReq.MaxTokens = &maxTokens
	} else {
		// Try to get default from options
		if maxTokens, ok := opts.Claude.DefaultMaxTokensFor(upstreamModel); ok {
			mt := uint(maxTokens)
			claudeReq.MaxTokens = &mt
		} else {
			return nil, fmt.Errorf("max_tokens is required for Claude API but not provided and no default available")
		}
	}

	// Temperature / TopP
	claudeReq.Temperature = openaiReq.Temperature
	claudeReq.TopP = openaiReq.TopP

	// TopK (OpenAI doesn't have this, but Claude does)
	// Leave as nil - Claude will use its default

	// Stop sequences
	if openaiReq.Stop != nil {
		switch v := openaiReq.Stop.(type) {
		case string:
			claudeReq.StopSequences = []string{v}
		case []string:
			claudeReq.StopSequences = v
		case []interface{}:
			for _, item := range v {
				if s, ok := item.(string); ok {
					claudeReq.StopSequences = append(claudeReq.StopSequences, s)
				}
			}
		}
	}

	// Convert messages
	systemPrompts := make([]string, 0)
	for _, msg := range openaiReq.Messages {
		if msg.Role == "system" {
			// System messages go to separate System field
			systemPrompts = append(systemPrompts, shared.MapTextContent(msg.Content))
			continue
		}

		claudeMsg := dto.ClaudeMessage{
			Role: msg.Role,
		}

		// Convert content
		switch content := msg.Content.(type) {
		case string:
			text := content
			claudeMsg.Content = []dto.ClaudeContentBlock{{Type: "text", Text: &text}}
		case []dto.ContentPart:
			claudeMsg.Content = shared.MapOpenAIContentPartsToClaude(content)
		default:
			// Try to extract text as fallback
			text := shared.MapTextContent(content)
			claudeMsg.Content = []dto.ClaudeContentBlock{{Type: "text", Text: &text}}
		}

		// Convert tool calls (assistant messages with tool_calls)
		if len(msg.ToolCalls) > 0 {
			toolBlocks := shared.MapOpenAIToolCallsToClaude(msg.ToolCalls)
			if blocks, ok := claudeMsg.Content.([]dto.ClaudeContentBlock); ok {
				claudeMsg.Content = append(blocks, toolBlocks...)
			} else {
				claudeMsg.Content = toolBlocks
			}
		}

		// Convert tool results (tool role messages)
		if msg.Role == "tool" && msg.ToolCallID != "" {
			// Tool result content
			resultText := shared.MapTextContent(msg.Content)
			claudeMsg.Role = "user" // Claude uses "user" role for tool results
			claudeMsg.Content = []dto.ClaudeContentBlock{
				{
					Type:      "tool_result",
					ToolUseID: msg.ToolCallID,
					Content:   resultText,
				},
			}
		}

		claudeReq.Messages = append(claudeReq.Messages, claudeMsg)
	}

	// Set system prompt
	if len(systemPrompts) > 0 {
		claudeReq.System = strings.Join(systemPrompts, "\n\n")
	}

	// Convert tools
	if len(openaiReq.Tools) > 0 {
		claudeReq.Tools = shared.MapOpenAIToolsToClaudeTools(openaiReq.Tools)

		// Convert tool_choice
		if openaiReq.ToolChoice != nil {
			switch tc := openaiReq.ToolChoice.(type) {
			case string:
				if tc == "auto" {
					claudeReq.ToolChoice = map[string]any{"type": "auto"}
				} else if tc == "required" {
					claudeReq.ToolChoice = map[string]any{"type": "any"}
				} else if tc == "none" {
					// Don't set tool_choice, or set to null
					claudeReq.ToolChoice = nil
				}
			case map[string]any:
				// {"type": "function", "function": {"name": "get_weather"}}
				if tc["type"] == "function" {
					if fn, ok := tc["function"].(map[string]any); ok {
						if name, ok := fn["name"].(string); ok {
							claudeReq.ToolChoice = map[string]any{
								"type": "tool",
								"name": name,
							}
						}
					}
				}
			}
		}
	}

	// Apply thinking adapter
	shared.ApplyThinkingToClaude(claudeReq, thinkingInfo, opts.Claude)

	return claudeReq, nil
}

// Converter registration happens at package initialization in the host
// application, not here in the internal implementation package.

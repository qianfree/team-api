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

// OpenAIToClaudeRequestConverter 将 OpenAI Chat Completions 请求转换为 Claude Messages API 请求。
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

	// 确定上游模型名
	upstreamModel := info.GetUpstreamModelName()
	if upstreamModel == "" {
		upstreamModel = info.GetOriginModelName()
	}

	opts := convmeta.OptionsOf(info)

	claudeReq := &dto.ClaudeRequest{
		Model:    upstreamModel,
		Messages: make([]dto.ClaudeMessage, 0),
		Stream:   openaiReq.Stream,
	}

	// MaxTokens（Claude API 必填）
	if openaiReq.MaxTokens != nil {
		maxTokens := uint(*openaiReq.MaxTokens)
		claudeReq.MaxTokens = &maxTokens
	} else {
		// 尝试从 options 中获取默认值
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

	// TopK（OpenAI 没有此字段，但 Claude 有）
	// 保持为 nil，由 Claude 使用其默认值

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

	// 转换 messages
	systemPrompts := make([]string, 0)
	for _, msg := range openaiReq.Messages {
		if msg.Role == "system" {
			// system 消息放入独立的 System 字段
			systemPrompts = append(systemPrompts, shared.MapTextContent(msg.Content))
			continue
		}

		claudeMsg := dto.ClaudeMessage{
			Role: msg.Role,
		}

		// 转换 content：字符串直转文本块；部件列表经 NormalizeContentParts 统一
		// （兼容宿主裸 unmarshal 的 []any 与链式转换构造的 []dto.ContentPart 两种真实形态）
		switch content := msg.Content.(type) {
		case string:
			text := content
			claudeMsg.Content = []dto.ClaudeContentBlock{{Type: "text", Text: &text}}
		default:
			if parts, ok := shared.NormalizeContentParts(content); ok {
				claudeMsg.Content = shared.MapOpenAIContentPartsToClaude(parts)
			} else {
				// 兜底：尝试提取文本
				text := shared.MapTextContent(content)
				claudeMsg.Content = []dto.ClaudeContentBlock{{Type: "text", Text: &text}}
			}
		}

		// 转换 tool calls（带 tool_calls 的 assistant 消息）
		if len(msg.ToolCalls) > 0 {
			toolBlocks := shared.MapOpenAIToolCallsToClaude(msg.ToolCalls)
			if blocks, ok := claudeMsg.Content.([]dto.ClaudeContentBlock); ok {
				claudeMsg.Content = append(blocks, toolBlocks...)
			} else {
				claudeMsg.Content = toolBlocks
			}
		}

		// 转换 tool 结果（role 为 tool 的消息）
		if msg.Role == "tool" && msg.ToolCallID != "" {
			// tool 结果内容
			resultText := shared.MapTextContent(msg.Content)
			claudeMsg.Role = "user" // Claude 用 "user" 角色承载 tool 结果
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

	// 设置 system prompt
	if len(systemPrompts) > 0 {
		claudeReq.System = strings.Join(systemPrompts, "\n\n")
	}

	// 转换 tools
	if len(openaiReq.Tools) > 0 {
		claudeReq.Tools = shared.MapOpenAIToolsToClaudeTools(openaiReq.Tools)
		// 转换 tool_choice
		if openaiReq.ToolChoice != nil {
			switch tc := openaiReq.ToolChoice.(type) {
			case string:
				if tc == "auto" {
					claudeReq.ToolChoice = map[string]any{"type": "auto"}
				} else if tc == "required" {
					claudeReq.ToolChoice = map[string]any{"type": "any"}
				} else if tc == "none" {
					// 不设置 tool_choice，或置为 null
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

	// 服务端联网搜索：chat 的 web_search_options 映射为 Claude 的 web_search 内置工具。
	// 与自定义函数并存（Claude 允许服务端工具与 custom 工具同列），因此独立于上面的
	// tools 分支——客户端可能只要搜索、不带任何函数。
	if spec := shared.DetectWebSearchFromOpenAI(openaiReq.WebSearchOptions); spec != nil {
		claudeReq.Tools = append(claudeReq.Tools, spec.ToClaudeTool())
	}

	// 应用 thinking 适配器

	return claudeReq, nil
}

// 转换器在宿主应用的包初始化阶段注册，而非在此 internal 实现包中完成。

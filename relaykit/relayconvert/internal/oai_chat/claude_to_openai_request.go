package oai_chat

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/qianfree/team-api/relaykit/dto"
	"github.com/qianfree/team-api/relaykit/relayconvert"
	"github.com/qianfree/team-api/relaykit/relayconvert/convmeta"
	"github.com/qianfree/team-api/relaykit/types"
)

// ClaudeToOpenAIRequestConverter 将 Claude Messages API 请求转换为 OpenAI Chat Completions 请求。
type ClaudeToOpenAIRequestConverter struct{}

func (c *ClaudeToOpenAIRequestConverter) ID() string {
	return relayconvert.ConverterClaudeMessagesToOpenAIChat
}

func (c *ClaudeToOpenAIRequestConverter) From() types.RelayFormat {
	return types.RelayFormatClaude
}

func (c *ClaudeToOpenAIRequestConverter) To() types.RelayFormat {
	return types.RelayFormatOpenAI
}

func (c *ClaudeToOpenAIRequestConverter) Quality() relayconvert.RequestConverterQuality {
	return relayconvert.RequestConverterQualityFair
}

func (c *ClaudeToOpenAIRequestConverter) ConvertRequest(
	ctx context.Context,
	info convmeta.Meta,
	request any,
) (any, error) {
	claudeReq, ok := request.(*dto.ClaudeRequest)
	if !ok {
		return nil, fmt.Errorf("expected *dto.ClaudeRequest, got %T", request)
	}

	openaiReq := &dto.GeneralOpenAIRequest{
		Model:    info.GetUpstreamModelName(),
		Messages: make([]dto.Message, 0),
	}

	if claudeReq.MaxTokens != nil {
		v := int(*claudeReq.MaxTokens)
		openaiReq.MaxTokens = &v
	} else {
		v := 4096
		openaiReq.MaxTokens = &v
	}

	openaiReq.Temperature = claudeReq.Temperature
	openaiReq.TopP = claudeReq.TopP
	openaiReq.Stream = claudeReq.Stream

	if claudeReq.Thinking != nil && claudeReq.Thinking.Type == "enabled" {
		openaiReq.ReasoningEffort = c2oConvertThinkingToReasoningEffort(claudeReq.Thinking)
	}

	if len(claudeReq.StopSequences) > 0 {
		openaiReq.Stop = claudeReq.StopSequences
	}

	if len(claudeReq.Tools) > 0 {
		openaiReq.Tools = make([]dto.Tool, 0, len(claudeReq.Tools))
		for _, t := range claudeReq.Tools {
			openaiReq.Tools = append(openaiReq.Tools, dto.Tool{
				Type: "function",
				Function: dto.FunctionDef{
					Name:        t.Name,
					Description: t.Description,
					Parameters:  t.InputSchema,
				},
			})
		}
	}

	if claudeReq.ToolChoice != nil {
		openaiReq.ToolChoice = c2oConvertToolChoice(claudeReq.ToolChoice)
	}

	if claudeReq.System != nil {
		systemText := extractClaudeSystemText(claudeReq.System)
		if systemText != "" {
			openaiReq.Messages = append(openaiReq.Messages, dto.Message{
				Role:    "system",
				Content: systemText,
			})
		}
	}

	for _, msg := range claudeReq.Messages {
		switch msg.Role {
		case "user":
			openaiMsgs := convertClaudeUserMessage(msg)
			openaiReq.Messages = append(openaiReq.Messages, openaiMsgs...)
		case "assistant":
			openaiMsg := convertClaudeAssistantMessage(msg)
			openaiReq.Messages = append(openaiReq.Messages, openaiMsg)
		}
	}

	return openaiReq, nil
}

// extractClaudeSystemText 提取 Claude system 字段中的文本（string 或 text 块数组）。
func extractClaudeSystemText(system any) string {
	switch v := system.(type) {
	case string:
		return v
	case []any:
		var parts []string
		for _, item := range v {
			if m, ok := item.(map[string]any); ok {
				if m["type"] == "text" {
					if text, ok := m["text"].(string); ok {
						parts = append(parts, text)
					}
				}
			}
		}
		return c2oJoinParts(parts)
	default:
		return ""
	}
}

// convertClaudeUserMessage 转换 Claude user 消息。
// tool_result 块会拆分为独立的 tool 角色消息，因此一条 Claude 消息可能产出多条 OpenAI 消息。
func convertClaudeUserMessage(msg dto.ClaudeMessage) []dto.Message {
	var results []dto.Message

	switch v := msg.Content.(type) {
	case string:
		results = append(results, dto.Message{Role: "user", Content: v})
	case []any:
		var contentParts []dto.ContentPart
		for _, item := range v {
			m, ok := item.(map[string]any)
			if !ok {
				continue
			}
			switch m["type"] {
			case "text":
				if text, ok := m["text"].(string); ok {
					contentParts = append(contentParts, dto.ContentPart{Type: "text", Text: text})
				}
			case "tool_result":
				toolUseID, _ := m["tool_use_id"].(string)
				toolContent := ""
				if c, ok := m["content"].(string); ok {
					toolContent = c
				} else if cMap, ok := m["content"].(map[string]any); ok {
					if b, err := json.Marshal(cMap); err == nil {
						toolContent = string(b)
					}
				} else if cArr, ok := m["content"].([]any); ok {
					// 内容块数组（Claude 规范形式，如 [{type:"text",text:"..."}]）：
					// 拼接文本块，否则工具结果会被替换为空字符串导致内容丢失
					var parts []string
					for _, item := range cArr {
						if b, ok := item.(map[string]any); ok && b["type"] == "text" {
							if text, ok := b["text"].(string); ok {
								parts = append(parts, text)
							}
						}
					}
					toolContent = c2oJoinParts(parts)
				}
				if len(contentParts) > 0 {
					if len(contentParts) == 1 && contentParts[0].Type == "text" {
						results = append(results, dto.Message{Role: "user", Content: contentParts[0].Text})
					} else {
						results = append(results, dto.Message{Role: "user", Content: contentParts})
					}
					contentParts = nil
				}
				results = append(results, dto.Message{Role: "tool", Content: toolContent, ToolCallID: toolUseID})
			case "image":
				if source, ok := m["source"].(map[string]any); ok {
					mediaType, _ := source["media_type"].(string)
					data, _ := source["data"].(string)
					url, _ := source["url"].(string)
					if data != "" && mediaType != "" {
						contentParts = append(contentParts, dto.ContentPart{
							Type:     "image_url",
							ImageURL: &dto.ImageURL{URL: fmt.Sprintf("data:%s;base64,%s", mediaType, data), Detail: "auto"},
						})
					} else if url != "" {
						contentParts = append(contentParts, dto.ContentPart{
							Type:     "image_url",
							ImageURL: &dto.ImageURL{URL: url, Detail: "auto"},
						})
					}
				}
			}
		}
		if len(contentParts) > 0 {
			if len(contentParts) == 1 && contentParts[0].Type == "text" {
				results = append(results, dto.Message{Role: "user", Content: contentParts[0].Text})
			} else {
				results = append(results, dto.Message{Role: "user", Content: contentParts})
			}
		}
	default:
		results = append(results, dto.Message{Role: "user", Content: fmt.Sprintf("%v", v)})
	}

	if len(results) == 0 {
		results = append(results, dto.Message{Role: "user", Content: ""})
	}
	return results
}

// convertClaudeAssistantMessage 转换 Claude assistant 消息（文本 / thinking / tool_use 块）。
func convertClaudeAssistantMessage(msg dto.ClaudeMessage) dto.Message {
	result := dto.Message{Role: "assistant"}
	switch v := msg.Content.(type) {
	case string:
		result.Content = v
	case []any:
		var textParts []string
		var toolCalls []dto.ToolCall
		var reasoningParts []string
		for _, item := range v {
			m, ok := item.(map[string]any)
			if !ok {
				continue
			}
			switch m["type"] {
			case "text":
				if text, ok := m["text"].(string); ok {
					textParts = append(textParts, text)
				}
			case "thinking":
				if thinking, ok := m["thinking"].(string); ok && thinking != "" {
					reasoningParts = append(reasoningParts, thinking)
				}
			case "tool_use":
				id, _ := m["id"].(string)
				name, _ := m["name"].(string)
				argsJSON := "{}"
				if input, ok := m["input"]; ok {
					if b, err := json.Marshal(input); err == nil {
						argsJSON = string(b)
					}
				}
				toolCalls = append(toolCalls, dto.ToolCall{
					ID: id, Type: "function",
					Function: dto.FunctionCall{Name: name, Arguments: argsJSON},
				})
			}
		}
		result.Content = c2oJoinParts(textParts)
		if len(reasoningParts) > 0 {
			rc := c2oJoinParts(reasoningParts)
			result.ReasoningContent = &rc
		}
		if len(toolCalls) > 0 {
			result.ToolCalls = toolCalls
		}
	default:
		result.Content = fmt.Sprintf("%v", v)
	}
	return result
}

// c2oJoinParts 用换行符拼接多个文本片段。
func c2oJoinParts(parts []string) string {
	if len(parts) == 0 {
		return ""
	}
	result := make([]byte, 0, 64)
	for i, p := range parts {
		if i > 0 {
			result = append(result, '\n')
		}
		result = append(result, p...)
	}
	return string(result)
}

// c2oConvertThinkingToReasoningEffort 将 Claude thinking budget 映射为 OpenAI reasoning_effort 档位。
func c2oConvertThinkingToReasoningEffort(thinking *dto.ClaudeThinking) string {
	if thinking.BudgetTokens == nil {
		return "medium"
	}
	budget := *thinking.BudgetTokens
	switch {
	case budget <= 2048:
		return "low"
	case budget <= 16384:
		return "medium"
	default:
		return "high"
	}
}

// c2oConvertToolChoice 将 Claude tool_choice 映射为 OpenAI tool_choice。
func c2oConvertToolChoice(toolChoice any) any {
	if toolChoice == nil {
		return nil
	}
	switch v := toolChoice.(type) {
	case string:
		return v
	case map[string]any:
		tcType, _ := v["type"].(string)
		switch tcType {
		case "auto":
			return "auto"
		case "any":
			return "required"
		case "none":
			return "none"
		case "tool":
			name, _ := v["name"].(string)
			if name != "" {
				return map[string]any{"type": "function", "function": map[string]any{"name": name}}
			}
			return "required"
		}
	}
	return toolChoice
}

// 转换器在宿主应用的包初始化阶段注册，而非在此 internal 实现包中完成。

package oai_chat

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/qianfree/team-api/relaykit/dto"
	"github.com/qianfree/team-api/relaykit/relayconvert"
	"github.com/qianfree/team-api/relaykit/relayconvert/convmeta"
	"github.com/qianfree/team-api/relaykit/types"
)

// ClaudeToOpenAIRequestConverter Claude 客户端 → OpenAI Chat 上游（请求侧）。
// 逐行移植宿主 relay/channel/openai/converter.go 的 ConvertClaudeToOpenAI（含全部 legacy
// 怪癖，golden/对拍锁定）。宿主侧接管点在共享函数 ConvertToOpenAI 内部（非 handler 桥接），
// 各 adaptor 的定制后处理照常执行，本转换器无需吸收任何后处理。
//
// legacy 怪癖清单（保持勿改）：
//   - Model 无条件取 UpstreamModelName（不判映射，catalog 保证未映射时即客户端模型名）
//   - MaxTokens 缺省 4096（硬编码）；Temperature/TopP/Stream 指针直传
//   - Metadata/ServiceTier/TopK 静默丢弃（g2o 方向映射 TopK——两侧不对称，各自保持）
//   - system []any 只收 text 块 join "\n"，非 text 块与非 map 项丢弃
//   - user content 非 string/[]any → Sprintf("%v")（nil 产出 "<nil>" 字面量）
//   - tool_result content 三形态：string / map→JSON / 块数组拼 text（其它类型产出空串）
//   - 空转换结果补一条 user 空消息；非 user/assistant 角色整条丢弃
//   - assistant thinking→ReasoningContent（signature 经 ThoughtSignature 透传——
//     Gemini 3 签名往返，2026-08 起不再丢弃）；tool_use input 缺失/marshal 失败→"{}"
//   - tool_choice string 原样透传（含非法 "any"）、{any}→"required"、{tool,name空}→"required"
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
	return relayconvert.RequestConverterQualityGood
}

// ConvertRequest 入参断言 *dto.ClaudeRequest，输出 *dto.GeneralOpenAIRequest
// （gemini→claude 链不走本转换器；gemini→openai→claude 链的第二跳入参契约由此保证）。
func (c *ClaudeToOpenAIRequestConverter) ConvertRequest(
	ctx context.Context, info convmeta.Meta, request any,
) (any, error) {
	req, ok := request.(*dto.ClaudeRequest)
	if !ok {
		return nil, fmt.Errorf("expected *dto.ClaudeRequest, got %T", request)
	}
	return buildOpenAIRequestFromClaude(info, req), nil
}

func buildOpenAIRequestFromClaude(info convmeta.Meta, claudeReq *dto.ClaudeRequest) *dto.GeneralOpenAIRequest {
	openaiReq := &dto.GeneralOpenAIRequest{
		Messages: make([]dto.Message, 0),
	}
	if info != nil {
		openaiReq.Model = info.GetUpstreamModelName()
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
		systemText := c2oExtractSystemText(claudeReq.System)
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
			openaiMsgs := c2oConvertUserMessage(msg)
			openaiReq.Messages = append(openaiReq.Messages, openaiMsgs...)
		case "assistant":
			openaiMsg := c2oConvertAssistantMessage(msg)
			openaiReq.Messages = append(openaiReq.Messages, openaiMsg)
		}
	}
	return openaiReq
}

func c2oExtractSystemText(system any) string {
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

func c2oConvertUserMessage(msg dto.ClaudeMessage) []dto.Message {
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
					// 内容块数组（Claude 规范形式）：拼接文本块，否则工具结果会被
					// 替换为空字符串导致内容丢失
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

func c2oConvertAssistantMessage(msg dto.ClaudeMessage) dto.Message {
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
				// signature 透传（Gemini thoughtSignature 往返载体，取首个非空；
				// 空 thinking 的签名承载块同样捕获——见 o2c 响应侧的补块逻辑）
				if sig, ok := m["signature"].(string); ok && sig != "" && result.ThoughtSignature == "" {
					result.ThoughtSignature = sig
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

func c2oJoinParts(parts []string) string {
	if len(parts) == 0 {
		return ""
	}
	var b strings.Builder
	for i, p := range parts {
		if i > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(p)
	}
	return b.String()
}

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

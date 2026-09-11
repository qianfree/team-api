package oai_gemini

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

// GeminiToOpenAIRequestConverter 将 Gemini Generate Content 请求转换为 OpenAI Chat Completions 请求。
type GeminiToOpenAIRequestConverter struct{}

func (c *GeminiToOpenAIRequestConverter) ID() string {
	return relayconvert.ConverterGeminiContentToOpenAIChat
}

func (c *GeminiToOpenAIRequestConverter) From() types.RelayFormat {
	return types.RelayFormatGemini
}

func (c *GeminiToOpenAIRequestConverter) To() types.RelayFormat {
	return types.RelayFormatOpenAI
}

func (c *GeminiToOpenAIRequestConverter) Quality() relayconvert.RequestConverterQuality {
	return relayconvert.RequestConverterQualityGood
}

// ConvertRequest 将 Gemini 格式请求转换为 OpenAI Chat Completions 格式。
func (c *GeminiToOpenAIRequestConverter) ConvertRequest(
	ctx context.Context,
	info convmeta.Meta,
	request any,
) (any, error) {
	geminiReq, ok := request.(*dto.GeminiChatRequest)
	if !ok {
		return nil, fmt.Errorf("expected *dto.GeminiChatRequest, got %T", request)
	}

	model := ""
	if info != nil {
		model = info.GetUpstreamModelName()
	}

	openaiReq := &dto.GeneralOpenAIRequest{
		Model:    model,
		Messages: make([]dto.Message, 0),
	}

	if geminiReq.GenerationConfig != nil {
		gc := geminiReq.GenerationConfig
		openaiReq.Temperature = gc.Temperature
		openaiReq.TopP = gc.TopP
		if gc.TopK != nil {
			v := int(*gc.TopK)
			openaiReq.TopK = &v
		}
		if gc.MaxOutputTokens != nil {
			v := int(*gc.MaxOutputTokens)
			openaiReq.MaxTokens = &v
		}
		if len(gc.StopSequences) > 0 {
			openaiReq.Stop = gc.StopSequences
		}
		if gc.Seed != nil {
			v := *gc.Seed
			openaiReq.Seed = &v
		}
		if gc.ThinkingConfig != nil {
			openaiReq.ReasoningEffort = g2oConvertThinkingConfig(gc.ThinkingConfig)
		}
		if gc.ResponseMimeType != "" {
			openaiReq.ResponseFormat = &dto.ResponseFormat{Type: "json_object"}
			if gc.ResponseSchema != nil {
				openaiReq.ResponseFormat.Type = "json_schema"
				openaiReq.ResponseFormat.JSONSchema = gc.ResponseSchema
			}
		}
	}

	if len(geminiReq.Tools) > 0 {
		openaiReq.Tools = g2oConvertTools(geminiReq.Tools)
	}
	if geminiReq.ToolConfig != nil {
		openaiReq.ToolChoice = g2oConvertToolConfig(geminiReq.ToolConfig)
	}

	if geminiReq.SystemInstruction != nil && len(geminiReq.SystemInstruction.Parts) > 0 {
		var textParts []string
		for _, p := range geminiReq.SystemInstruction.Parts {
			if p.Text != "" {
				textParts = append(textParts, p.Text)
			}
		}
		if len(textParts) > 0 {
			openaiReq.Messages = append(openaiReq.Messages, dto.Message{Role: "system", Content: strings.Join(textParts, "\n")})
		}
	}

	toolCallIDCounter := 0
	toolCallIDs := make(map[string]string)
	var pendingToolResults []dto.Message

	for _, content := range geminiReq.Contents {
		msgs := g2oConvertContent(content, &toolCallIDCounter, toolCallIDs)
		for i := range msgs {
			if msgs[i].Role == "tool" {
				pendingToolResults = append(pendingToolResults, msgs[i])
			} else {
				if len(pendingToolResults) > 0 {
					openaiReq.Messages = append(openaiReq.Messages, pendingToolResults...)
					pendingToolResults = nil
				}
				openaiReq.Messages = append(openaiReq.Messages, msgs[i])
			}
		}
	}
	if len(pendingToolResults) > 0 {
		openaiReq.Messages = append(openaiReq.Messages, pendingToolResults...)
	}

	return openaiReq, nil
}

// g2oConvertContent 将单条 Gemini Content 转换为一或多条 OpenAI Message。
// 函数响应（functionResponse）拆为独立的 tool 消息返回。
func g2oConvertContent(content dto.GeminiContent, toolCallIDCounter *int, toolCallIDs map[string]string) []dto.Message {
	role := g2oMapRole(content.Role)
	var results []dto.Message
	var textParts []string
	var imageParts []dto.ContentPart
	var funcCalls []dto.ToolCall
	var funcResponses []dto.Message

	for _, part := range content.Parts {
		switch {
		case part.Text != "":
			textParts = append(textParts, part.Text)
		case part.InlineData != nil:
			imageParts = append(imageParts, g2oConvertInlineData(part.InlineData))
		case part.FunctionCall != nil:
			id := fmt.Sprintf("call_%d", *toolCallIDCounter)
			*toolCallIDCounter++
			toolCallIDs[part.FunctionCall.FunctionName] = id
			argsJSON := "{}"
			if part.FunctionCall.Arguments != nil {
				if b, err := json.Marshal(part.FunctionCall.Arguments); err == nil {
					argsJSON = string(b)
				}
			}
			funcCalls = append(funcCalls, dto.ToolCall{ID: id, Type: "function", Function: dto.FunctionCall{Name: part.FunctionCall.FunctionName, Arguments: argsJSON}})
		case part.FunctionResponse != nil:
			name := part.FunctionResponse.Name
			callID, ok := toolCallIDs[name]
			if !ok {
				callID = fmt.Sprintf("call_%d", *toolCallIDCounter)
				*toolCallIDCounter++
				toolCallIDs[name] = callID
			}
			respJSON := ""
			if part.FunctionResponse.Response != nil {
				if b, err := json.Marshal(part.FunctionResponse.Response); err == nil {
					respJSON = string(b)
				}
			}
			funcResponses = append(funcResponses, dto.Message{Role: "tool", Content: respJSON, ToolCallID: callID})
		}
	}

	switch role {
	case "user":
		if len(textParts) == 1 && len(imageParts) == 0 && len(funcResponses) == 0 {
			results = append(results, dto.Message{Role: "user", Content: textParts[0]})
		} else if len(imageParts) > 0 {
			var parts []dto.ContentPart
			for _, t := range textParts {
				parts = append(parts, dto.ContentPart{Type: "text", Text: t})
			}
			parts = append(parts, imageParts...)
			results = append(results, dto.Message{Role: "user", Content: parts})
		} else if len(textParts) > 0 {
			results = append(results, dto.Message{Role: "user", Content: strings.Join(textParts, "\n")})
		}
	case "assistant":
		msg := dto.Message{Role: "assistant"}
		if len(textParts) > 0 {
			msg.Content = strings.Join(textParts, "\n")
		}
		if len(imageParts) > 0 {
			msg.Content = imageParts
		}
		if len(funcCalls) > 0 {
			msg.ToolCalls = funcCalls
			if msg.Content == nil {
				msg.Content = ""
			}
		}
		results = append(results, msg)
	}
	results = append(results, funcResponses...)
	return results
}

// g2oMapRole 将 Gemini 角色映射为 OpenAI 角色。
func g2oMapRole(role string) string {
	switch role {
	case "model":
		return "assistant"
	default:
		return role
	}
}

// g2oConvertInlineData 将 Gemini 内联数据转换为 OpenAI image_url 内容块。
func g2oConvertInlineData(data *dto.GeminiInlineData) dto.ContentPart {
	return dto.ContentPart{
		Type: "image_url",
		ImageURL: &dto.ImageURL{
			URL:    fmt.Sprintf("data:%s;base64,%s", data.MimeType, data.Data),
			Detail: "auto",
		},
	}
}

// g2oConvertTools 将 Gemini tools（functionDeclarations）转换为 OpenAI tools。
func g2oConvertTools(toolsJSON json.RawMessage) []dto.Tool {
	var geminiTools []struct {
		FunctionDeclarations []struct {
			Name        string `json:"name"`
			Description string `json:"description"`
			Parameters  any    `json:"parameters"`
		} `json:"functionDeclarations"`
	}
	if err := json.Unmarshal(toolsJSON, &geminiTools); err != nil {
		return nil
	}
	var result []dto.Tool
	for _, gt := range geminiTools {
		for _, fd := range gt.FunctionDeclarations {
			result = append(result, dto.Tool{
				Type: "function",
				Function: dto.FunctionDef{
					Name: fd.Name, Description: fd.Description, Parameters: fd.Parameters,
				},
			})
		}
	}
	return result
}

// g2oConvertToolConfig 将 Gemini toolConfig 转换为 OpenAI tool_choice。
func g2oConvertToolConfig(toolConfig any) any {
	tcMap, ok := toolConfig.(map[string]any)
	if !ok {
		return nil
	}
	fcc, ok := tcMap["functionCallingConfig"].(map[string]any)
	if !ok {
		return nil
	}
	mode, _ := fcc["mode"].(string)
	switch mode {
	case "NONE":
		return "none"
	case "AUTO":
		return "auto"
	case "ANY":
		if names, ok := fcc["allowedFunctionNames"].([]any); ok && len(names) == 1 {
			if name, ok := names[0].(string); ok {
				return map[string]any{"type": "function", "function": map[string]any{"name": name}}
			}
		}
		return "required"
	default:
		return "auto"
	}
}

// g2oConvertThinkingConfig 将 Gemini thinkingConfig 按预算档位映射为 OpenAI reasoning_effort。
// 未携带 thinkingBudget 时回退读 thinkingLevel（Gemini 3 客户端的表达）。
func g2oConvertThinkingConfig(tc *dto.GeminiThinkingConfig) string {
	if tc.ThinkingBudget == nil {
		// Gemini 3 客户端可能只带 thinkingLevel，按档位回退
		switch strings.ToLower(tc.ThinkingLevel) {
		case "low":
			return "low"
		case "high":
			return "high"
		}
		return "medium"
	}
	budget := *tc.ThinkingBudget
	switch {
	case budget <= 2048:
		return "low"
	case budget <= 16384:
		return "medium"
	default:
		return "high"
	}
}

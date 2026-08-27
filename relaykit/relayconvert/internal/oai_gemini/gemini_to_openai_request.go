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

// GeminiToOpenAIRequestConverter Gemini 客户端 → OpenAI Chat 上游（请求侧）。
// 逐行移植宿主 relay/channel/openai/converter.go 的 ConvertGeminiToOpenAI（含全部 legacy
// 怪癖）。宿主侧接管点在共享函数 ConvertToOpenAI 内部，各 adaptor 后处理照常执行。
// 输出 *dto.GeneralOpenAIRequest——gemini→claude 链第二跳（OpenAIToClaudeRequestConverter）
// 的入参类型契约由此保证。
//
// legacy 怪癖清单（保持勿改）：
//   - MaxOutputTokens 无默认（与 c2o 的 4096 不对称）；TopK 映射（c2o 丢弃——不对称）
//   - ThinkingConfig 优先取 ThinkingLevel（Gemini 3+ 的档位字段），缺失时按
//     ThinkingBudget 折算（≤2048→low / ≤16384→medium / else high，两者皆缺→medium）
//   - ResponseMimeType 任意非空→json_object；有 ResponseSchema→json_schema 且 schema 原样
//   - systemInstruction 只收非空 text（inlineData 丢弃）
//   - functionCall 合成 call_N ID + map[函数名] 反查（同名函数多次调用 ID 复用错配）；
//     functionResponse 无对应 call 时新发 ID
//   - tool 消息 pending 重排：functionResponse 缓存至下一条非 tool 消息前 flush
//   - 未知 role（含空）文本丢失但 functionResponse 仍产出 tool 消息
//   - user 三态：单 text 无图无工具响应→string；有图→parts（text+图）；多 text→join "\n"
//   - assistant 图文互斥图赢；无任何可识别 part 时 Content 保持 nil（有 funcCalls 时补 ""）
//   - inlineData 一律 image_url（不分 mimeType）；FileData/可执行代码 part 整个丢弃
//   - Seed 映射、Stream 不设；tools 解析失败静默 nil；toolConfig 非 map/缺 config→nil
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

// ConvertRequest 入参断言 *dto.GeminiChatRequest，输出 *dto.GeneralOpenAIRequest。
func (c *GeminiToOpenAIRequestConverter) ConvertRequest(
	ctx context.Context, info convmeta.Meta, request any,
) (any, error) {
	req, ok := request.(*dto.GeminiChatRequest)
	if !ok {
		return nil, fmt.Errorf("expected *dto.GeminiChatRequest, got %T", request)
	}
	return buildOpenAIRequestFromGemini(info, req), nil
}

func buildOpenAIRequestFromGemini(info convmeta.Meta, geminiReq *dto.GeminiChatRequest) *dto.GeneralOpenAIRequest {
	openaiReq := &dto.GeneralOpenAIRequest{
		Messages: make([]dto.Message, 0),
	}
	if info != nil {
		openaiReq.Model = info.GetUpstreamModelName()
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
	return openaiReq
}

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

func g2oMapRole(role string) string {
	switch role {
	case "model":
		return "assistant"
	default:
		return role
	}
}

func g2oConvertInlineData(data *dto.GeminiInlineData) dto.ContentPart {
	return dto.ContentPart{
		Type: "image_url",
		ImageURL: &dto.ImageURL{
			URL:    fmt.Sprintf("data:%s;base64,%s", data.MimeType, data.Data),
			Detail: "auto",
		},
	}
}

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

func g2oConvertThinkingConfig(tc *dto.GeminiThinkingConfig) string {
	if lvl := strings.ToLower(strings.TrimSpace(tc.ThinkingLevel)); lvl != "" {
		// Gemini 3+ 用 thinkingLevel 表达强度，直接映射回 chat 的 reasoning_effort
		switch lvl {
		case "low", "medium", "high":
			return lvl
		}
	}
	if tc.ThinkingBudget == nil {
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

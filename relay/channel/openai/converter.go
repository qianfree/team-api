package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gogf/gf/v2/frame/g"

	"github.com/qianfree/team-api/relay/common"
	"github.com/qianfree/team-api/relay/constant"
	"github.com/qianfree/team-api/relay/dto"
	"github.com/qianfree/team-api/relay/helper"
)

// ConvertToOpenAI 根据入站格式将请求转换为 OpenAI 格式。
// 如果入站已是 OpenAI 格式（或空），原样返回。
// 其他供应商适配器可调用此函数统一处理入站格式预转换。
func ConvertToOpenAI(requestBody []byte, info *common.RelayInfo) ([]byte, error) {
	switch info.InboundFormat {
	case constant.RelayFormatClaude:
		r, err := ConvertClaudeToOpenAI(requestBody, info)
		if err != nil {
			return nil, err
		}
		return io.ReadAll(r)
	case constant.RelayFormatGemini:
		r, err := ConvertGeminiToOpenAI(requestBody, info)
		if err != nil {
			return nil, err
		}
		return io.ReadAll(r)
	case constant.RelayFormatResponses:
		r, err := ConvertResponsesToOpenAI(requestBody, info)
		if err != nil {
			return nil, err
		}
		return io.ReadAll(r)
	default:
		return requestBody, nil
	}
}

// ===== Claude → OpenAI 请求转换 =====

// ConvertClaudeToOpenAI 将 Claude 格式请求转换为 OpenAI 格式。
func ConvertClaudeToOpenAI(requestBody []byte, info *common.RelayInfo) (io.Reader, error) {
	var claudeReq dto.ClaudeRequest
	if err := json.Unmarshal(requestBody, &claudeReq); err != nil {
		return nil, fmt.Errorf("parse claude request: %w", err)
	}

	openaiReq := dto.GeneralOpenAIRequest{
		Model:    info.ChannelMeta.UpstreamModelName,
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

	result, err := json.Marshal(openaiReq)
	if err != nil {
		return nil, fmt.Errorf("marshal openai request failed: %w", err)
	}

	return bytes.NewReader(result), nil
}

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

// ===== Gemini → OpenAI 请求转换 =====

// ConvertGeminiToOpenAI 将 Gemini 格式请求转换为 OpenAI Chat Completions 格式。
func ConvertGeminiToOpenAI(requestBody []byte, info *common.RelayInfo) (io.Reader, error) {
	var geminiReq dto.GeminiChatRequest
	if err := json.Unmarshal(requestBody, &geminiReq); err != nil {
		return nil, fmt.Errorf("parse gemini request: %w", err)
	}

	openaiReq := dto.GeneralOpenAIRequest{
		Model:    info.ChannelMeta.UpstreamModelName,
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

	result, err := json.Marshal(openaiReq)
	if err != nil {
		return nil, fmt.Errorf("marshal openai request failed: %w", err)
	}
	return bytes.NewReader(result), nil
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
	if tc.ThoughtBudget == nil {
		return "medium"
	}
	budget := *tc.ThoughtBudget
	switch {
	case budget <= 2048:
		return "low"
	case budget <= 16384:
		return "medium"
	default:
		return "high"
	}
}

// ===== Responses → OpenAI 请求转换 =====

// ConvertResponsesToOpenAI 将 Responses API 请求转换为 Chat Completions 格式
func ConvertResponsesToOpenAI(requestBody []byte, info *common.RelayInfo) (io.Reader, error) {
	var req dto.OpenAIResponsesRequest
	if err := json.Unmarshal(requestBody, &req); err != nil {
		return nil, fmt.Errorf("parse responses request: %w", err)
	}

	// 有状态请求快速失败：previous_response_id 的会话历史存储在上游 Responses 服务侧，
	// chat-only 渠道无法还原（本网关不存储响应体），降级转换会静默丢失全部上下文。
	// 返回哨兵错误，由 relay_handler 按渠道级致命上报调度 FSM 换渠道。
	if req.PreviousResponseID != "" {
		return nil, fmt.Errorf("stateful responses (previous_response_id) not supported by chat-only channels: %w", constant.ErrStatefulResponsesUnsupported)
	}

	// stash 请求快照，供上游 chat 响应合成回 Responses 格式时 echo 请求参数
	if info != nil {
		info.ResponsesRequest = &req
	}

	chatReq := make(map[string]any)
	if info.ChannelMeta.IsModelMapped {
		chatReq["model"] = info.ChannelMeta.UpstreamModelName
	} else {
		chatReq["model"] = req.Model
	}

	messages := make([]map[string]any, 0)
	if len(req.Instructions) > 0 {
		var instructions string
		if err := json.Unmarshal(req.Instructions, &instructions); err == nil && instructions != "" {
			messages = append(messages, map[string]any{"role": "system", "content": instructions})
		}
	}
	inputMessages, err := r2cConvertInputToMessages(req.Input)
	if err != nil {
		return nil, fmt.Errorf("convert input to messages: %w", err)
	}
	messages = append(messages, inputMessages...)
	chatReq["messages"] = messages

	if req.Stream != nil {
		chatReq["stream"] = *req.Stream
		if *req.Stream {
			chatReq["stream_options"] = map[string]any{"include_usage": true}
		}
	}
	if req.Temperature != nil {
		chatReq["temperature"] = *req.Temperature
	}
	if req.TopP != nil {
		chatReq["top_p"] = *req.TopP
	}
	if req.MaxOutputTokens != nil {
		chatReq["max_tokens"] = *req.MaxOutputTokens
	}
	if req.Logprobs != nil {
		chatReq["logprobs"] = true
		chatReq["top_logprobs"] = *req.Logprobs
	} else if req.TopLogProbs != nil {
		chatReq["top_logprobs"] = *req.TopLogProbs
		chatReq["logprobs"] = true
	}
	if len(req.Tools) > 0 {
		chatTools := r2cConvertTools(req.Tools)
		if len(chatTools) > 0 {
			chatReq["tools"] = chatTools
		}
	}
	if len(req.ToolChoice) > 0 {
		chatReq["tool_choice"] = r2cConvertToolChoice(req.ToolChoice)
	}
	if req.Reasoning != nil && req.Reasoning.Effort != "" {
		chatReq["reasoning_effort"] = req.Reasoning.Effort
	}
	if req.ServiceTier != "" {
		chatReq["service_tier"] = req.ServiceTier
	}
	if req.PromptCacheKey != "" {
		chatReq["prompt_cache_key"] = req.PromptCacheKey
	}
	if len(req.Text) > 0 {
		if rf := r2cParseTextFormat(req.Text); rf != nil {
			chatReq["response_format"] = rf
		}
	}
	if len(req.FrequencyPenalty) > 0 {
		var v float64
		if err := json.Unmarshal(req.FrequencyPenalty, &v); err == nil {
			chatReq["frequency_penalty"] = v
		}
	}
	if len(req.PresencePenalty) > 0 {
		var v float64
		if err := json.Unmarshal(req.PresencePenalty, &v); err == nil {
			chatReq["presence_penalty"] = v
		}
	}
	if len(req.Metadata) > 0 {
		chatReq["metadata"] = json.RawMessage(req.Metadata)
	}

	result, err := json.Marshal(chatReq)
	if err != nil {
		return nil, fmt.Errorf("marshal chat completions request failed: %w", err)
	}
	return bytes.NewReader(result), nil
}

type r2cInputItem struct {
	Type    string          `json:"type"`
	Role    string          `json:"role"`
	Content json.RawMessage `json:"content"`
	CallID  string          `json:"call_id,omitempty"`
	Output  string          `json:"output,omitempty"`
	Text    string          `json:"text,omitempty"`
	// function_call 项字段（Responses 历史中的助手工具调用）
	Name      string `json:"name,omitempty"`
	Arguments string `json:"arguments,omitempty"`
}

type r2cContentPart struct {
	Type     string `json:"type"`
	Text     string `json:"text,omitempty"`
	ImageURL string `json:"image_url,omitempty"`
	URL      string `json:"url,omitempty"`
	Detail   string `json:"detail,omitempty"`
	// input_audio：Responses 为 {"type":"input_audio","input_audio":{"data","format"}}，
	// chat 同形，原样透传
	InputAudio *r2cInputAudio `json:"input_audio,omitempty"`
	// input_file：Responses 为扁平 {"type":"input_file","file_data","filename"}，
	// 转换为 chat 的 {"type":"file","file":{"file_data","filename"}}
	FileData string `json:"file_data,omitempty"`
	Filename string `json:"filename,omitempty"`
}

type r2cInputAudio struct {
	Data   string `json:"data,omitempty"`
	Format string `json:"format,omitempty"`
}

func r2cConvertInputToMessages(input json.RawMessage) ([]map[string]any, error) {
	if len(input) == 0 {
		return nil, nil
	}
	var simpleText string
	if err := json.Unmarshal(input, &simpleText); err == nil {
		return []map[string]any{{"role": "user", "content": simpleText}}, nil
	}
	var items []json.RawMessage
	if err := json.Unmarshal(input, &items); err != nil {
		return nil, fmt.Errorf("input must be string or array: %w", err)
	}
	messages := make([]map[string]any, 0, len(items))
	// 连续的 function_call 项聚合为一条 assistant 消息（chat 协议的 tool_calls 数组语义），
	// 其后的 function_call_output 转为引用对应 tool_call_id 的 tool 消息
	var pendingToolCalls []map[string]any
	flushToolCalls := func() {
		if len(pendingToolCalls) == 0 {
			return
		}
		messages = append(messages, map[string]any{"role": "assistant", "content": nil, "tool_calls": pendingToolCalls})
		pendingToolCalls = nil
	}
	for _, raw := range items {
		var item r2cInputItem
		if err := json.Unmarshal(raw, &item); err != nil {
			continue
		}
		switch item.Type {
		case "message":
			flushToolCalls()
			if msg := r2cConvertMessage(item); msg != nil {
				messages = append(messages, msg)
			}
		case "function_call":
			// 历史中的助手工具调用：转为 assistant.tool_calls 条目，id 用 call_id（与 tool 消息的 tool_call_id 对应）
			if item.CallID == "" && item.Name == "" {
				continue
			}
			pendingToolCalls = append(pendingToolCalls, map[string]any{
				"id":   item.CallID,
				"type": "function",
				"function": map[string]any{
					"name":      item.Name,
					"arguments": item.Arguments,
				},
			})
		case "function_call_output":
			flushToolCalls()
			messages = append(messages, map[string]any{"role": "tool", "tool_call_id": item.CallID, "content": item.Output})
		case "reasoning":
			// reasoning 项（含加密思考内容）无 chat 协议对应物，跳过
			continue
		default:
			flushToolCalls()
			if item.Role != "" {
				if msg := r2cConvertMessage(item); msg != nil {
					messages = append(messages, msg)
				}
			}
		}
	}
	flushToolCalls()
	return messages, nil
}

func r2cConvertMessage(item r2cInputItem) map[string]any {
	role := item.Role
	if role == "" {
		role = "user"
	}
	// Responses 的 developer 角色（OpenAI 新式系统提示，codex 等客户端常用）
	// 多数第三方 chat 上游不识别（serde 严格校验直接拒绝），统一映射为 system
	if role == "developer" {
		role = "system"
	}
	if len(item.Content) == 0 {
		return nil
	}
	var textContent string
	if err := json.Unmarshal(item.Content, &textContent); err == nil {
		return map[string]any{"role": role, "content": textContent}
	}
	var parts []r2cContentPart
	if err := json.Unmarshal(item.Content, &parts); err != nil {
		return nil
	}
	chatParts := make([]map[string]any, 0, len(parts))
	for _, part := range parts {
		switch part.Type {
		case "input_text":
			chatParts = append(chatParts, map[string]any{"type": "text", "text": part.Text})
		case "input_audio":
			// 音频输入：chat 的 input_audio 与 Responses 同形（input_audio:{data,format}）
			if part.InputAudio != nil && part.InputAudio.Data != "" {
				audio := map[string]any{"data": part.InputAudio.Data}
				if part.InputAudio.Format != "" {
					audio["format"] = part.InputAudio.Format
				}
				chatParts = append(chatParts, map[string]any{"type": "input_audio", "input_audio": audio})
			}
		case "input_file":
			// 文件输入：Responses 扁平 {file_data,filename} → chat 的 file:{file_data,filename}
			if part.FileData != "" {
				file := map[string]any{"file_data": part.FileData}
				if part.Filename != "" {
					file["filename"] = part.Filename
				}
				chatParts = append(chatParts, map[string]any{"type": "file", "file": file})
			}
		case "input_image":
			imageURL := part.ImageURL
			if imageURL == "" {
				imageURL = part.URL
			}
			if imageURL != "" {
				imgPart := map[string]any{"type": "image_url", "image_url": map[string]any{"url": imageURL}}
				if part.Detail != "" {
					imgPart["image_url"].(map[string]any)["detail"] = part.Detail
				}
				chatParts = append(chatParts, imgPart)
			}
		case "output_text":
			chatParts = append(chatParts, map[string]any{"type": "text", "text": part.Text})
		}
	}
	if len(chatParts) == 0 {
		return nil
	}
	if len(chatParts) == 1 && chatParts[0]["type"] == "text" {
		return map[string]any{"role": role, "content": chatParts[0]["text"]}
	}
	return map[string]any{"role": role, "content": chatParts}
}

func r2cConvertTools(toolsRaw json.RawMessage) []map[string]any {
	var tools []map[string]any
	if err := json.Unmarshal(toolsRaw, &tools); err != nil {
		return nil
	}
	chatTools := make([]map[string]any, 0, len(tools))
	for _, tool := range tools {
		toolType, _ := tool["type"].(string)
		if toolType == "function" {
			chatTools = append(chatTools, map[string]any{
				"type":     "function",
				"function": map[string]any{"name": tool["name"], "description": tool["description"], "parameters": tool["parameters"]},
			})
		}
	}
	return chatTools
}

func r2cConvertToolChoice(tcRaw json.RawMessage) any {
	if len(tcRaw) == 0 {
		return "auto"
	}
	var strVal string
	if err := json.Unmarshal(tcRaw, &strVal); err == nil {
		return strVal
	}
	var tc map[string]any
	if err := json.Unmarshal(tcRaw, &tc); err != nil {
		return "auto"
	}
	if tc["type"] == "function" {
		if fn, ok := tc["function"].(map[string]any); ok {
			if name, ok := fn["name"].(string); ok {
				return map[string]any{"type": "function", "function": map[string]any{"name": name}}
			}
		}
	}
	return tc
}

// r2cParseTextFormat 解析 Responses text.format（扁平 {type,name,schema,strict}）
// 为 chat 的 response_format（json_schema 时嵌套为 json_schema:{name,schema,strict}）。
// text 或未知类型返回 nil（chat 无对应字段，不映射）。
func r2cParseTextFormat(raw json.RawMessage) map[string]any {
	var textCfg struct {
		Format struct {
			Type   string          `json:"type"`
			Name   string          `json:"name"`
			Schema json.RawMessage `json:"schema"`
			Strict *bool           `json:"strict"`
		} `json:"format"`
	}
	if err := json.Unmarshal(raw, &textCfg); err != nil {
		return nil
	}
	switch textCfg.Format.Type {
	case "json_object":
		return map[string]any{"type": "json_object"}
	case "json_schema":
		jsonSchema := make(map[string]any, 3)
		if textCfg.Format.Name != "" {
			jsonSchema["name"] = textCfg.Format.Name
		}
		if len(textCfg.Format.Schema) > 0 {
			var schema any
			if err := json.Unmarshal(textCfg.Format.Schema, &schema); err == nil {
				jsonSchema["schema"] = schema
			}
		}
		if textCfg.Format.Strict != nil {
			jsonSchema["strict"] = *textCfg.Format.Strict
		}
		return map[string]any{"type": "json_schema", "json_schema": jsonSchema}
	default:
		return nil
	}
}

// ===== OpenAI → Responses 请求转换（Responses API Bridge） =====

// ConvertOpenAIToResponses 将 Chat Completions 请求体转换为 Responses API 请求体
func ConvertOpenAIToResponses(body []byte, info *common.RelayInfo) ([]byte, error) {
	var req dto.GeneralOpenAIRequest
	if err := json.Unmarshal(body, &req); err != nil {
		return nil, fmt.Errorf("parse chat completions request: %w", err)
	}

	respReq := make(map[string]any)
	if info.ChannelMeta.IsModelMapped {
		respReq["model"] = info.ChannelMeta.UpstreamModelName
	} else {
		respReq["model"] = req.Model
	}

	if instructions := c2rExtractInstructions(req.Messages); instructions != "" {
		respReq["instructions"] = instructions
	}
	respReq["input"] = c2rConvertMessagesToInput(req.Messages)

	if req.Stream != nil {
		respReq["stream"] = *req.Stream
	}
	if req.Temperature != nil {
		respReq["temperature"] = *req.Temperature
	}
	if req.TopP != nil {
		respReq["top_p"] = *req.TopP
	}
	// prompt_cache_key 是官方 Responses API 参数，原样透传；
	// presence/frequency penalty 不属于官方 Responses API，透传会被严格上游
	// （如 api.openai.com）以未知参数拒绝，chat 入站时不注入、静默丢弃。
	if req.PromptCacheKey != "" {
		respReq["prompt_cache_key"] = req.PromptCacheKey
	}
	if maxTokens := c2rGetMaxTokens(req); maxTokens > 0 {
		respReq["max_output_tokens"] = maxTokens
	}
	if req.ResponseFormat != nil {
		if tf := c2rBuildTextFormat(req.ResponseFormat); tf != nil {
			respReq["text"] = tf
		}
	}
	// 桥接的响应不可被 chat 客户端经 previous_response_id 引用（chat 协议无此概念），
	// 显式 store:false 避免上游无谓存储；渠道配置 DisableStore 时 SanitizeFields 会删掉该字段
	respReq["store"] = false
	if req.ReasoningEffort != "" {
		respReq["reasoning"] = map[string]any{"effort": req.ReasoningEffort, "summary": "detailed"}
	}
	if len(req.Tools) > 0 {
		if respTools := c2rConvertTools(req.Tools); len(respTools) > 0 {
			respReq["tools"] = respTools
		}
	}
	if req.ToolChoice != nil {
		respReq["tool_choice"] = c2rConvertToolChoice(req.ToolChoice)
	}
	if req.User != "" {
		respReq["user"] = req.User
	}
	if req.ParallelToolCalls != nil {
		respReq["parallel_tool_calls"] = *req.ParallelToolCalls
	}

	result, err := json.Marshal(respReq)
	if err != nil {
		return nil, fmt.Errorf("marshal responses request: %w", err)
	}
	return result, nil
}

func c2rExtractInstructions(messages []dto.Message) string {
	var parts []string
	for _, msg := range messages {
		if msg.Role == "system" || msg.Role == "developer" {
			if text, ok := msg.Content.(string); ok && text != "" {
				parts = append(parts, text)
			}
		}
	}
	if len(parts) == 0 {
		return ""
	}
	instructions := parts[0]
	for i := 1; i < len(parts); i++ {
		instructions += "\n\n" + parts[i]
	}
	return instructions
}

func c2rConvertMessagesToInput(messages []dto.Message) []any {
	var input []any
	for _, msg := range messages {
		if msg.Role == "system" || msg.Role == "developer" {
			continue
		}
		switch msg.Role {
		case "user":
			input = append(input, c2rMakeMessageItem("user", msg.Content))
		case "assistant":
			if len(msg.ToolCalls) > 0 {
				if msg.Content != nil {
					if text, ok := msg.Content.(string); ok && text != "" {
						input = append(input, c2rMakeMessageItem("assistant", text))
					}
				}
				for _, tc := range msg.ToolCalls {
					input = append(input, map[string]any{
						"type": "function_call", "call_id": tc.ID, "name": tc.Function.Name, "arguments": tc.Function.Arguments,
					})
				}
			} else {
				input = append(input, c2rMakeMessageItem("assistant", msg.Content))
			}
		case "tool":
			input = append(input, map[string]any{
				"type": "function_call_output", "call_id": msg.ToolCallID, "output": c2rContentToString(msg.Content),
			})
		}
	}
	if len(input) == 0 {
		return []any{}
	}
	return input
}

func c2rMakeMessageItem(role string, content any) map[string]any {
	if content == nil {
		return map[string]any{"type": "message", "role": role, "content": []any{}}
	}
	switch v := content.(type) {
	case string:
		textType := "input_text"
		if role == "assistant" {
			textType = "output_text"
		}
		return map[string]any{"type": "message", "role": role, "content": []any{map[string]any{"type": textType, "text": v}}}
	case []any:
		parts := make([]any, 0, len(v))
		for _, item := range v {
			part, ok := item.(map[string]any)
			if !ok {
				continue
			}
			if converted := c2rConvertContentPart(part, role); converted != nil {
				parts = append(parts, converted)
			}
		}
		return map[string]any{"type": "message", "role": role, "content": parts}
	default:
		b, err := json.Marshal(content)
		if err != nil {
			return map[string]any{"type": "message", "role": role, "content": []any{}}
		}
		var parts []any
		if err := json.Unmarshal(b, &parts); err == nil {
			converted := make([]any, 0, len(parts))
			for _, item := range parts {
				part, ok := item.(map[string]any)
				if !ok {
					continue
				}
				if c := c2rConvertContentPart(part, role); c != nil {
					converted = append(converted, c)
				}
			}
			return map[string]any{"type": "message", "role": role, "content": converted}
		}
		return map[string]any{"type": "message", "role": role, "content": []any{}}
	}
}

func c2rConvertContentPart(part map[string]any, role string) map[string]any {
	partType, _ := part["type"].(string)
	switch partType {
	case "text":
		text, _ := part["text"].(string)
		textType := "input_text"
		if role == "assistant" {
			textType = "output_text"
		}
		return map[string]any{"type": textType, "text": text}
	case "image_url":
		imgURL, _ := part["image_url"].(map[string]any)
		if imgURL != nil {
			result := map[string]any{"type": "input_image"}
			if url, ok := imgURL["url"].(string); ok {
				result["image_url"] = url
			}
			if detail, ok := imgURL["detail"].(string); ok {
				result["detail"] = detail
			}
			return result
		}
	case "input_audio":
		data, _ := part["data"].(string)
		format, _ := part["format"].(string)
		return map[string]any{"type": "input_audio", "data": data, "format": format}
	case "file":
		result := map[string]any{"type": "input_file"}
		if fileData, ok := part["file_data"].(string); ok {
			result["file_data"] = fileData
		}
		if filename, ok := part["filename"].(string); ok {
			result["filename"] = filename
		}
		return result
	}
	return nil
}

func c2rConvertTools(tools []dto.Tool) []any {
	result := make([]any, 0, len(tools))
	for _, tool := range tools {
		if tool.Type == "function" {
			result = append(result, map[string]any{
				"type": "function", "name": tool.Function.Name, "description": tool.Function.Description, "parameters": tool.Function.Parameters,
			})
		}
	}
	return result
}

func c2rConvertToolChoice(toolChoice any) any {
	b, err := json.Marshal(toolChoice)
	if err != nil {
		return toolChoice
	}
	var tc map[string]any
	if err := json.Unmarshal(b, &tc); err != nil {
		return toolChoice
	}
	if tc["type"] == "function" {
		if fn, ok := tc["function"].(map[string]any); ok {
			if name, ok := fn["name"].(string); ok {
				return map[string]any{"type": "function", "name": name}
			}
		}
	}
	return tc
}

func c2rContentToString(content any) string {
	if content == nil {
		return ""
	}
	if s, ok := content.(string); ok {
		return s
	}
	b, _ := json.Marshal(content)
	return string(b)
}

func c2rGetMaxTokens(req dto.GeneralOpenAIRequest) int {
	max := 0
	if req.MaxTokens != nil && *req.MaxTokens > 0 {
		max = *req.MaxTokens
	}
	if req.MaxCompletionTokens != nil && *req.MaxCompletionTokens > max {
		max = *req.MaxCompletionTokens
	}
	return max
}

// c2rBuildTextFormat 将 chat 的 response_format 转换为 Responses 的 text 配置。
// chat 的 json_schema 为嵌套 {type,json_schema:{name,schema,strict}}，
// Responses 的 format 为扁平 {type,name,schema,strict}——需解包提升，不能原样塞入。
// 其余类型（json_object）两侧同形；无法识别时返回 nil 不映射。
func c2rBuildTextFormat(rf *dto.ResponseFormat) map[string]any {
	if rf == nil {
		return nil
	}
	switch rf.Type {
	case "json_object":
		return map[string]any{"format": map[string]any{"type": "json_object"}}
	case "json_schema":
		format := map[string]any{"type": "json_schema"}
		if js, ok := rf.JSONSchema.(map[string]any); ok {
			for _, k := range []string{"name", "schema", "strict"} {
				if v, ok := js[k]; ok {
					format[k] = v
				}
			}
		}
		return map[string]any{"format": format}
	default:
		return nil
	}
}

// ===== Responses → Chat 响应转换 =====

// ResponsesResponseToChatCompletions 将 Responses API 非流式响应转换为 Chat Completions 响应
func ResponsesResponseToChatCompletions(resp *dto.OpenAIResponsesResponse, id string, model string) (*dto.ChatCompletionResponse, *common.Usage, error) {
	text, toolCalls := extractOutputFromResponses(resp)

	finishReason := "stop"
	content := ""
	if len(toolCalls) > 0 && text == "" {
		finishReason = "tool_calls"
	}
	if text != "" || len(toolCalls) == 0 {
		content = text
	}

	usage := &dto.UsageWithDetails{}
	if resp.Usage != nil {
		usage.PromptTokens = resp.Usage.InputTokens
		usage.CompletionTokens = resp.Usage.OutputTokens
		usage.TotalTokens = resp.Usage.TotalTokens
		if usage.TotalTokens == 0 {
			usage.TotalTokens = usage.PromptTokens + usage.CompletionTokens
		}
		if resp.Usage.InputTokensDetails != nil {
			usage.PromptTokensDetails = &dto.TokenDetails{
				CachedTokens: resp.Usage.InputTokensDetails.CachedTokens,
				AudioTokens:  resp.Usage.InputTokensDetails.AudioTokens,
				TextTokens:   resp.Usage.InputTokensDetails.TextTokens,
				ImageTokens:  resp.Usage.InputTokensDetails.ImageTokens,
			}
		}
		if resp.Usage.OutputTokenDetails != nil {
			usage.CompletionTokenDetails = &dto.TokenDetails{
				ReasoningTokens:          resp.Usage.OutputTokenDetails.ReasoningTokens,
				AudioTokens:              resp.Usage.OutputTokenDetails.AudioTokens,
				TextTokens:               resp.Usage.OutputTokenDetails.TextTokens,
				AcceptedPredictionTokens: resp.Usage.OutputTokenDetails.AcceptedPredictionTokens,
				RejectedPredictionTokens: resp.Usage.OutputTokenDetails.RejectedPredictionTokens,
			}
		}
	} else if text != "" {
		estimated := len(text) / 4
		usage.CompletionTokens = estimated
		usage.TotalTokens = usage.PromptTokens + estimated
	}

	chatResp := &dto.ChatCompletionResponse{
		ID: id, Object: "chat.completion", Created: time.Now().Unix(), Model: model,
		Choices: []dto.Choice{{
			Index: 0, Message: dto.Message{Role: "assistant", Content: content, ToolCalls: toolCalls}, FinishReason: finishReason,
		}},
		Usage: *usage,
	}

	return chatResp, &common.Usage{
		PromptTokens:           usage.PromptTokens,
		CompletionTokens:       usage.CompletionTokens,
		TotalTokens:            usage.TotalTokens,
		PromptTokensDetails:    common.DtoTokenDetailsToCommon(usage.PromptTokensDetails),
		CompletionTokenDetails: common.DtoTokenDetailsToCommon(usage.CompletionTokenDetails),
	}, nil
}

func extractOutputFromResponses(resp *dto.OpenAIResponsesResponse) (string, []dto.ToolCall) {
	var textParts []string
	var toolCalls []dto.ToolCall
	for _, output := range resp.Output {
		switch output.Type {
		case "message":
			if output.Role == "assistant" {
				for _, c := range output.Content {
					if c.Type == "output_text" && c.Text != "" {
						textParts = append(textParts, c.Text)
					}
				}
			}
		case "function_call":
			toolCalls = append(toolCalls, dto.ToolCall{
				ID: output.CallID, Type: "function",
				Function: dto.FunctionCall{Name: output.Name, Arguments: output.Arguments},
			})
		}
	}
	return strings.Join(textParts, ""), toolCalls
}

// HandleResponsesStreamToChat 将 Responses API 流式响应实时转换为 Chat Completions SSE 流
func HandleResponsesStreamToChat(ctx context.Context, resp *http.Response, info *common.RelayInfo, writer http.ResponseWriter) (*common.Usage, error) {
	defer resp.Body.Close()

	// 非 200：上游在 SSE 开始前返回错误（非 SSE 体）。必须在 StreamScannerHandler 写出
	// 200 SSE 响应头之前拦截，否则错误体会被当作 SSE 流转发、且无法再向客户端传递状态码。
	if resp.StatusCode != http.StatusOK {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, constant.NewUpstreamError(resp.StatusCode, "read response body failed", err).WithRetryAfter(constant.RetryAfterFromHeader(resp.Header))
		}
		if isUpstreamOpenAIError(body) {
			writeUpstreamErrorResponse(writer, resp.StatusCode, body)
			upstreamErr := constant.NewUpstreamError(resp.StatusCode, string(body), nil).WithRetryAfter(constant.RetryAfterFromHeader(resp.Header))
			upstreamErr.ResponseWritten = true
			return &common.Usage{}, upstreamErr
		}
		return nil, constant.NewUpstreamErrorFromResponse(resp, body)
	}

	responseID := fmt.Sprintf("chatcmpl-%s", info.RequestID)
	createAt := time.Now().Unix()
	model := info.ChannelMeta.UpstreamModelName

	var (
		totalUsage            common.Usage
		usageText, outputText strings.Builder
		sentStart, sentStop   bool
		sawToolCall           bool
	)

	toolCallIndexByID := make(map[string]int)
	toolCallNameByID := make(map[string]string)
	toolCallArgsByID := make(map[string]string)
	toolCallNameSent := make(map[string]bool)

	sendChatChunk := func(chunk *dto.ChatCompletionStreamResponse) bool {
		if chunk == nil {
			return true
		}
		data, err := json.Marshal(chunk)
		if err != nil {
			return false
		}
		return helper.WriteSSEData(writer, string(data)) == nil
	}

	sendStartIfNeeded := func() bool {
		if sentStart {
			return true
		}
		chunk := &dto.ChatCompletionStreamResponse{
			ID: responseID, Object: "chat.completion.chunk", Created: createAt, Model: model,
			Choices: []dto.StreamChoice{{Index: 0, Delta: dto.Message{Role: "assistant", Content: ""}}},
		}
		if !sendChatChunk(chunk) {
			return false
		}
		sentStart = true
		return true
	}

	helper.StreamScannerHandler(ctx, resp, info, writer, func(data string, sr *helper.StreamResult) {
		var streamResp dto.ResponsesStreamResponse
		if err := json.Unmarshal([]byte(data), &streamResp); err != nil {
			sr.Error(fmt.Errorf("unmarshal responses stream: %w", err))
			return
		}

		switch streamResp.Type {
		case "response.created":
			if streamResp.Response != nil {
				if streamResp.Response.Model != "" {
					model = streamResp.Response.Model
				}
				if streamResp.Response.CreatedAt != 0 {
					createAt = int64(streamResp.Response.CreatedAt)
				}
			}

		case "response.reasoning_summary_text.delta":
			if streamResp.Delta == "" {
				return
			}
			if !sendStartIfNeeded() {
				sr.Stop(fmt.Errorf("send start chunk failed"))
				return
			}
			usageText.WriteString(streamResp.Delta)
			delta := streamResp.Delta
			chunk := &dto.ChatCompletionStreamResponse{
				ID: responseID, Object: "chat.completion.chunk", Created: createAt, Model: model,
				Choices: []dto.StreamChoice{{Index: 0, Delta: dto.Message{ReasoningContent: &delta}}},
			}
			if !sendChatChunk(chunk) {
				sr.Stop(fmt.Errorf("send reasoning chunk failed"))
				return
			}

		case "response.output_text.delta":
			if !sendStartIfNeeded() {
				sr.Stop(fmt.Errorf("send start chunk failed"))
				return
			}
			if streamResp.Delta != "" {
				outputText.WriteString(streamResp.Delta)
				usageText.WriteString(streamResp.Delta)
				delta := streamResp.Delta
				chunk := &dto.ChatCompletionStreamResponse{
					ID: responseID, Object: "chat.completion.chunk", Created: createAt, Model: model,
					Choices: []dto.StreamChoice{{Index: 0, Delta: dto.Message{Content: delta}}},
				}
				if !sendChatChunk(chunk) {
					sr.Stop(fmt.Errorf("send text chunk failed"))
					return
				}
			}

		case "response.output_item.added", "response.output_item.done":
			if streamResp.Item == nil || streamResp.Item.Type != "function_call" {
				return
			}
			callID := strings.TrimSpace(streamResp.Item.CallID)
			if callID == "" {
				callID = strings.TrimSpace(streamResp.Item.ID)
			}
			if callID == "" {
				return
			}
			name := strings.TrimSpace(streamResp.Item.Name)
			if name != "" {
				toolCallNameByID[callID] = name
			}
			newArgs := streamResp.Item.Arguments
			prevArgs := toolCallArgsByID[callID]
			var argsDelta string
			if newArgs != "" {
				if strings.HasPrefix(newArgs, prevArgs) {
					argsDelta = newArgs[len(prevArgs):]
				} else {
					argsDelta = newArgs
				}
				toolCallArgsByID[callID] = newArgs
			}
			if !r2cSendToolCallChunk(responseID, createAt, model, callID, name, argsDelta, toolCallIndexByID, toolCallNameByID, toolCallNameSent, writer, sendChatChunk) {
				sr.Stop(fmt.Errorf("send tool call chunk failed"))
				return
			}
			sawToolCall = true
			usageText.WriteString(name)
			usageText.WriteString(argsDelta)

		case "response.function_call_arguments.delta":
			itemID := strings.TrimSpace(streamResp.ItemID)
			callID := itemID
			if callID == "" {
				return
			}
			toolCallArgsByID[callID] += streamResp.Delta
			if !r2cSendToolCallChunk(responseID, createAt, model, callID, "", streamResp.Delta, toolCallIndexByID, toolCallNameByID, toolCallNameSent, writer, sendChatChunk) {
				sr.Stop(fmt.Errorf("send tool call args chunk failed"))
				return
			}
			sawToolCall = true
			usageText.WriteString(streamResp.Delta)

		case "response.completed":
			if streamResp.Response != nil {
				if streamResp.Response.Model != "" {
					model = streamResp.Response.Model
				}
				if streamResp.Response.CreatedAt != 0 {
					createAt = int64(streamResp.Response.CreatedAt)
				}
				if streamResp.Response.Usage != nil {
					totalUsage = *responsesUsageToCommon(streamResp.Response.Usage)
					if totalUsage.TotalTokens == 0 {
						totalUsage.TotalTokens = totalUsage.PromptTokens + totalUsage.CompletionTokens
					}
				}
			}
			if !sendStartIfNeeded() {
				sr.Stop(fmt.Errorf("send start chunk failed"))
				return
			}
			if !sentStop {
				finishReason := "stop"
				if sawToolCall && outputText.Len() == 0 {
					finishReason = "tool_calls"
				}
				fr := finishReason
				stopChunk := &dto.ChatCompletionStreamResponse{
					ID: responseID, Object: "chat.completion.chunk", Created: createAt, Model: model,
					Choices: []dto.StreamChoice{{Index: 0, FinishReason: &fr}},
				}
				if !sendChatChunk(stopChunk) {
					sr.Stop(fmt.Errorf("send stop chunk failed"))
					return
				}
				sentStop = true
			}

		case "response.error", "response.failed":
			errMsg := "responses stream error"
			if streamResp.Response != nil && streamResp.Response.Error != nil {
				if b, err := json.Marshal(streamResp.Response.Error); err == nil {
					errMsg = string(b)
				}
			}
			sr.Stop(fmt.Errorf("%s: %s", streamResp.Type, errMsg))
			return
		}
	})

	// 上游未返回 usage 时按累计文本估算（正常结束 4 字符/token，中断 2 字符/token）
	if totalUsage.TotalTokens == 0 && usageText.Len() > 0 {
		estimated := helper.EstimateStreamOutputTokens(info, usageText.Len())
		totalUsage.CompletionTokens = estimated
		totalUsage.TotalTokens = totalUsage.PromptTokens + estimated
	}
	// 流中断：输入 token 用请求侧估算值补齐，保证输入正常计费
	helper.ApplyInterruptedUsageFallback(info, &totalUsage, usageText.Len())
	if !sentStart {
		sendStartIfNeeded()
	}
	if !sentStop {
		fr := "stop"
		stopChunk := &dto.ChatCompletionStreamResponse{
			ID: responseID, Object: "chat.completion.chunk", Created: createAt, Model: model,
			Choices: []dto.StreamChoice{{Index: 0, FinishReason: &fr}},
		}
		sendChatChunk(stopChunk)
	}
	if totalUsage.TotalTokens > 0 {
		usageChunk := &dto.ChatCompletionStreamResponse{
			ID: responseID, Object: "chat.completion.chunk", Created: createAt, Model: model,
			Choices: []dto.StreamChoice{},
			Usage:   &dto.UsageWithDetails{PromptTokens: totalUsage.PromptTokens, CompletionTokens: totalUsage.CompletionTokens, TotalTokens: totalUsage.TotalTokens, PromptTokensDetails: common.CommonTokenDetailsToDto(totalUsage.PromptTokensDetails), CompletionTokenDetails: common.CommonTokenDetailsToDto(totalUsage.CompletionTokenDetails)},
		}
		sendChatChunk(usageChunk)
	}
	helper.WriteSSEData(writer, "[DONE]")

	g.Log().Infof(ctx, "[HandleResponsesStreamToChat] completed: usage=%+v, endReason=%s", totalUsage, info.StreamStatus.GetEndReason())
	return &totalUsage, nil
}

func r2cSendToolCallChunk(responseID string, createAt int64, model string, callID string, name string, argsDelta string, indexByID map[string]int, nameByID map[string]string, nameSent map[string]bool, writer http.ResponseWriter, sendChatChunk func(*dto.ChatCompletionStreamResponse) bool) bool {
	idx, ok := indexByID[callID]
	if !ok {
		idx = len(indexByID)
		indexByID[callID] = idx
	}
	if name != "" {
		nameByID[callID] = name
	}
	if nameByID[callID] != "" {
		name = nameByID[callID]
	}
	tool := dto.ToolCall{ID: callID, Type: "function", Index: idx, Function: dto.FunctionCall{Arguments: argsDelta}}
	if name != "" && !nameSent[callID] {
		tool.Function.Name = name
		nameSent[callID] = true
	}
	chunk := &dto.ChatCompletionStreamResponse{
		ID: responseID, Object: "chat.completion.chunk", Created: createAt, Model: model,
		Choices: []dto.StreamChoice{{Index: 0, Delta: dto.Message{ToolCalls: []dto.ToolCall{tool}}}},
	}
	return sendChatChunk(chunk)
}

// HandleResponsesNonStreamToChat 将 Responses API 非流式响应转换为 Chat Completions 并写入 writer
func HandleResponsesNonStreamToChat(ctx context.Context, resp *http.Response, info *common.RelayInfo, writer http.ResponseWriter) (*common.Usage, error) {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response body: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		writer.Header().Set("Content-Type", "application/json")
		writer.WriteHeader(resp.StatusCode)
		_, _ = writer.Write(body)
		// 错误体已写给客户端，标记 ResponseWritten 防止调度 FSM 误判为可重试导致二次写入
		upstreamErr := constant.NewUpstreamError(resp.StatusCode, string(body), nil).WithRetryAfter(constant.RetryAfterFromHeader(resp.Header))
		upstreamErr.ResponseWritten = true
		return nil, upstreamErr
	}

	var responsesResp dto.OpenAIResponsesResponse
	if err := json.Unmarshal(body, &responsesResp); err != nil {
		return nil, fmt.Errorf("parse responses response: %w", err)
	}

	chatID := fmt.Sprintf("chatcmpl-%s", info.RequestID)
	model := info.ChannelMeta.UpstreamModelName
	if !info.ChannelMeta.IsModelMapped && responsesResp.Model != "" {
		model = responsesResp.Model
	}

	chatResp, usage, err := ResponsesResponseToChatCompletions(&responsesResp, chatID, model)
	if err != nil {
		return nil, fmt.Errorf("convert responses to chat: %w", err)
	}
	if info.ChannelMeta.IsModelMapped {
		chatResp.Model = info.OriginModelName
	}

	resultBody, err := json.Marshal(chatResp)
	if err != nil {
		return nil, fmt.Errorf("marshal chat response: %w", err)
	}
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(http.StatusOK)
	writer.Write(resultBody)
	return usage, nil
}

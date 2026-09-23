package oai_responses

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/qianfree/team-api/relaykit/dto"
	"github.com/qianfree/team-api/relaykit/relayconvert"
	"github.com/qianfree/team-api/relaykit/relayconvert/convmeta"
	"github.com/qianfree/team-api/relaykit/relayconvert/internal/shared"
	"github.com/qianfree/team-api/relaykit/types"
)

// OpenAIToResponsesRequestConverter 将 Chat Completions 请求转换为 Responses API 请求
// （Responses API Bridge：chat 客户端 → Responses 原生上游）。
type OpenAIToResponsesRequestConverter struct{}

func (c *OpenAIToResponsesRequestConverter) ID() string {
	return relayconvert.ConverterOpenAIChatToOpenAIResponses
}

func (c *OpenAIToResponsesRequestConverter) From() types.RelayFormat {
	return types.RelayFormatOpenAI
}

func (c *OpenAIToResponsesRequestConverter) To() types.RelayFormat {
	return types.RelayFormatOpenAIResponses
}

func (c *OpenAIToResponsesRequestConverter) Quality() relayconvert.RequestConverterQuality {
	return relayconvert.RequestConverterQualityGood
}

// ConvertRequest 将 Chat Completions 请求转换为 Responses API 请求。
func (c *OpenAIToResponsesRequestConverter) ConvertRequest(
	ctx context.Context,
	info convmeta.Meta,
	request any,
) (any, error) {
	req, ok := request.(*dto.GeneralOpenAIRequest)
	if !ok {
		return nil, fmt.Errorf("expected *dto.GeneralOpenAIRequest, got %T", request)
	}

	respReq := &dto.OpenAIResponsesRequest{Model: req.Model}
	// 模型名：优先映射后的上游模型名，否则用客户端请求模型名
	if upstream := convmeta.UpstreamModelName(info); upstream != "" {
		respReq.Model = upstream
	}

	if instructions := c2rExtractInstructions(req.Messages); instructions != "" {
		raw, err := json.Marshal(instructions)
		if err != nil {
			return nil, fmt.Errorf("marshal responses instructions: %w", err)
		}
		respReq.Instructions = raw
	}
	inputRaw, err := json.Marshal(c2rConvertMessagesToInput(req.Messages))
	if err != nil {
		return nil, fmt.Errorf("marshal responses input: %w", err)
	}
	respReq.Input = inputRaw

	respReq.Stream = req.Stream
	respReq.Temperature = req.Temperature
	respReq.TopP = req.TopP
	// prompt_cache_key 是官方 Responses API 参数，原样透传；
	// presence/frequency penalty 不属于官方 Responses API，透传会被严格上游
	// （如 api.openai.com）以未知参数拒绝，chat 入站时不注入、静默丢弃。
	respReq.PromptCacheKey = req.PromptCacheKey
	if maxTokens := c2rGetMaxTokens(req); maxTokens > 0 {
		mt := uint(maxTokens)
		respReq.MaxOutputTokens = &mt
	}
	if req.ResponseFormat != nil {
		if tf := c2rBuildTextFormat(req.ResponseFormat); tf != nil {
			raw, err := json.Marshal(tf)
			if err != nil {
				return nil, fmt.Errorf("marshal responses text format: %w", err)
			}
			respReq.Text = raw
		}
	}
	// 桥接的响应不可被 chat 客户端经 previous_response_id 引用（chat 协议无此概念），
	// 显式 store:false 避免上游无谓存储；渠道配置 DisableStore 时宿主 SanitizeFields 会删掉该字段
	respReq.Store = json.RawMessage("false")
	if req.ReasoningEffort != "" {
		respReq.Reasoning = &dto.Reasoning{Effort: req.ReasoningEffort, Summary: "detailed"}
	}
	if len(req.Tools) > 0 {
		if respTools := c2rConvertTools(req.Tools); len(respTools) > 0 {
			raw, err := json.Marshal(respTools)
			if err != nil {
				return nil, fmt.Errorf("marshal responses tools: %w", err)
			}
			respReq.Tools = raw
		}
	}

	// 服务端联网搜索：chat 的 web_search_options → Responses 的 web_search 工具项。
	// Responses 把搜索表达为工具，需与已转换的自定义函数合并进同一个 tools 数组。
	if spec := shared.DetectWebSearchFromOpenAI(req.WebSearchOptions); spec != nil {
		tools := make([]any, 0, 1)
		if len(respReq.Tools) > 0 {
			if err := json.Unmarshal(respReq.Tools, &tools); err != nil {
				return nil, fmt.Errorf("merge web_search into responses tools: %w", err)
			}
		}
		tools = append(tools, spec.ToResponsesTool())
		raw, err := json.Marshal(tools)
		if err != nil {
			return nil, fmt.Errorf("marshal responses tools: %w", err)
		}
		respReq.Tools = raw
	}
	if req.ToolChoice != nil {
		raw, err := json.Marshal(c2rConvertToolChoice(req.ToolChoice))
		if err != nil {
			return nil, fmt.Errorf("marshal responses tool choice: %w", err)
		}
		respReq.ToolChoice = raw
	}
	if req.User != "" {
		raw, err := json.Marshal(req.User)
		if err != nil {
			return nil, fmt.Errorf("marshal responses user: %w", err)
		}
		respReq.User = raw
	}
	if req.ParallelToolCalls != nil {
		raw, err := json.Marshal(*req.ParallelToolCalls)
		if err != nil {
			return nil, fmt.Errorf("marshal responses parallel tool calls: %w", err)
		}
		respReq.ParallelToolCalls = raw
	}

	return respReq, nil
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

func c2rGetMaxTokens(req *dto.GeneralOpenAIRequest) int {
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

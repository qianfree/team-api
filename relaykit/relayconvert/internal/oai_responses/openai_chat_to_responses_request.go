package oai_responses

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/qianfree/team-api/relaykit/dto"
	"github.com/qianfree/team-api/relaykit/relayconvert"
	"github.com/qianfree/team-api/relaykit/relayconvert/convmeta"
	"github.com/qianfree/team-api/relaykit/types"
)

// OpenAIChatToResponsesRequestConverter OpenAI Chat 客户端 → Responses 上游（请求侧）。
// 用于 ChatViaResponses 渠道：chat 入站请求转换为 Responses 格式发送 /v1/responses。
// 吸收了旧路径 adaptor 在转换前对 chat 体注入 reasoning_effort 的后处理
// （显式 req.ReasoningEffort 优先，为空时取宿主 thinking 后缀映射）。
type OpenAIChatToResponsesRequestConverter struct{}

func (c *OpenAIChatToResponsesRequestConverter) ID() string {
	return relayconvert.ConverterOpenAIChatToOpenAIResponses
}

func (c *OpenAIChatToResponsesRequestConverter) From() types.RelayFormat {
	return types.RelayFormatOpenAI
}

func (c *OpenAIChatToResponsesRequestConverter) To() types.RelayFormat {
	return types.RelayFormatOpenAIResponses
}

func (c *OpenAIChatToResponsesRequestConverter) Quality() relayconvert.RequestConverterQuality {
	return relayconvert.RequestConverterQualityGood
}

// ConvertRequest 入参断言 *dto.GeneralOpenAIRequest，输出 *dto.OpenAIResponsesRequest。
func (c *OpenAIChatToResponsesRequestConverter) ConvertRequest(
	ctx context.Context, info convmeta.Meta, request any,
) (any, error) {
	req, ok := request.(*dto.GeneralOpenAIRequest)
	if !ok {
		return nil, fmt.Errorf("expected *dto.GeneralOpenAIRequest, got %T", request)
	}
	return buildResponsesRequest(info, req)
}

// rawJSON 将值序列化为 json.RawMessage（失败时返回 nil，等价于省略字段）。
func rawJSON(v any) json.RawMessage {
	b, err := json.Marshal(v)
	if err != nil {
		return nil
	}
	return b
}

func buildResponsesRequest(info convmeta.Meta, req *dto.GeneralOpenAIRequest) (*dto.OpenAIResponsesRequest, error) {
	respReq := &dto.OpenAIResponsesRequest{}

	// 模型：与 r2c 方向相同，catalog 保证未映射时 UpstreamModelName 即客户端模型名
	if info != nil && info.HasChannelMeta() {
		respReq.Model = info.GetUpstreamModelName()
	} else {
		respReq.Model = req.Model
	}

	if instructions := c2rExtractInstructions(req.Messages); instructions != "" {
		respReq.Instructions = json.RawMessage(strconvQuote(instructions))
	}
	input, err := json.Marshal(c2rConvertMessagesToInput(req.Messages))
	if err != nil {
		return nil, fmt.Errorf("marshal responses input: %w", err)
	}
	respReq.Input = input

	if req.Stream != nil {
		respReq.Stream = req.Stream
	}
	if req.Temperature != nil {
		respReq.Temperature = req.Temperature
	}
	if req.TopP != nil {
		respReq.TopP = req.TopP
	}
	// prompt_cache_key 是官方 Responses API 参数，原样透传；
	// presence/frequency penalty 不属于官方 Responses API，透传会被严格上游
	//（如 api.openai.com）以未知参数拒绝，chat 入站时不注入、静默丢弃。
	if req.PromptCacheKey != "" {
		respReq.PromptCacheKey = req.PromptCacheKey
	}
	if maxTokens := c2rGetMaxTokens(req); maxTokens > 0 {
		maxOutput := uint(maxTokens)
		respReq.MaxOutputTokens = &maxOutput
	}
	if req.ResponseFormat != nil {
		if tf := c2rBuildTextFormat(req.ResponseFormat); tf != nil {
			if b, err := json.Marshal(tf); err == nil {
				respReq.Text = b
			}
		}
	}
	// 桥接的响应不可被 chat 客户端经 previous_response_id 引用（chat 协议无此概念），
	// 显式 store:false 避免上游无谓存储；渠道配置 DisableStore 时宿主 SanitizeFields 会删掉该字段
	respReq.Store = json.RawMessage("false")
	// reasoning_effort：吸收旧路径 adaptor 转换前的注入（显式设置优先，宿主 thinking 后缀兜底）
	if req.ReasoningEffort != "" || (info != nil && info.GetReasoningEffort() != "") {
		effort := req.ReasoningEffort
		if effort == "" {
			effort = info.GetReasoningEffort()
		}
		respReq.Reasoning = &dto.Reasoning{Effort: effort, Summary: "detailed"}
	}
	if len(req.Tools) > 0 {
		if respTools := c2rConvertTools(req.Tools); len(respTools) > 0 {
			respReq.Tools = rawJSON(respTools)
		}
	}
	if req.ToolChoice != nil {
		respReq.ToolChoice = rawJSON(c2rConvertToolChoice(req.ToolChoice))
	}
	if req.User != "" {
		respReq.User = json.RawMessage(strconvQuote(req.User))
	}
	if req.ParallelToolCalls != nil {
		respReq.ParallelToolCalls = rawJSON(*req.ParallelToolCalls)
	}
	return respReq, nil
}

// strconvQuote 将字符串序列化为 JSON 字符串字面量（含引号）。
func strconvQuote(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
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
		// 典型为 []dto.ContentPart（结构化解析产物）：序列化往返后按通用 map 处理
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

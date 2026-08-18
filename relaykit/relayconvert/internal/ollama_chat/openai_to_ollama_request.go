// Package ollama_chat 实现 OpenAI Chat Completions ↔ Ollama /api/chat 双向转换器。
//
// 仅覆盖 chat 路径（RelayModeChatCompletions）。Ollama 的 generate（completions）
// 与 embedding 路径不在本阶段迁移（桥接未注册对应 converter，自动回退旧 adaptor）。
package ollama_chat

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

// OpenAIToOllamaRequestConverter 将 OpenAI Chat Completions 请求转换为 Ollama /api/chat 请求。
type OpenAIToOllamaRequestConverter struct{}

func (c *OpenAIToOllamaRequestConverter) ID() string {
	return relayconvert.ConverterOpenAIChatToOllama
}

func (c *OpenAIToOllamaRequestConverter) From() types.RelayFormat {
	return types.RelayFormatOpenAI
}

func (c *OpenAIToOllamaRequestConverter) To() types.RelayFormat {
	return types.RelayFormatOllama
}

func (c *OpenAIToOllamaRequestConverter) Quality() relayconvert.RequestConverterQuality {
	return relayconvert.RequestConverterQualityGood
}

// ConvertRequest 将 OpenAI Chat 请求转换为 Ollama Chat 请求。
func (c *OpenAIToOllamaRequestConverter) ConvertRequest(
	ctx context.Context,
	info convmeta.Meta,
	request any,
) (any, error) {
	openaiReq, ok := request.(*dto.GeneralOpenAIRequest)
	if !ok {
		return nil, fmt.Errorf("expected *dto.GeneralOpenAIRequest, got %T", request)
	}

	// 模型名：优先映射后的上游模型名，否则用客户端请求模型名
	model := openaiReq.Model
	if info != nil {
		if upstream := info.GetUpstreamModelName(); upstream != "" {
			model = upstream
		}
	}

	// 消息转换：Ollama Content 为纯文本（多模态文本部分拼接）。
	// 保留跨轮次的工具调用与推理上下文：
	// assistant 消息保留 tool_calls（含 id 与对象形式的 arguments），
	// tool 消息保留 tool_call_id/tool_name，reasoning_content 转为 thinking 回传。
	ollamaMessages := make([]dto.OllamaMessage, 0, len(openaiReq.Messages))
	toolNamesByCallID := make(map[string]string)
	for _, msg := range openaiReq.Messages {
		om := dto.OllamaMessage{
			Role:    msg.Role,
			Content: extractOllamaContent(msg.Content),
		}
		if msg.Role == "tool" {
			om.ToolCallID = msg.ToolCallID
			om.ToolName = msg.Name
			if om.ToolName == "" {
				// 消息未带 name 时按 tool_call_id 从前面 assistant 的 tool_calls 反查工具名；
				// Ollama 的 tool 消息靠 tool_name 关联结果（tool_call_id 仅原样透传）。
				om.ToolName = toolNamesByCallID[msg.ToolCallID]
			}
		}
		if msg.Role == "assistant" {
			if msg.ReasoningContent != nil {
				thinking, err := json.Marshal(*msg.ReasoningContent)
				if err != nil {
					return nil, fmt.Errorf("marshal ollama thinking: %w", err)
				}
				om.Thinking = thinking
			}
			if len(msg.ToolCalls) > 0 {
				calls, err := toOllamaToolCalls(msg.ToolCalls)
				if err != nil {
					return nil, err
				}
				for _, tc := range msg.ToolCalls {
					if tc.ID != "" {
						toolNamesByCallID[tc.ID] = tc.Function.Name
					}
				}
				om.ToolCalls = calls
			}
		}
		ollamaMessages = append(ollamaMessages, om)
	}

	stream := false
	if info != nil {
		stream = info.GetIsStream()
	}

	think, err := toOllamaThink(openaiReq.ReasoningEffort)
	if err != nil {
		return nil, err
	}

	format, err := toOllamaResponseFormat(openaiReq.ResponseFormat)
	if err != nil {
		return nil, err
	}

	ollamaReq := &dto.OllamaChatRequest{
		Model:    model,
		Messages: ollamaMessages,
		Stream:   stream,
		Options:  buildOllamaOptions(openaiReq),
		Tools:    toOllamaTools(openaiReq.Tools),
		Format:   format,
		Think:    think,
	}

	return ollamaReq, nil
}

// extractOllamaContent 从消息内容中提取 Ollama 所需的纯文本。
// Content 可以是 string 或 []any（多模态，JSON 解析后的原始数组）。
func extractOllamaContent(content any) string {
	switch c := content.(type) {
	case string:
		return c
	case []any:
		var sb strings.Builder
		for _, part := range c {
			m, ok := part.(map[string]any)
			if !ok {
				continue
			}
			if t, _ := m["type"].(string); t == "text" {
				if text, ok := m["text"].(string); ok {
					sb.WriteString(text)
				}
			}
		}
		return sb.String()
	default:
		return ""
	}
}

// buildOllamaOptions 从 OpenAI 请求参数构建 Ollama options 映射。
// 映射关系：temperature→temperature、top_p→top_p、max_tokens/max_completion_tokens→num_predict、
// stop→stop、frequency_penalty→frequency_penalty、presence_penalty→presence_penalty。
func buildOllamaOptions(req *dto.GeneralOpenAIRequest) map[string]any {
	options := make(map[string]any)

	if req.Temperature != nil {
		options["temperature"] = *req.Temperature
	}
	if req.TopP != nil {
		options["top_p"] = *req.TopP
	}

	// max_tokens / max_completion_tokens → num_predict
	if req.MaxTokens != nil {
		options["num_predict"] = *req.MaxTokens
	} else if req.MaxCompletionTokens != nil {
		options["num_predict"] = *req.MaxCompletionTokens
	}

	// stop：原样透传（string 或 []string）
	if req.Stop != nil {
		options["stop"] = req.Stop
	}

	if req.FrequencyPenalty != nil {
		options["frequency_penalty"] = *req.FrequencyPenalty
	}
	if req.PresencePenalty != nil {
		options["presence_penalty"] = *req.PresencePenalty
	}

	if len(options) == 0 {
		return nil
	}
	return options
}

// toOllamaTools 将 OpenAI tools 转换为 Ollama tools（两者 function 格式一致）。
func toOllamaTools(tools []dto.Tool) []dto.OllamaTool {
	if len(tools) == 0 {
		return nil
	}
	out := make([]dto.OllamaTool, 0, len(tools))
	for _, t := range tools {
		if t.Type != "function" {
			continue
		}
		out = append(out, dto.OllamaTool{
			Type: "function",
			Function: dto.OllamaToolFunction{
				Name:        t.Function.Name,
				Description: t.Function.Description,
				Parameters:  t.Function.Parameters,
			},
		})
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// toOllamaToolCalls 将 OpenAI tool_calls 转换为 Ollama tool_calls，
// 把字符串编码的 arguments 解码为 JSON 对象。
func toOllamaToolCalls(toolCalls []dto.ToolCall) ([]dto.OllamaToolCall, error) {
	out := make([]dto.OllamaToolCall, 0, len(toolCalls))
	for _, tc := range toolCalls {
		oc := dto.OllamaToolCall{ID: tc.ID}
		oc.Function.Name = tc.Function.Name
		var args any
		if tc.Function.Arguments != "" {
			if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil {
				return nil, fmt.Errorf("parse tool call arguments: %w", err)
			}
		}
		if args == nil {
			args = map[string]any{}
		}
		oc.Function.Arguments = args
		out = append(out, oc)
	}
	return out, nil
}

// toOllamaThink 将 reasoning_effort 映射为 Ollama think 参数。
// "none" 关闭思考；low/medium/high/max 作为思考档位原样透传；
// 未知取值直接跳过（不注入 think）而非报错。
func toOllamaThink(effort string) (json.RawMessage, error) {
	if effort == "" {
		return nil, nil
	}
	var value any
	switch effort {
	case "none":
		value = false
	case "low", "medium", "high", "max":
		value = effort
	default:
		return nil, nil
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("marshal ollama think: %w", err)
	}
	return raw, nil
}

// toOllamaResponseFormat 将 OpenAI response_format 映射为 Ollama format。
func toOllamaResponseFormat(responseFormat *dto.ResponseFormat) (any, error) {
	if responseFormat == nil {
		return nil, nil
	}
	switch responseFormat.Type {
	case "json", "json_object":
		return "json", nil
	case "json_schema":
		return extractJSONSchema(responseFormat.JSONSchema), nil
	default:
		return nil, nil
	}
}

// extractJSONSchema 从 OpenAI json_schema 包装结构（{name, description, schema, strict}）
// 中提取裸 schema；Ollama 的 format 字段期望 schema 对象本身。
func extractJSONSchema(v any) any {
	if m, ok := v.(map[string]any); ok {
		if schema, ok := m["schema"]; ok {
			return schema
		}
	}
	return v
}

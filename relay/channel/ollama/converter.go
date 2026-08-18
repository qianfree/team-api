package ollama

import (
	"encoding/json"
	"fmt"

	"github.com/qianfree/team-api/relay/common"
	"github.com/qianfree/team-api/relay/dto"
)

// convertChatRequest 将 OpenAI Chat Completions 请求转换为 Ollama Chat 格式
func convertChatRequest(requestBody []byte, info *common.RelayInfo) ([]byte, error) {
	var rawReq map[string]json.RawMessage
	if err := json.Unmarshal(requestBody, &rawReq); err != nil {
		return nil, fmt.Errorf("parse request body failed: %w", err)
	}

	// 确定模型名
	model := info.OriginModelName
	if info.ChannelMeta.IsModelMapped {
		model = info.ChannelMeta.UpstreamModelName
	}

	// 解析消息
	var openaiMessages []struct {
		Role             string         `json:"role"`
		Content          any            `json:"content"`
		ToolCallID       string         `json:"tool_call_id"`
		Name             string         `json:"name"`
		ReasoningContent *string        `json:"reasoning_content"`
		ToolCalls        []dto.ToolCall `json:"tool_calls"`
	}
	if raw, ok := rawReq["messages"]; ok {
		if err := json.Unmarshal(raw, &openaiMessages); err != nil {
			return nil, fmt.Errorf("parse messages failed: %w", err)
		}
	}

	ollamaMessages := make([]OllamaMessage, 0, len(openaiMessages))
	toolNamesByCallID := make(map[string]string)
	for _, msg := range openaiMessages {
		ollamaMsg := OllamaMessage{
			Role: msg.Role,
		}
		// content 可以是 string 或 []ContentPart（多模态）
		switch c := msg.Content.(type) {
		case string:
			ollamaMsg.Content = c
		case []any:
			// 多模态内容数组，提取文本部分
			for _, part := range c {
				partMap, ok := part.(map[string]any)
				if !ok {
					continue
				}
				if partType, _ := partMap["type"].(string); partType == "text" {
					if text, ok := partMap["text"].(string); ok {
						ollamaMsg.Content += text
					}
				}
			}
		default:
			// 尝试 JSON 序列化为字符串
			if c != nil {
				b, _ := json.Marshal(c)
				ollamaMsg.Content = string(b)
			}
		}
		// 保留跨轮次的工具调用与推理上下文（与 relaykit 转换器行为对齐）
		if msg.Role == "tool" {
			ollamaMsg.ToolCallID = msg.ToolCallID
			ollamaMsg.ToolName = msg.Name
			if ollamaMsg.ToolName == "" {
				// 消息未带 name 时按 tool_call_id 从前面 assistant 的 tool_calls 反查工具名；
				// Ollama 的 tool 消息靠 tool_name 关联结果（tool_call_id 仅原样透传）。
				ollamaMsg.ToolName = toolNamesByCallID[msg.ToolCallID]
			}
		}
		if msg.Role == "assistant" {
			if msg.ReasoningContent != nil {
				if b, err := json.Marshal(*msg.ReasoningContent); err == nil {
					ollamaMsg.Thinking = b
				}
			}
			if len(msg.ToolCalls) > 0 {
				calls := make([]OllamaToolCall, 0, len(msg.ToolCalls))
				for _, tc := range msg.ToolCalls {
					oc := OllamaToolCall{ID: tc.ID}
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
					calls = append(calls, oc)
					if tc.ID != "" {
						toolNamesByCallID[tc.ID] = tc.Function.Name
					}
				}
				ollamaMsg.ToolCalls = calls
			}
		}
		ollamaMessages = append(ollamaMessages, ollamaMsg)
	}

	// 解析 stream
	stream := info.IsStream

	// 构建 options
	options := buildOptions(rawReq)

	ollamaReq := OllamaChatRequest{
		Model:    model,
		Messages: ollamaMessages,
		Stream:   stream,
		Options:  options,
		Tools:    legacyOllamaTools(rawReq),
		Format:   legacyOllamaFormat(rawReq),
		Think:    legacyOllamaThink(rawReq),
	}

	return json.Marshal(ollamaReq)
}

// convertCompletionsRequest 将 OpenAI Completions 请求转换为 Ollama Generate 格式
func convertCompletionsRequest(requestBody []byte, info *common.RelayInfo) ([]byte, error) {
	var rawReq map[string]json.RawMessage
	if err := json.Unmarshal(requestBody, &rawReq); err != nil {
		return nil, fmt.Errorf("parse request body failed: %w", err)
	}

	// 确定模型名
	model := info.OriginModelName
	if info.ChannelMeta.IsModelMapped {
		model = info.ChannelMeta.UpstreamModelName
	}

	// 解析 prompt
	var prompt string
	if raw, ok := rawReq["prompt"]; ok {
		// prompt 可以是 string 或 []string
		if err := json.Unmarshal(raw, &prompt); err != nil {
			// 尝试 []string
			var prompts []string
			if err2 := json.Unmarshal(raw, &prompts); err2 == nil && len(prompts) > 0 {
				prompt = prompts[0]
			}
		}
	}

	// 解析 stream
	stream := info.IsStream

	// 构建 options
	options := buildOptions(rawReq)

	ollamaReq := OllamaGenerateRequest{
		Model:   model,
		Prompt:  prompt,
		Stream:  stream,
		Options: options,
	}

	return json.Marshal(ollamaReq)
}

// convertEmbeddingRequest 将 OpenAI Embedding 请求转换为 Ollama Embedding 格式
func convertEmbeddingRequest(requestBody []byte, info *common.RelayInfo) ([]byte, error) {
	var rawReq map[string]json.RawMessage
	if err := json.Unmarshal(requestBody, &rawReq); err != nil {
		return nil, fmt.Errorf("parse request body failed: %w", err)
	}

	// 确定模型名
	model := info.OriginModelName
	if info.ChannelMeta.IsModelMapped {
		model = info.ChannelMeta.UpstreamModelName
	}

	// 解析 input（string 或 []string）
	var input []string
	if raw, ok := rawReq["input"]; ok {
		var singleInput string
		if err := json.Unmarshal(raw, &singleInput); err == nil {
			input = []string{singleInput}
		} else {
			if err := json.Unmarshal(raw, &input); err != nil {
				return nil, fmt.Errorf("parse input failed: %w", err)
			}
		}
	}

	ollamaReq := OllamaEmbeddingRequest{
		Model: model,
		Input: input,
	}

	return json.Marshal(ollamaReq)
}

// buildOptions 从 OpenAI 请求参数构建 Ollama options
func buildOptions(rawReq map[string]json.RawMessage) map[string]any {
	options := make(map[string]any)

	// temperature → temperature
	if raw, ok := rawReq["temperature"]; ok {
		var v float64
		if json.Unmarshal(raw, &v) == nil {
			options["temperature"] = v
		}
	}

	// max_tokens / max_completion_tokens → num_predict
	if raw, ok := rawReq["max_tokens"]; ok {
		var v int
		if json.Unmarshal(raw, &v) == nil {
			options["num_predict"] = v
		}
	} else if raw, ok := rawReq["max_completion_tokens"]; ok {
		var v int
		if json.Unmarshal(raw, &v) == nil {
			options["num_predict"] = v
		}
	}

	// top_p → top_p
	if raw, ok := rawReq["top_p"]; ok {
		var v float64
		if json.Unmarshal(raw, &v) == nil {
			options["top_p"] = v
		}
	}

	// stop → stop
	if raw, ok := rawReq["stop"]; ok {
		var v any
		if json.Unmarshal(raw, &v) == nil && v != nil {
			options["stop"] = v
		}
	}

	// frequency_penalty → frequency_penalty
	if raw, ok := rawReq["frequency_penalty"]; ok {
		var v float64
		if json.Unmarshal(raw, &v) == nil {
			options["frequency_penalty"] = v
		}
	}

	// presence_penalty → presence_penalty
	if raw, ok := rawReq["presence_penalty"]; ok {
		var v float64
		if json.Unmarshal(raw, &v) == nil {
			options["presence_penalty"] = v
		}
	}

	if len(options) == 0 {
		return nil
	}
	return options
}

// legacyOllamaTools 将原始 OpenAI tools 映射为 Ollama tools（回退路径）。
func legacyOllamaTools(rawReq map[string]json.RawMessage) []OllamaTool {
	raw, ok := rawReq["tools"]
	if !ok {
		return nil
	}
	var tools []struct {
		Type     string `json:"type"`
		Function struct {
			Name        string `json:"name"`
			Description string `json:"description,omitempty"`
			Parameters  any    `json:"parameters,omitempty"`
		} `json:"function"`
	}
	if err := json.Unmarshal(raw, &tools); err != nil {
		return nil
	}
	out := make([]OllamaTool, 0, len(tools))
	for _, t := range tools {
		if t.Type != "function" {
			continue
		}
		out = append(out, OllamaTool{
			Type: "function",
			Function: OllamaToolFunction{
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

// legacyOllamaFormat 将原始 response_format 映射为 Ollama format（回退路径）。
func legacyOllamaFormat(rawReq map[string]json.RawMessage) any {
	raw, ok := rawReq["response_format"]
	if !ok {
		return nil
	}
	var rf struct {
		Type       string `json:"type"`
		JSONSchema any    `json:"json_schema,omitempty"`
	}
	if err := json.Unmarshal(raw, &rf); err != nil {
		return nil
	}
	switch rf.Type {
	case "json", "json_object":
		return "json"
	case "json_schema":
		return legacyExtractJSONSchema(rf.JSONSchema)
	default:
		return nil
	}
}

// legacyOllamaThink 将原始 reasoning_effort 映射为 Ollama think（回退路径）。
// "none" 关闭思考；low/medium/high/max 作为思考档位透传；未知取值跳过。
func legacyOllamaThink(rawReq map[string]json.RawMessage) json.RawMessage {
	raw, ok := rawReq["reasoning_effort"]
	if !ok {
		return nil
	}
	var effort string
	if err := json.Unmarshal(raw, &effort); err != nil || effort == "" {
		return nil
	}
	var value any
	switch effort {
	case "none":
		value = false
	case "low", "medium", "high", "max":
		value = effort
	default:
		return nil
	}
	b, err := json.Marshal(value)
	if err != nil {
		return nil
	}
	return b
}

// legacyExtractJSONSchema 从 OpenAI json_schema 包装结构（{name, description, schema, strict}）
// 中提取裸 schema；Ollama 的 format 字段期望 schema 对象本身。
func legacyExtractJSONSchema(v any) any {
	if m, ok := v.(map[string]any); ok {
		if schema, ok := m["schema"]; ok {
			return schema
		}
	}
	return v
}

package ollama

import (
	"encoding/json"
	"fmt"

	"github.com/qianfree/team-api/relay/common"
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
		Role    string `json:"role"`
		Content any    `json:"content"`
	}
	if raw, ok := rawReq["messages"]; ok {
		if err := json.Unmarshal(raw, &openaiMessages); err != nil {
			return nil, fmt.Errorf("parse messages failed: %w", err)
		}
	}

	ollamaMessages := make([]OllamaMessage, 0, len(openaiMessages))
	for _, msg := range openaiMessages {
		ollamaMsg := OllamaMessage{
			Role: msg.Role,
		}
		// content 可以是 string 或 []ContentPart
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
				partType, _ := partMap["type"].(string)
				if partType == "text" {
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

// Package ollama_chat 实现 OpenAI Chat Completions ↔ Ollama /api/chat 双向转换器。
//
// 仅覆盖 chat 路径（RelayModeChatCompletions）。Ollama 的 generate（completions）
// 与 embedding 路径不在本阶段迁移（桥接未注册对应 converter，自动回退旧 adaptor）。
package ollama_chat

import (
	"context"
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

	// 消息转换：Ollama Content 为纯文本（多模态文本部分拼接）
	ollamaMessages := make([]dto.OllamaMessage, 0, len(openaiReq.Messages))
	for _, msg := range openaiReq.Messages {
		ollamaMessages = append(ollamaMessages, dto.OllamaMessage{
			Role:    msg.Role,
			Content: extractOllamaContent(msg.Content),
		})
	}

	stream := false
	if info != nil {
		stream = info.GetIsStream()
	}

	ollamaReq := &dto.OllamaChatRequest{
		Model:    model,
		Messages: ollamaMessages,
		Stream:   stream,
		Options:  buildOllamaOptions(openaiReq),
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

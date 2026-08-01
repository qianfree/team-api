package ollama_chat

import (
	"context"
	"fmt"

	"github.com/qianfree/team-api/relaykit/dto"
	"github.com/qianfree/team-api/relaykit/relayconvert"
	"github.com/qianfree/team-api/relaykit/relayconvert/convmeta"
	"github.com/qianfree/team-api/relaykit/types"
)

// OllamaToOpenAIResponseConverter 将 Ollama Chat 响应转换为 OpenAI Chat Completions 响应。
type OllamaToOpenAIResponseConverter struct{}

func (c *OllamaToOpenAIResponseConverter) ID() string {
	return relayconvert.ResponseConverterOllamaChatToOAIChat
}

func (c *OllamaToOpenAIResponseConverter) From() types.RelayFormat {
	return types.RelayFormatOllama
}

func (c *OllamaToOpenAIResponseConverter) To() types.RelayFormat {
	return types.RelayFormatOpenAI
}

func (c *OllamaToOpenAIResponseConverter) Quality() relayconvert.ResponseConverterQuality {
	return relayconvert.ResponseConverterQualityGood
}

// ConvertResponse 将 Ollama Chat 非流式响应转换为 OpenAI 非流式响应。
// 注意：方法签名返回 (any, error)，注册时由 register 包适配闭包补 nil Usage。
func (c *OllamaToOpenAIResponseConverter) ConvertResponse(
	ctx context.Context,
	info convmeta.Meta,
	response any,
) (any, error) {
	ollamaResp, ok := response.(*dto.OllamaChatResponse)
	if !ok {
		return nil, fmt.Errorf("expected *dto.OllamaChatResponse, got %T", response)
	}

	modelName := ""
	if info != nil {
		modelName = info.GetOriginModelName()
	}

	openaiResp := &dto.ChatCompletionResponse{
		ID:      fmt.Sprintf("chatcmpl-%d", getCurrentTimestamp()),
		Object:  "chat.completion",
		Created: 0,
		Model:   modelName,
		Choices: []dto.Choice{{
			Index: 0,
			Message: dto.Message{
				Role:    ollamaResp.Message.Role,
				Content: ollamaResp.Message.Content,
			},
			FinishReason: "stop",
		}},
		Usage: dto.UsageWithDetails{
			PromptTokens:     ollamaResp.PromptEvalCount,
			CompletionTokens: ollamaResp.EvalCount,
			TotalTokens:      ollamaResp.PromptEvalCount + ollamaResp.EvalCount,
		},
	}

	return openaiResp, nil
}

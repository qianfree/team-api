package ollama_chat

import (
	"context"
	"encoding/json"
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

	message := dto.Message{
		Role:    ollamaResp.Message.Role,
		Content: ollamaResp.Message.Content,
	}
	if reasoning := ollamaThinkingToOpenAI(ollamaResp.Message.Thinking); reasoning != nil {
		message.ReasoningContent = reasoning
	}
	if toolCalls := ollamaToolCallsToOpenAI(ollamaResp.Message.ToolCalls); len(toolCalls) > 0 {
		message.ToolCalls = toolCalls
	}

	finishReason := ollamaDoneReasonToFinishReason(ollamaResp.DoneReason)
	if len(message.ToolCalls) > 0 {
		finishReason = "tool_calls"
	}

	openaiResp := &dto.ChatCompletionResponse{
		ID:      fmt.Sprintf("chatcmpl-%d", getCurrentTimestamp()),
		Object:  "chat.completion",
		Created: 0,
		Model:   modelName,
		Choices: []dto.Choice{{
			Index:        0,
			Message:      message,
			FinishReason: finishReason,
		}},
		Usage: dto.UsageWithDetails{
			PromptTokens:     ollamaResp.PromptEvalCount,
			CompletionTokens: ollamaResp.EvalCount,
			TotalTokens:      ollamaResp.PromptEvalCount + ollamaResp.EvalCount,
		},
	}

	return openaiResp, nil
}

// ollamaToolCallsToOpenAI 将 Ollama tool_calls（对象形式 arguments）转换为
// OpenAI tool_calls（字符串编码 arguments）。
func ollamaToolCallsToOpenAI(toolCalls []dto.OllamaToolCall) []dto.ToolCall {
	out := make([]dto.ToolCall, 0, len(toolCalls))
	for i, tc := range toolCalls {
		id := tc.ID
		if id == "" {
			id = fmt.Sprintf("call_%d", i)
		}
		argsBytes := []byte("{}")
		if tc.Function.Arguments != nil {
			if b, err := json.Marshal(tc.Function.Arguments); err == nil {
				argsBytes = b
			}
		}
		out = append(out, dto.ToolCall{
			ID:   id,
			Type: "function",
			Function: dto.FunctionCall{
				Name:      tc.Function.Name,
				Arguments: string(argsBytes),
			},
		})
	}
	return out
}

// ollamaThinkingToOpenAI 将 Ollama thinking 字段（JSON 字符串）转为 OpenAI reasoning_content。
// 字段缺失、null、空串或非字符串时返回 nil，避免产出空的 reasoning_content。
func ollamaThinkingToOpenAI(thinking json.RawMessage) *string {
	if len(thinking) == 0 {
		return nil
	}
	var s string
	if err := json.Unmarshal(thinking, &s); err != nil {
		return nil
	}
	// JSON null 反序列化不报错且保持零值，空思考内容同样不产出字段
	if s == "" {
		return nil
	}
	return &s
}

// ollamaDoneReasonToFinishReason 将 Ollama done_reason 映射为 OpenAI finish_reason。
// stop/length 均为 OpenAI 合法取值，其余（如 load 等运行态原因）归一为 stop。
func ollamaDoneReasonToFinishReason(doneReason string) string {
	if doneReason == "length" {
		return "length"
	}
	return "stop"
}

package oai_chat

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/qianfree/team-api/relaykit/dto"
	"github.com/qianfree/team-api/relaykit/relayconvert"
	"github.com/qianfree/team-api/relaykit/relayconvert/convmeta"
	"github.com/qianfree/team-api/relaykit/types"
)

// OpenAIToClaudeResponseConverter 将 OpenAI Chat Completions 响应转换为 Claude Messages API 响应。
type OpenAIToClaudeResponseConverter struct{}

func (c *OpenAIToClaudeResponseConverter) ID() string {
	return relayconvert.ResponseConverterOAIChatToClaudeMessages
}

func (c *OpenAIToClaudeResponseConverter) From() types.RelayFormat {
	return types.RelayFormatOpenAI
}

func (c *OpenAIToClaudeResponseConverter) To() types.RelayFormat {
	return types.RelayFormatClaude
}

func (c *OpenAIToClaudeResponseConverter) Quality() relayconvert.ResponseConverterQuality {
	return relayconvert.ResponseConverterQualityGood
}

func (c *OpenAIToClaudeResponseConverter) ConvertResponse(
	ctx context.Context,
	info convmeta.Meta,
	response any,
) (any, error) {
	openaiResp, ok := response.(*dto.ChatCompletionResponse)
	if !ok {
		return nil, fmt.Errorf("expected *dto.ChatCompletionResponse, got %T", response)
	}

	content := make([]dto.ClaudeContentBlock, 0)
	var textParts []string
	var thinkingParts []string
	var toolCalls []dto.ClaudeContentBlock

	if len(openaiResp.Choices) > 0 {
		choice := openaiResp.Choices[0]

		// 提取文本
		if text, ok := choice.Message.Content.(string); ok && text != "" {
			textParts = append(textParts, text)
		}

		// 提取思维内容
		if choice.Message.ReasoningContent != nil && *choice.Message.ReasoningContent != "" {
			thinkingParts = append(thinkingParts, *choice.Message.ReasoningContent)
		}

		// 提取工具调用
		for _, tc := range choice.Message.ToolCalls {
			var inputObj any
			if err := json.Unmarshal([]byte(tc.Function.Arguments), &inputObj); err != nil {
				inputObj = map[string]any{}
			}
			toolCalls = append(toolCalls, dto.ClaudeContentBlock{
				Type:  "tool_use",
				ID:    tc.ID,
				Name:  tc.Function.Name,
				Input: inputObj,
			})
		}
	}

	// 添加思维块（如果有）
	for _, thinking := range thinkingParts {
		content = append(content, dto.ClaudeContentBlock{
			Type:     "thinking",
			Thinking: strPtr(thinking),
		})
	}

	// 添加文本块
	for _, text := range textParts {
		content = append(content, dto.ClaudeContentBlock{
			Type: "text",
			Text: strPtr(text),
		})
	}

	// 添加工具调用块
	content = append(content, toolCalls...)

	if len(content) == 0 {
		content = append(content, dto.ClaudeContentBlock{
			Type: "text",
			Text: strPtr(""),
		})
	}

	modelName := openaiResp.Model
	if isModelMapped(info) {
		modelName = info.GetOriginModelName()
	}

	stopReason := "end_turn"
	if len(openaiResp.Choices) > 0 {
		stopReason = mapOpenAIFinishReasonToClaude(openaiResp.Choices[0].FinishReason)
	}

	// OpenAI 的 prompt_tokens 已含 cached（子集语义），Claude 的 input_tokens 不含 cache_read，
	// 需扣减后映射，否则 Claude 客户端按本协议语义计账时缓存部分被重复计入。
	// PromptTokensDetails 可能为 nil（多数 OpenAI 兼容上游不返回该字段），必须判空。
	cachedTokens := 0
	if openaiResp.Usage.PromptTokensDetails != nil {
		cachedTokens = openaiResp.Usage.PromptTokensDetails.CachedTokens
	}
	inputTokens := openaiResp.Usage.PromptTokens - cachedTokens
	if inputTokens < 0 {
		inputTokens = 0
	}

	return &dto.ClaudeResponse{
		ID:           generateClaudeMessageID(),
		Type:         "message",
		Role:         "assistant",
		Content:      content,
		StopReason:   stopReason,
		StopSequence: nil,
		Model:        modelName,
		Usage: &dto.ClaudeUsage{
			InputTokens:          inputTokens,
			OutputTokens:         openaiResp.Usage.CompletionTokens,
			CacheReadInputTokens: cachedTokens,
		},
	}, nil
}

// mapOpenAIFinishReasonToClaude 将 OpenAI finish_reason 映射为 Claude stop_reason。
func mapOpenAIFinishReasonToClaude(reason string) string {
	switch reason {
	case "stop":
		return "end_turn"
	case "length":
		return "max_tokens"
	case "tool_calls":
		return "tool_use"
	case "content_filter":
		return "refusal"
	default:
		if reason != "" {
			return reason
		}
		return "end_turn"
	}
}

// isModelMapped 判断是否经过模型名映射（等价于宿主 ChannelMeta.IsModelMapped：
// 上游模型名非空且与入站模型名不同）。
func isModelMapped(info convmeta.Meta) bool {
	if info == nil || !info.HasChannelMeta() {
		return false
	}
	upstream := info.GetUpstreamModelName()
	return upstream != "" && upstream != info.GetOriginModelName()
}

// generateClaudeMessageID 生成 Claude 消息 ID（与本包 generateResponseID 相同的派生方式）。
func generateClaudeMessageID() string {
	return fmt.Sprintf("msg_%d", getCurrentTimestamp())
}

// strPtr 返回 string 的指针
func strPtr(v string) *string {
	return &v
}

// 转换器在宿主应用的包初始化阶段注册，而非在此 internal 实现包中完成。

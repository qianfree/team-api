package oai_gemini

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/qianfree/team-api/relaykit/dto"
	"github.com/qianfree/team-api/relaykit/relayconvert"
	"github.com/qianfree/team-api/relaykit/relayconvert/convmeta"
	"github.com/qianfree/team-api/relaykit/types"
)

// OpenAIToGeminiResponseConverter 将 OpenAI Chat Completions 响应转换为 Gemini Generate Content 响应。
type OpenAIToGeminiResponseConverter struct{}

func (c *OpenAIToGeminiResponseConverter) ID() string {
	return relayconvert.ResponseConverterOAIChatToGeminiChat
}

func (c *OpenAIToGeminiResponseConverter) From() types.RelayFormat {
	return types.RelayFormatOpenAI
}

func (c *OpenAIToGeminiResponseConverter) To() types.RelayFormat {
	return types.RelayFormatGemini
}

func (c *OpenAIToGeminiResponseConverter) Quality() relayconvert.ResponseConverterQuality {
	return relayconvert.ResponseConverterQualityGood
}

// ConvertResponse 将 OpenAI ChatCompletion 非流式响应转换为 Gemini Chat 响应。
func (c *OpenAIToGeminiResponseConverter) ConvertResponse(
	ctx context.Context,
	info convmeta.Meta,
	response any,
) (any, error) {
	openaiResp, ok := response.(*dto.ChatCompletionResponse)
	if !ok {
		return nil, fmt.Errorf("expected *dto.ChatCompletionResponse, got %T", response)
	}

	resp := &dto.GeminiChatResponse{}

	if len(openaiResp.Choices) == 0 {
		return resp, nil
	}

	choice := openaiResp.Choices[0]
	parts := buildGeminiPartsFromMessage(&choice.Message)

	candidate := dto.GeminiCandidate{
		Index: choice.Index,
		Content: &dto.GeminiContent{
			Role:  "model",
			Parts: parts,
		},
		FinishReason: mapOpenAIFinishReasonToGemini(choice.FinishReason),
	}

	resp.Candidates = []dto.GeminiCandidate{candidate}

	// Gemini 语义：CandidatesTokenCount 不含 thoughts，OpenAI CompletionTokens 已含 reasoning
	// 需要扣减 reasoning 避免双计（Gemini 客户端按 total = prompt + candidates + thoughts 汇总）
	candidatesTokens := openaiResp.Usage.CompletionTokens
	if openaiResp.Usage.CompletionTokenDetails != nil && openaiResp.Usage.CompletionTokenDetails.ReasoningTokens > 0 {
		candidatesTokens -= openaiResp.Usage.CompletionTokenDetails.ReasoningTokens
		if candidatesTokens < 0 {
			candidatesTokens = 0
		}
	}

	resp.UsageMetadata = &dto.GeminiUsageMetadata{
		PromptTokenCount:     openaiResp.Usage.PromptTokens,
		CandidatesTokenCount: candidatesTokens,
		TotalTokenCount:      openaiResp.Usage.TotalTokens,
	}
	if openaiResp.Usage.PromptTokensDetails != nil {
		resp.UsageMetadata.CachedContentTokenCount = openaiResp.Usage.PromptTokensDetails.CachedTokens
	}
	if openaiResp.Usage.CompletionTokenDetails != nil {
		resp.UsageMetadata.ThoughtsTokenCount = openaiResp.Usage.CompletionTokenDetails.ReasoningTokens
	}

	return resp, nil
}

// buildGeminiPartsFromMessage 将 OpenAI Message 转换为 Gemini Parts
func buildGeminiPartsFromMessage(msg *dto.Message) []dto.GeminiPart {
	var parts []dto.GeminiPart

	// thinking 内容 → thought part
	if msg.ReasoningContent != nil && *msg.ReasoningContent != "" {
		parts = append(parts, dto.GeminiPart{
			Text:    *msg.ReasoningContent,
			Thought: boolPtr(true),
		})
	}

	// 文本内容
	if text, ok := msg.Content.(string); ok && text != "" {
		parts = append(parts, dto.GeminiPart{Text: text})
	}

	// 工具调用 → functionCall parts
	for _, tc := range msg.ToolCalls {
		var args any
		if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil {
			args = map[string]any{}
		}
		parts = append(parts, dto.GeminiPart{
			FunctionCall: &dto.GeminiFunctionCall{
				ID:           tc.ID,
				FunctionName: tc.Function.Name,
				Arguments:    args,
			},
		})
	}

	if len(parts) == 0 {
		parts = append(parts, dto.GeminiPart{Text: ""})
	}

	return parts
}

// mapOpenAIFinishReasonToGemini 将 OpenAI finish_reason 转换为 Gemini finishReason
func mapOpenAIFinishReasonToGemini(reason string) string {
	switch reason {
	case "stop":
		return "STOP"
	case "length":
		return "MAX_TOKENS"
	case "content_filter":
		return "SAFETY"
	case "tool_calls":
		return "STOP"
	default:
		return reason
	}
}

// boolPtr 返回 bool 的指针
func boolPtr(v bool) *bool {
	return &v
}

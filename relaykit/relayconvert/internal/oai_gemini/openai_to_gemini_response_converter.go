package oai_gemini

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/qianfree/team-api/relaykit/dto"
	"github.com/qianfree/team-api/relaykit/reasonmap"
	"github.com/qianfree/team-api/relaykit/relayconvert"
	"github.com/qianfree/team-api/relaykit/relayconvert/convmeta"
	"github.com/qianfree/team-api/relaykit/types"
)

// OpenAIToGeminiResponseConverter OpenAI Chat 上游 → Gemini 客户端（非流式响应侧，P1-B）。
// 逐行移植宿主 relay/channel/openai/gemini_response.go 的 openAIToGeminiResponse，
// 挂在 spec B（ConverterGeminiContentToOpenAIChat）的 Resp 侧（方向与请求相反）。
//
// legacy 怪癖清单（保持勿改）：
//   - 空 choices 返回完全空对象（与 claude 侧"空也要补骨架"不对称）
//   - choices 非空时 usageMetadata 无条件非 nil（即使全 0）
//   - CandidatesTokenCount = Completion - Reasoning（<0 归 0，2f0cc01 口径：
//     Gemini 语义 candidates 不含 thoughts，避免客户端 total 双计）
//   - CachedContentTokenCount ← CachedTokens、ThoughtsTokenCount ← ReasoningTokens（判空）
//   - ModelName（modelVersion）不填——legacy 不填，保持（info 参数不参与映射）
//   - finish_reason：tool_calls→STOP（legacy 怪癖）、未知透传
//   - Parts：ReasoningContent→{Text,Thought:true}；Content 仅 string；ToolCall ID 填 tc.ID；
//     空 parts 补 {Text:""}
type OpenAIToGeminiResponseConverter struct{}

func (c *OpenAIToGeminiResponseConverter) ID() string {
	return relayconvert.ConverterGeminiContentToOpenAIChat
}

func (c *OpenAIToGeminiResponseConverter) From() types.RelayFormat {
	return types.RelayFormatOpenAI
}

func (c *OpenAIToGeminiResponseConverter) To() types.RelayFormat {
	return types.RelayFormatGemini
}

// ConvertResponse 入参断言 *dto.ChatCompletionResponse，输出 *dto.GeminiChatResponse。
func (c *OpenAIToGeminiResponseConverter) ConvertResponse(
	ctx context.Context, info convmeta.Meta, response any,
) (any, *dto.UsageWithDetails, error) {
	resp, ok := response.(*dto.ChatCompletionResponse)
	if !ok {
		return nil, nil, fmt.Errorf("expected *dto.ChatCompletionResponse, got %T", response)
	}
	return buildGeminiResponseFromOpenAI(resp), nil, nil
}

func buildGeminiResponseFromOpenAI(openaiResp *dto.ChatCompletionResponse) *dto.GeminiChatResponse {
	resp := &dto.GeminiChatResponse{}

	if len(openaiResp.Choices) == 0 {
		return resp
	}

	choice := openaiResp.Choices[0]
	parts := buildGeminiPartsFromOpenAIMessage(&choice.Message)

	candidate := dto.GeminiCandidate{
		Index: choice.Index,
		Content: &dto.GeminiContent{
			Role:  "model",
			Parts: parts,
		},
		FinishReason: reasonmap.OpenAIFinishReasonToGeminiFinishReason(choice.FinishReason),
	}

	resp.Candidates = []dto.GeminiCandidate{candidate}

	// Gemini 语义：CandidatesTokenCount 不含 thoughts，OpenAI CompletionTokens 已含 reasoning，
	// 扣减避免双计（Gemini 客户端按 total = prompt + candidates + thoughts 汇总）
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

	return resp
}

// buildGeminiPartsFromOpenAIMessage 移植宿主 buildGeminiPartsFromMessage。
func buildGeminiPartsFromOpenAIMessage(msg *dto.Message) []dto.GeminiPart {
	var parts []dto.GeminiPart

	// thinking 内容 → thought part
	if msg.ReasoningContent != nil && *msg.ReasoningContent != "" {
		parts = append(parts, dto.GeminiPart{
			Text:    *msg.ReasoningContent,
			Thought: boolPtrLocal(true),
		})
	}

	// 文本内容（仅 string 形态——legacy 口径）
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

func boolPtrLocal(v bool) *bool { return &v }

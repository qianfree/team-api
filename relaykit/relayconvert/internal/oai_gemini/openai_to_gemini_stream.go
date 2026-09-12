package oai_gemini

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/qianfree/team-api/relaykit/dto"
	"github.com/qianfree/team-api/relaykit/relayconvert"
	"github.com/qianfree/team-api/relaykit/relayconvert/convmeta"
	"github.com/qianfree/team-api/relaykit/types"
)

// OpenAIToGeminiStreamConverter 将 OpenAI Chat Completions 流式响应转换为 Gemini 流式响应。
type OpenAIToGeminiStreamConverter struct{}

func (c *OpenAIToGeminiStreamConverter) ID() string {
	return relayconvert.ResponseConverterOAIChatToGeminiChatStream
}

func (c *OpenAIToGeminiStreamConverter) From() types.RelayFormat {
	return types.RelayFormatOpenAI
}

func (c *OpenAIToGeminiStreamConverter) To() types.RelayFormat {
	return types.RelayFormatGemini
}

func (c *OpenAIToGeminiStreamConverter) Quality() relayconvert.ResponseConverterQuality {
	return relayconvert.ResponseConverterQualityGood
}

// ConvertStreamResponse 将上游 OpenAI SSE 流转换为 Gemini 流式 GenerateContentResponse 帧。
// Gemini 客户端为纯 data 帧风格，每帧以 StreamEvent{Event: ""} 输出；
// 最终帧携带 usageMetadata 时同时填充 StreamEvent.Usage 供宿主捕获计费用量。
func (c *OpenAIToGeminiStreamConverter) ConvertStreamResponse(
	ctx context.Context,
	info convmeta.Meta,
	reader io.Reader,
	chunkWriter func(chunk any) error,
) error {
	scanner := bufio.NewScanner(reader)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 10*1024*1024)

	var (
		usage        dto.UsageWithDetails
		finishReason string
	)

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		line := scanner.Text()
		if line == "" {
			continue
		}

		if !strings.HasPrefix(line, "data:") {
			continue
		}

		data := strings.TrimPrefix(line, "data:")
		data = strings.TrimSpace(data)

		if data == "" {
			continue
		}
		if data == "[DONE]" {
			break
		}

		var streamResp dto.ChatCompletionStreamResponse
		if err := json.Unmarshal([]byte(data), &streamResp); err != nil {
			continue
		}

		// 收集 usage（通常在最后一个 chunk）
		if streamResp.Usage != nil {
			usage.PromptTokens = streamResp.Usage.PromptTokens
			usage.CompletionTokens = streamResp.Usage.CompletionTokens
			usage.TotalTokens = streamResp.Usage.TotalTokens
			usage.PromptTokensDetails = streamResp.Usage.PromptTokensDetails
			usage.CompletionTokenDetails = streamResp.Usage.CompletionTokenDetails
		}

		// 构造 Gemini 响应 chunk
		geminiChunk := dto.GeminiChatResponse{}

		for _, choice := range streamResp.Choices {
			if choice.FinishReason != nil && *choice.FinishReason != "" {
				finishReason = *choice.FinishReason
			}

			parts := buildGeminiPartsFromDelta(&choice.Delta)
			if len(parts) == 0 && choice.FinishReason == nil {
				continue
			}

			candidate := dto.GeminiCandidate{
				Index: choice.Index,
				Content: &dto.GeminiContent{
					Role:  "model",
					Parts: parts,
				},
			}

			geminiChunk.Candidates = append(geminiChunk.Candidates, candidate)
		}

		if len(geminiChunk.Candidates) > 0 {
			if err := chunkWriter(&relayconvert.StreamEvent{Data: &geminiChunk}); err != nil {
				return err
			}
		}
	}

	// 发送包含 finishReason 和 usageMetadata 的最终 chunk
	finalChunk := dto.GeminiChatResponse{}
	reason := mapOpenAIFinishReasonToGemini(finishReason)
	finalChunk.Candidates = []dto.GeminiCandidate{{
		Content: &dto.GeminiContent{
			Role: "model",
		},
		FinishReason: reason,
	}}
	if usage.PromptTokens > 0 || usage.CompletionTokens > 0 {
		// Gemini 语义：CandidatesTokenCount 不含 thoughts，OpenAI CompletionTokens 已含 reasoning
		// 需要扣减 reasoning 避免双计（Gemini 客户端按 total = prompt + candidates + thoughts 汇总）
		candidatesTokens := usage.CompletionTokens
		if usage.CompletionTokenDetails != nil && usage.CompletionTokenDetails.ReasoningTokens > 0 {
			candidatesTokens -= usage.CompletionTokenDetails.ReasoningTokens
			if candidatesTokens < 0 {
				candidatesTokens = 0
			}
		}

		finalChunk.UsageMetadata = &dto.GeminiUsageMetadata{
			PromptTokenCount:     usage.PromptTokens,
			CandidatesTokenCount: candidatesTokens,
			TotalTokenCount:      usage.TotalTokens,
		}
		if usage.PromptTokensDetails != nil {
			finalChunk.UsageMetadata.CachedContentTokenCount = usage.PromptTokensDetails.CachedTokens
		}
		if usage.CompletionTokenDetails != nil {
			finalChunk.UsageMetadata.ThoughtsTokenCount = usage.CompletionTokenDetails.ReasoningTokens
		}
	}
	// 最终帧同时携带 usage（含 token 明细，缓存明细影响计费），供宿主捕获为本次请求的计费用量
	capturedUsage := usage
	if err := chunkWriter(&relayconvert.StreamEvent{
		Data:  &finalChunk,
		Usage: &capturedUsage,
	}); err != nil {
		return err
	}

	if err := scanner.Err(); err != nil && err != io.EOF && ctx.Err() == nil {
		return fmt.Errorf("stream scanner error: %w", err)
	}

	return nil
}

// buildGeminiPartsFromDelta 将 OpenAI 流式 Delta 转换为 Gemini Parts
func buildGeminiPartsFromDelta(delta *dto.Message) []dto.GeminiPart {
	var parts []dto.GeminiPart

	// thinking 内容
	if delta.ReasoningContent != nil && *delta.ReasoningContent != "" {
		parts = append(parts, dto.GeminiPart{
			Text:    *delta.ReasoningContent,
			Thought: boolPtr(true),
		})
	}

	// 文本内容
	if text, ok := delta.Content.(string); ok && text != "" {
		parts = append(parts, dto.GeminiPart{Text: text})
	}

	// 工具调用
	for _, tc := range delta.ToolCalls {
		var args any
		if tc.Function.Arguments != "" {
			if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil {
				args = map[string]any{}
			}
		}
		parts = append(parts, dto.GeminiPart{
			FunctionCall: &dto.GeminiFunctionCall{
				ID:           tc.ID,
				FunctionName: tc.Function.Name,
				Arguments:    args,
			},
		})
	}

	return parts
}

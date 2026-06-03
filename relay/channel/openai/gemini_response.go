package openai

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/qianfree/team-api/relay/common"
	"github.com/qianfree/team-api/relay/constant"
	"github.com/qianfree/team-api/relay/dto"
	"github.com/qianfree/team-api/relay/helper"
)

// handleGeminiInboundNonStream 将 OpenAI 非流式响应转换为 Gemini 格式
func handleGeminiInboundNonStream(ctx context.Context, resp *http.Response, info *common.RelayInfo, writer http.ResponseWriter) (*common.Usage, error) {
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response body failed: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		writeOpenAIErrorAsGemini(writer, body, resp.StatusCode)
		return &common.Usage{}, constant.NewUpstreamError(resp.StatusCode, string(body), nil)
	}

	var openaiResp dto.ChatCompletionResponse
	if err := json.Unmarshal(body, &openaiResp); err != nil {
		return nil, fmt.Errorf("invalid response body: %w", err)
	}

	geminiResp := openAIToGeminiResponse(&openaiResp, info)

	respBody, _ := json.Marshal(geminiResp)
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(http.StatusOK)
	_, _ = writer.Write(respBody)

	usage := &common.Usage{
		PromptTokens:           openaiResp.Usage.PromptTokens,
		CompletionTokens:       openaiResp.Usage.CompletionTokens,
		TotalTokens:            openaiResp.Usage.TotalTokens,
		PromptTokensDetails:    common.DtoTokenDetailsToCommon(openaiResp.Usage.PromptTokensDetails),
		CompletionTokenDetails: common.DtoTokenDetailsToCommon(openaiResp.Usage.CompletionTokenDetails),
		CacheIncludedInPrompt:  true,
	}
	return usage, nil
}

// handleGeminiInboundStream 将 OpenAI SSE 流转换为 Gemini SSE 流
func handleGeminiInboundStream(ctx context.Context, resp *http.Response, info *common.RelayInfo, writer http.ResponseWriter) (*common.Usage, error) {
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		writeOpenAIErrorAsGemini(writer, body, resp.StatusCode)
		return &common.Usage{}, constant.NewUpstreamError(resp.StatusCode, string(body), nil)
	}

	helper.SetEventStreamHeaders(writer)
	var writeMu sync.Mutex
	defer helper.PingTicker(writer, 15*time.Second, &writeMu)()

	scanner := bufio.NewScanner(resp.Body)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 10*1024*1024)

	var (
		usage        common.Usage
		finishReason string
	)

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			info.StreamStatus.SetEndReason(common.StreamEndReasonClientGone, ctx.Err())
			return &usage, common.ErrStreamInterrupted
		default:
		}

		line := scanner.Text()
		if line == "" {
			continue
		}

		if !strings.HasPrefix(line, "data:") {
			continue
		}

		data, _ := helper.ExtractSSEData(line)

		if data != "" && data != "[DONE]" {
			info.SetFirstResponseTime()
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
			usage.PromptTokensDetails = common.DtoTokenDetailsToCommon(streamResp.Usage.PromptTokensDetails)
			usage.CompletionTokenDetails = common.DtoTokenDetailsToCommon(streamResp.Usage.CompletionTokenDetails)
			usage.CacheIncludedInPrompt = true
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
			chunkData, _ := json.Marshal(geminiChunk)
			_ = helper.WriteSSEData(writer, string(chunkData))
		}
	}

	// 发送包含 finishReason 和 usageMetadata 的最终 chunk
	finalChunk := dto.GeminiChatResponse{}
	reason := common.OpenAIFinishReasonToGemini(finishReason)
	finalChunk.Candidates = []dto.GeminiCandidate{{
		Content: &dto.GeminiContent{
			Role: "model",
		},
		FinishReason: reason,
	}}
	if usage.PromptTokens > 0 || usage.CompletionTokens > 0 {
		finalChunk.UsageMetadata = &dto.GeminiUsageMetadata{
			PromptTokenCount:     usage.PromptTokens,
			CandidatesTokenCount: usage.CompletionTokens,
			TotalTokenCount:      usage.TotalTokens,
		}
		if usage.PromptTokensDetails != nil {
			finalChunk.UsageMetadata.CachedContentTokenCount = usage.PromptTokensDetails.CachedTokens
		}
		if usage.CompletionTokenDetails != nil {
			finalChunk.UsageMetadata.ThoughtsTokenCount = usage.CompletionTokenDetails.ReasoningTokens
		}
	}
	chunkData, _ := json.Marshal(finalChunk)
	_ = helper.WriteSSEData(writer, string(chunkData))

	helper.WriteSSEData(writer, "[DONE]")
	info.StreamStatus.SetEndReason(common.StreamEndReasonDone, nil)

	if err := scanner.Err(); err != nil && err != io.EOF && ctx.Err() == nil {
		info.StreamStatus.SetEndReason(common.StreamEndReasonError, err)
		return &usage, fmt.Errorf("stream scanner error: %w", err)
	}

	return &usage, nil
}

// openAIToGeminiResponse 将 OpenAI ChatCompletion 响应转换为 Gemini Chat 响应
func openAIToGeminiResponse(openaiResp *dto.ChatCompletionResponse, info *common.RelayInfo) dto.GeminiChatResponse {
	resp := dto.GeminiChatResponse{}

	if len(openaiResp.Choices) == 0 {
		return resp
	}

	choice := openaiResp.Choices[0]
	parts := buildGeminiPartsFromMessage(&choice.Message)

	candidate := dto.GeminiCandidate{
		Index: choice.Index,
		Content: &dto.GeminiContent{
			Role:  "model",
			Parts: parts,
		},
		FinishReason: common.OpenAIFinishReasonToGemini(choice.FinishReason),
	}

	resp.Candidates = []dto.GeminiCandidate{candidate}

	resp.UsageMetadata = &dto.GeminiUsageMetadata{
		PromptTokenCount:     openaiResp.Usage.PromptTokens,
		CandidatesTokenCount: openaiResp.Usage.CompletionTokens,
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

// writeOpenAIErrorAsGemini 将上游 OpenAI 错误转换为 Gemini RPC Status 格式写入响应
func writeOpenAIErrorAsGemini(writer http.ResponseWriter, body []byte, defaultStatusCode int) {
	var openaiErr struct {
		Error struct {
			Message string `json:"message"`
			Type    string `json:"type"`
			Code    any    `json:"code"`
		} `json:"error"`
	}
	statusCode := defaultStatusCode
	message := string(body)

	if err := json.Unmarshal(body, &openaiErr); err == nil && openaiErr.Error.Message != "" {
		message = openaiErr.Error.Message
		statusCode = openAIErrorTypeToHTTPStatus(openaiErr.Error.Type, defaultStatusCode)
	}

	geminiErr, _ := json.Marshal(map[string]any{
		"error": map[string]any{
			"code":    statusCode,
			"message": message,
			"status":  openAIErrorTypeToGeminiStatus(openaiErr.Error.Type),
		},
	})
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(statusCode)
	_, _ = writer.Write(geminiErr)
}

// openAIErrorTypeToHTTPStatus 将 OpenAI error type 映射为 HTTP 状态码
func openAIErrorTypeToHTTPStatus(errorType string, defaultCode int) int {
	switch errorType {
	case "authentication_error":
		return 401
	case "permission_error":
		return 403
	case "invalid_request_error":
		return 400
	case "rate_limit_error":
		return 429
	case "server_error", "internal_error":
		return 500
	case "timeout_error":
		return 504
	default:
		return defaultCode
	}
}

// openAIErrorTypeToGeminiStatus 将 OpenAI error type 映射为 Gemini RPC status
func openAIErrorTypeToGeminiStatus(errorType string) string {
	switch errorType {
	case "authentication_error":
		return "UNAUTHENTICATED"
	case "permission_error":
		return "PERMISSION_DENIED"
	case "invalid_request_error":
		return "INVALID_ARGUMENT"
	case "rate_limit_error":
		return "RESOURCE_EXHAUSTED"
	case "server_error", "internal_error":
		return "INTERNAL"
	case "timeout_error":
		return "DEADLINE_EXCEEDED"
	default:
		return "INTERNAL"
	}
}

// boolPtr 返回 bool 的指针
func boolPtr(v bool) *bool {
	return &v
}

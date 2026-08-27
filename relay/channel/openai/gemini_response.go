package openai

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/qianfree/team-api/relay/common"
	"github.com/qianfree/team-api/relay/constant"
	"github.com/qianfree/team-api/relay/dto"
	"github.com/qianfree/team-api/relay/relaykit_bridge"
)

// handleGeminiInboundNonStream 将 OpenAI 非流式响应转换为 Gemini 格式。
// relaykit 唯一路径（P1-B/P3；legacy 回退已收割）：未接管按转换失败报错。
func handleGeminiInboundNonStream(ctx context.Context, resp *http.Response, info *common.RelayInfo, writer http.ResponseWriter) (*common.Usage, error) {
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response body failed: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		writeOpenAIErrorAsGemini(writer, body, resp.StatusCode)
		upstreamErr := constant.NewUpstreamError(resp.StatusCode, string(body), nil).WithRetryAfter(constant.RetryAfterFromHeader(resp.Header))
		upstreamErr.ResponseWritten = true
		return &common.Usage{}, upstreamErr
	}

	var openaiResp dto.ChatCompletionResponse
	if err := json.Unmarshal(body, &openaiResp); err != nil {
		return nil, fmt.Errorf("invalid response body: %w", err)
	}

	convertedBody, _, ok := relaykit_bridge.TryConvertResponseViaRelaykit(ctx, info, body)
	if !ok {
		return nil, fmt.Errorf("[relaykit] openai→gemini 响应转换失败")
	}
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(http.StatusOK)
	_, _ = writer.Write(convertedBody)

	// P3（UseResponsesAPI，上游为 /v1/responses）：上方按 chat 格式解析 responses 体
	// 必得零值，计费 usage 须从上游 responses 体提取（OpenAI 口径）
	if info.UseResponsesAPI {
		return relaykit_bridge.UsageFromResponsesBody(body), nil
	}

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

// handleGeminiInboundStream 将 OpenAI SSE 流转换为 Gemini SSE 流。
// relaykit 唯一路径（P2；legacy 回退已收割）：未接管按转换失败报错。
// 转换中途失败由桥接层优雅降级（补终止 chunk + [DONE] + end reason），不走本函数。
func handleGeminiInboundStream(ctx context.Context, resp *http.Response, info *common.RelayInfo, writer http.ResponseWriter) (*common.Usage, error) {
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		writeOpenAIErrorAsGemini(writer, body, resp.StatusCode)
		upstreamErr := constant.NewUpstreamError(resp.StatusCode, string(body), nil).WithRetryAfter(constant.RetryAfterFromHeader(resp.Header))
		upstreamErr.ResponseWritten = true
		return &common.Usage{}, upstreamErr
	}

	usage, ok, streamErr := relaykit_bridge.TryConvertStreamViaRelaykit(ctx, info, resp.Body, writer)
	if !ok {
		resp.Body.Close()
		if streamErr != nil {
			return nil, streamErr // 入口 ctx 已取消：透传中断错误，不再尝试写客户端
		}
		return nil, fmt.Errorf("[relaykit] openai→gemini 流式转换失败（无匹配转换器或转换失败）")
	}
	resp.Body.Close()
	return usage, streamErr
}

// openAIToGeminiResponse / buildGeminiParts* 等 legacy 转换已随 relaykit 收割删除：
// openai→gemini 方向统一走 relaykit_bridge.TryConvertResponse/TryConvertStreamViaRelaykit。

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

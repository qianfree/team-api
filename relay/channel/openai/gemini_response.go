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

// Gemini 入站（客户端说 Gemini、上游说 OpenAI）的响应侧接线。
// 协议转换本身在 relaykit（internal/oai_gemini 的 OpenAIToGemini 转换器）；
// 本文件只保留宿主职责：HTTP 状态码处理、错误按 Gemini 原生格式透传、
// 响应写出与计费用量提取。

// handleGeminiInboundNonStream 将 OpenAI 非流式响应转换为 Gemini 格式
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

	// relaykit 响应转换（唯一路径，hard-fail）
	convertedBody, _, handled, convErr := relaykit_bridge.TryConvertResponseViaRelaykit(ctx, info, body)
	if !handled {
		return nil, constant.NewChannelError("openai adaptor: no relaykit converter for gemini client response", nil)
	}
	if convErr != nil {
		return nil, fmt.Errorf("invalid response body: %w", convErr)
	}
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(http.StatusOK)
	_, _ = writer.Write(convertedBody)

	// 计费用量取上游 OpenAI 原始口径（prompt 含缓存，cached 为其子集）
	usage := &common.Usage{CacheIncludedInPrompt: true}
	var openaiResp dto.ChatCompletionResponse
	if err := json.Unmarshal(body, &openaiResp); err == nil {
		usage.PromptTokens = openaiResp.Usage.PromptTokens
		usage.CompletionTokens = openaiResp.Usage.CompletionTokens
		usage.TotalTokens = openaiResp.Usage.TotalTokens
		usage.PromptTokensDetails = common.DtoTokenDetailsToCommon(openaiResp.Usage.PromptTokensDetails)
		usage.CompletionTokenDetails = common.DtoTokenDetailsToCommon(openaiResp.Usage.CompletionTokenDetails)
	}
	return usage, nil
}

// handleGeminiInboundStream 将 OpenAI SSE 流转换为 Gemini SSE 流
func handleGeminiInboundStream(ctx context.Context, resp *http.Response, info *common.RelayInfo, writer http.ResponseWriter) (*common.Usage, error) {
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		writeOpenAIErrorAsGemini(writer, body, resp.StatusCode)
		upstreamErr := constant.NewUpstreamError(resp.StatusCode, string(body), nil).WithRetryAfter(constant.RetryAfterFromHeader(resp.Header))
		upstreamErr.ResponseWritten = true
		return &common.Usage{}, upstreamErr
	}

	// relaykit 流式转换（唯一路径，hard-fail）：桥接层负责 Gemini data 帧写出与 usage 捕获
	if usage, ok, err := relaykit_bridge.TryConvertStreamViaRelaykit(ctx, info, resp.Body, writer); ok {
		// 桥接层已写出 SSE 并完成收尾裁决：err 非空即流以错误/客户端中断结束
		//（已带 ResponseWritten，上层只记账与上报调度，不重写响应体、不换渠道重试），
		// usage 中断兜底亦已在桥接层完成，此处原样上抛即可。
		return usage, err
	}
	return nil, constant.NewChannelError("openai adaptor: relaykit stream converter unavailable for gemini client", nil)
}

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

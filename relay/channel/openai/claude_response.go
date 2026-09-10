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

// Claude 入站（客户端说 Claude、上游说 OpenAI）的响应侧接线。
// 协议转换本身在 relaykit（internal/oai_chat 的 OpenAIToClaude 转换器）；
// 本文件只保留宿主职责：HTTP 状态码处理、错误按 Claude 原生格式透传、
// 响应写出与计费用量提取。

// handleClaudeInboundNonStream 将 OpenAI 非流式响应转换为 Claude 格式
func handleClaudeInboundNonStream(ctx context.Context, resp *http.Response, info *common.RelayInfo, writer http.ResponseWriter) (*common.Usage, error) {
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response body failed: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		// 将上游错误转换为 Claude 格式透传给客户端
		writer.Header().Set("Content-Type", "application/json")
		writer.WriteHeader(resp.StatusCode)
		claudeErr, _ := json.Marshal(map[string]any{
			"type":  "error",
			"error": map[string]any{"type": "api_error", "message": string(body)},
		})
		_, _ = writer.Write(claudeErr)
		upstreamErr := constant.NewUpstreamError(resp.StatusCode, string(body), nil).WithRetryAfter(constant.RetryAfterFromHeader(resp.Header))
		upstreamErr.ResponseWritten = true
		return &common.Usage{}, upstreamErr
	}

	// relaykit 响应转换（唯一路径，hard-fail）
	convertedBody, _, handled, convErr := relaykit_bridge.TryConvertResponseViaRelaykit(ctx, info, body)
	if !handled {
		return nil, constant.NewChannelError("openai adaptor: no relaykit converter for claude client response", nil)
	}
	if convErr != nil {
		return nil, fmt.Errorf("invalid response body: %w", convErr)
	}
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(http.StatusOK)
	_, _ = writer.Write(convertedBody)

	// 计费用量取上游 OpenAI 原始口径（prompt 含缓存，cached 为其子集）
	usage := &common.Usage{}
	var openaiResp dto.ChatCompletionResponse
	if err := json.Unmarshal(body, &openaiResp); err == nil && len(openaiResp.Choices) > 0 {
		usage.PromptTokens = openaiResp.Usage.PromptTokens
		usage.CompletionTokens = openaiResp.Usage.CompletionTokens
		usage.TotalTokens = openaiResp.Usage.TotalTokens
		usage.PromptTokensDetails = common.DtoTokenDetailsToCommon(openaiResp.Usage.PromptTokensDetails)
		usage.CompletionTokenDetails = common.DtoTokenDetailsToCommon(openaiResp.Usage.CompletionTokenDetails)
	}
	usage.CacheIncludedInPrompt = true
	return usage, nil
}

// handleClaudeInboundStream 将 OpenAI SSE 流转换为 Claude SSE 流
func handleClaudeInboundStream(ctx context.Context, resp *http.Response, info *common.RelayInfo, writer http.ResponseWriter) (*common.Usage, error) {
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		writer.Header().Set("Content-Type", "application/json")
		writer.WriteHeader(resp.StatusCode)
		claudeErr, _ := json.Marshal(map[string]any{
			"type":  "error",
			"error": map[string]any{"type": "api_error", "message": string(body)},
		})
		_, _ = writer.Write(claudeErr)
		upstreamErr := constant.NewUpstreamError(resp.StatusCode, string(body), nil).WithRetryAfter(constant.RetryAfterFromHeader(resp.Header))
		upstreamErr.ResponseWritten = true
		return &common.Usage{}, upstreamErr
	}

	// relaykit 流式转换（唯一路径，hard-fail）：桥接层负责 Claude 事件帧写出与 usage 捕获
	if usage, ok, err := relaykit_bridge.TryConvertStreamViaRelaykit(ctx, info, resp.Body, writer); ok {
		// 桥接层已写出 SSE 并完成收尾裁决：err 非空即流以错误/客户端中断结束
		//（已带 ResponseWritten，上层只记账与上报调度，不重写响应体、不换渠道重试），
		// usage 中断兜底亦已在桥接层完成，此处原样上抛即可。
		return usage, err
	}
	return nil, constant.NewChannelError("openai adaptor: relaykit stream converter unavailable for claude client", nil)
}

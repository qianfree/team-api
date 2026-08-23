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

// handleClaudeInboundNonStream 将 OpenAI 非流式响应转换为 Claude 格式。
// relaykit 唯一路径（P1-B/P3；legacy 回退已收割）：未接管按转换失败报错。
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

	var openaiResp dto.ChatCompletionResponse
	if err := json.Unmarshal(body, &openaiResp); err != nil {
		return nil, fmt.Errorf("invalid response body: %w", err)
	}

	convertedBody, _, ok := relaykit_bridge.TryConvertResponseViaRelaykit(ctx, info, body)
	if !ok {
		return nil, fmt.Errorf("[relaykit] openai→claude 响应转换失败")
	}
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(http.StatusOK)
	_, _ = writer.Write(convertedBody)

	// P3（UseResponsesAPI，上游为 /v1/responses）：上方按 chat 格式解析 responses 体
	// 必得零值，计费 usage 须从上游 responses 体提取（OpenAI 口径）
	if info.UseResponsesAPI {
		return relaykit_bridge.UsageFromResponsesBody(body), nil
	}

	// 计费 usage 提取保持既有口径（choices>0 守卫怪癖保留；与转换路径无关）
	usage := &common.Usage{}
	if len(openaiResp.Choices) > 0 {
		usage.PromptTokens = openaiResp.Usage.PromptTokens
		usage.CompletionTokens = openaiResp.Usage.CompletionTokens
		usage.TotalTokens = openaiResp.Usage.TotalTokens
		usage.PromptTokensDetails = common.DtoTokenDetailsToCommon(openaiResp.Usage.PromptTokensDetails)
		usage.CompletionTokenDetails = common.DtoTokenDetailsToCommon(openaiResp.Usage.CompletionTokenDetails)
	}
	usage.CacheIncludedInPrompt = true
	return usage, nil
}

// handleClaudeInboundStream 将 OpenAI SSE 流转换为 Claude SSE 流。
// relaykit 唯一路径（P2；legacy 回退已收割）：未接管按转换失败报错。
// 转换中途失败由桥接层优雅降级（补 message_delta/message_stop + end reason），不走本函数。
func handleClaudeInboundStream(ctx context.Context, resp *http.Response, info *common.RelayInfo, writer http.ResponseWriter) (*common.Usage, error) {
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
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

	usage, ok := relaykit_bridge.TryConvertStreamViaRelaykit(ctx, info, resp.Body, writer)
	if !ok {
		resp.Body.Close()
		return nil, fmt.Errorf("[relaykit] openai→claude 流式转换失败（无匹配转换器或转换失败）")
	}
	resp.Body.Close()
	return usage, nil
}

// openAIToClaudeResponse / writeClaudeSSE 等 legacy 转换已随 relaykit 收割删除：
// openai→claude 方向统一走 relaykit_bridge.TryConvertResponse/TryConvertStreamViaRelaykit。

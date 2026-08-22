package openai

import (
	"context"
	"fmt"
	"io"
	"net/http"

	"github.com/gogf/gf/v2/frame/g"

	"github.com/qianfree/team-api/relay/common"
	"github.com/qianfree/team-api/relay/constant"
	"github.com/qianfree/team-api/relay/relaykit_bridge"
)

// handleChatViaResponsesNonStream 处理 Chat Completions via Responses API 的非流式响应
func (a *Adaptor) handleChatViaResponsesNonStream(
	ctx context.Context,
	resp *http.Response,
	info *common.RelayInfo,
	writer http.ResponseWriter,
) (*common.Usage, error) {
	g.Log().Infof(ctx, "[OpenAI] Chat via Responses bridge: non-stream mode")
	return HandleResponsesNonStreamToChat(ctx, resp, info, writer)
}

// handleChatViaResponsesStream 处理 Chat Completions via Responses API 的流式响应。
// relaykit 直达流式转换器唯一路径（legacy HandleResponsesStreamToChat 双实现已收割）：
// 非 200 在桥接写出 200 SSE 响应头之前由本层拦截透传，200 后经通用流式桥
// （responses→chat，bridgeUpstreamFormat 按 UseResponsesAPI 判定上游格式）转换输出。
func (a *Adaptor) handleChatViaResponsesStream(
	ctx context.Context,
	resp *http.Response,
	info *common.RelayInfo,
	writer http.ResponseWriter,
) (*common.Usage, error) {
	g.Log().Infof(ctx, "[OpenAI] Chat via Responses bridge: stream mode")

	// 非 200：上游在 SSE 开始前返回错误（非 SSE 体）。必须在桥接写出 200 SSE 响应头
	// 之前拦截（原 HandleResponsesStreamToChat 内的同款逻辑前移至此），否则错误体会
	// 被当作 SSE 流转发、且无法再向客户端传递状态码。
	if resp.StatusCode != http.StatusOK {
		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return nil, constant.NewUpstreamError(resp.StatusCode, "read response body failed", err).
				WithRetryAfter(constant.RetryAfterFromHeader(resp.Header))
		}
		if isUpstreamOpenAIError(body) {
			writeUpstreamErrorResponse(writer, resp.StatusCode, body)
			upstreamErr := constant.NewUpstreamError(resp.StatusCode, string(body), nil).
				WithRetryAfter(constant.RetryAfterFromHeader(resp.Header))
			upstreamErr.ResponseWritten = true
			return &common.Usage{}, upstreamErr
		}
		return nil, constant.NewUpstreamErrorFromResponse(resp, body)
	}

	usage, ok := relaykit_bridge.TryConvertStreamViaRelaykit(ctx, info, resp.Body, writer)
	resp.Body.Close()
	if !ok {
		return nil, fmt.Errorf("[relaykit] responses→chat 流式转换失败（无匹配转换器）")
	}
	return usage, nil
}

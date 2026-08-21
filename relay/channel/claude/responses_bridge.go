package claude

import (
	"context"
	"fmt"
	"io"
	"net/http"

	"github.com/qianfree/team-api/relay/common"
	"github.com/qianfree/team-api/relay/constant"
	"github.com/qianfree/team-api/relay/relaykit_bridge"
)

// ========== Responses 入站桥接：Claude Messages → OpenAI Responses ==========
//
// 请求侧由 ConvertResponsesToClaude 完成（Responses → OpenAI → Claude），
// 这里做响应侧：把 Claude 上游的响应/SSE 转回 Responses 格式，
// 事件发射器复用 openai 包（EmitResponsesSSE / BuildResponsesObjectMap），
// 与 chat→responses 桥接（openai/responses.go）的事件序列保持一致。

// handleNonStreamToResponses 将 Claude 非流式响应转换为 Responses 格式
func (a *Adaptor) handleNonStreamToResponses(ctx context.Context, resp *http.Response, info *common.RelayInfo, writer http.ResponseWriter) (*common.Usage, error) {
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, constant.NewUpstreamError(resp.StatusCode, "read response body failed", err).WithRetryAfter(constant.RetryAfterFromHeader(resp.Header))
	}
	if resp.StatusCode != http.StatusOK {
		return nil, constant.NewUpstreamError(resp.StatusCode, string(body), nil).WithRetryAfter(constant.RetryAfterFromHeader(resp.Header))
	}

	// relaykit 唯一路径（legacy 回退已收割）：未接管按转换失败报错
	responsesBody, usage, ok := relaykit_bridge.TryConvertResponsesResponseViaRelaykit(ctx, info, body)
	if !ok {
		return nil, fmt.Errorf("[relaykit] claude→responses 响应转换失败")
	}
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(http.StatusOK)
	_, _ = writer.Write(responsesBody)
	return usage, nil
}

// handleStreamToResponses 将 Claude 流式响应转换为 Responses 格式的 SSE。
// relaykit 唯一路径（legacy 回退已收割）：200 时由 adaptor 层 TryConvertResponsesStreamViaRelaykit
// 接管（未接管即报错）；本函数只兜非 200 的错误透传。
func (a *Adaptor) handleStreamToResponses(ctx context.Context, resp *http.Response, info *common.RelayInfo, writer http.ResponseWriter) (*common.Usage, error) {
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, constant.NewUpstreamError(resp.StatusCode, string(body), nil).WithRetryAfter(constant.RetryAfterFromHeader(resp.Header))
	}
	resp.Body.Close()
	return nil, fmt.Errorf("[relaykit] claude→responses 流式转换失败（无匹配转换器）")
}

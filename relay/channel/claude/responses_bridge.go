package claude

import (
	"context"
	"encoding/json"
	"io"
	"net/http"

	"github.com/qianfree/team-api/relay/common"
	"github.com/qianfree/team-api/relay/constant"
	"github.com/qianfree/team-api/relay/dto"
	"github.com/qianfree/team-api/relay/relaykit_bridge"
)

// ========== Responses 入站桥接：Claude Messages → OpenAI Responses ==========
//
// 请求侧由 relaykit 的步骤链完成（Responses → OpenAI 中枢 → Claude），
// 响应侧同样由 relaykit 转换器完成（Claude → Responses，含 SSE 事件序列产出）。
// 本文件只保留宿主侧接线：HTTP 状态码/错误处理、响应写出与计费用量提取。
//
// 计费口径：写给客户端的 usage 为 OpenAI 语义（input 含缓存，cached 为子集，由转换器产出），
// 返回给上层计费的是 Claude 原生口径（input 不含缓存、cache 独立列），
// 因此非流式路径仍从**原始上游响应体**提取 usage。

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

	// relaykit 响应转换（唯一路径，hard-fail：解析/转换失败即向上返回错误，不再回退旧实现）
	convertedBody, _, handled, convErr := relaykit_bridge.TryConvertResponseViaRelaykit(ctx, info, body)
	if !handled {
		return nil, constant.NewChannelError("claude adaptor: no relaykit converter for responses client response", nil)
	}
	if convErr != nil {
		return nil, constant.NewUpstreamError(resp.StatusCode, "invalid response body", convErr).WithRetryAfter(constant.RetryAfterFromHeader(resp.Header))
	}
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(http.StatusOK)
	_, _ = writer.Write(convertedBody)

	// 计费返回值按 Claude 口径（input 不含缓存，cache 独立列）
	var claudeResp dto.ClaudeResponse
	if err := json.Unmarshal(body, &claudeResp); err != nil {
		// Usage 解析失败，返回空 Usage（已写响应，不能重试；非致命错误）
		return &common.Usage{}, nil
	}
	if claudeResp.Usage != nil {
		return buildUsageFromClaude(claudeResp.Usage), nil
	}
	return &common.Usage{}, nil
}

// handleStreamToResponses 将 Claude 流式响应转换为 Responses 格式的 SSE
func (a *Adaptor) handleStreamToResponses(ctx context.Context, resp *http.Response, info *common.RelayInfo, writer http.ResponseWriter) (*common.Usage, error) {
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, constant.NewUpstreamError(resp.StatusCode, string(body), nil).WithRetryAfter(constant.RetryAfterFromHeader(resp.Header))
	}

	// relaykit 流式转换（唯一路径，hard-fail）：写入前失败由桥接层返回未接管，此处显式报错
	if usage, ok, err := relaykit_bridge.TryConvertStreamViaRelaykit(ctx, info, resp.Body, writer); ok {
		// 桥接层已写出 SSE 并完成收尾裁决：err 非空即流以错误/客户端中断结束
		//（已带 ResponseWritten，上层只记账与上报调度，不重写响应体、不换渠道重试），
		// usage 中断兜底亦已在桥接层完成，此处原样上抛即可。
		return usage, err
	}
	return nil, constant.NewChannelError("claude adaptor: relaykit stream converter unavailable for responses client", nil)
}

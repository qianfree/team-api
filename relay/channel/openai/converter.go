package openai

import (
	"context"
	"fmt"
	"io"
	"net/http"

	"github.com/qianfree/team-api/relay/common"
	"github.com/qianfree/team-api/relay/constant"
	"github.com/qianfree/team-api/relay/relaykit_bridge"
)

// ConvertToOpenAI 根据入站格式将请求转换为 OpenAI 格式。
// 如果入站已是 OpenAI 格式（或空），原样返回。
// 其他供应商适配器可调用此函数统一处理入站格式预转换。
//
// claude/gemini/responses 入站：relaykit 唯一路径（P1-A 接管点在此函数内部而非 handler
// 桥接——20+ 个 openai 兼容 adaptor 在本函数之后各有定制后处理，接管点在内部可保持后处理
// 不变。responses 入站对 ollama/coze/dify 等原生格式上游也在此转换——handler 桥的 responses
// 路由只认 openai/claude/gemini 上游）。legacy 回退转换器已收割：转换失败直接报错，
// 问题经 monitor.TrackConverterCall 显式暴露。
func ConvertToOpenAI(requestBody []byte, info *common.RelayInfo) ([]byte, error) {
	switch info.InboundFormat {
	case constant.RelayFormatClaude, constant.RelayFormatGemini, constant.RelayFormatResponses:
		out, ok, err := relaykit_bridge.TryConvertInboundToOpenAIChat(context.Background(), info, requestBody)
		// 有状态哨兵（ErrStatefulResponsesUnsupported）：透传给 adaptor → convertRequestBody →
		// relay_handler 的哨兵判定点驱动 FSM 换渠道，不得降级为普通转换失败
		if err != nil {
			return nil, err
		}
		if ok {
			return out, nil
		}
		// ok=false 仅剩「入站格式不在桥接覆盖范围」一种可能（防御分支，正常不可达）
		return nil, fmt.Errorf("[relaykit] %s→openai 请求转换失败（无匹配转换器）", info.InboundFormat)
	default:
		return requestBody, nil
	}
}

// HandleResponsesNonStreamToChat 将 Responses API 非流式响应转换为 Chat Completions 并写入 writer。
// relaykit 唯一路径（openai 客户端 × UseResponsesAPI 渠道的非流式响应）；失败直接报错。
func HandleResponsesNonStreamToChat(ctx context.Context, resp *http.Response, info *common.RelayInfo, writer http.ResponseWriter) (*common.Usage, error) {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response body: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		writer.Header().Set("Content-Type", "application/json")
		writer.WriteHeader(resp.StatusCode)
		_, _ = writer.Write(body)
		// 错误体已写给客户端，标记 ResponseWritten 防止调度 FSM 误判为可重试导致二次写入
		upstreamErr := constant.NewUpstreamError(resp.StatusCode, string(body), nil).WithRetryAfter(constant.RetryAfterFromHeader(resp.Header))
		upstreamErr.ResponseWritten = true
		return nil, upstreamErr
	}

	chatBody, usage, ok := relaykit_bridge.TryConvertChatViaResponsesResponseViaRelaykit(ctx, info, body)
	if !ok {
		return nil, fmt.Errorf("[relaykit] responses→chat 非流式响应转换失败")
	}
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(http.StatusOK)
	_, _ = writer.Write(chatBody)
	return usage, nil
}

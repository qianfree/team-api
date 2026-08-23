package dify

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/qianfree/team-api/relay/common"
	"github.com/qianfree/team-api/relay/constant"
	"github.com/qianfree/team-api/relay/override"
	"github.com/qianfree/team-api/relay/relaykit_bridge"
)

// Adaptor Dify 供应商适配器。
// Dify 是开源 LLM 应用开发平台，通过 App API 提供对话能力。
// 本适配器将 OpenAI Chat Completions 请求转换为 Dify chat-messages 格式，
// 并将 Dify 响应（blocking/streaming）转换回 OpenAI 格式。
type Adaptor struct {
	info *common.RelayInfo
}

func (a *Adaptor) Init(info *common.RelayInfo) {
	a.info = info
}

// GetRequestURL 构建上游请求 URL。Dify chat-messages 端点: {baseURL}/v1/chat-messages
func (a *Adaptor) GetRequestURL(info *common.RelayInfo) (string, error) {
	baseURL := strings.TrimSuffix(info.ChannelMeta.BaseURL, "/")

	switch constant.RelayMode(info.RelayMode) {
	case constant.RelayModeChatCompletions:
		return baseURL + "/v1/chat-messages", nil
	default:
		return "", fmt.Errorf("unsupported relay mode for Dify: %d", info.RelayMode)
	}
}

// SetupRequestHeader 设置上游请求头。ApiKey 为 Dify App API Key。
func (a *Adaptor) SetupRequestHeader(header http.Header, info *common.RelayInfo) error {
	header.Set("Authorization", "Bearer "+info.ChannelMeta.ApiKey)
	header.Set("Content-Type", "application/json")
	header.Set("Accept", "application/json")
	return nil
}

// ConvertRequest 请求侧转换已由 handler 层 relaykit 桥接统一接管
// （openai 入站 → ConverterOpenAIChatToDify，streaming 模式按 info.IsStream 选择）。
// 走到本函数说明桥接未接管，按转换失败报错：
//   - legacy 本地转换已随收割删除（原 convertOpenAIToDify 平行实现，防语义漂移）；
//   - 交叉客户端（claude/gemini/responses）× dify 上游：响应侧只有 dify→openai 转换器，
//     不 fail-fast 会形成「请求已发上游、响应转换失败」，白耗上游 token 且触发重试风暴。
func (a *Adaptor) ConvertRequest(ctx context.Context, info *common.RelayInfo, requestBody []byte) (io.Reader, error) {
	return nil, fmt.Errorf("[relaykit] %s→dify 请求转换失败（handler 层桥接未接管）", info.InboundFormat)
}

// DoRequest 发送请求到 Dify 上游
func (a *Adaptor) DoRequest(ctx context.Context, info *common.RelayInfo, requestBody io.Reader) (*http.Response, error) {
	reqURL, err := a.GetRequestURL(info)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL, requestBody)
	if err != nil {
		return nil, fmt.Errorf("create request failed: %w", err)
	}

	if err := a.SetupRequestHeader(httpReq.Header, info); err != nil {
		return nil, fmt.Errorf("setup request header failed: %w", err)
	}

	if hdrOverrides, hdrErr := override.ApplyHeaderOverride(info); hdrErr == nil && len(hdrOverrides) > 0 {
		override.MergeHeaderOverrides(httpReq.Header, hdrOverrides)
	}

	timeout := info.ChannelMeta.Settings.TimeoutSeconds
	if timeout <= 0 {
		timeout = 120 // Dify 应用可能包含复杂工作流，给更长超时
	}
	if constant.RelayMode(info.RelayMode) == constant.RelayModeImagesGenerations && timeout < 600 {
		timeout = 600
	}

	client := common.NewPooledClient(timeout, info.ChannelMeta.Settings.UseProxy, info.IsStream)

	return client.Do(httpReq)
}

// DoResponse 处理 Dify 上游响应，转换为 OpenAI 格式。
//
// Dify blocking 模式返回 JSON:
//
//	{"answer": "...", "metadata": {"usage": {"total_tokens": N}}}
//
// Dify streaming 模式返回 SSE:
//
//	data: {"event": "message", "answer": "chunk text"}
//	data: {"event": "message_end", "metadata": {"usage": {...}}}
func (a *Adaptor) DoResponse(ctx context.Context, resp *http.Response, info *common.RelayInfo, writer http.ResponseWriter) (*common.Usage, error) {
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, constant.NewUpstreamError(resp.StatusCode, string(body), nil)
	}

	if info.IsStream {
		return a.handleStreamResponse(ctx, resp, info, writer)
	}
	return a.handleNonStreamResponse(ctx, resp, info, writer)
}

// handleNonStreamResponse 处理 Dify blocking 模式响应
func (a *Adaptor) handleNonStreamResponse(ctx context.Context, resp *http.Response, info *common.RelayInfo, writer http.ResponseWriter) (*common.Usage, error) {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read Dify response body failed: %w", err)
	}

	info.SetFirstResponseTime()

	// relaykit 唯一路径（legacy 回退已收割）：未接管按转换失败报错
	convertedBody, _, ok := relaykit_bridge.TryConvertResponseViaRelaykit(ctx, info, body)
	if !ok {
		return nil, fmt.Errorf("[relaykit] dify→openai 响应转换失败")
	}
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(http.StatusOK)
	_, _ = writer.Write(convertedBody)
	if usage, ok := relaykit_bridge.UsageFromConvertedChatResponse(convertedBody); ok {
		return usage, nil
	}
	return &common.Usage{}, nil
}

// handleStreamResponse 处理 Dify streaming 模式 SSE 响应
func (a *Adaptor) handleStreamResponse(ctx context.Context, resp *http.Response, info *common.RelayInfo, writer http.ResponseWriter) (*common.Usage, error) {
	// relaykit 唯一路径（legacy 回退已收割）：未接管按转换失败报错
	usage, ok := relaykit_bridge.TryConvertStreamViaRelaykit(ctx, info, resp.Body, writer)
	if !ok {
		return nil, fmt.Errorf("[relaykit] dify→openai 流式转换失败（无匹配转换器或转换失败）")
	}
	return usage, nil
}

func (a *Adaptor) GetChannelName() string {
	return ChannelName
}

var _ common.Adaptor = (*Adaptor)(nil)

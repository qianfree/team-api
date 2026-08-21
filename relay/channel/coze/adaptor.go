package coze

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/qianfree/team-api/relay/common"
	"github.com/qianfree/team-api/relay/constant"
	"github.com/qianfree/team-api/relay/override"
	"github.com/qianfree/team-api/relay/relaykit_bridge"
)

// Adaptor Coze 供应商适配器。
// Coze v3 API 支持流式 SSE 响应，本适配器将 Coze SSE 事件转换为 OpenAI 格式的 SSE 流。
// 非流式请求也通过流式端点收集完整响应后以 OpenAI JSON 格式返回。
type Adaptor struct {
	info *common.RelayInfo
}

func (a *Adaptor) Init(info *common.RelayInfo) {
	a.info = info
}

// GetRequestURL 构建上游请求 URL。Coze v3 chat 端点: {baseURL}/v3/chat
func (a *Adaptor) GetRequestURL(info *common.RelayInfo) (string, error) {
	baseURL := strings.TrimSuffix(info.ChannelMeta.BaseURL, "/")
	return baseURL + "/v3/chat", nil
}

// SetupRequestHeader 设置上游请求头。ApiKey 为 Coze Personal Access Token。
func (a *Adaptor) SetupRequestHeader(header http.Header, info *common.RelayInfo) error {
	header.Set("Authorization", "Bearer "+info.ChannelMeta.ApiKey)
	header.Set("Content-Type", "application/json")
	header.Set("Accept", "application/json")
	return nil
}

// ConvertRequest 将 OpenAI Chat 请求转换为 Coze v3 请求格式
func (a *Adaptor) ConvertRequest(ctx context.Context, info *common.RelayInfo, requestBody []byte) (io.Reader, error) {
	// 非 OpenAI 格式先转换为 OpenAI
	if info.InboundFormat != "" && info.InboundFormat != constant.RelayFormatOpenAI {
		// 交叉客户端（claude/gemini/responses）× coze 上游：请求侧虽可转换，但响应侧
		// 只有 coze→openai 转换器——不 fail-fast 会形成「请求已发上游、响应转换失败」，
		// 白耗上游 token 且触发全渠道重试风暴。响应侧组合链注册前不支持该方向
		return nil, fmt.Errorf("[relaykit] %s 客户端 × coze 上游暂不支持（响应侧无注册转换器）", info.InboundFormat)
	}

	// 非流式请求也强制使用流式模式，以便在 DoResponse 中统一处理
	cozeBody, err := convertOpenAIToCoze(requestBody, info)
	if err != nil {
		return nil, fmt.Errorf("convert to Coze request failed: %w", err)
	}

	// 强制开启流式模式：Coze 非流式需要轮询，实现复杂，
	// 这里统一走流式，非流式场景在 DoResponse 中收集完整响应后一次性返回
	var cozeReq CozeCreateRequest
	if err := json.Unmarshal(cozeBody, &cozeReq); err == nil {
		cozeReq.Stream = true
		cozeBody, _ = json.Marshal(cozeReq)
	}

	return bytes.NewReader(cozeBody), nil
}

// DoRequest 发送请求到 Coze 上游
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
		timeout = 120 // Coze 可能响应较慢，给更长超时
	}
	if constant.RelayMode(info.RelayMode) == constant.RelayModeImagesGenerations && timeout < 600 {
		timeout = 600
	}

	client := common.NewPooledClient(timeout, info.ChannelMeta.Settings.UseProxy, info.IsStream)

	return client.Do(httpReq)
}

// DoResponse 处理 Coze 上游 SSE 响应，转换为 OpenAI 格式。
// Coze SSE 事件格式:
//
//	event: conversation.message.delta
//	data: {"role":"assistant","type":"answer","content":"Hello"}
//
//	event: conversation.message.completed
//	data: {"role":"assistant","type":"answer","content":"Hello world"}
//
//	event: done
//	data: {}
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

// handleStreamResponse 流式模式：Coze SSE 经 relaykit 转换为 OpenAI SSE 输出。
// relaykit 唯一路径（legacy 回退已收割）：未接管按转换失败报错。
func (a *Adaptor) handleStreamResponse(ctx context.Context, resp *http.Response, info *common.RelayInfo, writer http.ResponseWriter) (*common.Usage, error) {
	usage, ok := relaykit_bridge.TryConvertStreamViaRelaykit(ctx, info, resp.Body, writer)
	if !ok {
		return nil, fmt.Errorf("[relaykit] coze→openai 流式转换失败（无匹配转换器）")
	}
	return usage, nil
}

// handleNonStreamResponse 非流式模式：读取 Coze SSE 收集完整内容，转换为 OpenAI JSON 响应
func (a *Adaptor) handleNonStreamResponse(ctx context.Context, resp *http.Response, info *common.RelayInfo, writer http.ResponseWriter) (*common.Usage, error) {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read coze response body failed: %w", err)
	}

	convertedBody, _, ok := relaykit_bridge.TryConvertResponseViaRelaykit(ctx, info, body)
	if !ok {
		return nil, fmt.Errorf("[relaykit] coze→openai 响应转换失败")
	}
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(http.StatusOK)
	_, _ = writer.Write(convertedBody)
	if usage, ok := relaykit_bridge.UsageFromConvertedChatResponse(convertedBody); ok {
		return usage, nil
	}
	return &common.Usage{}, nil
}

func (a *Adaptor) GetChannelName() string {
	return ChannelName
}

var _ common.Adaptor = (*Adaptor)(nil)

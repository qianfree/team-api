package baidu_v2

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/qianfree/team-api/relay/channel/openai"
	"github.com/qianfree/team-api/relay/common"
	"github.com/qianfree/team-api/relay/constant"
	"github.com/qianfree/team-api/relay/override"
)

// Adaptor 百度 V2 供应商适配器。
// OpenAI 兼容格式，支持特殊的 ApiKey 分割（token|appid）和搜索模式。
type Adaptor struct {
	info  *common.RelayInfo
	token string
	appID string
}

func (a *Adaptor) Init(info *common.RelayInfo) {
	a.info = info
	a.parseAPIKey(info.ChannelMeta.ApiKey)
}

// parseAPIKey 解析 ApiKey。格式为 "token|appid"，按 "|" 分割。
func (a *Adaptor) parseAPIKey(apiKey string) {
	parts := strings.SplitN(apiKey, "|", 2)
	a.token = parts[0]
	if len(parts) > 1 {
		a.appID = parts[1]
	}
}

// GetRequestURL 构建上游请求 URL。
func (a *Adaptor) GetRequestURL(info *common.RelayInfo) (string, error) {
	baseURL := strings.TrimSuffix(info.ChannelMeta.BaseURL, "/")

	switch constant.RelayMode(info.RelayMode) {
	// Claude/Gemini/Responses 入站：矩阵已把请求体转成 OpenAI chat 格式，走 chat 端点
	case constant.RelayModeChatCompletions, constant.RelayModeClaudeMessages,
		constant.RelayModeGeminiChat,
		constant.RelayModeResponses, constant.RelayModeResponsesCompact:
		return baseURL + "/v2/chat/completions", nil
	case constant.RelayModeEmbeddings:
		return baseURL + "/v2/embeddings", nil
	case constant.RelayModeImagesGenerations:
		return baseURL + "/v2/images/generations", nil
	default:
		return "", fmt.Errorf("baidu_v2: unsupported relay mode: %d", info.RelayMode)
	}
}

// SetupRequestHeader 设置上游请求头。
// 使用分割后的 token 作为 Bearer 认证，appid 作为额外请求头。
func (a *Adaptor) SetupRequestHeader(header http.Header, info *common.RelayInfo) error {
	header.Set("Authorization", "Bearer "+a.token)
	header.Set("Content-Type", "application/json")
	header.Set("Accept", "application/json")
	if a.appID != "" {
		header.Set("appid", a.appID)
	}
	return nil
}

// ConvertRequest 转换请求体。
// 如果模型名以 "-search" 结尾，去掉后缀并注入 web_search 配置。
func (a *Adaptor) ConvertRequest(ctx context.Context, info *common.RelayInfo, requestBody []byte) (io.Reader, error) {
	processed, err := postProcessRequest(requestBody, info)
	if err != nil {
		return nil, err
	}
	return bytes.NewReader(processed), nil
}

// postProcessRequest 百度请求适配：模型名映射。
// 历史上还承担 -search 模型名后缀的剥离与 web_search 注入；该后缀语法已于
// 2026-09 随全局虚拟模型后缀机制一并移除。模型映射已由 relaykit 覆盖，
// 故不再实现 RequestPostProcessor 钩子。
func postProcessRequest(requestBody []byte, info *common.RelayInfo) ([]byte, error) {
	if !info.ChannelMeta.IsModelMapped {
		return requestBody, nil
	}
	var rawMap map[string]json.RawMessage
	if err := json.Unmarshal(requestBody, &rawMap); err != nil {
		return requestBody, nil
	}
	rawMap["model"] = json.RawMessage(`"` + info.ChannelMeta.UpstreamModelName + `"`)
	converted, err := json.Marshal(rawMap)
	if err != nil {
		return nil, fmt.Errorf("marshal converted request failed: %w", err)
	}
	return converted, nil
}

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

	timeout := info.ChannelMeta.Settings.GetTimeoutSeconds(info.RelayMode)

	client := common.NewPooledClient(timeout, info.ChannelMeta.Settings.UseProxy, info.IsStream)

	return client.Do(httpReq)
}

// DoResponse 处理上游响应。百度 V2 返回格式与 OpenAI 一致，委托 OpenAI 适配器处理。
func (a *Adaptor) DoResponse(ctx context.Context, resp *http.Response, info *common.RelayInfo, writer http.ResponseWriter) (*common.Usage, error) {
	delegate := &openai.Adaptor{}
	delegate.Init(info)
	return delegate.DoResponse(ctx, resp, info, writer)
}

func (a *Adaptor) GetChannelName() string {
	return ChannelName
}

var _ common.Adaptor = (*Adaptor)(nil)

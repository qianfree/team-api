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
	case constant.RelayModeChatCompletions, constant.RelayModeClaudeMessages:
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
	// 非 OpenAI 格式先转换为 OpenAI
	if info.InboundFormat != "" && info.InboundFormat != constant.RelayFormatOpenAI {
		converted, err := openai.ConvertToOpenAI(requestBody, info)
		if err != nil {
			return nil, err
		}
		requestBody = converted
	}

	var rawMap map[string]json.RawMessage
	if err := json.Unmarshal(requestBody, &rawMap); err != nil {
		return bytes.NewReader(requestBody), nil
	}

	// 确定上游模型名
	modelName := info.OriginModelName
	if info.ChannelMeta.IsModelMapped {
		modelName = info.ChannelMeta.UpstreamModelName
	}

	// 检测 "-search" 后缀，启用搜索模式
	if strings.HasSuffix(modelName, "-search") {
		modelName = strings.TrimSuffix(modelName, "-search")
		webSearch := map[string]interface{}{
			"enable":          true,
			"enable_citation": true,
			"enable_trace":    true,
		}
		wsJSON, err := json.Marshal(webSearch)
		if err != nil {
			return nil, fmt.Errorf("marshal web_search failed: %w", err)
		}
		rawMap["web_search"] = json.RawMessage(wsJSON)
	}

	// 设置模型名
	modelJSON, _ := json.Marshal(modelName)
	rawMap["model"] = json.RawMessage(modelJSON)

	converted, err := json.Marshal(rawMap)
	if err != nil {
		return nil, fmt.Errorf("marshal converted request failed: %w", err)
	}
	return bytes.NewReader(converted), nil
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

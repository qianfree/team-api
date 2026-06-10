package cloudflare

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

// Adaptor Cloudflare Workers AI 供应商适配器
// ApiKey 格式为 "token|accountid"，用 "|" 分隔认证令牌和账户 ID
type Adaptor struct {
	info      *common.RelayInfo
	token     string
	accountID string
}

// 确保接口实现
var _ common.Adaptor = (*Adaptor)(nil)

// Init 初始化适配器，从 ApiKey 中解析 token 和 accountID
func (a *Adaptor) Init(info *common.RelayInfo) {
	a.info = info
	a.parseApiKey(info.ChannelMeta.ApiKey)
}

// parseApiKey 解析 "token|accountid" 格式的 ApiKey
func (a *Adaptor) parseApiKey(apiKey string) {
	parts := strings.SplitN(apiKey, "|", 2)
	if len(parts) == 2 {
		a.token = parts[0]
		a.accountID = parts[1]
	} else {
		a.token = apiKey
		a.accountID = ""
	}
}

// GetRequestURL 构建上游请求 URL
// Cloudflare Workers AI URL 格式: {baseURL}/client/v4/accounts/{accountID}/ai/v1/{endpoint}
func (a *Adaptor) GetRequestURL(info *common.RelayInfo) (string, error) {
	if a.accountID == "" {
		return "", fmt.Errorf("cloudflare: account ID not found, ApiKey must be in 'token|accountid' format")
	}

	baseURL := strings.TrimSuffix(info.ChannelMeta.BaseURL, "/")
	prefix := fmt.Sprintf("%s/client/v4/accounts/%s/ai/v1", baseURL, a.accountID)

	switch constant.RelayMode(info.RelayMode) {
	case constant.RelayModeChatCompletions:
		return prefix + "/chat/completions", nil
	case constant.RelayModeCompletions:
		return prefix + "/completions", nil
	case constant.RelayModeEmbeddings:
		return prefix + "/embeddings", nil
	case constant.RelayModeImagesGenerations:
		return prefix + "/images/generations", nil
	default:
		return "", fmt.Errorf("cloudflare: unsupported relay mode: %d", info.RelayMode)
	}
}

// SetupRequestHeader 设置上游请求头
func (a *Adaptor) SetupRequestHeader(header http.Header, info *common.RelayInfo) error {
	header.Set("Authorization", "Bearer "+a.token)
	header.Set("Content-Type", "application/json")
	header.Set("Accept", "application/json")
	return nil
}

// ConvertRequest 转换请求体，执行模型名映射
func (a *Adaptor) ConvertRequest(ctx context.Context, info *common.RelayInfo, requestBody []byte) (io.Reader, error) {
	// 非 OpenAI 格式先转换为 OpenAI
	if info.InboundFormat != "" && info.InboundFormat != constant.RelayFormatOpenAI {
		converted, err := openai.ConvertToOpenAI(requestBody, info)
		if err != nil {
			return nil, err
		}
		requestBody = converted
	}

	if !info.ChannelMeta.IsModelMapped {
		return bytes.NewReader(requestBody), nil
	}

	var rawMap map[string]json.RawMessage
	if err := json.Unmarshal(requestBody, &rawMap); err != nil {
		return bytes.NewReader(requestBody), nil
	}

	rawMap["model"] = json.RawMessage(`"` + info.ChannelMeta.UpstreamModelName + `"`)

	converted, err := json.Marshal(rawMap)
	if err != nil {
		return nil, fmt.Errorf("cloudflare: marshal converted request failed: %w", err)
	}
	return bytes.NewReader(converted), nil
}

// DoRequest 发送请求到上游
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

// DoResponse 处理上游响应，委托给 OpenAI 适配器
func (a *Adaptor) DoResponse(ctx context.Context, resp *http.Response, info *common.RelayInfo, writer http.ResponseWriter) (*common.Usage, error) {
	delegate := &openai.Adaptor{}
	delegate.Init(info)
	return delegate.DoResponse(ctx, resp, info, writer)
}

// GetChannelName 返回渠道名称
func (a *Adaptor) GetChannelName() string {
	return ChannelName
}

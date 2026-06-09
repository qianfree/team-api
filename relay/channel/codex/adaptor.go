package codex

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

// codexCredentials 解析 ApiKey JSON 中的凭证信息
type codexCredentials struct {
	AccessToken string `json:"access_token"`
	AccountID   string `json:"account_id"`
}

// Adaptor Codex 供应商适配器（ChatGPT /v1/responses 端点）
type Adaptor struct {
	info  *common.RelayInfo
	creds codexCredentials
}

func (a *Adaptor) Init(info *common.RelayInfo) {
	a.info = info
	// ApiKey 格式为 JSON: {"access_token": "...", "account_id": "..."}
	_ = json.Unmarshal([]byte(info.ChannelMeta.ApiKey), &a.creds)
}

// GetRequestURL 构建上游请求 URL。Codex 仅支持 /v1/responses 端点。
func (a *Adaptor) GetRequestURL(info *common.RelayInfo) (string, error) {
	baseURL := strings.TrimSuffix(info.ChannelMeta.BaseURL, "/")

	switch constant.RelayMode(info.RelayMode) {
	case constant.RelayModeResponses:
		return baseURL + "/v1/responses", nil
	default:
		return "", fmt.Errorf("codex only supports Responses mode, got relay mode: %d", info.RelayMode)
	}
}

func (a *Adaptor) SetupRequestHeader(header http.Header, info *common.RelayInfo) error {
	header.Set("Authorization", "Bearer "+a.creds.AccessToken)
	header.Set("chatgpt-account-id", a.creds.AccountID)
	header.Set("Content-Type", "application/json")
	return nil
}

// ConvertRequest 转换请求体。Codex 仅支持 Responses 模式，直通并做模型名映射。
func (a *Adaptor) ConvertRequest(ctx context.Context, info *common.RelayInfo, requestBody []byte) (io.Reader, error) {
	// 非 OpenAI 格式先转换为 OpenAI
	if info.InboundFormat != "" && info.InboundFormat != constant.RelayFormatOpenAI {
		converted, err := openai.ConvertToOpenAI(requestBody, info)
		if err != nil {
			return nil, err
		}
		requestBody = converted
	}

	if constant.RelayMode(info.RelayMode) != constant.RelayModeResponses {
		return nil, fmt.Errorf("codex only supports Responses mode, got relay mode: %d", info.RelayMode)
	}

	// 模型名映射
	if info.ChannelMeta.IsModelMapped {
		var rawMap map[string]json.RawMessage
		if err := json.Unmarshal(requestBody, &rawMap); err != nil {
			return bytes.NewReader(requestBody), nil
		}
		rawMap["model"] = json.RawMessage(`"` + info.ChannelMeta.UpstreamModelName + `"`)
		converted, err := json.Marshal(rawMap)
		if err != nil {
			return nil, fmt.Errorf("marshal converted request failed: %w", err)
		}
		return bytes.NewReader(converted), nil
	}

	return bytes.NewReader(requestBody), nil
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

// DoResponse 处理上游响应。委托 OpenAI 适配器处理。
func (a *Adaptor) DoResponse(ctx context.Context, resp *http.Response, info *common.RelayInfo, writer http.ResponseWriter) (*common.Usage, error) {
	delegate := &openai.Adaptor{}
	delegate.Init(info)
	return delegate.DoResponse(ctx, resp, info, writer)
}

func (a *Adaptor) GetChannelName() string {
	return ChannelName
}

var _ common.Adaptor = (*Adaptor)(nil)

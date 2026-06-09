package aws

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

// Adaptor AWS Bedrock 供应商适配器。
// 采用网关代理模式：用户在 BaseURL 中配置 Bedrock 兼容网关（如 LiteLLM、Bedrock Access Gateway），
// 适配器以 OpenAI 兼容格式与网关交互。
//
// ApiKey 格式：
//   - "apiKey"          — 纯 API Key，BaseURL 须包含完整网关地址
//   - "apiKey|region"   — 管道分隔，region 仅用于日志/调试，实际路由由网关决定
type Adaptor struct {
	info   *common.RelayInfo
	apiKey string
	region string
}

func (a *Adaptor) Init(info *common.RelayInfo) {
	a.info = info

	// 解析 ApiKey：支持 "apiKey" 或 "apiKey|region" 格式
	key := info.ChannelMeta.ApiKey
	if parts := strings.SplitN(key, "|", 2); len(parts) == 2 {
		a.apiKey = strings.TrimSpace(parts[0])
		a.region = strings.TrimSpace(parts[1])
	} else {
		a.apiKey = key
	}
}

// GetRequestURL 构建上游请求 URL。
// 使用 BaseURL 指向的 OpenAI 兼容网关，拼接标准 OpenAI 路径。
func (a *Adaptor) GetRequestURL(info *common.RelayInfo) (string, error) {
	baseURL := strings.TrimSuffix(info.ChannelMeta.BaseURL, "/")

	switch constant.RelayMode(info.RelayMode) {
	case constant.RelayModeChatCompletions, constant.RelayModeClaudeMessages:
		return baseURL + "/v1/chat/completions", nil
	case constant.RelayModeEmbeddings:
		return baseURL + "/v1/embeddings", nil
	default:
		return "", fmt.Errorf("aws: unsupported relay mode: %d", info.RelayMode)
	}
}

func (a *Adaptor) SetupRequestHeader(header http.Header, info *common.RelayInfo) error {
	header.Set("Authorization", "Bearer "+a.apiKey)
	header.Set("Content-Type", "application/json")
	header.Set("Accept", "application/json")
	return nil
}

// ConvertRequest 转换请求体。网关接受 OpenAI 格式，只需做模型名映射。
func (a *Adaptor) ConvertRequest(ctx context.Context, info *common.RelayInfo, requestBody []byte) (io.Reader, error) {
	// 非 OpenAI 格式先转换为 OpenAI
	if info.InboundFormat != "" && info.InboundFormat != constant.RelayFormatOpenAI {
		converted, err := openai.ConvertToOpenAI(requestBody, info)
		if err != nil {
			return nil, err
		}
		requestBody = converted
	}

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

// DoResponse 处理上游响应。网关返回 OpenAI 格式，委托 OpenAI 适配器处理。
func (a *Adaptor) DoResponse(ctx context.Context, resp *http.Response, info *common.RelayInfo, writer http.ResponseWriter) (*common.Usage, error) {
	delegate := &openai.Adaptor{}
	delegate.Init(info)
	return delegate.DoResponse(ctx, resp, info, writer)
}

func (a *Adaptor) GetChannelName() string {
	return ChannelName
}

var _ common.Adaptor = (*Adaptor)(nil)

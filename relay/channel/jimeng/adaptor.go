package jimeng

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

// Adaptor 即梦（Jimeng）供应商适配器。
// 即梦是字节跳动的 AI 图像生成平台，仅支持图像生成模式。
// 完整的即梦原生 HMAC 签名较为复杂，本适配器为简化版本，
// 假设通过一个兼容网关代理，返回 OpenAI 兼容的图像生成响应格式。
//
// ApiKey 格式: "accessKey|secretKey"（竖线分隔）
type Adaptor struct {
	info      *common.RelayInfo
	accessKey string
	secretKey string
}

func (a *Adaptor) Init(info *common.RelayInfo) {
	a.info = info
	// 解析 ApiKey: "accessKey|secretKey"
	parts := strings.SplitN(info.ChannelMeta.ApiKey, "|", 2)
	if len(parts) == 2 {
		a.accessKey = parts[0]
		a.secretKey = parts[1]
	} else {
		a.accessKey = info.ChannelMeta.ApiKey
	}
}

// GetRequestURL 构建上游请求 URL。
// 仅支持图像生成: {baseURL}/v2/images/generations
func (a *Adaptor) GetRequestURL(info *common.RelayInfo) (string, error) {
	baseURL := strings.TrimSuffix(info.ChannelMeta.BaseURL, "/")

	switch constant.RelayMode(info.RelayMode) {
	case constant.RelayModeImagesGenerations:
		return baseURL + "/v2/images/generations", nil
	default:
		return "", fmt.Errorf("unsupported relay mode for Jimeng: %d (only image generation is supported)", info.RelayMode)
	}
}

// SetupRequestHeader 设置上游请求头。使用 accessKey 作为 Bearer Token。
func (a *Adaptor) SetupRequestHeader(header http.Header, info *common.RelayInfo) error {
	header.Set("Authorization", "Bearer "+a.accessKey)
	header.Set("Content-Type", "application/json")
	header.Set("Accept", "application/json")
	return nil
}

// ConvertRequest 转换请求体。
// 仅支持图像生成模式，对非图像模式返回错误。
// 请求体直通，仅做模型名映射。
func (a *Adaptor) ConvertRequest(ctx context.Context, info *common.RelayInfo, requestBody []byte) (io.Reader, error) {
	// 非 OpenAI 格式先转换为 OpenAI
	if info.InboundFormat != "" && info.InboundFormat != constant.RelayFormatOpenAI {
		converted, err := openai.ConvertToOpenAI(requestBody, info)
		if err != nil {
			return nil, err
		}
		requestBody = converted
	}

	if constant.RelayMode(info.RelayMode) != constant.RelayModeImagesGenerations {
		return nil, fmt.Errorf("Jimeng only supports image generation mode")
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

// DoRequest 发送请求到即梦上游
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
		timeout = 120 // 图像生成通常较慢
	}
	if constant.RelayMode(info.RelayMode) == constant.RelayModeImagesGenerations && timeout < 600 {
		timeout = 600
	}

	client := common.NewPooledClient(timeout, info.ChannelMeta.Settings.UseProxy, info.IsStream)

	return client.Do(httpReq)
}

// DoResponse 处理即梦上游响应。
// 假设网关返回 OpenAI 兼容格式，委托给 openai.Adaptor 处理。
func (a *Adaptor) DoResponse(ctx context.Context, resp *http.Response, info *common.RelayInfo, writer http.ResponseWriter) (*common.Usage, error) {
	openaiAdaptor := &openai.Adaptor{}
	openaiAdaptor.Init(info)
	return openaiAdaptor.DoResponse(ctx, resp, info, writer)
}

func (a *Adaptor) GetChannelName() string {
	return ChannelName
}

var _ common.Adaptor = (*Adaptor)(nil)

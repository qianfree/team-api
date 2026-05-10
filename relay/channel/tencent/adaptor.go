package tencent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/qianfree/team-api/relay/channel/openai"
	"github.com/qianfree/team-api/relay/common"
	"github.com/qianfree/team-api/relay/constant"
	"github.com/qianfree/team-api/relay/override"
)

const (
	defaultHost    = "hunyuan.tencentcloudapi.com"
	defaultBaseURL = "https://hunyuan.tencentcloudapi.com/"
	apiVersion     = "2023-09-01"
	serviceName    = "hunyuan"
)

// Adaptor 腾讯混元供应商适配器。
// 腾讯混元新版 API 兼容 OpenAI 格式，响应委托 OpenAI 适配器处理。
//
// ApiKey 格式："secretId|secretKey"（管道分隔）
type Adaptor struct {
	info      *common.RelayInfo
	secretID  string
	secretKey string
}

func (a *Adaptor) Init(info *common.RelayInfo) {
	a.info = info

	// 解析 ApiKey："secretId|secretKey"
	key := info.ChannelMeta.ApiKey
	if parts := strings.SplitN(key, "|", 2); len(parts) == 2 {
		a.secretID = strings.TrimSpace(parts[0])
		a.secretKey = strings.TrimSpace(parts[1])
	} else {
		// 降级：当作纯 API Key 使用（用于已签名的代理网关场景）
		a.secretID = key
	}
}

// GetRequestURL 返回腾讯混元 API 地址。
// 如果配置了 BaseURL 则使用配置值，否则使用默认地址。
func (a *Adaptor) GetRequestURL(info *common.RelayInfo) (string, error) {
	if info.ChannelMeta.BaseURL != "" {
		return strings.TrimSuffix(info.ChannelMeta.BaseURL, "/") + "/", nil
	}
	return defaultBaseURL, nil
}

// SetupRequestHeader 设置腾讯云 API 请求头，包括 TC3-HMAC-SHA256 签名。
func (a *Adaptor) SetupRequestHeader(header http.Header, info *common.RelayInfo) error {
	contentType := "application/json"
	header.Set("Content-Type", contentType)
	header.Set("Accept", "application/json")

	// 确定 Host
	host := defaultHost
	if info.ChannelMeta.BaseURL != "" {
		// 从 BaseURL 提取 host
		trimmed := strings.TrimPrefix(info.ChannelMeta.BaseURL, "https://")
		trimmed = strings.TrimPrefix(trimmed, "http://")
		trimmed = strings.TrimSuffix(trimmed, "/")
		if trimmed != "" {
			host = trimmed
		}
	}
	header.Set("Host", host)

	// 确定 Action
	action := "ChatCompletions"
	switch constant.RelayMode(info.RelayMode) {
	case constant.RelayModeEmbeddings:
		action = "GetEmbedding"
	}

	header.Set("X-TC-Action", action)
	header.Set("X-TC-Version", apiVersion)

	timestamp := time.Now().Unix()
	header.Set("X-TC-Timestamp", fmt.Sprintf("%d", timestamp))

	// 如果有 secretKey 则计算签名；否则跳过（代理网关场景）
	if a.secretKey != "" {
		// 签名需要请求体，此处在 DoRequest 中重新计算
		// SetupRequestHeader 只设置非签名头，签名在 DoRequest 中补充
	}

	return nil
}

// ConvertRequest 转换请求体。腾讯混元新版 API 兼容 OpenAI 格式，做模型名映射即可。
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

// DoRequest 发送请求到腾讯混元 API。
// 由于 TC3 签名需要完整的请求体，签名在此方法中计算。
func (a *Adaptor) DoRequest(ctx context.Context, info *common.RelayInfo, requestBody io.Reader) (*http.Response, error) {
	reqURL, err := a.GetRequestURL(info)
	if err != nil {
		return nil, err
	}

	// 读取请求体用于签名
	var bodyBytes []byte
	if requestBody != nil {
		bodyBytes, err = io.ReadAll(requestBody)
		if err != nil {
			return nil, fmt.Errorf("read request body failed: %w", err)
		}
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("create request failed: %w", err)
	}

	if err := a.SetupRequestHeader(httpReq.Header, info); err != nil {
		return nil, fmt.Errorf("setup request header failed: %w", err)
	}

	if hdrOverrides, hdrErr := override.ApplyHeaderOverride(info); hdrErr == nil && len(hdrOverrides) > 0 {
		override.MergeHeaderOverrides(httpReq.Header, hdrOverrides)
	}

	// 计算 TC3-HMAC-SHA256 签名（需要完整请求体）
	if a.secretKey != "" {
		contentType := httpReq.Header.Get("Content-Type")
		host := httpReq.Header.Get("Host")
		timestamp := time.Now().Unix()

		// 更新时间戳（确保签名和头一致）
		httpReq.Header.Set("X-TC-Timestamp", fmt.Sprintf("%d", timestamp))

		authorization := sign(a.secretID, a.secretKey, serviceName, host, contentType, bodyBytes, timestamp)
		httpReq.Header.Set("Authorization", authorization)
	}

	timeout := info.ChannelMeta.Settings.TimeoutSeconds
	if timeout <= 0 {
		timeout = 60
	}

	client := common.NewPooledClient(timeout, info.ChannelMeta.Settings.UseProxy, info.IsStream)

	return client.Do(httpReq)
}

// DoResponse 处理上游响应。腾讯混元新版 API 返回 OpenAI 格式，委托 OpenAI 适配器处理。
func (a *Adaptor) DoResponse(ctx context.Context, resp *http.Response, info *common.RelayInfo, writer http.ResponseWriter) (*common.Usage, error) {
	delegate := &openai.Adaptor{}
	delegate.Init(info)
	return delegate.DoResponse(ctx, resp, info, writer)
}

func (a *Adaptor) GetChannelName() string {
	return ChannelName
}

var _ common.Adaptor = (*Adaptor)(nil)

package xai

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

// Adaptor xAI (Grok) 供应商适配器
type Adaptor struct {
	info *common.RelayInfo
}

func (a *Adaptor) Init(info *common.RelayInfo) {
	a.info = info
}

// GetRequestURL 构建上游请求 URL。xAI 使用标准 OpenAI 路径。
func (a *Adaptor) GetRequestURL(info *common.RelayInfo) (string, error) {
	baseURL := strings.TrimSuffix(info.ChannelMeta.BaseURL, "/")

	switch constant.RelayMode(info.RelayMode) {
	case constant.RelayModeChatCompletions, constant.RelayModeClaudeMessages:
		return baseURL + "/v1/chat/completions", nil
	case constant.RelayModeCompletions:
		return baseURL + "/v1/completions", nil
	case constant.RelayModeEmbeddings:
		return baseURL + "/v1/embeddings", nil
	case constant.RelayModeImagesGenerations:
		return baseURL + "/v1/images/generations", nil
	default:
		return baseURL + "/v1/chat/completions", nil
	}
}

func (a *Adaptor) SetupRequestHeader(header http.Header, info *common.RelayInfo) error {
	header.Set("Authorization", "Bearer "+info.ChannelMeta.ApiKey)
	header.Set("Content-Type", "application/json")
	header.Set("Accept", "application/json")
	return nil
}

// ConvertRequest 转换请求体。
// xAI 特殊处理：
//   - 模型名以 "-search" 结尾：剥离后缀，添加 search_parameters.mode = "on"
//   - 模型名以 "-high" 结尾：剥离后缀，添加 reasoning_effort = "high"
//   - 模型名以 "-low" 结尾：剥离后缀，添加 reasoning_effort = "low"
//
// 剥离后缀后再做模型名映射（如果配置了映射）。
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

	// 获取当前模型名
	modelName := info.OriginModelName

	// 处理 "-search" 后缀
	if strings.HasSuffix(modelName, "-search") {
		modelName = strings.TrimSuffix(modelName, "-search")
		rawMap["search_parameters"] = json.RawMessage(`{"mode":"on"}`)
	}

	// 处理 "-high" 后缀
	if strings.HasSuffix(modelName, "-high") {
		modelName = strings.TrimSuffix(modelName, "-high")
		rawMap["reasoning_effort"] = json.RawMessage(`"high"`)
	}

	// 处理 "-low" 后缀
	if strings.HasSuffix(modelName, "-low") {
		modelName = strings.TrimSuffix(modelName, "-low")
		rawMap["reasoning_effort"] = json.RawMessage(`"low"`)
	}

	// 模型名映射：优先使用上游映射名，否则使用剥离后缀后的名称
	if info.ChannelMeta.IsModelMapped {
		rawMap["model"] = json.RawMessage(`"` + info.ChannelMeta.UpstreamModelName + `"`)
	} else {
		rawMap["model"] = json.RawMessage(`"` + modelName + `"`)
	}

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

// DoResponse 处理上游响应。xAI 返回格式与 OpenAI 一致，委托 OpenAI 适配器处理。
func (a *Adaptor) DoResponse(ctx context.Context, resp *http.Response, info *common.RelayInfo, writer http.ResponseWriter) (*common.Usage, error) {
	delegate := &openai.Adaptor{}
	delegate.Init(info)
	return delegate.DoResponse(ctx, resp, info, writer)
}

func (a *Adaptor) GetChannelName() string {
	return ChannelName
}

var _ common.Adaptor = (*Adaptor)(nil)

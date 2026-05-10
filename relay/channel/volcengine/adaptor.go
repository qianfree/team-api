package volcengine

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/qianfree/team-api/relay/channel/claude"
	"github.com/qianfree/team-api/relay/channel/openai"
	"github.com/qianfree/team-api/relay/common"
	"github.com/qianfree/team-api/relay/constant"
	"github.com/qianfree/team-api/relay/override"
)

// Adaptor 火山引擎（豆包）供应商适配器。
// OpenAI 兼容格式，bot- 前缀模型使用 bots 端点。
type Adaptor struct {
	info *common.RelayInfo
}

func (a *Adaptor) Init(info *common.RelayInfo) {
	a.info = info
}

// GetRequestURL 构建上游请求 URL。
// Claude 协议走 Anthropic 兼容端点，bot- 前缀模型使用 bots 端点，其余使用标准路径。
func (a *Adaptor) GetRequestURL(info *common.RelayInfo) (string, error) {
	baseURL := strings.TrimSuffix(info.ChannelMeta.BaseURL, "/")

	// 确定实际模型名
	modelName := info.OriginModelName
	if info.ChannelMeta.IsModelMapped {
		modelName = info.ChannelMeta.UpstreamModelName
	}

	switch constant.RelayMode(info.RelayMode) {
	case constant.RelayModeClaudeMessages:
		return baseURL + "/api/coding/v1/messages", nil
	case constant.RelayModeChatCompletions:
		if strings.HasPrefix(modelName, "bot-") {
			return baseURL + "/api/v3/bots/chat/completions", nil
		}
		return baseURL + "/api/v3/chat/completions", nil
	case constant.RelayModeEmbeddings:
		return baseURL + "/api/v3/embeddings", nil
	case constant.RelayModeImagesGenerations:
		return baseURL + "/api/v3/images/generations", nil
	default:
		return "", fmt.Errorf("volcengine: unsupported relay mode: %d", info.RelayMode)
	}
}

func (a *Adaptor) SetupRequestHeader(header http.Header, info *common.RelayInfo) error {
	header.Set("Authorization", "Bearer "+info.ChannelMeta.ApiKey)
	header.Set("Content-Type", "application/json")
	header.Set("Accept", "application/json")
	return nil
}

// ConvertRequest 转换请求体。
// Claude 入站直接透传到 Anthropic 兼容端点，其他格式先转为 OpenAI 再做模型映射。
func (a *Adaptor) ConvertRequest(ctx context.Context, info *common.RelayInfo, requestBody []byte) (io.Reader, error) {
	// Claude 入站：仅做模型映射
	if info.InboundFormat == constant.RelayFormatClaude {
		return convertClaudeRequest(requestBody, info)
	}

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

	timeout := info.ChannelMeta.Settings.TimeoutSeconds
	if timeout <= 0 {
		timeout = 60
	}

	client := common.NewPooledClient(timeout, info.ChannelMeta.Settings.UseProxy, info.IsStream)

	return client.Do(httpReq)
}

// DoResponse 处理上游响应。
// Claude 入站委托 claude.Adaptor 原生直通；其他格式委托 openai.Adaptor。
func (a *Adaptor) DoResponse(ctx context.Context, resp *http.Response, info *common.RelayInfo, writer http.ResponseWriter) (*common.Usage, error) {
	if info.GetOriginalClientFormat() == constant.RelayFormatClaude {
		delegate := &claude.Adaptor{}
		delegate.Init(info)
		return delegate.DoResponse(ctx, resp, info, writer)
	}

	delegate := &openai.Adaptor{}
	delegate.Init(info)
	return delegate.DoResponse(ctx, resp, info, writer)
}

func (a *Adaptor) GetChannelName() string {
	return ChannelName
}

var _ common.Adaptor = (*Adaptor)(nil)

// convertClaudeRequest 处理 Claude 入站请求的模型映射。
func convertClaudeRequest(requestBody []byte, info *common.RelayInfo) (io.Reader, error) {
	if !info.ChannelMeta.IsModelMapped {
		return bytes.NewReader(requestBody), nil
	}
	var rawMap map[string]json.RawMessage
	if err := json.Unmarshal(requestBody, &rawMap); err != nil {
		return bytes.NewReader(requestBody), nil
	}
	rawMap["model"] = json.RawMessage(`"` + info.ChannelMeta.UpstreamModelName + `"`)
	result, err := json.Marshal(rawMap)
	if err != nil {
		return bytes.NewReader(requestBody), nil
	}
	return bytes.NewReader(result), nil
}

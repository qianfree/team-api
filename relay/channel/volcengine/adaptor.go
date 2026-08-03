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
)

// openaiAdaptor 火山 OpenAI 兼容链路（/api/v3/*），处理非 Claude 入站。
// bot- 前缀模型使用 bots 端点。
type openaiAdaptor struct{}

func (a *openaiAdaptor) Init(info *common.RelayInfo) {}

// GetRequestURL 构建上游请求 URL。
func (a *openaiAdaptor) GetRequestURL(info *common.RelayInfo) (string, error) {
	baseURL := strings.TrimSuffix(info.ChannelMeta.BaseURL, "/")

	modelName := info.OriginModelName
	if info.ChannelMeta.IsModelMapped {
		modelName = info.ChannelMeta.UpstreamModelName
	}

	switch constant.RelayMode(info.RelayMode) {
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
		return "", fmt.Errorf("volcengine(openai): unsupported relay mode: %d", info.RelayMode)
	}
}

func (a *openaiAdaptor) SetupRequestHeader(header http.Header, info *common.RelayInfo) error {
	setupHeader(header, info)
	return nil
}

// ConvertRequest 非 OpenAI 格式先转换为 OpenAI，再做模型映射。
// 流式请求注入 stream_options.include_usage，确保火山 OpenAI 端点在流式响应末尾
// 返回 usage（计费需要）；images 模式使用原生参数，不注入。
// 注：不注入 reasoning_effort——火山 OpenAI 端点用 doubao 原生 thinking 而非 reasoning_effort。
func (a *openaiAdaptor) ConvertRequest(ctx context.Context, info *common.RelayInfo, requestBody []byte) (io.Reader, error) {
	if info.InboundFormat != "" && info.InboundFormat != constant.RelayFormatOpenAI {
		converted, err := openai.ConvertToOpenAI(requestBody, info)
		if err != nil {
			return nil, err
		}
		requestBody = converted
	}
	result := mapModelIfNeeded(requestBody, info)
	mode := constant.RelayMode(info.RelayMode)
	if mode != constant.RelayModeImagesGenerations && mode != constant.RelayModeImagesEdits {
		result = openai.InjectStreamOptions(result, info)
	}
	return result, nil
}

func (a *openaiAdaptor) DoRequest(ctx context.Context, info *common.RelayInfo, requestBody io.Reader) (*http.Response, error) {
	url, err := a.GetRequestURL(info)
	if err != nil {
		return nil, err
	}
	return doSend(ctx, info, url, requestBody)
}

// DoResponse 委托 OpenAI 适配器处理上游响应。
func (a *openaiAdaptor) DoResponse(ctx context.Context, resp *http.Response, info *common.RelayInfo, writer http.ResponseWriter) (*common.Usage, error) {
	delegate := &openai.Adaptor{}
	delegate.Init(info)
	return delegate.DoResponse(ctx, resp, info, writer)
}

func (a *openaiAdaptor) GetChannelName() string { return ChannelName }

// claudeAdaptor 火山 Anthropic 兼容链路（/api/coding/v1/messages），处理 Claude 入站。
type claudeAdaptor struct{}

func (a *claudeAdaptor) Init(info *common.RelayInfo) {}

// GetRequestURL 构建上游请求 URL（Anthropic 兼容端点）。
func (a *claudeAdaptor) GetRequestURL(info *common.RelayInfo) (string, error) {
	baseURL := strings.TrimSuffix(info.ChannelMeta.BaseURL, "/")
	switch constant.RelayMode(info.RelayMode) {
	case constant.RelayModeClaudeMessages:
		return baseURL + "/api/coding/v1/messages", nil
	default:
		return "", fmt.Errorf("volcengine(claude): unsupported relay mode: %d", info.RelayMode)
	}
}

func (a *claudeAdaptor) SetupRequestHeader(header http.Header, info *common.RelayInfo) error {
	setupHeader(header, info)
	return nil
}

// ConvertRequest Claude 入站：火山 Anthropic 端点接受原生 Claude 格式，仅做模型映射。
func (a *claudeAdaptor) ConvertRequest(ctx context.Context, info *common.RelayInfo, requestBody []byte) (io.Reader, error) {
	return mapModelIfNeeded(requestBody, info), nil
}

func (a *claudeAdaptor) DoRequest(ctx context.Context, info *common.RelayInfo, requestBody io.Reader) (*http.Response, error) {
	url, err := a.GetRequestURL(info)
	if err != nil {
		return nil, err
	}
	return doSend(ctx, info, url, requestBody)
}

// DoResponse 委托 Claude 适配器处理上游响应（Claude 客户端走原生直通）。
func (a *claudeAdaptor) DoResponse(ctx context.Context, resp *http.Response, info *common.RelayInfo, writer http.ResponseWriter) (*common.Usage, error) {
	delegate := &claude.Adaptor{}
	delegate.Init(info)
	return delegate.DoResponse(ctx, resp, info, writer)
}

func (a *claudeAdaptor) GetChannelName() string { return ChannelName }

// mapModelIfNeeded 模型名映射（两链路共用）。等价于原 convertClaudeRequest 的映射逻辑。
func mapModelIfNeeded(requestBody []byte, info *common.RelayInfo) io.Reader {
	if !info.ChannelMeta.IsModelMapped {
		return bytes.NewReader(requestBody)
	}
	var rawMap map[string]json.RawMessage
	if err := json.Unmarshal(requestBody, &rawMap); err != nil {
		return bytes.NewReader(requestBody)
	}
	rawMap["model"] = json.RawMessage(`"` + info.ChannelMeta.UpstreamModelName + `"`)
	converted, err := json.Marshal(rawMap)
	if err != nil {
		return bytes.NewReader(requestBody)
	}
	return bytes.NewReader(converted)
}

// 确保两个子适配器实现接口
var (
	_ common.Adaptor = (*openaiAdaptor)(nil)
	_ common.Adaptor = (*claudeAdaptor)(nil)
)

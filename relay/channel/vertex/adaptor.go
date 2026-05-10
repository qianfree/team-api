package vertex

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/qianfree/team-api/relay/channel/claude"
	"github.com/qianfree/team-api/relay/channel/gemini"
	"github.com/qianfree/team-api/relay/common"
	"github.com/qianfree/team-api/relay/override"
)

// modelType 模型类型（决定使用哪个供应商的适配器进行请求转换和响应处理）
type modelType int

const (
	modelTypeGemini modelType = iota
	modelTypeClaude
)

// Adaptor Vertex AI 供应商适配器
// Vertex AI 是 Google Cloud 的 AI 平台，可同时接入 Gemini 和 Claude 模型。
// 本适配器根据模型名称检测类型，委托对应的供应商适配器处理请求转换和响应。
type Adaptor struct {
	info             *common.RelayInfo
	detectedType     modelType
	isServiceAccount bool
	projectID        string
	region           string

	// 委托适配器
	geminiAdaptor *gemini.Adaptor
	claudeAdaptor *claude.Adaptor
}

// Init 初始化适配器
func (a *Adaptor) Init(info *common.RelayInfo) {
	a.info = info

	// 检测认证模式
	apiKey := info.ChannelMeta.ApiKey
	if isServiceAccountJSON(apiKey) {
		a.isServiceAccount = true
		if pid, err := parseServiceAccountProjectID(apiKey); err == nil {
			a.projectID = pid
		}
	} else {
		a.isServiceAccount = false
		// API Key 模式下，projectID 需要从 BaseURL 或其他配置中获取
		a.projectID = apiKey
	}

	// 默认区域
	a.region = "us-central1"

	// 检测模型类型
	model := info.ChannelMeta.UpstreamModelName
	if strings.Contains(strings.ToLower(model), "claude") {
		a.detectedType = modelTypeClaude
	} else {
		a.detectedType = modelTypeGemini
	}

	// 初始化委托适配器
	a.geminiAdaptor = &gemini.Adaptor{}
	a.geminiAdaptor.Init(info)
	a.claudeAdaptor = &claude.Adaptor{}
	a.claudeAdaptor.Init(info)
}

// GetRequestURL 构建上游请求 URL
func (a *Adaptor) GetRequestURL(info *common.RelayInfo) (string, error) {
	baseURL := strings.TrimSuffix(info.ChannelMeta.BaseURL, "/")
	model := info.ChannelMeta.UpstreamModelName

	switch a.detectedType {
	case modelTypeClaude:
		// Claude on Vertex AI
		suffix := ":rawPredict"
		if info.IsStream {
			suffix = ":streamRawPredict"
		}
		return fmt.Sprintf("%s/v1/projects/%s/locations/%s/publishers/anthropic/models/%s%s",
			baseURL, a.projectID, a.region, model, suffix), nil

	default:
		// Gemini on Vertex AI
		suffix := ":generateContent"
		if info.IsStream {
			suffix = ":streamGenerateContent?alt=sse"
		}
		return fmt.Sprintf("%s/v1/projects/%s/locations/%s/publishers/google/models/%s%s",
			baseURL, a.projectID, a.region, model, suffix), nil
	}
}

// SetupRequestHeader 设置上游请求头
func (a *Adaptor) SetupRequestHeader(header http.Header, info *common.RelayInfo) error {
	header.Set("Content-Type", "application/json")

	if a.isServiceAccount {
		// 服务账号模式：获取 OAuth2 访问令牌
		token, err := getVertexAccessToken(info.ChannelMeta.ApiKey)
		if err != nil {
			return fmt.Errorf("get Vertex access token failed: %w", err)
		}
		header.Set("Authorization", "Bearer "+token)
	} else {
		// API Key 模式（不常用，通常 Vertex 使用服务账号）
		header.Set("Authorization", "Bearer "+info.ChannelMeta.ApiKey)
	}

	if info.RequestHeaders != nil {
		for _, h := range []string{"X-Request-Id"} {
			if v := info.RequestHeaders.Get(h); v != "" {
				header.Set(h, v)
			}
		}
	}

	return nil
}

// ConvertRequest 转换请求体（委托给对应的供应商适配器）
func (a *Adaptor) ConvertRequest(ctx context.Context, info *common.RelayInfo, requestBody []byte) (io.Reader, error) {
	switch a.detectedType {
	case modelTypeClaude:
		return a.claudeAdaptor.ConvertRequest(ctx, info, requestBody)
	default:
		return a.geminiAdaptor.ConvertRequest(ctx, info, requestBody)
	}
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

	timeout := info.ChannelMeta.Settings.TimeoutSeconds
	if timeout <= 0 {
		timeout = 60
	}

	client := common.NewPooledClient(timeout, info.ChannelMeta.Settings.UseProxy, info.IsStream)

	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("send request failed: %w", err)
	}

	return resp, nil
}

// DoResponse 处理上游响应（委托给对应的供应商适配器）
func (a *Adaptor) DoResponse(ctx context.Context, resp *http.Response, info *common.RelayInfo, writer http.ResponseWriter) (*common.Usage, error) {
	switch a.detectedType {
	case modelTypeClaude:
		return a.claudeAdaptor.DoResponse(ctx, resp, info, writer)
	default:
		return a.geminiAdaptor.DoResponse(ctx, resp, info, writer)
	}
}

// GetChannelName 返回渠道名称
func (a *Adaptor) GetChannelName() string {
	return ChannelName
}

// 确保接口实现
var _ common.Adaptor = (*Adaptor)(nil)

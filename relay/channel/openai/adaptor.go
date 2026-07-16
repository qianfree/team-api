package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/gogf/gf/v2/frame/g"

	loauth "github.com/qianfree/team-api/internal/logic/common/oauth"
	"github.com/qianfree/team-api/relay/common"
	"github.com/qianfree/team-api/relay/constant"
	"github.com/qianfree/team-api/relay/dto"
	"github.com/qianfree/team-api/relay/helper"
	"github.com/qianfree/team-api/relay/override"
)

// Adaptor OpenAI 供应商适配器
type Adaptor struct {
	info *common.RelayInfo
}

// Init 初始化适配器
func (a *Adaptor) Init(info *common.RelayInfo) {
	a.info = info
}

// GetRequestURL 构建上游请求 URL
func (a *Adaptor) GetRequestURL(info *common.RelayInfo) (string, error) {
	baseURL := strings.TrimSuffix(info.ChannelMeta.BaseURL, "/")

	switch constant.RelayMode(info.RelayMode) {
	case constant.RelayModeChatCompletions, constant.RelayModeClaudeMessages:
		if info.UseResponsesAPI {
			return baseURL + "/v1/responses", nil
		}
		return baseURL + "/v1/chat/completions", nil
	case constant.RelayModeCompletions:
		return baseURL + "/v1/completions", nil
	case constant.RelayModeEmbeddings:
		return baseURL + "/v1/embeddings", nil
	case constant.RelayModeImagesGenerations:
		return baseURL + "/v1/images/generations", nil
	case constant.RelayModeImagesEdits:
		return baseURL + "/v1/images/edits", nil
	case constant.RelayModeModerations:
		return baseURL + "/v1/moderations", nil
	case constant.RelayModeAudioSpeech:
		return baseURL + "/v1/audio/speech", nil
	case constant.RelayModeAudioTranscription:
		return baseURL + "/v1/audio/transcriptions", nil
	case constant.RelayModeAudioTranslation:
		return baseURL + "/v1/audio/translations", nil
	case constant.RelayModeRerank:
		return baseURL + "/v1/rerank", nil
	case constant.RelayModeResponses, constant.RelayModeResponsesCompact:
		// Responses API 请求会被转换为 Chat Completions 格式发送
		return baseURL + "/v1/chat/completions", nil
	default:
		return "", fmt.Errorf("unsupported relay mode: %d", info.RelayMode)
	}
}

// SetupRequestHeader 设置上游请求头
func (a *Adaptor) SetupRequestHeader(header http.Header, info *common.RelayInfo) error {
	apiKey := info.ChannelMeta.ApiKey

	// OAuth 模式：Bearer token + chatgpt-account-id
	if loauth.IsOAuthKeyData(apiKey) {
		var oauthData loauth.OAuthKeyData
		if err := json.Unmarshal([]byte(apiKey), &oauthData); err == nil {
			header.Set("Authorization", "Bearer "+oauthData.AccessToken)
			if oauthData.AccountID != "" {
				header.Set("chatgpt-account-id", oauthData.AccountID)
			}
		}
	} else {
		header.Set("Authorization", "Bearer "+apiKey)
	}

	// Audio STT/翻译使用 multipart form，不强制 JSON Content-Type
	// ImagesEdits 支持 JSON 和 multipart 两种格式，需要透传客户端原始 Content-Type
	mode := constant.RelayMode(info.RelayMode)
	switch mode {
	case constant.RelayModeAudioTranscription, constant.RelayModeAudioTranslation:
		// multipart form：由 DoRequest 从客户端原始头复制 Content-Type
	case constant.RelayModeImagesEdits:
		// 透传客户端原始 Content-Type（JSON 或 multipart 均支持）
		if ct := info.RequestHeaders.Get("Content-Type"); ct != "" {
			header.Set("Content-Type", ct)
		} else {
			header.Set("Content-Type", "application/json")
		}
	default:
		// 透传客户端 Content-Type，无则回退 application/json
		if ct := info.RequestHeaders.Get("Content-Type"); ct != "" {
			header.Set("Content-Type", ct)
		} else {
			header.Set("Content-Type", "application/json")
		}
	}

	// 透传客户端 Accept，流式请求回退 text/event-stream，否则 application/json
	if accept := info.RequestHeaders.Get("Accept"); accept != "" {
		header.Set("Accept", accept)
	} else if info.IsStream {
		header.Set("Accept", "text/event-stream")
	} else {
		header.Set("Accept", "application/json")
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

// ConvertRequest 根据入站格式转换请求体为 OpenAI 格式，然后做 OpenAI 特有后处理。
func (a *Adaptor) ConvertRequest(ctx context.Context, info *common.RelayInfo, requestBody []byte) (io.Reader, error) {
	mode := constant.RelayMode(info.RelayMode)

	// Audio/Rerank: OpenAI 原生格式直接透传（不做 replaceModelIfNeeded 和 injectStreamOptions）
	switch mode {
	case constant.RelayModeAudioSpeech, constant.RelayModeAudioTranscription,
		constant.RelayModeAudioTranslation, constant.RelayModeRerank:
		return bytes.NewReader(requestBody), nil
	}

	var converted io.Reader
	switch info.InboundFormat {
	case constant.RelayFormatClaude:
		r, err := ConvertClaudeToOpenAI(requestBody, info)
		if err != nil {
			return nil, err
		}
		converted = r
	case constant.RelayFormatGemini:
		r, err := ConvertGeminiToOpenAI(requestBody, info)
		if err != nil {
			return nil, err
		}
		converted = r
	case constant.RelayFormatResponses:
		r, err := ConvertResponsesToOpenAI(requestBody, info)
		if err != nil {
			return nil, err
		}
		converted = r
	default:
		converted = bytes.NewReader(requestBody)
	}
	result := replaceModelIfNeeded(converted, info)
	// stream_options 是 Chat Completions 专属字段，GPT Image 使用 stream/partial_images 原生参数
	if info.IsStream && mode != constant.RelayModeImagesGenerations && mode != constant.RelayModeImagesEdits {
		result = injectStreamOptions(result, info)
	}

	// Thinking 后缀路由：注入 reasoning_effort
	if info.ReasoningEffort != "" {
		result = injectReasoningEffort(result, info.ReasoningEffort)
	}

	return result, nil
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

	// 应用渠道级 Header Override（透传客户端 header 等）
	if hdrOverrides, hdrErr := override.ApplyHeaderOverride(info); hdrErr == nil && len(hdrOverrides) > 0 {
		override.MergeHeaderOverrides(httpReq.Header, hdrOverrides)
	}

	mode := constant.RelayMode(info.RelayMode)
	if httpReq.Header.Get("Content-Type") == "" && requestBody != nil &&
		mode != constant.RelayModeAudioTranscription && mode != constant.RelayModeAudioTranslation {
		httpReq.Header.Set("Content-Type", "application/json")
	}

	// Audio multipart form: 由 SetupRequestHeader 跳过，这里补上客户端原始 Content-Type（含 boundary）
	if (mode == constant.RelayModeAudioTranscription || mode == constant.RelayModeAudioTranslation) &&
		info.RequestHeaders != nil {
		if ct := info.RequestHeaders.Get("Content-Type"); ct != "" {
			httpReq.Header.Set("Content-Type", ct)
		}
	}

	timeout := info.ChannelMeta.Settings.GetTimeoutSeconds(info.RelayMode)

	client := common.NewPooledClient(timeout, info.ChannelMeta.Settings.UseProxy, info.IsStream)

	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("send request failed: %w", err)
	}

	// 请求体已完整发送，立即关闭释放资源
	if httpReq.Body != nil {
		_ = httpReq.Body.Close()
	}

	return resp, nil
}

// DoResponse 处理上游响应，按客户端格式和 RelayMode 分发
func (a *Adaptor) DoResponse(ctx context.Context, resp *http.Response, info *common.RelayInfo, writer http.ResponseWriter) (*common.Usage, error) {
	clientFormat := info.GetOriginalClientFormat()
	g.Log().Debugf(ctx, "[OpenAI.Adaptor.DoResponse] Entry: clientFormat=%s, relayMode=%d, isStream=%v, statusCode=%d",
		clientFormat, info.RelayMode, info.IsStream, resp.StatusCode)

	// 根据客户端格式转换响应
	switch clientFormat {
	case constant.RelayFormatClaude:
		if info.IsStream {
			return handleClaudeInboundStream(ctx, resp, info, writer)
		}
		return handleClaudeInboundNonStream(ctx, resp, info, writer)
	case constant.RelayFormatGemini:
		if info.IsStream {
			return handleGeminiInboundStream(ctx, resp, info, writer)
		}
		return handleGeminiInboundNonStream(ctx, resp, info, writer)
	case constant.RelayFormatResponses:
		if info.IsStream {
			return a.handleResponsesInboundStream(ctx, resp, info, writer)
		}
		return a.handleResponsesInboundNonStream(ctx, resp, info, writer)
	}

	// Chat Completions via Responses API 桥接
	if info.UseResponsesAPI {
		g.Log().Infof(ctx, "[OpenAI.Adaptor.DoResponse] Chat via Responses bridge (stream=%v)", info.IsStream)
		if info.IsStream {
			return a.handleChatViaResponsesStream(ctx, resp, info, writer)
		}
		return a.handleChatViaResponsesNonStream(ctx, resp, info, writer)
	}

	switch constant.RelayMode(info.RelayMode) {
	case constant.RelayModeChatCompletions:
		if info.IsStream {
			return a.handleChatStreamResponse(ctx, resp, info, writer)
		}
		return a.handleChatNonStreamResponse(ctx, resp, info, writer)

	case constant.RelayModeCompletions:
		if info.IsStream {
			return a.handleCompletionStreamResponse(ctx, resp, info, writer)
		}
		return a.handleCompletionNonStreamResponse(ctx, resp, info, writer)

	case constant.RelayModeEmbeddings:
		return a.handleEmbeddingResponse(ctx, resp, info, writer)

	case constant.RelayModeImagesGenerations, constant.RelayModeImagesEdits:
		return a.handleImageResponse(ctx, resp, info, writer)
	case constant.RelayModeModerations:
		return a.handleModerationResponse(ctx, resp, info, writer)

	case constant.RelayModeAudioSpeech:
		return handleAudioSpeechResponse(ctx, resp, info, writer)
	case constant.RelayModeAudioTranscription, constant.RelayModeAudioTranslation:
		return handleAudioTranscriptionResponse(ctx, resp, info, writer)
	case constant.RelayModeRerank:
		return handleRerankResponse(ctx, resp, info, writer)

	default:
		return a.handleChatNonStreamResponse(ctx, resp, info, writer)
	}
}

// handleChatNonStreamResponse 处理 Chat Completions 非流式响应
func (a *Adaptor) handleChatNonStreamResponse(ctx context.Context, resp *http.Response, info *common.RelayInfo, writer http.ResponseWriter) (*common.Usage, error) {
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, constant.NewUpstreamError(resp.StatusCode, "read response body failed", err)
	}

	if resp.StatusCode != http.StatusOK {
		// 尝试解析上游错误格式，如果是标准格式则透传原始响应
		if isUpstreamOpenAIError(body) {
			writeUpstreamErrorResponse(writer, resp.StatusCode, body)
			upstreamErr := constant.NewUpstreamError(resp.StatusCode, string(body), nil)
			upstreamErr.ResponseWritten = true
			return &common.Usage{}, upstreamErr
		}
		return nil, constant.NewUpstreamError(resp.StatusCode, string(body), nil)
	}

	if info.ChannelMeta.IsModelMapped {
		body = helper.ReplaceModelName(body, info.OriginModelName)
	}

	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(http.StatusOK)
	_, _ = writer.Write(body)

	var chatResp dto.ChatCompletionResponse
	if err := json.Unmarshal(body, &chatResp); err == nil {
		return &common.Usage{
			PromptTokens:           chatResp.Usage.PromptTokens,
			CompletionTokens:       chatResp.Usage.CompletionTokens,
			TotalTokens:            chatResp.Usage.TotalTokens,
			PromptTokensDetails:    common.DtoTokenDetailsToCommon(chatResp.Usage.PromptTokensDetails),
			CompletionTokenDetails: common.DtoTokenDetailsToCommon(chatResp.Usage.CompletionTokenDetails),
			CacheIncludedInPrompt:  true,
		}, nil
	}

	return &common.Usage{}, nil
}

// handleChatStreamResponse 处理 Chat Completions 流式响应
func (a *Adaptor) handleChatStreamResponse(ctx context.Context, resp *http.Response, info *common.RelayInfo, writer http.ResponseWriter) (*common.Usage, error) {
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		// 尝试透传上游错误格式
		if isUpstreamOpenAIError(body) {
			writeUpstreamErrorResponse(writer, resp.StatusCode, body)
			upstreamErr := constant.NewUpstreamError(resp.StatusCode, string(body), nil)
			upstreamErr.ResponseWritten = true
			return &common.Usage{}, upstreamErr
		}
		return nil, constant.NewUpstreamError(resp.StatusCode, string(body), nil)
	}

	return StreamHandler(ctx, resp, info, writer)
}

// handleCompletionStreamResponse 处理 Completions 流式响应
func (a *Adaptor) handleCompletionStreamResponse(ctx context.Context, resp *http.Response, info *common.RelayInfo, writer http.ResponseWriter) (*common.Usage, error) {
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, constant.NewUpstreamError(resp.StatusCode, string(body), nil)
	}

	return StreamHandlerForCompletions(ctx, resp, info, writer)
}

// GetChannelName 返回渠道名称
func (a *Adaptor) GetChannelName() string {
	return "OpenAI"
}

// 确保接口实现
var _ common.Adaptor = (*Adaptor)(nil)

// replaceModelIfNeeded 如果渠道有模型映射，替换请求体中的模型名
func replaceModelIfNeeded(r io.Reader, info *common.RelayInfo) io.Reader {
	if !info.ChannelMeta.IsModelMapped {
		return r
	}
	body, err := io.ReadAll(r)
	if err != nil {
		return r
	}
	var rawMap map[string]json.RawMessage
	if err := json.Unmarshal(body, &rawMap); err != nil {
		return bytes.NewReader(body)
	}
	rawMap["model"] = json.RawMessage(`"` + info.ChannelMeta.UpstreamModelName + `"`)
	result, err := json.Marshal(rawMap)
	if err != nil {
		return bytes.NewReader(body)
	}
	return bytes.NewReader(result)
}

// injectStreamOptions 为流式请求注入 stream_options:{include_usage:true}
// 确保上游在流式响应的最后一个 chunk 中返回 usage 信息
func injectStreamOptions(r io.Reader, info *common.RelayInfo) io.Reader {
	if !info.IsStream {
		return r
	}
	body, err := io.ReadAll(r)
	if err != nil {
		return r
	}
	var rawMap map[string]json.RawMessage
	if err := json.Unmarshal(body, &rawMap); err != nil {
		return bytes.NewReader(body)
	}
	// 仅在客户端未显式设置 stream_options 时注入
	if _, exists := rawMap["stream_options"]; !exists {
		rawMap["stream_options"] = json.RawMessage(`{"include_usage":true}`)
		result, err := json.Marshal(rawMap)
		if err != nil {
			return bytes.NewReader(body)
		}
		return bytes.NewReader(result)
	}
	return bytes.NewReader(body)
}

// injectReasoningEffort 注入 reasoning_effort 字段到请求体
func injectReasoningEffort(r io.Reader, effort string) io.Reader {
	body, err := io.ReadAll(r)
	if err != nil {
		return r
	}
	var rawMap map[string]json.RawMessage
	if err := json.Unmarshal(body, &rawMap); err != nil {
		return bytes.NewReader(body)
	}
	// 仅在客户端未显式设置时注入
	if _, exists := rawMap["reasoning_effort"]; !exists {
		rawMap["reasoning_effort"], _ = json.Marshal(effort)
		result, err := json.Marshal(rawMap)
		if err != nil {
			return bytes.NewReader(body)
		}
		return bytes.NewReader(result)
	}
	return bytes.NewReader(body)
}

// isUpstreamOpenAIError 检查上游响应是否为标准 OpenAI 错误格式
// 标准格式: {"error": {"type": "...", "message": "...", ...}}
func isUpstreamOpenAIError(body []byte) bool {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(body, &raw); err != nil {
		return false
	}
	_, hasError := raw["error"]
	return hasError
}

// writeUpstreamErrorResponse 将上游错误响应原样写入客户端
func writeUpstreamErrorResponse(writer http.ResponseWriter, statusCode int, body []byte) {
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(statusCode)
	_, _ = writer.Write(body)
}

// handleModerationResponse 处理 Moderations 响应（透传）
func (a *Adaptor) handleModerationResponse(_ context.Context, resp *http.Response, _ *common.RelayInfo, writer http.ResponseWriter) (*common.Usage, error) {
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, constant.NewUpstreamError(resp.StatusCode, "read response body failed", err)
	}
	if resp.StatusCode != http.StatusOK {
		writeUpstreamErrorResponse(writer, resp.StatusCode, body)
		upstreamErr := constant.NewUpstreamError(resp.StatusCode, string(body), nil)
		upstreamErr.ResponseWritten = true
		return &common.Usage{}, upstreamErr
	}
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(http.StatusOK)
	_, _ = writer.Write(body)
	return &common.Usage{}, nil
}

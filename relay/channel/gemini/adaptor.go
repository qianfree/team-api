package gemini

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/qianfree/team-api/relay/common"

	loauth "github.com/qianfree/team-api/internal/logic/common/oauth"
	"github.com/qianfree/team-api/relay/constant"
	"github.com/qianfree/team-api/relay/dto"
	"github.com/qianfree/team-api/relay/helper"
	"github.com/qianfree/team-api/relay/override"
)

const codeAssistBaseURL = "https://cloudcode-pa.googleapis.com"

const geminiCLIUserAgent = "GeminiCLI/0.1.5 (Windows; AMD64)"

// Adaptor Gemini 供应商适配器
type Adaptor struct {
	info *common.RelayInfo
}

// Init 初始化适配器
func (a *Adaptor) Init(info *common.RelayInfo) {
	a.info = info
}

// isCodeAssistMode 检测是否为 Code Assist 模式（OAuth + project_id）
func (a *Adaptor) isCodeAssistMode() (*loauth.OAuthKeyData, bool) {
	apiKey := a.info.ChannelMeta.ApiKey
	if !loauth.IsOAuthKeyData(apiKey) {
		return nil, false
	}
	var oauthData loauth.OAuthKeyData
	if err := json.Unmarshal([]byte(apiKey), &oauthData); err != nil {
		return nil, false
	}
	if oauthData.ProjectID != "" {
		return &oauthData, true
	}
	return nil, false
}

// isCodeAssistActive 检测当前是否为 Code Assist 模式
func (a *Adaptor) isCodeAssistActive() bool {
	_, isCA := a.isCodeAssistMode()
	return isCA
}

// isCodeAssistForcedStream 检测 Code Assist 模式下是否需要强制流式（非流式请求被强制转为流式）
func (a *Adaptor) isCodeAssistForcedStream() bool {
	_, isCA := a.isCodeAssistMode()
	if !isCA || a.info.IsStream {
		return false
	}
	mode := constant.RelayMode(a.info.RelayMode)
	return mode == constant.RelayModeChatCompletions || mode == constant.RelayModeGeminiChat
}

// getRelayAction 获取当前 relay 模式对应的 Gemini action 名称
func (a *Adaptor) getRelayAction(info *common.RelayInfo) (string, error) {
	switch constant.RelayMode(info.RelayMode) {
	case constant.RelayModeChatCompletions, constant.RelayModeGeminiChat:
		if info.IsStream {
			return "streamGenerateContent", nil
		}
		return "generateContent", nil
	case constant.RelayModeEmbeddings:
		return "embedContent", nil
	case constant.RelayModeImagesGenerations:
		if strings.HasPrefix(info.ChannelMeta.UpstreamModelName, "imagen") {
			return "predict", nil
		}
		return "generateContent", nil
	default:
		return "", fmt.Errorf("unsupported relay mode for Gemini: %d", info.RelayMode)
	}
}

// GetRequestURL 构建上游请求 URL
func (a *Adaptor) GetRequestURL(info *common.RelayInfo) (string, error) {
	// Code Assist 模式：路由到 cloudcode-pa.googleapis.com（图片生成不支持，需 API Key 渠道）
	if _, isCA := a.isCodeAssistMode(); isCA {
		if constant.RelayMode(info.RelayMode) == constant.RelayModeImagesGenerations {
			return "", fmt.Errorf("Code Assist OAuth 不支持图片生成，请使用 API Key 渠道")
		}
		action, err := a.getRelayAction(info)
		if err != nil {
			return "", err
		}
		if action == "generateContent" {
			action = "streamGenerateContent"
		}
		url := fmt.Sprintf("%s/v1internal:%s", codeAssistBaseURL, action)
		if action == "streamGenerateContent" {
			url += "?alt=sse"
		}
		return url, nil
	}

	// 标准模式（API Key 或 OAuth 无 project_id）
	baseURL := strings.TrimSuffix(info.ChannelMeta.BaseURL, "/")
	model := info.ChannelMeta.UpstreamModelName

	switch constant.RelayMode(info.RelayMode) {
	case constant.RelayModeChatCompletions, constant.RelayModeGeminiChat:
		if info.IsStream {
			return fmt.Sprintf("%s/v1beta/models/%s:streamGenerateContent?alt=sse", baseURL, model), nil
		}
		return fmt.Sprintf("%s/v1beta/models/%s:generateContent", baseURL, model), nil
	case constant.RelayModeEmbeddings:
		return fmt.Sprintf("%s/v1beta/models/%s:embedContent", baseURL, model), nil
	case constant.RelayModeImagesGenerations:
		if strings.HasPrefix(model, "imagen") {
			return fmt.Sprintf("%s/v1beta/models/%s:predict", baseURL, model), nil
		}
		return fmt.Sprintf("%s/v1beta/models/%s:generateContent", baseURL, model), nil
	default:
		return "", fmt.Errorf("unsupported relay mode for Gemini: %d", info.RelayMode)
	}
}

// SetupRequestHeader 设置上游请求头
func (a *Adaptor) SetupRequestHeader(header http.Header, info *common.RelayInfo) error {
	apiKey := info.ChannelMeta.ApiKey

	// OAuth 模式：使用 Bearer token 而非 x-goog-api-key
	if loauth.IsOAuthKeyData(apiKey) {
		var oauthData loauth.OAuthKeyData
		if err := json.Unmarshal([]byte(apiKey), &oauthData); err == nil {
			header.Set("Authorization", "Bearer "+oauthData.AccessToken)
		}
		// Code Assist 模式：添加 Gemini CLI User-Agent
		if oauthData.ProjectID != "" {
			header.Set("User-Agent", geminiCLIUserAgent)
		}
	} else {
		header.Set("x-goog-api-key", apiKey)
	}
	header.Set("Content-Type", "application/json")
	header.Set("Accept", "application/json")

	if info.RequestHeaders != nil {
		for _, h := range []string{"X-Request-Id"} {
			if v := info.RequestHeaders.Get(h); v != "" {
				header.Set(h, v)
			}
		}
	}

	return nil
}

// ConvertRequest 根据入站格式转换请求体为 Gemini 格式
func (a *Adaptor) ConvertRequest(ctx context.Context, info *common.RelayInfo, requestBody []byte) (io.Reader, error) {
	// 图片生成模式：Imagen 走 predict 格式，其他走 generateContent + ResponseModalities
	if constant.RelayMode(info.RelayMode) == constant.RelayModeImagesGenerations {
		if strings.HasPrefix(info.ChannelMeta.UpstreamModelName, "imagen") {
			return convertImageRequest(requestBody, info)
		}
		return convertImageRequestToChat(requestBody, info)
	}

	var converted io.Reader
	switch info.InboundFormat {
	case constant.RelayFormatGemini:
		// Gemini 原生格式通过 URL 路径控制流式，body 中的 "stream" 字段会导致上游报错
		cleaned := helper.StripStreamField(requestBody)
		converted = bytes.NewReader(cleaned)
	case constant.RelayFormatOpenAI:
		r, err := ConvertOpenAIToGemini(requestBody, info)
		if err != nil {
			return nil, err
		}
		converted = r
	case constant.RelayFormatClaude:
		r, err := ConvertClaudeToGemini(requestBody, info)
		if err != nil {
			return nil, err
		}
		converted = r
	case constant.RelayFormatResponses:
		r, err := ConvertResponsesToGemini(requestBody, info)
		if err != nil {
			return nil, err
		}
		converted = r
	default:
		r, err := ConvertOpenAIToGemini(requestBody, info)
		if err != nil {
			return nil, err
		}
		converted = r
	}

	// Thinking 后缀路由
	if info.ThinkingEnabled || info.ReasoningEffort != "" {
		converted = injectGeminiThinking(converted, info)
	}

	return converted, nil
}

// injectGeminiThinking 注入 Gemini thinking 配置
func injectGeminiThinking(r io.Reader, info *common.RelayInfo) io.Reader {
	body, err := io.ReadAll(r)
	if err != nil {
		return r
	}
	var req map[string]json.RawMessage
	if err := json.Unmarshal(body, &req); err != nil {
		return bytes.NewReader(body)
	}

	if info.ThinkingEnabled {
		// -thinking: 设置 thoughtBudget
		var maxTokens int
		if mt, ok := req["maxOutputTokens"]; ok {
			_ = json.Unmarshal(mt, &maxTokens)
		}
		if maxTokens < 128 {
			maxTokens = 8192
		}
		budget := maxTokens * 80 / 100
		if budget < 128 {
			budget = 128
		}
		req["thinkingConfig"] = json.RawMessage(fmt.Sprintf(`{"thoughtBudget":%d,"includeThoughts":true}`, budget))
	} else if info.ReasoningEffort != "" {
		// effort 后缀：设置 thinkingLevel
		req["thinkingConfig"] = json.RawMessage(fmt.Sprintf(`{"thinkingLevel":"%s","includeThoughts":true}`,
			strings.ToUpper(info.ReasoningEffort)))
	}

	result, err := json.Marshal(req)
	if err != nil {
		return bytes.NewReader(body)
	}
	return bytes.NewReader(result)
}

// wrapCodeAssistBody 将 Gemini 请求体包装为 Code Assist 格式
// Code Assist 格式：{"model":"...", "project":"...", "request":{原始body}}
func (a *Adaptor) wrapCodeAssistBody(body io.Reader, info *common.RelayInfo, oauthData *loauth.OAuthKeyData) (io.Reader, error) {
	bodyBytes, err := io.ReadAll(body)
	if err != nil {
		return nil, fmt.Errorf("read request body: %w", err)
	}

	wrapped := map[string]any{
		"model":   info.ChannelMeta.UpstreamModelName,
		"project": oauthData.ProjectID,
		"request": json.RawMessage(bodyBytes),
	}

	result, err := json.Marshal(wrapped)
	if err != nil {
		return nil, fmt.Errorf("marshal wrapped request: %w", err)
	}
	return bytes.NewReader(result), nil
}

// DoRequest 发送请求到上游
func (a *Adaptor) DoRequest(ctx context.Context, info *common.RelayInfo, requestBody io.Reader) (*http.Response, error) {
	reqURL, err := a.GetRequestURL(info)
	if err != nil {
		return nil, err
	}

	// Code Assist 模式：包装请求体（图片生成跳过，已在 GetRequestURL 拦截）
	if oauthData, isCA := a.isCodeAssistMode(); isCA {
		wrappedBody, err := a.wrapCodeAssistBody(requestBody, info, oauthData)
		if err != nil {
			return nil, fmt.Errorf("wrap code assist body failed: %w", err)
		}
		requestBody = wrappedBody
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

// DoResponse 处理上游响应，根据客户端格式分发
func (a *Adaptor) DoResponse(ctx context.Context, resp *http.Response, info *common.RelayInfo, writer http.ResponseWriter) (*common.Usage, error) {
	// 图片生成响应：Imagen 走 predict 响应格式，其他走 generateContent 响应格式
	if constant.RelayMode(info.RelayMode) == constant.RelayModeImagesGenerations {
		if strings.HasPrefix(info.ChannelMeta.UpstreamModelName, "imagen") {
			return handleImagenResponse(ctx, resp, info, writer)
		}
		return handleBananaImageResponse(ctx, resp, writer)
	}

	// Code Assist 强制流式：聚合 SSE 为非流式响应
	if a.isCodeAssistForcedStream() {
		return a.handleCodeAssistAggregatedStream(ctx, resp, info, writer)
	}

	clientFormat := info.GetOriginalClientFormat()

	switch clientFormat {
	case constant.RelayFormatGemini:
		return a.handleGeminiNativeResponse(ctx, resp, info, writer)
	case constant.RelayFormatOpenAI:
		if info.IsStream {
			return a.handleStreamToOpenAI(ctx, resp, info, writer)
		}
		return a.handleNonStreamToOpenAI(ctx, resp, info, writer)
	default:
		if info.IsStream {
			return a.handleStreamToOpenAI(ctx, resp, info, writer)
		}
		return a.handleNonStreamToOpenAI(ctx, resp, info, writer)
	}
}

// handleCodeAssistAggregatedStream 聚合 Code Assist 强制流式响应为非流式
func (a *Adaptor) handleCodeAssistAggregatedStream(ctx context.Context, resp *http.Response, info *common.RelayInfo, writer http.ResponseWriter) (*common.Usage, error) {
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		body = unwrapCodeAssistData(body)
		clientFormat := info.GetOriginalClientFormat()
		if clientFormat == constant.RelayFormatGemini {
			writer.Header().Set("Content-Type", "application/json")
			writer.WriteHeader(resp.StatusCode)
			_, _ = writer.Write(body)
		} else {
			writeGeminiErrorAsOpenAI(writer, body, resp.StatusCode)
		}
		return nil, constant.NewUpstreamError(resp.StatusCode, string(body), nil)
	}

	// 读取所有 SSE chunks 并聚合
	scanner := bufio.NewScanner(resp.Body)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 10*1024*1024)

	var (
		aggregated  dto.GeminiChatResponse
		totalUsage  dto.GeminiUsageMetadata
		modelName   string
		textParts   []string
		thoughtText []string
		lastFinish  string
		toolCalls   []dto.ToolCall
		toolCallIdx int
	)

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return &common.Usage{}, common.ErrStreamInterrupted
		default:
		}

		line := scanner.Text()
		if line == "" {
			continue
		}
		if !strings.HasPrefix(line, "data:") {
			continue
		}

		data, _ := helper.ExtractSSEData(line)
		if data == "" || data == "[DONE]" {
			if data == "[DONE]" {
				break
			}
			continue
		}

		info.SetFirstResponseTime()

		rawData := []byte(data)
		rawData = unwrapCodeAssistData(rawData)

		var chunk dto.GeminiChatResponse
		if err := json.Unmarshal(rawData, &chunk); err != nil {
			continue
		}

		// 收集 usage
		if chunk.UsageMetadata != nil {
			totalUsage = *chunk.UsageMetadata
		}
		if chunk.ModelName != "" {
			modelName = chunk.ModelName
		}

		// 检查安全过滤
		if chunk.PromptFeedback != nil && chunk.PromptFeedback.BlockReason != "" {
			return nil, constant.NewRequestError(
				fmt.Sprintf("request blocked by Gemini safety filter: %s", chunk.PromptFeedback.BlockReason), nil,
			)
		}

		// 聚合 candidates
		for _, candidate := range chunk.Candidates {
			if candidate.FinishReason != "" {
				lastFinish = candidate.FinishReason
			}
			if candidate.Content == nil {
				continue
			}
			for _, part := range candidate.Content.Parts {
				isThought := part.Thought != nil && *part.Thought
				if part.Text != "" {
					if isThought {
						thoughtText = append(thoughtText, part.Text)
					} else {
						textParts = append(textParts, part.Text)
					}
				}
				if part.FunctionCall != nil {
					toolCalls = appendToolCall(toolCalls, &toolCallIdx, part)
				}
			}
		}
	}

	// 构建 Gemini 响应
	var fullText string
	if len(textParts) > 0 {
		fullText = strings.Join(textParts, "")
	}

	var thoughtFullText string
	if len(thoughtText) > 0 {
		thoughtFullText = strings.Join(thoughtText, "")
	}

	content := &dto.GeminiContent{Role: "model"}
	if fullText != "" {
		content.Parts = append(content.Parts, dto.GeminiPart{Text: fullText})
	}
	if thoughtFullText != "" {
		t := true
		content.Parts = append(content.Parts, dto.GeminiPart{Text: thoughtFullText, Thought: &t})
	}
	for _, tc := range toolCalls {
		content.Parts = append(content.Parts, dto.GeminiPart{FunctionCall: &dto.GeminiFunctionCall{
			FunctionName: tc.Function.Name,
			Arguments:    tc.Function.Arguments,
		}})
	}

	aggregated.Candidates = []dto.GeminiCandidate{{
		Content:      content,
		FinishReason: lastFinish,
	}}
	aggregated.UsageMetadata = &totalUsage
	aggregated.ModelName = modelName

	// 按客户端格式返回
	clientFormat := info.GetOriginalClientFormat()
	switch clientFormat {
	case constant.RelayFormatGemini:
		respBody, _ := json.Marshal(aggregated)
		writer.Header().Set("Content-Type", "application/json")
		writer.WriteHeader(http.StatusOK)
		_, _ = writer.Write(respBody)
	default:
		openaiResp := geminiToOpenAIResponse(&aggregated, info)
		respBody, _ := json.Marshal(openaiResp)
		writer.Header().Set("Content-Type", "application/json")
		writer.WriteHeader(http.StatusOK)
		_, _ = writer.Write(respBody)
	}

	return geminiUsageToCommon(&totalUsage), nil
}

// appendToolCall 添加工具调用到列表
func appendToolCall(toolCalls []dto.ToolCall, idx *int, part dto.GeminiPart) []dto.ToolCall {
	if part.FunctionCall == nil {
		return toolCalls
	}
	argsJSON, _ := json.Marshal(part.FunctionCall.Arguments)
	tc := dto.ToolCall{
		ID:    fmt.Sprintf("call_%d", *idx),
		Type:  "function",
		Index: *idx,
		Function: dto.FunctionCall{
			Name:      part.FunctionCall.FunctionName,
			Arguments: string(argsJSON),
		},
	}
	*idx++
	return append(toolCalls, tc)
}

// GetChannelName 返回渠道名称
func (a *Adaptor) GetChannelName() string {
	return "Gemini"
}

// 确保接口实现
var _ common.Adaptor = (*Adaptor)(nil)

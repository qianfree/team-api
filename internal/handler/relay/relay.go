package relay

import (
	"encoding/json"
	"fmt"
	"runtime/debug"
	"strings"
	"time"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/net/ghttp"

	"github.com/qianfree/team-api/internal/logic/billing"
	"github.com/qianfree/team-api/internal/logic/monitor"
	"github.com/qianfree/team-api/internal/logic/relay"
	"github.com/qianfree/team-api/internal/middleware"
	relay_common "github.com/qianfree/team-api/relay/common"
	relay_constant "github.com/qianfree/team-api/relay/constant"
	"github.com/qianfree/team-api/relay/handler"
)

var (
	dataProvider    = relay.NewDataProvider()
	billingProvider = billing.NewBillingProvider()
)

// handleRelayChatCompletions 处理 /v1/chat/completions
func HandleChatCompletions(r *ghttp.Request) {
	// DEBUG: 记录请求入口
	g.Log().Debugf(r.Context(), "[HandleChatCompletions] Entry: method=%s, path=%s, uri=%s",
		r.Method, r.URL.Path, r.RequestURI)

	body := r.GetBody()
	if len(body) == 0 {
		r.Response.WriteJsonExit(g.Map{
			"error": g.Map{
				"type":    "invalid_request_error",
				"message": "request body is empty",
			},
		})
		return
	}

	monitor.RecordBytesIn(len(body))

	rc := buildRelayContext(r)
	capture := NewResponseCaptureWriter(r.Response.Writer)
	rc.Writer = capture

	start := time.Now()
	_, billingResult, err := handler.HandleChatCompletions(r.Context(), body, "/chat/completions", r.Header, rc, dataProvider, billingProvider)
	latencyMs := int(time.Since(start).Milliseconds())
	monitor.RecordBytesOut(int(capture.BytesWritten()))
	setRateLimitHeaders(r, billingResult)
	setDeprecationHeaders(r, billingResult)

	if err != nil {
		handler.WriteRelayError(capture, err)
		recordAudit(r, rc, capture, body, "/chat/completions", latencyMs, firstTokenMs(billingResult))
		return
	}

	recordAudit(r, rc, capture, body, "/chat/completions", latencyMs, firstTokenMs(billingResult))
}

// handleRelayEmbeddings 处理 /v1/embeddings
func HandleEmbeddings(r *ghttp.Request) {
	body := r.GetBody()
	if len(body) == 0 {
		r.Response.WriteJsonExit(g.Map{
			"error": g.Map{
				"type":    "invalid_request_error",
				"message": "request body is empty",
			},
		})
		return
	}

	monitor.RecordBytesIn(len(body))
	rc := buildRelayContext(r)
	capture := NewResponseCaptureWriter(r.Response.Writer)
	rc.Writer = capture

	start := time.Now()
	_, billingResult, err := handler.HandleEmbeddings(r.Context(), body, "/embeddings", r.Header, rc, dataProvider, billingProvider)
	latencyMs := int(time.Since(start).Milliseconds())
	monitor.RecordBytesOut(int(capture.BytesWritten()))
	setRateLimitHeaders(r, billingResult)
	setDeprecationHeaders(r, billingResult)

	if err != nil {
		handler.WriteRelayError(capture, err)
		recordAudit(r, rc, capture, body, "/embeddings", latencyMs, firstTokenMs(billingResult))
		return
	}

	recordAudit(r, rc, capture, body, "/embeddings", latencyMs, firstTokenMs(billingResult))
}

// handleRelayImagesGenerations 处理 /v1/images/generations
func HandleImagesGenerations(r *ghttp.Request) {
	body := r.GetBody()
	if len(body) == 0 {
		r.Response.WriteJsonExit(g.Map{
			"error": g.Map{
				"type":    "invalid_request_error",
				"message": "request body is empty",
			},
		})
		return
	}

	monitor.RecordBytesIn(len(body))
	rc := buildRelayContext(r)
	capture := NewResponseCaptureWriter(r.Response.Writer)
	rc.Writer = capture

	start := time.Now()
	_, billingResult, err := handler.HandleImagesGenerations(r.Context(), body, "/images/generations", r.Header, rc, dataProvider, billingProvider)
	latencyMs := int(time.Since(start).Milliseconds())
	monitor.RecordBytesOut(int(capture.BytesWritten()))
	setRateLimitHeaders(r, billingResult)
	setDeprecationHeaders(r, billingResult)

	if err != nil {
		handler.WriteRelayError(capture, err)
		recordAudit(r, rc, capture, body, "/images/generations", latencyMs, firstTokenMs(billingResult))
		return
	}

	recordAudit(r, rc, capture, body, "/images/generations", latencyMs, firstTokenMs(billingResult))
}

// handleRelayCompletions 处理 /v1/completions
func HandleCompletions(r *ghttp.Request) {
	body := r.GetBody()
	if len(body) == 0 {
		r.Response.WriteJsonExit(g.Map{
			"error": g.Map{
				"type":    "invalid_request_error",
				"message": "request body is empty",
			},
		})
		return
	}

	monitor.RecordBytesIn(len(body))
	rc := buildRelayContext(r)
	capture := NewResponseCaptureWriter(r.Response.Writer)
	rc.Writer = capture

	start := time.Now()
	_, billingResult, err := handler.HandleCompletions(r.Context(), body, "/completions", r.Header, rc, dataProvider, billingProvider)
	latencyMs := int(time.Since(start).Milliseconds())
	monitor.RecordBytesOut(int(capture.BytesWritten()))
	setRateLimitHeaders(r, billingResult)
	setDeprecationHeaders(r, billingResult)

	if err != nil {
		handler.WriteRelayError(capture, err)
		recordAudit(r, rc, capture, body, "/completions", latencyMs, firstTokenMs(billingResult))
		return
	}

	recordAudit(r, rc, capture, body, "/completions", latencyMs, firstTokenMs(billingResult))
}

// HandleResponses 处理 /v1/responses（OpenAI Responses API）
func HandleResponses(r *ghttp.Request) {
	// DEBUG: 记录请求入口
	g.Log().Debugf(r.Context(), "[HandleResponses] Entry: method=%s, path=%s, uri=%s",
		r.Method, r.URL.Path, r.RequestURI)

	body := r.GetBody()
	if len(body) == 0 {
		r.Response.WriteJsonExit(g.Map{
			"error": g.Map{
				"type":    "invalid_request_error",
				"message": "request body is empty",
			},
		})
		return
	}

	monitor.RecordBytesIn(len(body))
	rc := buildRelayContext(r)
	capture := NewResponseCaptureWriter(r.Response.Writer)
	rc.Writer = capture

	start := time.Now()
	_, billingResult, err := handler.HandleResponses(r.Context(), body, r.URL.Path, r.Header, rc, dataProvider, billingProvider)
	latencyMs := int(time.Since(start).Milliseconds())
	monitor.RecordBytesOut(int(capture.BytesWritten()))
	setRateLimitHeaders(r, billingResult)
	setDeprecationHeaders(r, billingResult)

	if err != nil {
		handler.WriteRelayError(capture, err)
		recordAudit(r, rc, capture, body, "/responses", latencyMs, firstTokenMs(billingResult))
		return
	}

	recordAudit(r, rc, capture, body, "/responses", latencyMs, firstTokenMs(billingResult))
}

// handleRelayMessages 处理 /v1/messages（Claude 原生格式）
func HandleMessages(r *ghttp.Request) {
	body := r.GetBody()
	if len(body) == 0 {
		r.Response.WriteJsonExit(g.Map{
			"type":  "error",
			"error": g.Map{"type": "invalid_request_error", "message": "request body is empty"},
		})
		return
	}

	monitor.RecordBytesIn(len(body))
	rc := buildRelayContext(r)
	capture := NewResponseCaptureWriter(r.Response.Writer)
	rc.Writer = capture

	start := time.Now()
	_, billingResult, err := handler.HandleClaudeMessages(r.Context(), body, "/messages", r.Header, rc, dataProvider, billingProvider)
	latencyMs := int(time.Since(start).Milliseconds())
	monitor.RecordBytesOut(int(capture.BytesWritten()))
	setRateLimitHeaders(r, billingResult)
	setDeprecationHeaders(r, billingResult)

	if err != nil {
		handler.WriteClaudeRelayError(capture, err)
		recordAudit(r, rc, capture, body, "/messages", latencyMs, firstTokenMs(billingResult))
		return
	}

	recordAudit(r, rc, capture, body, "/messages", latencyMs, firstTokenMs(billingResult))
}

// HandleGeminiGenerateContent 处理 /v1beta/models/{model}:generateContent（Gemini 原生格式）
func HandleGeminiGenerateContent(r *ghttp.Request) {
	g.Log().Debugf(r.Context(), "[HandleGeminiGenerateContent] Entry: method=%s, path=%s, uri=%s",
		r.Method, r.URL.Path, r.RequestURI)

	body := r.GetBody()
	if len(body) == 0 {
		r.Response.WriteJsonExit(g.Map{
			"error": g.Map{"code": 400, "message": "request body is empty", "status": "INVALID_ARGUMENT"},
		})
		return
	}

	// 从 URL 参数提取 model:action（如 "gemini-2.0-flash:generateContent"）
	modelAction := r.Get("model").String()
	if modelAction == "" {
		r.Response.WriteJsonExit(g.Map{
			"error": g.Map{"code": 400, "message": "model not found in path", "status": "INVALID_ARGUMENT"},
		})
		return
	}

	// 解析模型名和 action
	var modelName string
	action := "generateContent"
	if idx := strings.Index(modelAction, ":"); idx != -1 {
		modelName = modelAction[:idx]
		action = modelAction[idx+1:]
	} else {
		modelName = modelAction
	}

	if modelName == "" {
		r.Response.WriteJsonExit(g.Map{
			"error": g.Map{"code": 400, "message": "model name is empty", "status": "INVALID_ARGUMENT"},
		})
		return
	}

	// 判断是否流式
	isStream := action == "streamGenerateContent"

	// 向 body 注入 "model" 和 "stream" 字段（Gemini body 原本不含这些字段）
	var rawBody map[string]json.RawMessage
	if err := json.Unmarshal(body, &rawBody); err != nil {
		r.Response.WriteJsonExit(g.Map{
			"error": g.Map{"code": 400, "message": "invalid request body", "status": "INVALID_ARGUMENT"},
		})
		return
	}
	modelJSON, _ := json.Marshal(modelName)
	rawBody["model"] = modelJSON
	streamJSON, _ := json.Marshal(isStream)
	rawBody["stream"] = streamJSON
	modifiedBody, err := json.Marshal(rawBody)
	if err != nil {
		r.Response.WriteJsonExit(g.Map{
			"error": g.Map{"code": 400, "message": "failed to encode request body", "status": "INVALID_ARGUMENT"},
		})
		return
	}

	// 构造路径（使 Path2RelayMode 能识别为 Gemini 模式）
	path := "/models/" + modelAction

	monitor.RecordBytesIn(len(body))
	rc := buildRelayContext(r)
	capture := NewResponseCaptureWriter(r.Response.Writer)
	rc.Writer = capture

	start := time.Now()
	_, billingResult, err := handler.HandleGeminiGenerateContent(r.Context(), modifiedBody, path, r.Header, rc, dataProvider, billingProvider)
	latencyMs := int(time.Since(start).Milliseconds())
	monitor.RecordBytesOut(int(capture.BytesWritten()))
	setRateLimitHeaders(r, billingResult)
	setDeprecationHeaders(r, billingResult)

	if err != nil {
		handler.WriteGeminiRelayError(capture, err)
		recordAudit(r, rc, capture, modifiedBody, path, latencyMs, firstTokenMs(billingResult))
		return
	}

	recordAudit(r, rc, capture, modifiedBody, path, latencyMs, firstTokenMs(billingResult))
}

// handleRelayModels 处理 /v1/models
func HandleModels(r *ghttp.Request) {
	rc := buildRelayContext(r) // 获取 TenantID

	if !billingProvider.CheckScope(rc.Scope, "") {
		handler.WriteRelayError(r.Response.Writer, relay_constant.NewAuthError("API key scope denied"))
		return
	}
	if !billingProvider.CheckIPWhitelist(rc.KeyIpWhitelist, rc.ClientIP) {
		handler.WriteRelayError(r.Response.Writer, relay_constant.NewAuthError("IP address is not allowed"))
		return
	}

	resp, err := handler.HandleModels(r.Context(), rc.TenantID, rc.ApiKeyID, rc.UserID, dataProvider)
	if err != nil {
		r.Response.WriteJsonExit(g.Map{
			"error": g.Map{"type": "internal_error", "message": err.Error()},
		})
		return
	}

	r.Response.WriteJsonExit(resp)
}

// handleRelayModelDetail 处理 /v1/models/{model_id}
func HandleModelDetail(r *ghttp.Request) {
	rc := buildRelayContext(r)
	modelName := r.Get("model_id").String()

	if !billingProvider.CheckScope(rc.Scope, "") {
		handler.WriteRelayError(r.Response.Writer, relay_constant.NewAuthError("API key scope denied"))
		return
	}
	if !billingProvider.CheckIPWhitelist(rc.KeyIpWhitelist, rc.ClientIP) {
		handler.WriteRelayError(r.Response.Writer, relay_constant.NewAuthError("IP address is not allowed"))
		return
	}

	if modelName == "" {
		r.Response.WriteJsonExit(g.Map{
			"error": g.Map{"type": "invalid_request_error", "message": "model_id is required"},
		})
		return
	}

	resp, err := handler.HandleModelDetail(r.Context(), rc.TenantID, modelName, dataProvider)
	if err != nil {
		// 根据错误类型返回不同状态码
		statusCode := 500
		errType := "internal_error"
		errMsg := err.Error()

		if err == relay_common.ErrModelNotFound {
			statusCode = 404
			errType = "model_not_found"
			errMsg = "The model '" + modelName + "' does not exist"
		} else if err == relay_common.ErrTenantModelNotEnabled {
			statusCode = 403
			errType = "permission_denied"
			errMsg = "You do not have access to model '" + modelName + "'"
		}

		r.Response.WriteHeader(statusCode)
		r.Response.WriteJson(g.Map{
			"error": g.Map{"type": errType, "message": errMsg},
		})
		return
	}

	r.Response.WriteJsonExit(resp)
}

// HandleGeminiModels 处理 GET /v1beta/models（Gemini 格式模型列表）
func HandleGeminiModels(r *ghttp.Request) {
	rc := buildRelayContext(r)

	if !billingProvider.CheckScope(rc.Scope, "") {
		handler.WriteGeminiRelayError(r.Response.Writer, relay_constant.NewAuthError("API key scope denied"))
		return
	}
	if !billingProvider.CheckIPWhitelist(rc.KeyIpWhitelist, rc.ClientIP) {
		handler.WriteGeminiRelayError(r.Response.Writer, relay_constant.NewAuthError("IP address is not allowed"))
		return
	}

	resp, err := handler.HandleGeminiModels(r.Context(), rc.TenantID, rc.ApiKeyID, rc.UserID, dataProvider)
	if err != nil {
		handler.WriteGeminiRelayError(r.Response.Writer, err)
		return
	}

	r.Response.WriteJsonExit(resp)
}

// HandleGeminiModelDetail 处理 GET /v1beta/models/{model}（Gemini 格式模型详情）
func HandleGeminiModelDetail(r *ghttp.Request) {
	rc := buildRelayContext(r)
	modelName := r.Get("model").String()

	if !billingProvider.CheckScope(rc.Scope, "") {
		handler.WriteGeminiRelayError(r.Response.Writer, relay_constant.NewAuthError("API key scope denied"))
		return
	}
	if !billingProvider.CheckIPWhitelist(rc.KeyIpWhitelist, rc.ClientIP) {
		handler.WriteGeminiRelayError(r.Response.Writer, relay_constant.NewAuthError("IP address is not allowed"))
		return
	}

	if modelName == "" {
		r.Response.WriteHeader(400)
		r.Response.WriteJson(g.Map{
			"error": g.Map{"code": 400, "message": "model name is required", "status": "INVALID_ARGUMENT"},
		})
		return
	}

	resp, err := handler.HandleGeminiModelDetail(r.Context(), rc.TenantID, modelName, dataProvider)
	if err != nil {
		handler.WriteGeminiRelayError(r.Response.Writer, err)
		return
	}

	r.Response.WriteJsonExit(resp)
}

// buildRelayContext 从 GoFrame 请求中构建 relay 上下文
func buildRelayContext(r *ghttp.Request) *handler.RelayContext {
	return &handler.RelayContext{
		TenantID:        middleware.GetTenantID(r.Context()),
		UserID:          middleware.GetUserID(r.Context()),
		ApiKeyID:        middleware.GetApiKeyID(r.Context()),
		ProjectID:       middleware.GetProjectID(r.Context()),
		RequestID:       r.GetCtxVar("RequestId").String(),
		Writer:          r.Response.Writer,
		Scope:           r.GetCtxVar("ApiKeyScope").String(),
		ClientIP:        r.GetClientIp(),
		KeyRateLimitQps: r.GetCtxVar(middleware.CtxKeyApiKeyRateLimitQps).Int(),
		KeyConcurrency:  r.GetCtxVar(middleware.CtxKeyApiKeyRateLimitConcurrency).Int(),
		KeyIpWhitelist:  r.GetCtxVar(middleware.CtxKeyApiKeyIpWhitelist).String(),
		KeyTotalQuota:   r.GetCtxVar(middleware.CtxKeyApiKeyTotalQuota).Float64(),
		KeyUsedQuota:    r.GetCtxVar(middleware.CtxKeyApiKeyUsedQuota).Float64(),
	}
}

// firstTokenMs 从 BillingResult 提取首个 Token 用时
func firstTokenMs(br *handler.BillingResult) int {
	if br == nil {
		return 0
	}
	return br.FirstTokenMs
}

// captureRequestHeaders 从请求中提取需要记录的请求头（排除敏感头）
func captureRequestHeaders(r *ghttp.Request) map[string]string {
	// 排除的敏感头，不记录到审计日志
	skip := map[string]bool{
		"authorization": true,
		"cookie":        true,
	}
	headers := make(map[string]string)
	for k, vals := range r.Header {
		if skip[strings.ToLower(k)] {
			continue
		}
		if len(vals) > 0 {
			headers[k] = vals[0]
		}
	}
	return headers
}

// recordAudit 收集请求元数据并异步记录审计日志
func recordAudit(r *ghttp.Request, rc *handler.RelayContext, capture *ResponseCaptureWriter, body []byte, path string, latencyMs int, ttft int) {
	// multipart / 二进制请求体（如图片编辑、音频转写）含整段二进制，强行 Unmarshal
	// 无意义，原样写入会把 base64 图片灌进审计表。这里按 Content-Type 判定后只记元数据。
	requestBody := string(body)
	isMultipart := isBinaryContentType(r.Header.Get("Content-Type"))

	// 解析 stream 标志
	var isStream bool
	var modelName string
	if !isMultipart {
		var rawRequest map[string]json.RawMessage
		if err := json.Unmarshal(body, &rawRequest); err == nil {
			if streamVal, ok := rawRequest["stream"]; ok {
				_ = json.Unmarshal(streamVal, &isStream)
			}
			if modelVal, ok := rawRequest["model"]; ok {
				_ = json.Unmarshal(modelVal, &modelName)
			}
		}
	} else {
		// 二进制体不落原文，仅标注类型，便于排查又不污染审计表
		requestBody = "[binary body omitted]"
	}

	// 捕获响应体（流式请求记录 SSE 原始数据，非流式记录完整响应）
	var responseBody string
	responseBody = capture.Body()

	dataProvider.RecordAudit(r.Context(), &relay_common.AuditRecord{
		TenantID:        rc.TenantID,
		UserID:          rc.UserID,
		ApiKeyID:        rc.ApiKeyID,
		ProjectID:       rc.ProjectID,
		RequestID:       rc.RequestID,
		Method:          r.Method,
		Path:            path,
		QueryParams:     r.URL.RawQuery,
		StatusCode:      capture.StatusCode(),
		ClientIP:        rc.ClientIP,
		UserAgent:       r.Header.Get("User-Agent"),
		Model:           modelName,
		RequestBody:     requestBody,
		ResponseBody:    responseBody,
		LatencyMs:       latencyMs,
		FirstTokenMs:    ttft,
		IsStream:        isStream,
		RequestHeaders:  captureRequestHeaders(r),
		ResponseHeaders: capture.ResponseHeaders(),
		ForwardingTrace: rc.ForwardingTrace,
	})
}

// isBinaryContentType 判断请求是否为 multipart 或其它二进制类型，
// 用于跳过对请求体的 JSON 解析与原文落库。与 response.isBinaryContentType 语义一致，
// 此处独立实现避免 handler → response 的反向依赖。
func isBinaryContentType(contentType string) bool {
	lower := strings.ToLower(contentType)
	if lower == "" {
		return false
	}
	if i := strings.IndexByte(lower, ';'); i >= 0 {
		lower = strings.TrimSpace(lower[:i])
	}
	switch {
	case strings.HasPrefix(lower, "multipart/"),
		strings.HasPrefix(lower, "image/"),
		strings.HasPrefix(lower, "audio/"),
		strings.HasPrefix(lower, "video/"),
		lower == "application/octet-stream":
		return true
	}
	return false
}

// setRateLimitHeaders 设置限流响应头
func setRateLimitHeaders(r *ghttp.Request, billingResult *handler.BillingResult) {
	if billingResult == nil || billingResult.RateLimitInfo == nil {
		return
	}
	info := billingResult.RateLimitInfo
	r.Response.Header().Set("X-RateLimit-Limit", fmt.Sprintf("%d", info.Limit))
	r.Response.Header().Set("X-RateLimit-Remaining", fmt.Sprintf("%d", info.Remaining))
	r.Response.Header().Set("X-RateLimit-Reset", fmt.Sprintf("%d", info.ResetAt))
}

// setDeprecationHeaders 设置模型弃用响应头
func setDeprecationHeaders(r *ghttp.Request, billingResult *handler.BillingResult) {
	if billingResult == nil || billingResult.Deprecation == nil {
		return
	}
	dep := billingResult.Deprecation
	r.Response.Header().Set("Deprecation", "true")
	if dep.SunsetDate != "" {
		r.Response.Header().Set("Sunset", dep.SunsetDate)
	}
	if dep.ReplacementModel != "" {
		r.Response.Header().Set("Link", fmt.Sprintf("</v1/models/%s>; rel=\"successor-version\"", dep.ReplacementModel))
	}
}

// HandleAudioSpeech 处理 /v1/audio/speech（TTS）
func HandleAudioSpeech(r *ghttp.Request) {
	body := r.GetBody()
	if len(body) == 0 {
		r.Response.WriteJsonExit(g.Map{
			"error": g.Map{"type": "invalid_request_error", "message": "request body is empty"},
		})
		return
	}

	monitor.RecordBytesIn(len(body))
	rc := buildRelayContext(r)
	capture := NewResponseCaptureWriter(r.Response.Writer)
	rc.Writer = capture

	start := time.Now()
	_, billingResult, err := handler.HandleAudioSpeech(r.Context(), body, "/audio/speech", r.Header, rc, dataProvider, billingProvider)
	latencyMs := int(time.Since(start).Milliseconds())
	monitor.RecordBytesOut(int(capture.BytesWritten()))
	setRateLimitHeaders(r, billingResult)
	setDeprecationHeaders(r, billingResult)

	if err != nil {
		handler.WriteRelayError(capture, err)
		recordAudit(r, rc, capture, body, "/audio/speech", latencyMs, firstTokenMs(billingResult))
		return
	}

	recordAudit(r, rc, capture, body, "/audio/speech", latencyMs, firstTokenMs(billingResult))
}

// HandleAudioTranscription 处理 /v1/audio/transcriptions（STT）
func HandleAudioTranscription(r *ghttp.Request) {
	body := r.GetBody()
	if len(body) == 0 {
		r.Response.WriteJsonExit(g.Map{
			"error": g.Map{"type": "invalid_request_error", "message": "request body is empty"},
		})
		return
	}

	monitor.RecordBytesIn(len(body))
	rc := buildRelayContext(r)
	capture := NewResponseCaptureWriter(r.Response.Writer)
	rc.Writer = capture

	start := time.Now()
	_, billingResult, err := handler.HandleAudioTranscription(r.Context(), body, "/audio/transcriptions", r.Header, rc, dataProvider, billingProvider)
	latencyMs := int(time.Since(start).Milliseconds())
	monitor.RecordBytesOut(int(capture.BytesWritten()))
	setRateLimitHeaders(r, billingResult)
	setDeprecationHeaders(r, billingResult)

	if err != nil {
		handler.WriteRelayError(capture, err)
		recordAudit(r, rc, capture, body, "/audio/transcriptions", latencyMs, firstTokenMs(billingResult))
		return
	}

	recordAudit(r, rc, capture, body, "/audio/transcriptions", latencyMs, firstTokenMs(billingResult))
}

// HandleAudioTranslation 处理 /v1/audio/translations
func HandleAudioTranslation(r *ghttp.Request) {
	body := r.GetBody()
	if len(body) == 0 {
		r.Response.WriteJsonExit(g.Map{
			"error": g.Map{"type": "invalid_request_error", "message": "request body is empty"},
		})
		return
	}

	monitor.RecordBytesIn(len(body))
	rc := buildRelayContext(r)
	capture := NewResponseCaptureWriter(r.Response.Writer)
	rc.Writer = capture

	start := time.Now()
	_, billingResult, err := handler.HandleAudioTranslation(r.Context(), body, "/audio/translations", r.Header, rc, dataProvider, billingProvider)
	latencyMs := int(time.Since(start).Milliseconds())
	monitor.RecordBytesOut(int(capture.BytesWritten()))
	setRateLimitHeaders(r, billingResult)
	setDeprecationHeaders(r, billingResult)

	if err != nil {
		handler.WriteRelayError(capture, err)
		recordAudit(r, rc, capture, body, "/audio/translations", latencyMs, firstTokenMs(billingResult))
		return
	}

	recordAudit(r, rc, capture, body, "/audio/translations", latencyMs, firstTokenMs(billingResult))
}

// HandleRerank 处理 /v1/rerank
func HandleRerank(r *ghttp.Request) {
	body := r.GetBody()
	if len(body) == 0 {
		r.Response.WriteJsonExit(g.Map{
			"error": g.Map{"type": "invalid_request_error", "message": "request body is empty"},
		})
		return
	}

	monitor.RecordBytesIn(len(body))
	rc := buildRelayContext(r)
	capture := NewResponseCaptureWriter(r.Response.Writer)
	rc.Writer = capture

	start := time.Now()
	_, billingResult, err := handler.HandleRerank(r.Context(), body, "/rerank", r.Header, rc, dataProvider, billingProvider)
	latencyMs := int(time.Since(start).Milliseconds())
	monitor.RecordBytesOut(int(capture.BytesWritten()))
	setRateLimitHeaders(r, billingResult)
	setDeprecationHeaders(r, billingResult)

	if err != nil {
		handler.WriteRelayError(capture, err)
		recordAudit(r, rc, capture, body, "/rerank", latencyMs, firstTokenMs(billingResult))
		return
	}

	recordAudit(r, rc, capture, body, "/rerank", latencyMs, firstTokenMs(billingResult))
}

// HandleRealtime 处理 /v1/realtime（WebSocket）
func HandleRealtime(r *ghttp.Request) {
	// WebSocket 长连接脱离标准 HTTP 中间件链路，全局 Recovery 中间件无法兜底
	// 连接升级后的 panic，故在此显式 recover，避免单个连接 panic 导致 goroutine 崩溃。
	defer func() {
		if rec := recover(); rec != nil {
			stack := debugStack()
			g.Log().Criticalf(r.Context(), "[HandleRealtime] panic recovered: %v\nRequest-ID: %s\nPath: %s\n%s",
				rec, r.GetCtxVar("RequestId").String(), r.URL.Path, stack)
		}
	}()

	start := time.Now()
	g.Log().Debugf(r.Context(), "[HandleRealtime] connection accepted: request_id=%s, ip=%s",
		r.GetCtxVar("RequestId").String(), r.GetClientIp())

	rc := &handler.RealtimeContext{
		TenantID:        middleware.GetTenantID(r.Context()),
		UserID:          middleware.GetUserID(r.Context()),
		ApiKeyID:        middleware.GetApiKeyID(r.Context()),
		ProjectID:       middleware.GetProjectID(r.Context()),
		RequestID:       r.GetCtxVar("RequestId").String(),
		ClientIP:        r.GetClientIp(),
		Scope:           r.GetCtxVar("ApiKeyScope").String(),
		KeyRateLimitQps: r.GetCtxVar(middleware.CtxKeyApiKeyRateLimitQps).Int(),
		KeyConcurrency:  r.GetCtxVar(middleware.CtxKeyApiKeyRateLimitConcurrency).Int(),
		KeyIpWhitelist:  r.GetCtxVar(middleware.CtxKeyApiKeyIpWhitelist).String(),
		KeyTotalQuota:   r.GetCtxVar(middleware.CtxKeyApiKeyTotalQuota).Float64(),
		KeyUsedQuota:    r.GetCtxVar(middleware.CtxKeyApiKeyUsedQuota).Float64(),
	}

	usage, billingResult, err := handler.HandleRealtime(r.Response.Writer, r.Request, rc, dataProvider, billingProvider)

	// 监控埋点：WebSocket 字节流量由会话内部循环统计，此处无法从外部捕获；
	// 改为按 token 用量记录可观测指标，便于排障与计费对账。
	totalTokens := 0
	if usage != nil {
		totalTokens = usage.TotalTokens
	}

	if err != nil {
		g.Log().Errorf(r.Context(), "[HandleRealtime] error: %v (duration=%s, total_tokens=%d)",
			err, time.Since(start), totalTokens)
	} else {
		g.Log().Debugf(r.Context(), "[HandleRealtime] connection closed: duration=%s, total_tokens=%d, billed=%v",
			time.Since(start), totalTokens, billingResult != nil)
	}
}

// debugStack 返回当前 goroutine 的堆栈字符串，供 panic 日志使用。
func debugStack() string {
	return string(debug.Stack())
}

// HandleModerations 处理 /v1/moderations
func HandleModerations(r *ghttp.Request) {
	body := r.GetBody()
	if len(body) == 0 {
		r.Response.WriteJsonExit(g.Map{
			"error": g.Map{"type": "invalid_request_error", "message": "request body is empty"},
		})
		return
	}

	monitor.RecordBytesIn(len(body))
	rc := buildRelayContext(r)
	capture := NewResponseCaptureWriter(r.Response.Writer)
	rc.Writer = capture

	start := time.Now()
	_, billingResult, err := handler.HandleModerations(r.Context(), body, "/moderations", r.Header, rc, dataProvider, billingProvider)
	latencyMs := int(time.Since(start).Milliseconds())
	monitor.RecordBytesOut(int(capture.BytesWritten()))
	setRateLimitHeaders(r, billingResult)
	setDeprecationHeaders(r, billingResult)

	if err != nil {
		handler.WriteRelayError(capture, err)
		recordAudit(r, rc, capture, body, "/moderations", latencyMs, firstTokenMs(billingResult))
		return
	}

	recordAudit(r, rc, capture, body, "/moderations", latencyMs, firstTokenMs(billingResult))
}

// HandleImagesEdits 处理 /v1/images/edits（支持 JSON 和 multipart 两种请求格式）
func HandleImagesEdits(r *ghttp.Request) {
	body := r.GetBody()
	if len(body) == 0 {
		r.Response.WriteJsonExit(g.Map{
			"error": g.Map{"type": "invalid_request_error", "message": "request body is empty"},
		})
		return
	}

	monitor.RecordBytesIn(len(body))
	rc := buildRelayContext(r)
	capture := NewResponseCaptureWriter(r.Response.Writer)
	rc.Writer = capture

	start := time.Now()
	_, billingResult, err := handler.HandleImagesEdits(r.Context(), body, "/images/edits", r.Header, rc, dataProvider, billingProvider)
	latencyMs := int(time.Since(start).Milliseconds())
	monitor.RecordBytesOut(int(capture.BytesWritten()))
	setRateLimitHeaders(r, billingResult)
	setDeprecationHeaders(r, billingResult)

	if err != nil {
		handler.WriteRelayError(capture, err)
		recordAudit(r, rc, capture, body, "/images/edits", latencyMs, firstTokenMs(billingResult))
		return
	}

	recordAudit(r, rc, capture, body, "/images/edits", latencyMs, firstTokenMs(billingResult))
}

package tenant

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"time"

	"github.com/gogf/gf/v2/frame/g"

	v1 "github.com/qianfree/team-api/api/tenant/v1"
	"github.com/qianfree/team-api/internal/dao"
	"github.com/qianfree/team-api/internal/logic/billing"
	"github.com/qianfree/team-api/internal/logic/common"
	"github.com/qianfree/team-api/internal/logic/relay"
	relaycommon "github.com/qianfree/team-api/relay/common"
	"github.com/qianfree/team-api/relay/constant"
	"github.com/qianfree/team-api/relay/handler"
)

// ============================================================
// 共享 helper
// ============================================================

type playgroundApiKey struct {
	Id    int64  `json:"id"`
	Scope string `json:"scope"`
}

func findActiveApiKey(ctx context.Context) (*playgroundApiKey, error) {
	tenantID := ctxTenantID(ctx)
	userID := ctxUserID(ctx)

	var key *playgroundApiKey
	err := dao.ApiKeys.Ctx(ctx).
		Where("tenant_id", tenantID).
		Where("user_id", userID).
		Where("status", "active").
		OrderDesc("id").
		Scan(&key)
	if err != nil {
		return nil, err
	}
	if key == nil {
		return nil, common.NewBusinessError(10055, "没有可用的 API Key，请先创建")
	}
	return key, nil
}

func buildPlaygroundRelayContext(ctx context.Context, key *playgroundApiKey, recorder http.ResponseWriter) *handler.RelayContext {
	requestID := ""
	if v := ctx.Value("requestId"); v != nil {
		requestID = v.(string)
	}
	return &handler.RelayContext{
		TenantID:  ctxTenantID(ctx),
		UserID:    ctxUserID(ctx),
		ApiKeyID:  key.Id,
		RequestID: requestID,
		Writer:    recorder,
		Scope:     key.Scope,
		ClientIP:  "",
	}
}

func callRelayAndParseUsage(relayFunc func() (*relaycommon.Usage, *handler.BillingResult, error)) (promptTokens, completionTokens, totalTokens int, err error) {
	usage, _, relayErr := relayFunc()
	if relayErr != nil {
		return 0, 0, 0, convertRelayError(relayErr)
	}
	if usage != nil {
		promptTokens = usage.PromptTokens
		completionTokens = usage.CompletionTokens
		totalTokens = usage.TotalTokens
	}
	return
}

// convertRelayError 将 relay 层的 RelayError 转换为 GoFrame gerror，使响应中间件能正确处理
func convertRelayError(err error) error {
	var relayErr *constant.RelayError
	if !errors.As(err, &relayErr) {
		return err
	}
	switch relayErr.StatusCode {
	case 402:
		// 区分余额不足和成员额度超限
		if relayErr.Message == "member quota exceeded" {
			return common.NewBusinessError(10001, "成员额度已用完，请联系管理员调整")
		}
		return common.NewBusinessError(10001, "余额不足，请充值后重试")
	case 429:
		return common.NewBusinessError(10014, "请求过于频繁，请稍后重试")
	case 401:
		return common.NewBadRequestError("API Key 认证失败")
	case 503:
		return common.NewBusinessError(10003, "当前无可用渠道，请稍后重试")
	default:
		return common.NewBadRequestError(relayErr.Message)
	}
}

func getRequestID(ctx context.Context) string {
	if v := ctx.Value("requestId"); v != nil {
		return v.(string)
	}
	return ""
}

// ============================================================
// Chat
// ============================================================

func (s *sTenant) PlaygroundChat(ctx context.Context, req *v1.PlaygroundChatReq) (*v1.PlaygroundChatRes, error) {
	key, err := findActiveApiKey(ctx)
	if err != nil {
		return nil, err
	}

	reqBody := map[string]any{
		"model":    req.Model,
		"messages": req.Messages,
		"stream":   false,
	}
	if req.Temperature != nil {
		reqBody["temperature"] = *req.Temperature
	}
	if req.MaxTokens != nil {
		reqBody["max_tokens"] = *req.MaxTokens
	}
	if req.TopP != nil {
		reqBody["top_p"] = *req.TopP
	}
	if req.FrequencyPenalty != nil {
		reqBody["frequency_penalty"] = *req.FrequencyPenalty
	}
	if req.PresencePenalty != nil {
		reqBody["presence_penalty"] = *req.PresencePenalty
	}
	if req.ImageConfig != nil {
		reqBody["image_config"] = map[string]any{
			"aspect_ratio": req.ImageConfig.AspectRatio,
			"image_size":   req.ImageConfig.ImageSize,
		}
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	recorder := httptest.NewRecorder()
	rc := buildPlaygroundRelayContext(ctx, key, recorder)
	provider := relay.NewDataProvider()
	billingProvider := billing.NewBillingProvider()

	promptTokens, completionTokens, totalTokens, relayErr := callRelayAndParseUsage(func() (*relaycommon.Usage, *handler.BillingResult, error) {
		return handler.HandleChatCompletions(ctx, body, "/v1/chat/completions", http.Header{}, rc, provider, billingProvider)
	})
	if relayErr != nil {
		return nil, relayErr
	}

	res := &v1.PlaygroundChatRes{Model: req.Model}
	res.PromptTokens = promptTokens
	res.CompletionTokens = completionTokens
	res.TotalTokens = totalTokens

	if recorder.Body.Len() > 0 {
		var relayResp struct {
			Choices []struct {
				Message struct {
					Content string `json:"content"`
				} `json:"message"`
			} `json:"choices"`
		}
		if err := json.Unmarshal(recorder.Body.Bytes(), &relayResp); err == nil && len(relayResp.Choices) > 0 {
			res.Content = relayResp.Choices[0].Message.Content
		}
	}

	return res, nil
}

// ============================================================
// Image
// ============================================================

func (s *sTenant) PlaygroundImage(ctx context.Context, req *v1.PlaygroundImageReq) (*v1.PlaygroundImageRes, error) {
	key, err := findActiveApiKey(ctx)
	if err != nil {
		return nil, err
	}

	reqBody := map[string]any{
		"model":  req.Model,
		"prompt": req.Prompt,
	}
	if req.N != nil {
		reqBody["n"] = *req.N
	}
	if req.Size != "" {
		reqBody["size"] = req.Size
	}
	if req.Quality != "" {
		reqBody["quality"] = req.Quality
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	recorder := httptest.NewRecorder()
	rc := buildPlaygroundRelayContext(ctx, key, recorder)

	promptTokens, completionTokens, totalTokens, relayErr := callRelayAndParseUsage(func() (*relaycommon.Usage, *handler.BillingResult, error) {
		return handler.HandleImagesGenerations(ctx, body, "/v1/images/generations", http.Header{}, rc, relay.NewDataProvider(), billing.NewBillingProvider())
	})
	if relayErr != nil {
		return nil, relayErr
	}

	res := &v1.PlaygroundImageRes{
		PromptTokens:     promptTokens,
		CompletionTokens: completionTokens,
		TotalTokens:      totalTokens,
	}

	if recorder.Body.Len() > 0 {
		var relayResp struct {
			Data []struct {
				B64JSON       string `json:"b64_json"`
				URL           string `json:"url"`
				RevisedPrompt string `json:"revised_prompt"`
			} `json:"data"`
		}
		if err := json.Unmarshal(recorder.Body.Bytes(), &relayResp); err == nil {
			for _, d := range relayResp.Data {
				res.Images = append(res.Images, v1.PlaygroundImageData{
					B64JSON:       d.B64JSON,
					URL:           d.URL,
					RevisedPrompt: d.RevisedPrompt,
				})
			}
		}
	}

	return res, nil
}

// ============================================================
// Audio TTS
// ============================================================

func (s *sTenant) PlaygroundAudioTTS(ctx context.Context, req *v1.PlaygroundAudioTTSReq) (*v1.PlaygroundAudioTTSRes, error) {
	key, err := findActiveApiKey(ctx)
	if err != nil {
		return nil, err
	}

	reqBody := map[string]any{
		"model": req.Model,
		"input": req.Input,
		"voice": req.Voice,
	}
	if req.ResponseFormat != "" {
		reqBody["response_format"] = req.ResponseFormat
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	recorder := httptest.NewRecorder()
	rc := buildPlaygroundRelayContext(ctx, key, recorder)
	provider := relay.NewDataProvider()
	billingProvider := billing.NewBillingProvider()

	promptTokens, completionTokens, totalTokens, relayErr := callRelayAndParseUsage(func() (*relaycommon.Usage, *handler.BillingResult, error) {
		return handler.HandleAudioSpeech(ctx, body, "/v1/audio/speech", http.Header{}, rc, provider, billingProvider)
	})
	if relayErr != nil {
		return nil, relayErr
	}

	res := &v1.PlaygroundAudioTTSRes{
		PromptTokens:     promptTokens,
		CompletionTokens: completionTokens,
		TotalTokens:      totalTokens,
	}

	if recorder.Body.Len() > 0 {
		res.AudioBase64 = base64.StdEncoding.EncodeToString(recorder.Body.Bytes())
		res.ContentType = recorder.Header().Get("Content-Type")
		if res.ContentType == "" {
			res.ContentType = "audio/mpeg"
		}
	}

	return res, nil
}

// ============================================================
// Embedding
// ============================================================

func (s *sTenant) PlaygroundEmbedding(ctx context.Context, req *v1.PlaygroundEmbeddingReq) (*v1.PlaygroundEmbeddingRes, error) {
	key, err := findActiveApiKey(ctx)
	if err != nil {
		return nil, err
	}

	reqBody := map[string]any{
		"model": req.Model,
		"input": req.Input,
	}
	if req.Dimensions != nil {
		reqBody["dimensions"] = *req.Dimensions
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	recorder := httptest.NewRecorder()
	rc := buildPlaygroundRelayContext(ctx, key, recorder)
	provider := relay.NewDataProvider()
	billingProvider := billing.NewBillingProvider()

	promptTokens, _, totalTokens, relayErr := callRelayAndParseUsage(func() (*relaycommon.Usage, *handler.BillingResult, error) {
		return handler.HandleEmbeddings(ctx, body, "/v1/embeddings", http.Header{}, rc, provider, billingProvider)
	})
	if relayErr != nil {
		return nil, relayErr
	}

	res := &v1.PlaygroundEmbeddingRes{
		PromptTokens: promptTokens,
		TotalTokens:  totalTokens,
	}

	if recorder.Body.Len() > 0 {
		var relayResp struct {
			Data []struct {
				Index     int       `json:"index"`
				Embedding []float64 `json:"embedding"`
			} `json:"data"`
		}
		if err := json.Unmarshal(recorder.Body.Bytes(), &relayResp); err == nil {
			for _, d := range relayResp.Data {
				res.Embeddings = append(res.Embeddings, v1.PlaygroundEmbeddingData{
					Index:     d.Index,
					Embedding: d.Embedding,
				})
			}
		}
	}

	return res, nil
}

// ============================================================
// Rerank
// ============================================================

func (s *sTenant) PlaygroundRerank(ctx context.Context, req *v1.PlaygroundRerankReq) (*v1.PlaygroundRerankRes, error) {
	key, err := findActiveApiKey(ctx)
	if err != nil {
		return nil, err
	}

	reqBody := map[string]any{
		"model":     req.Model,
		"query":     req.Query,
		"documents": req.Documents,
	}
	if req.TopN != nil {
		reqBody["top_n"] = *req.TopN
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	recorder := httptest.NewRecorder()
	rc := buildPlaygroundRelayContext(ctx, key, recorder)
	provider := relay.NewDataProvider()
	billingProvider := billing.NewBillingProvider()

	promptTokens, _, totalTokens, relayErr := callRelayAndParseUsage(func() (*relaycommon.Usage, *handler.BillingResult, error) {
		return handler.HandleRerank(ctx, body, "/v1/rerank", http.Header{}, rc, provider, billingProvider)
	})
	if relayErr != nil {
		return nil, relayErr
	}

	res := &v1.PlaygroundRerankRes{
		PromptTokens: promptTokens,
		TotalTokens:  totalTokens,
	}

	if recorder.Body.Len() > 0 {
		var relayResp struct {
			Results []struct {
				Index          int     `json:"index"`
				RelevanceScore float64 `json:"relevance_score"`
				Document       *struct {
					Text string `json:"text"`
				} `json:"document"`
			} `json:"results"`
		}
		if err := json.Unmarshal(recorder.Body.Bytes(), &relayResp); err == nil {
			for _, r := range relayResp.Results {
				item := v1.PlaygroundRerankResult{
					Index:          r.Index,
					RelevanceScore: r.RelevanceScore,
				}
				if r.Document != nil {
					item.Document = &v1.PlaygroundRerankDoc{Text: r.Document.Text}
				}
				res.Results = append(res.Results, item)
			}
		}
	}

	return res, nil
}

// ============================================================
// Sandbox（模拟调用，不计费）
// ============================================================

func (s *sTenant) SandboxChat(ctx context.Context, req *v1.SandboxChatReq) (*v1.SandboxChatRes, error) {
	tenantID := ctxTenantID(ctx)

	now := time.Now()
	quotaKey := fmt.Sprintf("sandbox:quota:%d:%s", tenantID, now.Format("200601"))
	remaining, err := g.Redis().Do(ctx, "GET", quotaKey)
	if err != nil {
		return nil, err
	}

	defaultQuota := g.Cfg().MustGet(ctx, "sandbox.sandbox_default_quota").Int()
	if defaultQuota <= 0 {
		defaultQuota = 100
	}

	remainInt := defaultQuota
	if !remaining.IsNil() && !remaining.IsEmpty() {
		remainInt = remaining.Int()
	}

	if remainInt <= 0 {
		return nil, common.NewBusinessError(10056, "本月沙箱额度已用完")
	}

	_, err = g.Redis().Do(ctx, "DECR", quotaKey)
	if err != nil {
		return nil, err
	}
	if remainInt == defaultQuota {
		_, _ = g.Redis().Do(ctx, "EXPIRE", quotaKey, 86400*30)
	}

	content := generateSimulatedResponse(req.Model, req.Messages)

	return &v1.SandboxChatRes{
		Content:        content,
		IsSandbox:      true,
		RemainingQuota: remainInt - 1,
	}, nil
}

func (s *sTenant) SandboxQuota(ctx context.Context, req *v1.SandboxQuotaReq) (*v1.SandboxQuotaRes, error) {
	tenantID := ctxTenantID(ctx)

	defaultQuota := g.Cfg().MustGet(ctx, "sandbox.sandbox_default_quota").Int()
	if defaultQuota <= 0 {
		defaultQuota = 100
	}

	now := time.Now()
	quotaKey := fmt.Sprintf("sandbox:quota:%d:%s", tenantID, now.Format("200601"))
	remaining, err := g.Redis().Do(ctx, "GET", quotaKey)
	if err != nil {
		return nil, err
	}

	remainInt := defaultQuota
	if !remaining.IsNil() && !remaining.IsEmpty() {
		remainInt = remaining.Int()
	}

	used := defaultQuota - remainInt
	if used < 0 {
		used = 0
	}

	return &v1.SandboxQuotaRes{
		TotalQuota:     defaultQuota,
		RemainingQuota: remainInt,
		UsedQuota:      used,
	}, nil
}

func generateSimulatedResponse(model string, messages []v1.PlaygroundMessage) string {
	userMsg := ""
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == "user" {
			userMsg = messages[i].Content
			break
		}
	}
	if userMsg == "" {
		userMsg = "你的请求"
	}
	return fmt.Sprintf("这是来自模型 %s 的模拟响应。您发送的消息是：%q\n\n（沙箱模式：此响应为模拟数据，不会产生实际费用）", model, userMsg)
}

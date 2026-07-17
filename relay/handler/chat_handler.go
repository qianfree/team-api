package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/gogf/gf/v2/frame/g"

	"github.com/qianfree/team-api/relay/common"
	"github.com/qianfree/team-api/relay/constant"
	"github.com/qianfree/team-api/relay/dto"
)

// HandleChatCompletions 处理 /v1/chat/completions 请求
func HandleChatCompletions(ctx context.Context, body []byte, path string, headers http.Header, rc *RelayContext, provider common.DataProvider, billing common.BillingProvider) (*common.Usage, *BillingResult, error) {
	return RelayHandler(ctx, body, path, headers, rc, provider, billing)
}

// HandleEmbeddings 处理 /v1/embeddings 请求
func HandleEmbeddings(ctx context.Context, body []byte, path string, headers http.Header, rc *RelayContext, provider common.DataProvider, billing common.BillingProvider) (*common.Usage, *BillingResult, error) {
	return RelayHandler(ctx, body, path, headers, rc, provider, billing)
}

// HandleImagesGenerations 处理 /v1/images/generations 请求
func HandleImagesGenerations(ctx context.Context, body []byte, path string, headers http.Header, rc *RelayContext, provider common.DataProvider, billing common.BillingProvider) (*common.Usage, *BillingResult, error) {
	return RelayHandler(ctx, body, path, headers, rc, provider, billing)
}

// HandleCompletions 处理 /v1/completions 请求
func HandleCompletions(ctx context.Context, body []byte, path string, headers http.Header, rc *RelayContext, provider common.DataProvider, billing common.BillingProvider) (*common.Usage, *BillingResult, error) {
	return RelayHandler(ctx, body, path, headers, rc, provider, billing)
}

// HandleResponses 处理 /v1/responses 请求（OpenAI Responses API）
func HandleResponses(ctx context.Context, body []byte, path string, headers http.Header, rc *RelayContext, provider common.DataProvider, billing common.BillingProvider) (*common.Usage, *BillingResult, error) {
	return RelayHandler(ctx, body, path, headers, rc, provider, billing)
}

// HandleModels 处理 /v1/models 请求（列出可用模型）
// tenantID 用于过滤该租户有权使用的模型，apiKeyID 用于进一步按 Key 的模型范围过滤
// userID 用于进一步按成员的模型范围过滤
func HandleModels(ctx context.Context, tenantID, apiKeyID, userID int64, provider common.DataProvider) (*dto.ModelsResponse, error) {
	models, err := provider.GetAvailableModels(ctx, tenantID, apiKeyID, userID)
	if err != nil {
		return nil, err
	}

	data := make([]dto.ModelDTO, 0, len(models))
	for _, m := range models {
		item := dto.ModelDTO{
			ID:              m.ModelId,
			Object:          "model",
			Created:         0,
			OwnedBy:         "platform",
			ModelName:       m.ModelName,
			Category:        m.Category,
			ContextWindow:   m.MaxContextTokens,
			MaxOutputTokens: m.MaxOutputTokens,
			Capabilities:    m.Capabilities,
			Modalities:      buildModalities(m.Category, m.Capabilities),
		}
		data = append(data, item)
	}

	return &dto.ModelsResponse{
		Object: "list",
		Data:   data,
	}, nil
}

// HandleModelDetail 处理 /v1/models/{model_id} 请求（获取单个模型详情）
func HandleModelDetail(ctx context.Context, tenantID int64, modelName string, provider common.DataProvider) (*dto.ModelDetailResponse, error) {
	detail, err := provider.GetModelDetail(ctx, tenantID, modelName)
	if err != nil {
		return nil, err
	}

	return &dto.ModelDetailResponse{
		ID:              detail.ID,
		Object:          detail.Object,
		Created:         detail.Created,
		OwnedBy:         detail.OwnedBy,
		ModelName:       detail.ModelName,
		Description:     detail.Description,
		Category:        detail.Category,
		Status:          detail.Status,
		ContextWindow:   detail.MaxContextTokens,
		MaxOutputTokens: detail.MaxOutputTokens,
		Capabilities:    detail.Capabilities,
		Modalities:      buildModalities(detail.Category, detail.Capabilities),
		Deprecated:      detail.Status == "deprecated",
	}, nil
}

func buildModalities(category string, capabilities map[string]bool) *dto.ModelModalities {
	input := []string{"text"}
	output := []string{"text"}

	switch category {
	case "embedding":
		output = []string{"embedding"}
	case "image":
		input = []string{"text"}
		output = []string{"image"}
	case "audio":
		input = []string{"audio"}
		output = []string{"text"}
	}

	if capabilities != nil {
		if capabilities["vision"] {
			input = appendUnique(input, "image")
		}
		if capabilities["audio_input"] {
			input = appendUnique(input, "audio")
		}
		if capabilities["audio_output"] {
			output = appendUnique(output, "audio")
		}
		if capabilities["pdf_input"] {
			input = appendUnique(input, "pdf")
		}
	}

	return &dto.ModelModalities{Input: input, Output: output}
}

func appendUnique(slice []string, item string) []string {
	for _, s := range slice {
		if s == item {
			return slice
		}
	}
	return append(slice, item)
}

// WriteRelayError 写入 relay 错误响应（导出，供 GoFrame handler 调用）
func WriteRelayError(w http.ResponseWriter, err error) {
	// 流式中断（客户端已断开）：降级为 INFO，跳过写响应
	if errors.Is(err, common.ErrStreamInterrupted) {
		g.Log().Infof(context.Background(), "[RelayError] Client disconnected during stream")
		return
	}

	// adaptor 已直接写入响应体（如 Gemini 原生格式透传），跳过二次写入
	var prewritten *constant.RelayError
	if errors.As(err, &prewritten) && prewritten.ResponseWritten {
		return
	}

	var relayErr *constant.RelayError
	var rateLimitErr *RelayErrorWithRateLimit
	statusCode := http.StatusInternalServerError
	errMsg := err.Error()
	errType := "internal_error"

	if errors.As(err, &rateLimitErr) {
		statusCode = rateLimitErr.StatusCode
		errMsg = rateLimitErr.Message
		errType = "rate_limit_error"
		w.Header().Set("X-RateLimit-Limit", "0")
		w.Header().Set("X-RateLimit-Remaining", fmt.Sprintf("%d", rateLimitErr.Remaining))
		w.Header().Set("X-RateLimit-Reset", fmt.Sprintf("%d", rateLimitErr.ResetAt))
	} else if errors.As(err, &relayErr) {
		statusCode = relayErr.StatusCode
		errMsg = relayErr.Message
		if relayErr.Cause != nil {
			errMsg = relayErr.Message + ": " + relayErr.Cause.Error()
		}
		errType = relayErr.Type
	}

	if statusCode < 100 || statusCode > 599 {
		statusCode = http.StatusInternalServerError
	}

	// 无可用渠道是正常业务条件，已在 handleChannelUnavailable 中以 Warning 记录，此处跳过避免重复日志；
	// 其余 5xx 为真实错误，保留 ERROR 但禁用堆栈打印（此处调用栈固定，无调试价值）
	if statusCode >= 500 && !errors.Is(err, common.ErrChannelUnavailable) {
		g.Log().Stack(false).Errorf(context.Background(), "[RelayError] statusCode=%d type=%s message=%s originalError=%v",
			statusCode, errType, errMsg, err)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	errBody, _ := json.Marshal(map[string]any{
		"error": map[string]any{
			"message": errMsg,
			"type":    errType,
			"param":   nil,
			"code":    nil,
		},
	})
	_, _ = w.Write(errBody)
}

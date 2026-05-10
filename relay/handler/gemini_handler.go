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

// HandleGeminiGenerateContent 处理 /v1beta/models/{model}:generateContent 请求（Gemini 原生格式）
func HandleGeminiGenerateContent(ctx context.Context, body []byte, path string, headers http.Header, rc *RelayContext, provider common.DataProvider, billing common.BillingProvider) (*common.Usage, *BillingResult, error) {
	return RelayHandler(ctx, body, path, headers, rc, provider, billing)
}

// HandleGeminiModels 处理 GET /v1beta/models 请求（Gemini 原生格式模型列表）
func HandleGeminiModels(ctx context.Context, tenantID int64, apiKeyID int64, provider common.DataProvider) (*dto.GeminiModelsResponse, error) {
	models, err := provider.GetAvailableModels(ctx, tenantID, apiKeyID)
	if err != nil {
		return nil, err
	}

	items := make([]dto.GeminiModel, 0, len(models))
	for _, m := range models {
		items = append(items, dto.GeminiModel{
			Name:                       fmt.Sprintf("models/%s", m.ModelId),
			BaseModelId:                m.ModelId,
			DisplayName:                m.ModelName,
			SupportedGenerationMethods: []string{"generateContent", "countTokens"},
		})
	}

	return &dto.GeminiModelsResponse{
		Models: items,
	}, nil
}

// HandleGeminiModelDetail 处理 GET /v1beta/models/{model} 请求（Gemini 原生格式模型详情）
func HandleGeminiModelDetail(ctx context.Context, tenantID int64, modelName string, provider common.DataProvider) (*dto.GeminiModel, error) {
	detail, err := provider.GetModelDetail(ctx, tenantID, modelName)
	if err != nil {
		return nil, err
	}

	return &dto.GeminiModel{
		Name:                       fmt.Sprintf("models/%s", detail.ID),
		BaseModelId:                detail.ID,
		DisplayName:                detail.ModelName,
		Description:                detail.Description,
		InputTokenLimit:            detail.MaxContextTokens,
		OutputTokenLimit:           detail.MaxOutputTokens,
		SupportedGenerationMethods: []string{"generateContent", "countTokens"},
	}, nil
}

// WriteGeminiRelayError 写入 Gemini 格式的错误响应
func WriteGeminiRelayError(w http.ResponseWriter, err error) {
	var relayErr *constant.RelayError
	var rateLimitErr *RelayErrorWithRateLimit
	statusCode := http.StatusInternalServerError
	errMsg := err.Error()
	errStatus := "INTERNAL"

	if errors.As(err, &rateLimitErr) {
		statusCode = rateLimitErr.StatusCode
		errMsg = rateLimitErr.Message
		errStatus = "RESOURCE_EXHAUSTED"
	} else if errors.As(err, &relayErr) {
		statusCode = relayErr.StatusCode
		errMsg = relayErr.Message
		if relayErr.Cause != nil {
			errMsg = relayErr.Message + ": " + relayErr.Cause.Error()
		}
		switch statusCode {
		case 401:
			errStatus = "UNAUTHENTICATED"
		case 403:
			errStatus = "PERMISSION_DENIED"
		case 400:
			errStatus = "INVALID_ARGUMENT"
		case 404:
			errStatus = "NOT_FOUND"
		default:
			errStatus = "INTERNAL"
		}
	}

	if statusCode < 100 || statusCode > 599 {
		statusCode = http.StatusInternalServerError
	}

	g.Log().Errorf(context.Background(), "[GeminiRelayError] statusCode=%d status=%s message=%s originalError=%v",
		statusCode, errStatus, errMsg, err)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	errBody, _ := json.Marshal(map[string]any{
		"error": map[string]any{
			"code":    statusCode,
			"message": errMsg,
			"status":  errStatus,
		},
	})
	_, _ = w.Write(errBody)
}

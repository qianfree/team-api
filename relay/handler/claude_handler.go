package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/gogf/gf/v2/frame/g"

	"github.com/qianfree/team-api/relay/common"
	"github.com/qianfree/team-api/relay/constant"
)

// HandleClaudeMessages 处理 /v1/messages 请求（Claude 原生格式）
func HandleClaudeMessages(ctx context.Context, body []byte, path string, headers http.Header, rc *RelayContext, provider common.DataProvider, billing common.BillingProvider) (*common.Usage, *BillingResult, error) {
	return RelayHandler(ctx, body, path, headers, rc, provider, billing)
}

// WriteClaudeRelayError 写入 Claude 格式的错误响应
func WriteClaudeRelayError(w http.ResponseWriter, err error) {
	// 流式中断（客户端已断开）：降级为 INFO，跳过写响应（客户端已不在）
	if errors.Is(err, common.ErrStreamInterrupted) {
		g.Log().Infof(context.Background(), "[ClaudeRelayError] Client disconnected during stream")
		return
	}

	var relayErr *constant.RelayError
	var rateLimitErr *RelayErrorWithRateLimit
	statusCode := http.StatusInternalServerError
	errMsg := err.Error()
	errType := "api_error"

	if errors.As(err, &rateLimitErr) {
		statusCode = rateLimitErr.StatusCode
		errMsg = rateLimitErr.Message
		errType = "rate_limit_error"
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

	g.Log().Errorf(context.Background(), "[ClaudeRelayError] statusCode=%d type=%s message=%s originalError=%v",
		statusCode, errType, errMsg, err)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	errBody, _ := json.Marshal(map[string]any{
		"type": "error",
		"error": map[string]any{
			"type":    errType,
			"message": errMsg,
		},
	})
	_, _ = w.Write(errBody)
}

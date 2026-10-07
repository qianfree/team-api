package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/gogf/gf/v2/frame/g"

	"github.com/qianfree/team-api/relay/common"
	"github.com/qianfree/team-api/relay/constant"
	"github.com/qianfree/team-api/relay/helper"
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
	if responseCommitted(w) {
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
	errMsg := helper.SafeUpstreamErrorMessage(err)
	errType := "api_error"
	// 上游错误响应体原文（Anthropic 信封时可从中还原原始 error.type）
	upstreamBody := ""

	if errors.As(err, &rateLimitErr) {
		statusCode = rateLimitErr.StatusCode
		errMsg = rateLimitErr.Message
		errType = "rate_limit_error"
	} else if errors.As(err, &relayErr) {
		statusCode = relayErr.StatusCode
		// 上游错误的 Message 是上游响应体原文，解包出可读消息再暴露给客户端
		upstreamBody = relayErr.Message
		errMsg = helper.UnwrapUpstreamErrorMessage(relayErr.Message)
		if relayErr.Cause != nil {
			// 传输层错误（client.Do 的 *url.Error）的 Cause 含上游域名，必须脱敏后再暴露给用户；
			// 日志侧仍用 originalError=%v 打印完整错误供运维定位。
			errMsg = errMsg + ": " + helper.SafeUpstreamErrorMessage(relayErr.Cause)
		}
		errType = relayErr.Type
	}

	if statusCode < 100 || statusCode > 599 {
		statusCode = http.StatusInternalServerError
	}

	// error.type 必须取自 Claude 官方词表：内部口径（upstream_error 等）对
	// Anthropic SDK 不可识别；上游为 Anthropic 信封时优先还原其原始 type
	errType = helper.ClaudeErrorType(statusCode, errType, upstreamBody)

	// 无可用渠道是正常业务条件，已在 handleChannelUnavailable 中以 Warning 记录，此处跳过避免重复日志；
	// 其余 5xx 为真实错误，保留 ERROR 但禁用堆栈打印（此处调用栈固定，无调试价值）
	if statusCode >= 500 && !errors.Is(err, common.ErrChannelUnavailable) {
		g.Log().Stack(false).Errorf(context.Background(), "[ClaudeRelayError] statusCode=%d type=%s message=%s originalError=%v",
			statusCode, errType, errMsg, err)
	}

	w.Header().Set("Content-Type", "application/json")
	// x-should-retry：告知客户端该错误是否值得重试。429/5xx 置 true（退避或换
	// 渠道后可能成功），其余 4xx 置 false（如 402 余额不足，重试必然同结果）
	if helper.ShouldRetryStatus(statusCode) {
		w.Header().Set("x-should-retry", "true")
	} else {
		w.Header().Set("x-should-retry", "false")
	}
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

package helper

import (
	"net/http"
	"testing"
)

func TestClaudeErrorType(t *testing.T) {
	tests := []struct {
		name         string
		statusCode   int
		internalType string
		upstreamBody string
		wantType     string
	}{
		// 上游为 Anthropic 信封：原样保留其 error.type
		{"Anthropic 信封保留 overloaded_error", 503, "upstream_error",
			`{"type":"error","error":{"type":"overloaded_error","message":"Overloaded"}}`, "overloaded_error"},
		{"Anthropic 信封保留 invalid_request_error", 400, "upstream_error",
			`{"type":"error","error":{"type":"invalid_request_error","message":"Extra inputs are not permitted"}}`, "invalid_request_error"},
		{"信封缺 error.type 落状态码映射", 400, "upstream_error",
			`{"type":"error","error":{"message":"bad request"}}`, "invalid_request_error"},

		// 跨协议上游信封：不取其 type（词表不通用），走状态码映射
		{"OpenAI 信封不取 insufficient_quota", 429, "upstream_error",
			`{"error":{"message":"insufficient quota","type":"insufficient_quota","code":"insufficient_quota"}}`, "rate_limit_error"},
		{"Gemini 信封不取其 type", 429, "upstream_error",
			`{"error":{"code":429,"message":"Resource exhausted","status":"RESOURCE_EXHAUSTED"}}`, "rate_limit_error"},

		// 网关自产错误：内部类型映射
		{"内部 auth_error 映射", 401, "auth_error", "", "authentication_error"},
		{"内部 request_error 映射", 400, "request_error", "", "invalid_request_error"},
		{"内部 insufficient_quota 映射", 402, "insufficient_quota", "", "invalid_request_error"},
		{"内部 model_gone 映射", 410, "model_gone", "", "not_found_error"},
		{"内部 channel_error 映射", 503, "channel_error", "", "api_error"},

		// 状态码兜底（官方映射表 + 未列出状态码口径）
		{"状态码 403", 403, "", "", "permission_error"},
		{"状态码 404", 404, "", "", "not_found_error"},
		{"状态码 413", 413, "", "", "request_too_large"},
		{"状态码 529", 529, "", "", "overloaded_error"},
		{"未列出 4xx 归 invalid_request_error", 418, "", "", "invalid_request_error"},
		{"未列出 5xx 归 api_error", 502, "", "", "api_error"},
		{"非 JSON 体走状态码映射", 500, "upstream_error", "upstream exploded", "api_error"},
		{"空内部类型与空体", 400, "", "", "invalid_request_error"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ClaudeErrorType(tt.statusCode, tt.internalType, tt.upstreamBody); got != tt.wantType {
				t.Errorf("ClaudeErrorType(%d, %q, ...) = %q, want %q",
					tt.statusCode, tt.internalType, got, tt.wantType)
			}
		})
	}
}

func TestShouldRetryStatus(t *testing.T) {
	retryable := []int{
		http.StatusTooManyRequests,
		http.StatusInternalServerError,
		http.StatusBadGateway,
		http.StatusServiceUnavailable,
		http.StatusGatewayTimeout,
		529,
	}
	for _, code := range retryable {
		if !ShouldRetryStatus(code) {
			t.Errorf("ShouldRetryStatus(%d) = false, want true", code)
		}
	}

	nonRetryable := []int{
		http.StatusBadRequest,
		http.StatusUnauthorized,
		http.StatusPaymentRequired,
		http.StatusForbidden,
		http.StatusNotFound,
		http.StatusRequestEntityTooLarge,
	}
	for _, code := range nonRetryable {
		if ShouldRetryStatus(code) {
			t.Errorf("ShouldRetryStatus(%d) = true, want false", code)
		}
	}
}

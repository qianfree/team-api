package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/qianfree/team-api/relay/constant"
)

type committedTestWriter struct {
	*httptest.ResponseRecorder
}

func (w *committedTestWriter) ResponseCommitted() bool {
	return true
}

// TestErrorWriters_SkipResponseWritten 验证当 adaptor 已置 ResponseWritten=true 时，
// 三个错误写入器都跳过二次写入（不写 header、不写 body）。
// 这是 Gemini/OpenAI「双重写入」修复的核心机制兜底。
func TestErrorWriters_SkipResponseWritten(t *testing.T) {
	writtenErr := constant.NewUpstreamError(http.StatusTooManyRequests, "upstream 429", nil)
	writtenErr.ResponseWritten = true

	cases := []struct {
		name string
		fn   func(http.ResponseWriter, error)
	}{
		{"WriteRelayError", WriteRelayError},
		{"WriteClaudeRelayError", WriteClaudeRelayError},
		{"WriteGeminiRelayError", WriteGeminiRelayError},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			c.fn(rec, writtenErr)

			if rec.Code != http.StatusOK {
				t.Errorf("Code = %d, want 200 (no WriteHeader should happen)", rec.Code)
			}
			if rec.Body.Len() != 0 {
				t.Errorf("body = %q, want empty (no write should happen)", rec.Body.String())
			}
		})
	}
}

// TestErrorWriters_WriteNormalError 验证普通（未标记 ResponseWritten）的 RelayError
// 仍被三个写入器正常写入一次，确保短路逻辑不影响正常错误路径。
func TestErrorWriters_WriteNormalError(t *testing.T) {
	normalErr := constant.NewUpstreamError(http.StatusTooManyRequests, "upstream 429", nil)

	cases := []struct {
		name string
		fn   func(http.ResponseWriter, error)
	}{
		{"WriteRelayError", WriteRelayError},
		{"WriteClaudeRelayError", WriteClaudeRelayError},
		{"WriteGeminiRelayError", WriteGeminiRelayError},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			c.fn(rec, normalErr)

			if rec.Code != http.StatusTooManyRequests {
				t.Errorf("Code = %d, want 429", rec.Code)
			}
			if rec.Body.Len() == 0 {
				t.Errorf("body empty, want non-empty error response")
			}
		})
	}
}

func TestErrorWriters_SkipCommittedResponse(t *testing.T) {
	cases := []struct {
		name string
		fn   func(http.ResponseWriter, error)
	}{
		{"WriteRelayError", WriteRelayError},
		{"WriteClaudeRelayError", WriteClaudeRelayError},
		{"WriteGeminiRelayError", WriteGeminiRelayError},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			_, _ = recorder.WriteString("data: partial\n\n")
			writer := &committedTestWriter{ResponseRecorder: recorder}
			before := recorder.Body.String()

			c.fn(writer, errors.New("upstream stream failed"))

			if got := recorder.Body.String(); got != before {
				t.Fatalf("committed body changed: got %q, want %q", got, before)
			}
		})
	}
}

// TestWriteClaudeRelayError_TypeVocabulary 验证 Claude 错误体的 error.type
// 取自 Claude 官方词表：内部口径不外泄，上游为 Anthropic 信封时还原原始 type。
func TestWriteClaudeRelayError_TypeVocabulary(t *testing.T) {
	cases := []struct {
		name     string
		err      error
		wantCode int
		wantType string
	}{
		{"上游 Anthropic 信封保留原 type",
			constant.NewUpstreamError(503, `{"type":"error","error":{"type":"overloaded_error","message":"Overloaded"}}`, nil),
			503, "overloaded_error"},
		{"内部 auth_error 映射为 authentication_error",
			constant.NewAuthError("invalid api key"),
			401, "authentication_error"},
		{"内部 insufficient_quota 映射为 invalid_request_error",
			constant.NewQuotaError("账户余额不足", nil),
			402, "invalid_request_error"},
		{"内部 model_gone 映射为 not_found_error",
			constant.NewModelGoneError("claude-2.0", "2026-01-01"),
			410, "not_found_error"},
		{"上游 429 映射为 rate_limit_error",
			constant.NewUpstreamError(429, `{"error":{"message":"rate limited","type":"insufficient_quota"}}`, nil),
			429, "rate_limit_error"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			WriteClaudeRelayError(rec, c.err)

			if rec.Code != c.wantCode {
				t.Fatalf("Code = %d, want %d", rec.Code, c.wantCode)
			}
			var body struct {
				Type  string `json:"type"`
				Error struct {
					Type string `json:"type"`
				} `json:"error"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatalf("错误体非法 JSON: %v, body=%q", err, rec.Body.String())
			}
			if body.Type != "error" {
				t.Errorf("顶层 type = %q, want \"error\"", body.Type)
			}
			if body.Error.Type != c.wantType {
				t.Errorf("error.type = %q, want %q", body.Error.Type, c.wantType)
			}
		})
	}
}

// TestWriteClaudeRelayError_ShouldRetryHeader 验证 x-should-retry 头：
// 429/5xx 置 true（退避或换渠道可能成功），其余 4xx 置 false（确定性失败）。
func TestWriteClaudeRelayError_ShouldRetryHeader(t *testing.T) {
	cases := []struct {
		name     string
		err      error
		wantCode int
		want     string
	}{
		{"429 网关限流可重试", &RelayErrorWithRateLimit{StatusCode: 429, Message: "rate limited"}, 429, "true"},
		{"503 渠道不可用可重试", constant.NewChannelError("no available channels", nil), 503, "true"},
		{"500 上游错误可重试", constant.NewUpstreamError(500, "boom", nil), 500, "true"},
		{"402 余额不足不重试", constant.NewQuotaError("账户余额不足", nil), 402, "false"},
		{"401 认证失败不重试", constant.NewAuthError("invalid api key"), 401, "false"},
		{"400 请求错误不重试", constant.NewRequestError("bad request", nil), 400, "false"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			WriteClaudeRelayError(rec, c.err)

			if rec.Code != c.wantCode {
				t.Fatalf("Code = %d, want %d", rec.Code, c.wantCode)
			}
			if got := rec.Header().Get("x-should-retry"); got != c.want {
				t.Errorf("x-should-retry = %q, want %q", got, c.want)
			}
		})
	}
}

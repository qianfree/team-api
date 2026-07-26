package handler

import (
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

package gemini

import (
	"strings"
	"testing"
)

// TestBuildGeminiUpstreamError_ResourceExhausted 验证真实 Gemini 429（区域配额耗尽）
// 被解析为携带正确 type 的 RelayError，且 message 为简短文案而非整个 body。
// 回归保护：OpenAI 出站路径上游报错时，由上层写入器写出单条 rate_limit_error，
// 不再出现 adaptor 与上层各写一次的「双重写入」。
func TestBuildGeminiUpstreamError_ResourceExhausted(t *testing.T) {
	body := []byte(`{
	  "error": {
	    "code": 429,
	    "message": "Quota exceeded for quota metric 'API requests' and limit 'Request limit per minute for a region' of service 'generativelanguage.googleapis.com' for consumer 'project_number:121235835710'.",
	    "status": "RESOURCE_EXHAUSTED"
	  }
	}`)

	err := buildGeminiUpstreamError(body, 200)

	if err.StatusCode != 429 {
		t.Errorf("StatusCode = %d, want 429", err.StatusCode)
	}
	if err.Type != "rate_limit_error" {
		t.Errorf("Type = %q, want rate_limit_error", err.Type)
	}
	if !strings.HasPrefix(err.Message, "Quota exceeded") {
		t.Errorf("Message = %q, want short upstream message (not the whole body)", err.Message)
	}
}

// TestBuildGeminiUpstreamError_FallbackStatusCode 验证 body 无法解析出 code 时回退到默认状态码
func TestBuildGeminiUpstreamError_FallbackStatusCode(t *testing.T) {
	// 非 JSON body：parseGeminiError 返回 code=0，应回退到 defaultStatusCode
	err := buildGeminiUpstreamError([]byte("plain text error"), 503)
	if err.StatusCode != 503 {
		t.Errorf("StatusCode = %d, want 503 (fallback)", err.StatusCode)
	}
}

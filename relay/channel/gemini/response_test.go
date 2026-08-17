package gemini

import (
	"strings"
	"testing"

	"github.com/qianfree/team-api/relay/dto"
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

// TestGeminiUsageToCommon_CacheSemantics 验证 Gemini usage 转换的缓存与思考语义：
//  1. Gemini 的 promptTokenCount 已含 cachedContentTokenCount（cached 为其子集），
//     必须置 CacheIncludedInPrompt=true 让计费扣减缓存部分，否则缓存 token 会被
//     「input 全价 + cache 价」双重计费；
//  2. Gemini 的 candidatesTokenCount 不含思考 token，thoughtsTokenCount 是输出侧
//     独立字段（按输出价计费），completion 必须为 candidates+thoughts 合计，否则思考漏计费。
func TestGeminiUsageToCommon_CacheSemantics(t *testing.T) {
	usage := geminiUsageToCommon(&dto.GeminiUsageMetadata{
		PromptTokenCount:        414,
		CandidatesTokenCount:    219,
		TotalTokenCount:         633,
		CachedContentTokenCount: 231,
		ThoughtsTokenCount:      100,
	})

	if !usage.CacheIncludedInPrompt {
		t.Error("CacheIncludedInPrompt = false, want true (cachedContentTokenCount 是 promptTokenCount 的子集)")
	}
	// 计费输出须含思考 token：completion = candidates(219) + thoughts(100)
	if usage.PromptTokens != 414 || usage.CompletionTokens != 319 || usage.TotalTokens != 633 {
		t.Errorf("token counts = %+v, want prompt=414 completion=319 total=633", usage)
	}
	if usage.PromptTokensDetails == nil || usage.PromptTokensDetails.CachedTokens != 231 {
		t.Errorf("cached tokens = %+v, want 231", usage.PromptTokensDetails)
	}
	if usage.CompletionTokenDetails == nil || usage.CompletionTokenDetails.ReasoningTokens != 100 {
		t.Errorf("reasoning tokens = %+v, want 100", usage.CompletionTokenDetails)
	}
}

package gemini

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/qianfree/team-api/relay/common"
	"github.com/qianfree/team-api/relay/dto"
)

// nativeStreamInfo 构造 Gemini 原生透传（/v1beta 直通）的 RelayInfo
func nativeStreamInfo() *common.RelayInfo {
	return &common.RelayInfo{
		IsStream:        true,
		OriginModelName: "gemini-3-pro",
		StartTime:       time.Now(),
		StreamStatus:    common.NewStreamStatus(),
		ChannelMeta: &common.ChannelMeta{
			BaseURL:           "https://upstream.example.com",
			UpstreamModelName: "gemini-3-pro",
		},
	}
}

func sseResponse(body io.Reader) *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": {"text/event-stream"}},
		Body:       io.NopCloser(body),
	}
}

// sseFrameRecorder 记录每次 Write 的内容，用于断言「一次 Write = 一个完整 SSE 帧」
type sseFrameRecorder struct {
	header http.Header
	chunks []string
	body   strings.Builder
}

func (r *sseFrameRecorder) Header() http.Header {
	if r.header == nil {
		r.header = http.Header{}
	}
	return r.header
}

func (r *sseFrameRecorder) WriteHeader(int) {}

func (r *sseFrameRecorder) Write(p []byte) (int, error) {
	r.chunks = append(r.chunks, string(p))
	r.body.Write(p)
	return len(p), nil
}

func (r *sseFrameRecorder) Flush() {}

// TestHandleGeminiNativeStream_FramesForwardedIntact 正常路径：内容逐字节原样转发，
// 且每次 Write 都落在帧边界（帧原子，将来接入保活 ping 也不会被空行劈开）。
func TestHandleGeminiNativeStream_FramesForwardedIntact(t *testing.T) {
	upstream := strings.Join([]string{
		`data: {"candidates":[{"content":{"parts":[{"text":"你好"}]}}]}`,
		``,
		`data: {"candidates":[{"content":{"parts":[{"text":"世界"}]}}],"usageMetadata":{"promptTokenCount":3,"candidatesTokenCount":7,"totalTokenCount":10}}`,
		``,
		``,
	}, "\n")

	info := nativeStreamInfo()
	a := &Adaptor{}
	a.Init(info)
	rec := &sseFrameRecorder{}

	usage, err := a.handleGeminiNativeStream(context.Background(), sseResponse(strings.NewReader(upstream)), info, rec)
	if err != nil {
		t.Fatalf("handleGeminiNativeStream error: %v", err)
	}

	if got := rec.body.String(); got != upstream {
		t.Errorf("转发内容与上游不一致:\ngot:  %q\nwant: %q", got, upstream)
	}
	for i, c := range rec.chunks {
		if !strings.HasSuffix(c, "\n\n") {
			t.Errorf("第 %d 次 Write 未落在帧边界:\n%q", i, c)
		}
	}
	if usage == nil || usage.PromptTokens != 3 || usage.CompletionTokens != 7 {
		t.Errorf("usage = %+v, want prompt=3 completion=7", usage)
	}
}

// TestHandleGeminiNativeStream_UpstreamErrorDiscardsPartialFrame 上游读取出错时，
// 缓冲里的残留是不完整的帧，不得写给客户端 —— 旧实现逐行写会把半行/半帧留在连接上。
func TestHandleGeminiNativeStream_UpstreamErrorDiscardsPartialFrame(t *testing.T) {
	upstreamErr := errors.New("unexpected EOF from upstream")
	body := io.MultiReader(
		strings.NewReader(`data: {"candidates":[{"content":{"parts":[{"text":"你好"}]}}]}`+"\n\n"+`data: {"candidates":[`+"\n"),
		readerFunc(func(p []byte) (int, error) { return 0, upstreamErr }),
	)

	info := nativeStreamInfo()
	a := &Adaptor{}
	a.Init(info)
	rec := httptest.NewRecorder()

	if _, err := a.handleGeminiNativeStream(context.Background(), sseResponse(body), info, rec); err != nil {
		t.Fatalf("handleGeminiNativeStream error: %v", err)
	}
	if reason := info.StreamStatus.GetEndReason(); reason != common.StreamEndReasonError {
		t.Errorf("EndReason = %q, want %q", reason, common.StreamEndReasonError)
	}
	if got := rec.Body.String(); strings.Contains(got, `{"candidates":[`+"\n") {
		t.Errorf("上游出错时半帧泄漏到客户端:\n%q", got)
	}
}

// TestHandleGeminiNativeStream_LastFrameWithoutBlankLine 上游未以空行结尾时，
// 最后一帧必须冲刷出去，不能滞留在帧缓冲里。
func TestHandleGeminiNativeStream_LastFrameWithoutBlankLine(t *testing.T) {
	upstream := `data: {"candidates":[{"content":{"parts":[{"text":"末帧"}]}}]}`

	info := nativeStreamInfo()
	a := &Adaptor{}
	a.Init(info)
	rec := httptest.NewRecorder()

	if _, err := a.handleGeminiNativeStream(context.Background(), sseResponse(strings.NewReader(upstream)), info, rec); err != nil {
		t.Fatalf("handleGeminiNativeStream error: %v", err)
	}
	if got := rec.Body.String(); got != upstream {
		t.Errorf("无空行结尾的最后一帧被吞掉:\ngot:  %q\nwant: %q", got, upstream)
	}
}

// readerFunc 把函数适配成 io.Reader
type readerFunc func(p []byte) (int, error)

func (f readerFunc) Read(p []byte) (int, error) { return f(p) }

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

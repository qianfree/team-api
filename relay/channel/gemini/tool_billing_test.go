package gemini

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/qianfree/team-api/relay/common"
	"github.com/qianfree/team-api/relay/constant"
	"github.com/qianfree/team-api/relay/dto"
)

// geminiRespFromJSON 测试助手：从原始响应 JSON 解析 GeminiChatResponse
func geminiRespFromJSON(t *testing.T, raw string) *dto.GeminiChatResponse {
	t.Helper()
	resp := &dto.GeminiChatResponse{}
	if err := json.Unmarshal([]byte(raw), resp); err != nil {
		t.Fatalf("unmarshal gemini response: %v", err)
	}
	return resp
}

// TestGeminiSearchRequestCount grounding 查询计数语义：
// webSearchQueries 条数；缺失但有来源按 1；无 grounding 为 0
func TestGeminiSearchRequestCount(t *testing.T) {
	// 2 条查询
	resp := geminiRespFromJSON(t, `{"candidates":[{"groundingMetadata":{
		"webSearchQueries":["q1","q2"],
		"groundingChunks":[{"web":{"uri":"https://a","title":"A"}},{"web":{"uri":"https://b","title":"B"}}]
	}}]}`)
	if got := geminiSearchRequestCount(resp); got != 2 {
		t.Errorf("queries count = %d, want 2", got)
	}

	// 无查询但有来源：按 1 次
	resp2 := geminiRespFromJSON(t, `{"candidates":[{"groundingMetadata":{
		"groundingChunks":[{"web":{"uri":"https://a","title":"A"}}]
	}}]}`)
	if got := geminiSearchRequestCount(resp2); got != 1 {
		t.Errorf("sources-only count = %d, want 1", got)
	}

	// 无 grounding
	resp3 := geminiRespFromJSON(t, `{"candidates":[{"content":{"parts":[{"text":"hi"}]}}]}`)
	if got := geminiSearchRequestCount(resp3); got != 0 {
		t.Errorf("no grounding count = %d, want 0", got)
	}
}

// TestGeminiNativeStream_SearchCount 流式口径：grounding 随末帧到达，
// 计数写入返回的计费用量
func TestGeminiNativeStream_SearchCount(t *testing.T) {
	upstream := strings.Join([]string{
		`data: {"candidates":[{"content":{"parts":[{"text":"答案"}]}}],"usageMetadata":{"promptTokenCount":10,"candidatesTokenCount":5}}`,
		``,
		`data: {"candidates":[{"finishReason":"STOP","groundingMetadata":{"webSearchQueries":["q1","q2","q3"],"groundingChunks":[{"web":{"uri":"https://a"}}]}}],"usageMetadata":{"promptTokenCount":10,"candidatesTokenCount":9}}`,
		``,
	}, "\n")

	info := &common.RelayInfo{
		RelayMode:       int(constant.RelayModeGeminiChat),
		IsStream:        true,
		InboundFormat:   constant.RelayFormatGemini,
		ClientFormat:    constant.RelayFormatGemini,
		OriginModelName: "gemini-2.5-pro",
		StartTime:       time.Now(),
		StreamStatus:    common.NewStreamStatus(),
		ChannelMeta:     &common.ChannelMeta{UpstreamModelName: "gemini-2.5-pro"},
	}
	rec := httptest.NewRecorder()
	a := &Adaptor{}
	a.Init(info)
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": {"text/event-stream"}},
		Body:       io.NopCloser(strings.NewReader(upstream)),
	}
	usage, err := a.handleGeminiNativeStream(context.Background(), resp, info, rec)
	if err != nil {
		t.Fatalf("handleGeminiNativeStream: %v", err)
	}
	if got := usage.ToolCalls["google_search"]; got != 3 {
		t.Errorf("ToolCalls[google_search] = %d, want 3", got)
	}
	if usage.CompletionTokens != 9 {
		t.Errorf("CompletionTokens = %d, want 9", usage.CompletionTokens)
	}
}

// TestGeminiNativeNonStream_SearchCount 非流式直通：grounding 计数透出（响应体原样透传）
func TestGeminiNativeNonStream_SearchCount(t *testing.T) {
	body := `{"candidates":[{"content":{"parts":[{"text":"hi"}]},"groundingMetadata":{"webSearchQueries":["q1"],"groundingChunks":[{"web":{"uri":"https://a"}}]}}],"usageMetadata":{"promptTokenCount":10,"candidatesTokenCount":5,"totalTokenCount":15}}`

	info := &common.RelayInfo{
		RelayMode:       int(constant.RelayModeGeminiChat),
		IsStream:        false,
		InboundFormat:   constant.RelayFormatGemini,
		ClientFormat:    constant.RelayFormatGemini,
		OriginModelName: "gemini-2.5-pro",
		StartTime:       time.Now(),
		StreamStatus:    common.NewStreamStatus(),
		ChannelMeta:     &common.ChannelMeta{UpstreamModelName: "gemini-2.5-pro"},
	}
	rec := httptest.NewRecorder()
	a := &Adaptor{}
	a.Init(info)
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": {"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}
	usage, err := a.handleGeminiNativeNonStream(context.Background(), resp, info, rec)
	if err != nil {
		t.Fatalf("handleGeminiNativeNonStream: %v", err)
	}
	if got := usage.ToolCalls["google_search"]; got != 1 {
		t.Errorf("ToolCalls[google_search] = %d, want 1", got)
	}
	if rec.Body.String() != body {
		t.Errorf("response body must be forwarded intact")
	}
}

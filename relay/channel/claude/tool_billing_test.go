package claude

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/qianfree/team-api/relay/dto"
)

// TestBuildUsageFromClaude_ServerToolUsage 非流式口径：usage.server_tool_use.web_search_requests
// 透出为 ToolCalls["web_search"]（按次附加费计数的采集点）
func TestBuildUsageFromClaude_ServerToolUsage(t *testing.T) {
	cu := &dto.ClaudeUsage{
		InputTokens:   100,
		OutputTokens:  50,
		ServerToolUse: &dto.ClaudeServerToolUsage{WebSearchRequests: 3},
	}
	usage := buildUsageFromClaude(cu)
	if got := usage.ToolCalls["web_search"]; got != 3 {
		t.Errorf("ToolCalls[web_search] = %d, want 3", got)
	}

	// 无服务端工具：ToolCalls 为空
	usage2 := buildUsageFromClaude(&dto.ClaudeUsage{InputTokens: 10, OutputTokens: 5})
	if len(usage2.ToolCalls) != 0 {
		t.Errorf("ToolCalls = %v, want empty", usage2.ToolCalls)
	}
}

// TestClaudeNativeStream_ServerToolUsage 流式口径：message_delta 携带的累计
// server_tool_use 计数随最终 usage 透出（厂商报数替换语义，最终帧胜出）
func TestClaudeNativeStream_ServerToolUsage(t *testing.T) {
	upstream := strings.Join([]string{
		`event: message_start`,
		`data: {"type":"message_start","message":{"id":"msg_01","usage":{"input_tokens":10,"output_tokens":1}}}`,
		``,
		`event: content_block_delta`,
		`data: {"type":"content_block_delta","delta":{"type":"text_delta","text":"答案"}}`,
		``,
		`event: message_delta`,
		`data: {"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":8,"server_tool_use":{"web_search_requests":2}}}`,
		``,
		`event: message_stop`,
		`data: {"type":"message_stop"}`,
		``,
	}, "\n")

	info := nativeStreamInfo()
	rec := httptest.NewRecorder()
	a := &Adaptor{}
	usage, err := a.handleClaudeNativeStream(context.Background(), sseResponse(strings.NewReader(upstream)), info, rec)
	if err != nil {
		t.Fatalf("handleClaudeNativeStream: %v", err)
	}
	if got := usage.ToolCalls["web_search"]; got != 2 {
		t.Errorf("ToolCalls[web_search] = %d, want 2", got)
	}
	if usage.CompletionTokens != 8 {
		t.Errorf("CompletionTokens = %d, want 8", usage.CompletionTokens)
	}
}

// TestClaudeNonStream_ServerToolUsage 非流式直通：原始 body 中的 server_tool_use
// 被提取（响应体仍逐字节透传）
func TestClaudeNonStream_ServerToolUsage(t *testing.T) {
	body := `{"id":"msg_02","type":"message","role":"assistant","content":[{"type":"text","text":"hi"}],
		"usage":{"input_tokens":20,"output_tokens":7,"server_tool_use":{"web_search_requests":5}}}`

	info := nativeStreamInfo()
	info.IsStream = false
	rec := httptest.NewRecorder()
	a := &Adaptor{}
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": {"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}
	usage, err := a.handleClaudeNativeNonStream(context.Background(), resp, info, rec)
	if err != nil {
		t.Fatalf("handleClaudeNativeNonStream: %v", err)
	}
	if got := usage.ToolCalls["web_search"]; got != 5 {
		t.Errorf("ToolCalls[web_search] = %d, want 5", got)
	}
	if rec.Body.String() != body {
		t.Errorf("response body must be forwarded intact")
	}
}

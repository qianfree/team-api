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
		ServerToolUse: &dto.ClaudeServerToolUsage{Counts: map[string]int{"web_search": 3}},
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

// TestClaudeServerToolUsage_AllFields 三工具齐报：web_search / web_fetch /
// code_execution 各自独立计数透出（计费 key 与快照展示映射对齐）
func TestClaudeServerToolUsage_AllFields(t *testing.T) {
	// 非流式：usage 直构
	cu := &dto.ClaudeUsage{
		InputTokens:   100,
		OutputTokens:  50,
		ServerToolUse: &dto.ClaudeServerToolUsage{Counts: map[string]int{"web_search": 3, "web_fetch": 4, "code_execution": 2}},
	}
	usage := buildUsageFromClaude(cu)
	for key, want := range map[string]int{"web_search": 3, "web_fetch": 4, "code_execution": 2} {
		if got := usage.ToolCalls[key]; got != want {
			t.Errorf("ToolCalls[%s] = %d, want %d", key, got, want)
		}
	}

	// 流式：message_delta 携带三工具累计报数（替换语义，最终帧胜出）
	upstream := strings.Join([]string{
		`event: message_start`,
		`data: {"type":"message_start","message":{"id":"msg_03","usage":{"input_tokens":10,"output_tokens":1}}}`,
		``,
		`event: content_block_delta`,
		`data: {"type":"content_block_delta","delta":{"type":"text_delta","text":"答案"}}`,
		``,
		`event: message_delta`,
		`data: {"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":8,"server_tool_use":{"web_search_requests":2,"web_fetch_requests":6,"code_execution_requests":1}}}`,
		``,
		`event: message_stop`,
		`data: {"type":"message_stop"}`,
		``,
	}, "\n")

	info := nativeStreamInfo()
	rec := httptest.NewRecorder()
	a := &Adaptor{}
	susage, err := a.handleClaudeNativeStream(context.Background(), sseResponse(strings.NewReader(upstream)), info, rec)
	if err != nil {
		t.Fatalf("handleClaudeNativeStream: %v", err)
	}
	for key, want := range map[string]int{"web_search": 2, "web_fetch": 6, "code_execution": 1} {
		if got := susage.ToolCalls[key]; got != want {
			t.Errorf("stream ToolCalls[%s] = %d, want %d", key, got, want)
		}
	}
}

// TestClaudeServerToolUsage_FutureTool 上游未来新增工具（未知「工具名_requests」字段）
// 自动采集透出，无需跟进代码——价格表配了该工具名即计费，未配置则免费放行
func TestClaudeServerToolUsage_FutureTool(t *testing.T) {
	upstream := strings.Join([]string{
		`event: message_start`,
		`data: {"type":"message_start","message":{"id":"msg_04","usage":{"input_tokens":10,"output_tokens":1}}}`,
		``,
		`event: message_delta`,
		`data: {"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":8,"server_tool_use":{"browser_use_requests":2}}}`,
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
	if got := usage.ToolCalls["browser_use"]; got != 2 {
		t.Errorf("ToolCalls[browser_use] = %d, want 2", got)
	}
}

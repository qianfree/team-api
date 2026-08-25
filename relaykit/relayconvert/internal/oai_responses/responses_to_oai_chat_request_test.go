package oai_responses

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/qianfree/team-api/relaykit/dto"
	"github.com/qianfree/team-api/relaykit/relayconvert"
	"github.com/qianfree/team-api/relaykit/relayconvert/convmeta"
)

// mustParseResponsesRequest 从 JSON 构造测试请求。
func mustParseResponsesRequest(t *testing.T, raw string) *dto.OpenAIResponsesRequest {
	t.Helper()
	req := &dto.OpenAIResponsesRequest{}
	if err := json.Unmarshal([]byte(raw), req); err != nil {
		t.Fatalf("parse responses request: %v", err)
	}
	return req
}

// mustCastChatRequest 断言转换输出为 chat 请求。
func mustCastChatRequest(t *testing.T, result any) *dto.GeneralOpenAIRequest {
	t.Helper()
	chat, ok := result.(*dto.GeneralOpenAIRequest)
	if !ok {
		t.Fatalf("expected *dto.GeneralOpenAIRequest, got %T", result)
	}
	return chat
}

// 流式 + thinking 后缀吸收：info.IsStream 注入 stream_options，
// reasoning_effort 缺席时取宿主注入的后缀映射。
func TestResponsesToOpenAIChatRequest_StreamAndEffortAbsorption(t *testing.T) {
	req := mustParseResponsesRequest(t, `{
		"model": "gpt-4o",
		"input": "你好",
		"stream": true
	}`)
	info := &convmeta.Values{
		ChannelMetaAttached: true,
		UpstreamModelName:   "gpt-4o-2024-11-20",
		IsStream:            true,
		ReasoningEffort:     "high",
	}

	result, err := (&ResponsesToOpenAIChatRequestConverter{}).ConvertRequest(context.Background(), info, req)
	if err != nil {
		t.Fatalf("ConvertRequest: %v", err)
	}
	chat := mustCastChatRequest(t, result)

	if chat.StreamOptions == nil || !chat.StreamOptions.IncludeUsage {
		t.Error("expected stream_options.include_usage=true when info.IsStream")
	}
	if chat.ReasoningEffort != "high" {
		t.Errorf("ReasoningEffort = %q, want high (宿主 thinking 后缀兜底)", chat.ReasoningEffort)
	}
}

// 客户端显式 reasoning.effort 优先于宿主注入的后缀映射。
func TestResponsesToOpenAIChatRequest_ExplicitEffortWins(t *testing.T) {
	req := mustParseResponsesRequest(t, `{
		"model": "o3",
		"input": "hi",
		"reasoning": {"effort": "low"}
	}`)
	info := &convmeta.Values{
		ChannelMetaAttached: true,
		UpstreamModelName:   "o3",
		ReasoningEffort:     "high",
	}

	result, err := (&ResponsesToOpenAIChatRequestConverter{}).ConvertRequest(context.Background(), info, req)
	if err != nil {
		t.Fatalf("ConvertRequest: %v", err)
	}
	chat := mustCastChatRequest(t, result)
	if chat.ReasoningEffort != "low" {
		t.Errorf("ReasoningEffort = %q, want low (客户端显式设置优先)", chat.ReasoningEffort)
	}
}

// 入参类型断言失败返回明确错误。
func TestResponsesToOpenAIChatRequest_TypeMismatch(t *testing.T) {
	_, err := (&ResponsesToOpenAIChatRequestConverter{}).ConvertRequest(context.Background(), nil, "not-a-request")
	if err == nil {
		t.Fatal("expected type assertion error")
	}
}

// degradationReport fake 上报记录。
type degradationReport struct {
	converterID string
	reason      string
	count       int
}

// recordingReporter 嵌入 convmeta.Values（Meta 能力）并实现 DegradationReporter，
// 捕获降级上报供断言。
type recordingReporter struct {
	convmeta.Values
	reports []degradationReport
}

func (r *recordingReporter) ReportConversionDegradation(converterID, reason string, count int) {
	r.reports = append(r.reports, degradationReport{converterID: converterID, reason: reason, count: count})
}

// TestR2C_DegradationReporting codex 形状的请求（additional_tools 输入项、namespace/custom
// 工具、reasoning 项、custom 工具调用历史）经 responses→chat 转换时，丢弃必须上报——
// 降级可见，不允许静默砍能力；同时可转换部分（function 工具、消息、function_call 历史）不受影响。
func TestR2C_DegradationReporting(t *testing.T) {
	req := mustParseResponsesRequest(t, `{
		"model": "gpt-5.6-sol",
		"input": [
			{"type":"additional_tools","role":"developer","tools":[{"type":"namespace","name":"functions","tools":[{"type":"custom","name":"exec"}]}]},
			{"type":"message","role":"user","content":[{"type":"input_text","text":"hi"}]},
			{"type":"message","role":"assistant","content":[{"type":"output_text","text":"hello"}]},
			{"type":"reasoning","id":"rs_1","encrypted_content":"enc"},
			{"type":"custom_tool_call","call_id":"call_1","name":"exec","input":"text(...)"},
			{"type":"custom_tool_call_output","call_id":"call_1","output":"done"},
			{"type":"function_call","call_id":"call_2","name":"get_weather","arguments":"{}"},
			{"type":"function_call_output","call_id":"call_2","output":"{}"}
		],
		"tools": [
			{"type":"function","name":"f1","parameters":{"type":"object"}},
			{"type":"namespace","name":"collaboration","tools":[]},
			{"type":"custom","name":"exec","format":{"type":"grammar"}}
		]
	}`)
	info := &recordingReporter{Values: convmeta.Values{
		ChannelMetaAttached: true,
		UpstreamModelName:   "gpt-5.6-sol",
	}}

	result, err := (&ResponsesToOpenAIChatRequestConverter{}).ConvertRequest(context.Background(), info, req)
	if err != nil {
		t.Fatalf("ConvertRequest: %v", err)
	}
	chat := mustCastChatRequest(t, result)

	// 降级上报：按 reason 聚合，reason 有序
	want := map[string]int{
		"input_item:additional_tools":        1,
		"input_item:reasoning":               1,
		"input_item:custom_tool_call":        1,
		"input_item:custom_tool_call_output": 1,
		"tool:namespace":                     1,
		"tool:custom":                        1,
	}
	got := map[string]int{}
	converterIDs := map[string]bool{}
	for _, r := range info.reports {
		got[r.reason] = r.count
		converterIDs[r.converterID] = true
	}
	if len(got) != len(want) {
		t.Fatalf("degradation reports = %v, want %v", got, want)
	}
	for reason, n := range want {
		if got[reason] != n {
			t.Errorf("degradation %s = %d, want %d (all: %v)", reason, got[reason], n, got)
		}
	}
	if len(converterIDs) != 1 || !converterIDs[relayconvert.ConverterOpenAIResponsesToOpenAIChat] {
		t.Errorf("converter IDs = %v, want only %s", converterIDs, relayconvert.ConverterOpenAIResponsesToOpenAIChat)
	}

	// 可转换部分不受影响：消息序列 user → assistant(text) → assistant(tool_calls) → tool
	if len(chat.Messages) != 4 {
		t.Fatalf("messages = %d, want 4: %+v", len(chat.Messages), chat.Messages)
	}
	if chat.Messages[0].Role != "user" || chat.Messages[1].Role != "assistant" ||
		chat.Messages[2].Role != "assistant" || chat.Messages[3].Role != "tool" {
		t.Errorf("message roles = %s %s %s %s", chat.Messages[0].Role, chat.Messages[1].Role, chat.Messages[2].Role, chat.Messages[3].Role)
	}
	if len(chat.Tools) != 1 || chat.Tools[0].Function.Name != "f1" {
		t.Errorf("tools = %+v, want only f1", chat.Tools)
	}
}

// TestR2C_NoDegradationReportForCleanInput 纯 function/消息请求零上报。
func TestR2C_NoDegradationReportForCleanInput(t *testing.T) {
	req := mustParseResponsesRequest(t, `{
		"model": "gpt-4o",
		"input": "hi",
		"tools": [{"type":"function","name":"f1","parameters":{"type":"object"}}]
	}`)
	info := &recordingReporter{Values: convmeta.Values{
		ChannelMetaAttached: true,
		UpstreamModelName:   "gpt-4o",
	}}

	if _, err := (&ResponsesToOpenAIChatRequestConverter{}).ConvertRequest(context.Background(), info, req); err != nil {
		t.Fatalf("ConvertRequest: %v", err)
	}
	if len(info.reports) != 0 {
		t.Errorf("clean input should report nothing, got %v", info.reports)
	}
}

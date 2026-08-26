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
// 工具、reasoning 项、custom 工具调用历史）经 responses→chat 转换时：可映射的工具与历史
// 项全部保留（additional_tools/namespace 子工具展开、custom 工具映射为 function、
// custom_tool_call 历史还原为 tool_calls + tool 消息），仅剩无 chat 对应物的内容
// （空 namespace、加密 reasoning、重名工具）按降级上报——降级可见，不允许静默砍能力。
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

	// 降级上报：仅无可还原内容的上报（空 namespace、加密 reasoning、
	// 与 additional_tools 中已映射 exec 重名的顶层 custom exec）
	want := map[string]int{
		"input_item:reasoning": 1,
		"tool:namespace":       1,
		"tool:custom":          1,
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

	// 消息序列：custom_tool_call 历史还原为 assistant tool_calls（exec 并入紧邻的
	// assistant 文本消息）+ tool 消息；function_call 历史另起 assistant 消息 + tool 消息
	if len(chat.Messages) != 5 {
		t.Fatalf("messages = %d, want 5: %+v", len(chat.Messages), chat.Messages)
	}
	if chat.Messages[0].Role != "user" || chat.Messages[1].Role != "assistant" ||
		chat.Messages[2].Role != "tool" || chat.Messages[3].Role != "assistant" ||
		chat.Messages[4].Role != "tool" {
		t.Errorf("message roles = %s %s %s %s %s",
			chat.Messages[0].Role, chat.Messages[1].Role, chat.Messages[2].Role,
			chat.Messages[3].Role, chat.Messages[4].Role)
	}
	// exec 并入 assistant 文本消息（同一助手轮），arguments 为 {"input":"text(...)"} 包裹形态
	if len(chat.Messages[1].ToolCalls) != 1 || chat.Messages[1].ToolCalls[0].Function.Name != "exec" {
		t.Fatalf("messages[1].tool_calls = %+v, want exec", chat.Messages[1].ToolCalls)
	}
	if got := chat.Messages[1].ToolCalls[0].Function.Arguments; got != `{"input":"text(...)"}` {
		t.Errorf("exec arguments = %q, want wrapped input form", got)
	}
	if chat.Messages[2].ToolCallID != "call_1" || chat.Messages[2].Content != "done" {
		t.Errorf("custom_tool_call_output 未还原为 tool 消息: %+v", chat.Messages[2])
	}
	if len(chat.Messages[3].ToolCalls) != 1 || chat.Messages[3].ToolCalls[0].Function.Name != "get_weather" {
		t.Errorf("messages[3].tool_calls = %+v, want get_weather", chat.Messages[3].ToolCalls)
	}

	// 工具：f1 保留 + additional_tools 中的 custom exec 映射为 function 工具
	if len(chat.Tools) != 2 {
		t.Fatalf("tools = %+v, want [f1 exec]", chat.Tools)
	}
	if chat.Tools[0].Function.Name != "f1" || chat.Tools[1].Function.Name != "exec" {
		t.Errorf("tools = %+v, want [f1 exec]", chat.Tools)
	}
	// 映射的 exec 工具带 {"input": string} 包裹 schema
	params, _ := json.Marshal(chat.Tools[1].Function.Parameters)
	var schema struct {
		Required []string `json:"required"`
	}
	if err := json.Unmarshal(params, &schema); err != nil || len(schema.Required) != 1 || schema.Required[0] != "input" {
		t.Errorf("exec schema = %s, want required [input]", params)
	}
}

// TestR2C_CodexShellAndPatchToolMapping local_shell / apply_patch 工具映射为固定名
// function 工具，其调用历史（action 对象）原样还原为 arguments；stash 记录原始类型
// 供响应侧还原输出项。
func TestR2C_CodexShellAndPatchToolMapping(t *testing.T) {
	req := mustParseResponsesRequest(t, `{
		"model": "gpt-5.6-sol",
		"input": [
			{"type":"message","role":"user","content":[{"type":"input_text","text":"run tests"}]},
			{"type":"local_shell_call","id":"lsh_1","call_id":"call_s1","action":{"type":"exec","command":["npm","test"],"timeout":60000}},
			{"type":"local_shell_call_output","call_id":"call_s1","output":"ok"},
			{"type":"apply_patch_call","id":"ap_1","call_id":"call_p1","action":{"type":"update_file","path":"a.go","patch":"@@"}},
			{"type":"apply_patch_call_output","call_id":"call_p1","output":"patched"}
		],
		"tools": [
			{"type":"local_shell"},
			{"type":"apply_patch"}
		]
	}`)
	stash := newToolKindStashFake()
	info := &recordingReporterWithStash{
		recordingReporter: recordingReporter{Values: convmeta.Values{
			ChannelMetaAttached: true,
			UpstreamModelName:   "gpt-5.6-sol",
		}},
		stash: stash,
	}

	result, err := (&ResponsesToOpenAIChatRequestConverter{}).ConvertRequest(context.Background(), info, req)
	if err != nil {
		t.Fatalf("ConvertRequest: %v", err)
	}
	chat := mustCastChatRequest(t, result)

	// 工具映射：local_shell→shell、apply_patch→apply_patch
	if len(chat.Tools) != 2 || chat.Tools[0].Function.Name != "shell" || chat.Tools[1].Function.Name != "apply_patch" {
		t.Fatalf("tools = %+v, want [shell apply_patch]", chat.Tools)
	}
	// stash 记录原始类型（响应侧还原依据）
	if stash.kinds["shell"] != "local_shell" || stash.kinds["apply_patch"] != "apply_patch" {
		t.Errorf("stash = %v, want shell→local_shell, apply_patch→apply_patch", stash.kinds)
	}
	// 调用历史：action 原样作为 arguments，tool 消息引用 call_id
	if len(chat.Messages) != 5 {
		t.Fatalf("messages = %d, want 5: %+v", len(chat.Messages), chat.Messages)
	}
	shellCall := chat.Messages[1].ToolCalls
	if len(shellCall) != 1 || shellCall[0].Function.Name != "shell" ||
		shellCall[0].Function.Arguments != `{"type":"exec","command":["npm","test"],"timeout":60000}` {
		t.Errorf("shell history tool_call = %+v", shellCall)
	}
	if chat.Messages[2].Role != "tool" || chat.Messages[2].ToolCallID != "call_s1" || chat.Messages[2].Content != "ok" {
		t.Errorf("local_shell_call_output = %+v", chat.Messages[2])
	}
	patchCall := chat.Messages[3].ToolCalls
	if len(patchCall) != 1 || patchCall[0].Function.Name != "apply_patch" ||
		patchCall[0].Function.Arguments != `{"type":"update_file","path":"a.go","patch":"@@"}` {
		t.Errorf("patch history tool_call = %+v", patchCall)
	}
	if len(info.reports) != 0 {
		t.Errorf("fully mapped request should report nothing, got %v", info.reports)
	}
}

// toolKindStashFake 测试用 responsesToolKindStash 实现。
type toolKindStashFake struct {
	kinds map[string]string
}

func newToolKindStashFake() *toolKindStashFake {
	return &toolKindStashFake{kinds: map[string]string{}}
}

func (f *toolKindStashFake) StashResponsesToolKind(name, kind string) { f.kinds[name] = kind }
func (f *toolKindStashFake) ResponsesToolKind(name string) string     { return f.kinds[name] }

// recordingReporterWithStash 组合降级上报与工具类型 stash 两个能力接口。
type recordingReporterWithStash struct {
	recordingReporter
	stash *toolKindStashFake
}

func (r *recordingReporterWithStash) StashResponsesToolKind(name, kind string) {
	r.stash.StashResponsesToolKind(name, kind)
}
func (r *recordingReporterWithStash) ResponsesToolKind(name string) string {
	return r.stash.ResponsesToolKind(name)
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

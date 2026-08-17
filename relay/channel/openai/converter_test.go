package openai

import (
	"encoding/json"
	"errors"
	"io"
	"testing"

	"github.com/qianfree/team-api/relay/common"
	"github.com/qianfree/team-api/relay/constant"
	"github.com/qianfree/team-api/relay/dto"
)

func TestExtractClaudeSystemText(t *testing.T) {
	t.Run("plain string", func(t *testing.T) {
		if got := extractClaudeSystemText("you are helpful"); got != "you are helpful" {
			t.Errorf("got %q", got)
		}
	})
	t.Run("array of text blocks joined", func(t *testing.T) {
		sys := []any{
			map[string]any{"type": "text", "text": "line1"},
			map[string]any{"type": "text", "text": "line2"},
		}
		if got := extractClaudeSystemText(sys); got != "line1\nline2" {
			t.Errorf("got %q, want \"line1\\nline2\"", got)
		}
	})
	t.Run("non-text blocks ignored", func(t *testing.T) {
		sys := []any{
			map[string]any{"type": "text", "text": "keep"},
			map[string]any{"type": "image", "text": "drop"},
		}
		if got := extractClaudeSystemText(sys); got != "keep" {
			t.Errorf("got %q, want \"keep\"", got)
		}
	})
	t.Run("unknown type returns empty", func(t *testing.T) {
		if got := extractClaudeSystemText(42); got != "" {
			t.Errorf("got %q, want empty", got)
		}
	})
}

func TestC2oJoinParts(t *testing.T) {
	tests := []struct {
		in   []string
		want string
	}{
		{nil, ""},
		{[]string{}, ""},
		{[]string{"only"}, "only"},
		{[]string{"a", "b", "c"}, "a\nb\nc"},
	}
	for _, tt := range tests {
		if got := c2oJoinParts(tt.in); got != tt.want {
			t.Errorf("c2oJoinParts(%v) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestC2oConvertThinkingToReasoningEffort(t *testing.T) {
	tests := []struct {
		name   string
		budget *int
		want   string
	}{
		{"nil budget defaults medium", nil, "medium"},
		{"low boundary", intPtr(2048), "low"},
		{"low", intPtr(1000), "low"},
		{"medium boundary", intPtr(16384), "medium"},
		{"medium", intPtr(8000), "medium"},
		{"high", intPtr(16385), "high"},
		{"high large", intPtr(50000), "high"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := c2oConvertThinkingToReasoningEffort(&dto.ClaudeThinking{BudgetTokens: tt.budget})
			if got != tt.want {
				t.Errorf("budget=%v => %q, want %q", tt.budget, got, tt.want)
			}
		})
	}
}

func TestG2oConvertThinkingConfig(t *testing.T) {
	// 与 Claude 同阈值
	tests := []struct {
		budget *int
		want   string
	}{
		{nil, "medium"},
		{intPtr(2048), "low"},
		{intPtr(16384), "medium"},
		{intPtr(20000), "high"},
	}
	for _, tt := range tests {
		got := g2oConvertThinkingConfig(&dto.GeminiThinkingConfig{ThoughtBudget: tt.budget})
		if got != tt.want {
			t.Errorf("budget=%v => %q, want %q", tt.budget, got, tt.want)
		}
	}
}

func TestC2oConvertToolChoice(t *testing.T) {
	t.Run("nil", func(t *testing.T) {
		if got := c2oConvertToolChoice(nil); got != nil {
			t.Errorf("got %v, want nil", got)
		}
	})
	t.Run("string passthrough", func(t *testing.T) {
		if got := c2oConvertToolChoice("auto"); got != "auto" {
			t.Errorf("got %v", got)
		}
	})
	t.Run("type mappings", func(t *testing.T) {
		cases := map[string]string{"auto": "auto", "any": "required", "none": "none"}
		for in, want := range cases {
			got := c2oConvertToolChoice(map[string]any{"type": in})
			if got != want {
				t.Errorf("type=%q => %v, want %q", in, got, want)
			}
		}
	})
	t.Run("specific tool maps to function", func(t *testing.T) {
		got := c2oConvertToolChoice(map[string]any{"type": "tool", "name": "get_weather"})
		m, ok := got.(map[string]any)
		if !ok {
			t.Fatalf("expected map, got %T", got)
		}
		if m["type"] != "function" {
			t.Errorf("type = %v, want function", m["type"])
		}
		fn, ok := m["function"].(map[string]any)
		if !ok || fn["name"] != "get_weather" {
			t.Errorf("function = %v, want name=get_weather", m["function"])
		}
	})
	t.Run("tool without name falls back to required", func(t *testing.T) {
		if got := c2oConvertToolChoice(map[string]any{"type": "tool"}); got != "required" {
			t.Errorf("got %v, want required", got)
		}
	})
}

func TestG2oMapRole(t *testing.T) {
	tests := map[string]string{
		"model": "assistant",
		"user":  "user",
		"tool":  "tool",
	}
	for in, want := range tests {
		if got := g2oMapRole(in); got != want {
			t.Errorf("g2oMapRole(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestC2rContentToString(t *testing.T) {
	if got := c2rContentToString(nil); got != "" {
		t.Errorf("nil => %q, want empty", got)
	}
	if got := c2rContentToString("hello"); got != "hello" {
		t.Errorf("string => %q", got)
	}
	// 非字符串 -> JSON 编码
	got := c2rContentToString([]any{map[string]any{"type": "text", "text": "x"}})
	var back []any
	if err := json.Unmarshal([]byte(got), &back); err != nil {
		t.Errorf("non-string content should be JSON-encoded, got %q (%v)", got, err)
	}
}

func TestC2rGetMaxTokens(t *testing.T) {
	tests := []struct {
		name    string
		maxTok  *int
		maxComp *int
		want    int
	}{
		{"both nil", nil, nil, 0},
		{"max_tokens only", intPtr(100), nil, 100},
		{"max_completion larger wins", intPtr(100), intPtr(200), 200},
		{"max_completion smaller keeps max_tokens", intPtr(300), intPtr(50), 300},
		{"zero max_tokens ignored", intPtr(0), nil, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := dto.GeneralOpenAIRequest{MaxTokens: tt.maxTok, MaxCompletionTokens: tt.maxComp}
			if got := c2rGetMaxTokens(req); got != tt.want {
				t.Errorf("got %d, want %d", got, tt.want)
			}
		})
	}
}

func TestConvertClaudeUserMessage_StringContent(t *testing.T) {
	msgs := convertClaudeUserMessage(dto.ClaudeMessage{Role: "user", Content: "hi there"})
	if len(msgs) != 1 {
		t.Fatalf("got %d messages, want 1", len(msgs))
	}
	if msgs[0].Role != "user" || msgs[0].Content != "hi there" {
		t.Errorf("got %+v", msgs[0])
	}
}

// TestConvertClaudeUserMessage_ToolResultArrayContent tool_result.content 为内容块数组
// （Claude 规范形式 [{type:"text",text:"..."}]）时，文本块须拼接保留，
// 否则工具结果会被替换为空字符串导致内容丢失。
func TestConvertClaudeUserMessage_ToolResultArrayContent(t *testing.T) {
	msgs := convertClaudeUserMessage(dto.ClaudeMessage{
		Role: "user",
		Content: []any{
			map[string]any{
				"type":        "tool_result",
				"tool_use_id": "toolu_123",
				"content": []any{
					map[string]any{"type": "text", "text": "result-a"},
					map[string]any{"type": "text", "text": "result-b"},
				},
			},
		},
	})
	if len(msgs) != 1 {
		t.Fatalf("got %d messages, want 1", len(msgs))
	}
	if msgs[0].Role != "tool" {
		t.Errorf("role = %q, want tool", msgs[0].Role)
	}
	if msgs[0].ToolCallID != "toolu_123" {
		t.Errorf("ToolCallID = %q, want toolu_123", msgs[0].ToolCallID)
	}
	if msgs[0].Content != "result-a\nresult-b" {
		t.Errorf("content = %q, want %q", msgs[0].Content, "result-a\nresult-b")
	}
}

// TestConvertResponsesToOpenAI_PenaltyPassthrough Responses 入站转 chat 出站时
// 透传 presence/frequency penalty 与 prompt_cache_key（vLLM 等 OpenAI 兼容上游接受）。
func TestConvertResponsesToOpenAI_PenaltyPassthrough(t *testing.T) {
	info := &common.RelayInfo{ChannelMeta: &common.ChannelMeta{}}
	body := []byte(`{"model":"gpt-4o","input":"hi","frequency_penalty":0.5,"presence_penalty":0.2}`)
	out, err := ConvertResponsesToOpenAI(body, info)
	if err != nil {
		t.Fatalf("ConvertResponsesToOpenAI: %v", err)
	}
	raw, _ := io.ReadAll(out)
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("bad json: %v\n%s", err, raw)
	}
	if m["frequency_penalty"] != 0.5 {
		t.Errorf("frequency_penalty = %v, want 0.5", m["frequency_penalty"])
	}
	if m["presence_penalty"] != 0.2 {
		t.Errorf("presence_penalty = %v, want 0.2", m["presence_penalty"])
	}
	if _, ok := m["messages"]; !ok {
		t.Error("messages should be present")
	}
}

// TestConvertOpenAIToResponses_PenaltyDropped chat 入站转 Responses 出站时
// 丢弃官方不支持的 presence/frequency penalty（透传会被严格上游拒绝），
// 保留官方参数 prompt_cache_key。
func TestConvertOpenAIToResponses_PenaltyDropped(t *testing.T) {
	info := &common.RelayInfo{ChannelMeta: &common.ChannelMeta{}}
	body := []byte(`{"model":"gpt-4o","messages":[{"role":"user","content":"hi"}],"frequency_penalty":0.5,"presence_penalty":0.2,"prompt_cache_key":"abc"}`)
	out, err := ConvertOpenAIToResponses(body, info)
	if err != nil {
		t.Fatalf("ConvertOpenAIToResponses: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(out, &m); err != nil {
		t.Fatalf("bad json: %v\n%s", err, out)
	}
	if _, ok := m["frequency_penalty"]; ok {
		t.Error("frequency_penalty should be dropped (not official Responses API)")
	}
	if _, ok := m["presence_penalty"]; ok {
		t.Error("presence_penalty should be dropped (not official Responses API)")
	}
	if m["prompt_cache_key"] != "abc" {
		t.Errorf("prompt_cache_key = %v, want abc", m["prompt_cache_key"])
	}
	if _, ok := m["input"]; !ok {
		t.Error("input should be present")
	}
}

// TestConvertResponsesToOpenAI_PreviousResponseIDRejected 有状态请求（previous_response_id）
// 落在 chat-only 渠道时必须快速失败：降级转换会静默丢失全部会话上下文。
// 错误须携带哨兵以供 relay_handler 识别并驱动调度 FSM 换渠道。
func TestConvertResponsesToOpenAI_PreviousResponseIDRejected(t *testing.T) {
	info := &common.RelayInfo{ChannelMeta: &common.ChannelMeta{}}
	body := []byte(`{"model":"gpt-4o","input":"hi","previous_response_id":"resp_prev"}`)
	_, err := ConvertResponsesToOpenAI(body, info)
	if err == nil {
		t.Fatal("previous_response_id should be rejected on chat-only conversion")
	}
	if !errors.Is(err, constant.ErrStatefulResponsesUnsupported) {
		t.Errorf("error should wrap ErrStatefulResponsesUnsupported, got: %v", err)
	}
}

// TestConvertResponsesToOpenAI_TextFormat Responses text.format（扁平结构）转 chat
// response_format（json_schema 嵌套结构）；text 类型不映射。
func TestConvertResponsesToOpenAI_TextFormat(t *testing.T) {
	cases := []struct {
		name string
		text string
		want map[string]any
	}{
		{
			name: "json_object",
			text: `{"format":{"type":"json_object"}}`,
			want: map[string]any{"type": "json_object"},
		},
		{
			name: "json_schema",
			text: `{"format":{"type":"json_schema","name":"out","schema":{"type":"object"},"strict":true}}`,
			want: map[string]any{
				"type": "json_schema",
				"json_schema": map[string]any{
					"name":   "out",
					"schema": map[string]any{"type": "object"},
					"strict": true,
				},
			},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			info := &common.RelayInfo{ChannelMeta: &common.ChannelMeta{}}
			body := []byte(`{"model":"gpt-4o","input":"hi","text":` + c.text + `}`)
			out, err := ConvertResponsesToOpenAI(body, info)
			if err != nil {
				t.Fatalf("ConvertResponsesToOpenAI: %v", err)
			}
			raw, _ := io.ReadAll(out)
			var m map[string]any
			if err := json.Unmarshal(raw, &m); err != nil {
				t.Fatalf("bad json: %v\n%s", err, raw)
			}
			got, ok := m["response_format"].(map[string]any)
			if !ok {
				t.Fatalf("response_format missing or wrong type: %v", m["response_format"])
			}
			wantJSON, _ := json.Marshal(c.want)
			gotJSON, _ := json.Marshal(got)
			if string(gotJSON) != string(wantJSON) {
				t.Errorf("response_format = %s, want %s", gotJSON, wantJSON)
			}
		})
	}

	t.Run("text format not mapped", func(t *testing.T) {
		info := &common.RelayInfo{ChannelMeta: &common.ChannelMeta{}}
		body := []byte(`{"model":"gpt-4o","input":"hi","text":{"format":{"type":"text"}}}`)
		out, err := ConvertResponsesToOpenAI(body, info)
		if err != nil {
			t.Fatalf("ConvertResponsesToOpenAI: %v", err)
		}
		raw, _ := io.ReadAll(out)
		var m map[string]any
		_ = json.Unmarshal(raw, &m)
		if _, ok := m["response_format"]; ok {
			t.Error("text format should not map to response_format")
		}
	})
}

// TestConvertResponsesToOpenAI_StashesResponsesRequest 转换时 stash 请求快照，
// 供上游 chat 响应合成回 Responses 格式时 echo 请求参数。
func TestConvertResponsesToOpenAI_StashesResponsesRequest(t *testing.T) {
	info := &common.RelayInfo{ChannelMeta: &common.ChannelMeta{}}
	temp := 0.7
	// 先填充再转换，断言转换覆盖为最新请求体
	info.ResponsesRequest = &dto.OpenAIResponsesRequest{Model: "stale"}
	body := []byte(`{"model":"gpt-4o","input":"hi","temperature":0.7}`)
	if _, err := ConvertResponsesToOpenAI(body, info); err != nil {
		t.Fatalf("ConvertResponsesToOpenAI: %v", err)
	}
	if info.ResponsesRequest == nil {
		t.Fatal("ResponsesRequest should be stashed")
	}
	if info.ResponsesRequest.Model != "gpt-4o" {
		t.Errorf("stashed model = %q, want gpt-4o", info.ResponsesRequest.Model)
	}
	if info.ResponsesRequest.Temperature == nil || *info.ResponsesRequest.Temperature != temp {
		t.Errorf("stashed temperature = %v, want 0.7", info.ResponsesRequest.Temperature)
	}
}

// TestConvertResponsesToOpenAI_InputAudioAndFile 多模态输入透传：
// input_audio 与 Responses 同形透传；input_file 扁平结构转 chat 的 file 嵌套结构。
func TestConvertResponsesToOpenAI_InputAudioAndFile(t *testing.T) {
	info := &common.RelayInfo{ChannelMeta: &common.ChannelMeta{}}
	body := []byte(`{"model":"gpt-4o","input":[{"type":"message","role":"user","content":[` +
		`{"type":"input_text","text":"listen"},` +
		`{"type":"input_audio","input_audio":{"data":"QUJD","format":"wav"}},` +
		`{"type":"input_file","file_data":"data:text/plain;base64,aGk=","filename":"a.txt"}` +
		`]}]}`)
	out, err := ConvertResponsesToOpenAI(body, info)
	if err != nil {
		t.Fatalf("ConvertResponsesToOpenAI: %v", err)
	}
	raw, _ := io.ReadAll(out)
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("bad json: %v\n%s", err, raw)
	}
	msgs, _ := m["messages"].([]any)
	if len(msgs) != 1 {
		t.Fatalf("messages = %v", msgs)
	}
	parts, _ := msgs[0].(map[string]any)["content"].([]any)
	if len(parts) != 3 {
		t.Fatalf("content parts = %v, want 3（audio/file 不再被丢弃）", parts)
	}
	audio, _ := parts[1].(map[string]any)
	if audio["type"] != "input_audio" {
		t.Errorf("audio part = %v", audio)
	}
	ia, _ := audio["input_audio"].(map[string]any)
	if ia["data"] != "QUJD" || ia["format"] != "wav" {
		t.Errorf("input_audio = %v", ia)
	}
	file, _ := parts[2].(map[string]any)
	if file["type"] != "file" {
		t.Errorf("file part = %v", file)
	}
	f, _ := file["file"].(map[string]any)
	if f["file_data"] != "data:text/plain;base64,aGk=" || f["filename"] != "a.txt" {
		t.Errorf("file = %v", f)
	}
}

// TestConvertResponsesToOpenAI_DeveloperRoleMappedToSystem codex 等客户端会在 input 中
// 发送 developer 角色消息（新式系统提示）；第三方 chat 上游（serde 严格校验）不识别该角色，
// 必须映射为 system，否则上游直接拒绝整个请求体。
func TestConvertResponsesToOpenAI_DeveloperRoleMappedToSystem(t *testing.T) {
	info := &common.RelayInfo{ChannelMeta: &common.ChannelMeta{}}
	body := []byte(`{"model":"deepseek-v4-flash","instructions":"You are helpful.",` +
		`"input":[{"type":"message","role":"developer","content":[{"type":"input_text","text":"You are a coding agent."}]},` +
		`{"type":"message","role":"user","content":[{"type":"input_text","text":"hi"}]}],"stream":true}`)
	out, err := ConvertResponsesToOpenAI(body, info)
	if err != nil {
		t.Fatalf("ConvertResponsesToOpenAI: %v", err)
	}
	raw, _ := io.ReadAll(out)
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("bad json: %v\n%s", err, raw)
	}
	msgs, _ := m["messages"].([]any)
	if len(msgs) != 3 {
		t.Fatalf("messages = %v, want 3 (instructions→system + developer→system + user)", msgs)
	}
	for i, want := range []string{"system", "system", "user"} {
		msg := msgs[i].(map[string]any)
		if got := msg["role"]; got != want {
			t.Errorf("messages[%d].role = %v, want %q", i, got, want)
		}
	}
}

// TestConvertResponsesToOpenAI_FunctionCallHistory codex 多轮会把历史 function_call /
// function_call_output / reasoning 项放进 input：function_call 需聚合为 assistant.tool_calls
// （其后的 tool 消息才有对应的 tool_call_id），reasoning 项直接跳过。
func TestConvertResponsesToOpenAI_FunctionCallHistory(t *testing.T) {
	info := &common.RelayInfo{ChannelMeta: &common.ChannelMeta{}}
	body := []byte(`{"model":"deepseek-v4-flash","input":[` +
		`{"type":"message","role":"user","content":[{"type":"input_text","text":"list files"}]},` +
		`{"type":"reasoning","summary":[]},` +
		`{"type":"function_call","call_id":"call_a","name":"shell","arguments":"{\"cmd\":\"ls\"}"},` +
		`{"type":"function_call","call_id":"call_b","name":"read","arguments":"{\"path\":\"a.go\"}"},` +
		`{"type":"function_call_output","call_id":"call_a","output":"a.go b.go"},` +
		`{"type":"function_call_output","call_id":"call_b","output":"package main"}` +
		`]}`)
	out, err := ConvertResponsesToOpenAI(body, info)
	if err != nil {
		t.Fatalf("ConvertResponsesToOpenAI: %v", err)
	}
	raw, _ := io.ReadAll(out)
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("bad json: %v\n%s", err, raw)
	}
	msgs, _ := m["messages"].([]any)
	if len(msgs) != 4 {
		t.Fatalf("messages = %v, want 4 (user + assistant[2 calls] + tool + tool)", msgs)
	}

	// 连续两条 function_call 聚合为一条 assistant 消息
	assistant := msgs[1].(map[string]any)
	if assistant["role"] != "assistant" {
		t.Fatalf("messages[1].role = %v, want assistant", assistant["role"])
	}
	calls, _ := assistant["tool_calls"].([]any)
	if len(calls) != 2 {
		t.Fatalf("tool_calls = %v, want 2 aggregated calls", calls)
	}
	first, _ := calls[0].(map[string]any)
	if first["id"] != "call_a" || first["type"] != "function" {
		t.Errorf("tool_calls[0] = %v", first)
	}
	fn, _ := first["function"].(map[string]any)
	if fn["name"] != "shell" || fn["arguments"] != `{"cmd":"ls"}` {
		t.Errorf("tool_calls[0].function = %v", fn)
	}

	// function_call_output → tool 消息，tool_call_id 与聚合条目对应
	for i, wantID := range []string{"call_a", "call_b"} {
		tool := msgs[2+i].(map[string]any)
		if tool["role"] != "tool" || tool["tool_call_id"] != wantID {
			t.Errorf("messages[%d] = %v, want tool with tool_call_id=%s", 2+i, tool, wantID)
		}
	}
}

// TestConvertOpenAIToResponses_TextFormatUnpack chat response_format 转 Responses
// text.format：json_schema 需解包为扁平结构（不能原样塞入嵌套形状）。
func TestConvertOpenAIToResponses_TextFormatUnpack(t *testing.T) {
	t.Run("json_schema", func(t *testing.T) {
		info := &common.RelayInfo{ChannelMeta: &common.ChannelMeta{}}
		body := []byte(`{"model":"gpt-4o","messages":[{"role":"user","content":"hi"}],` +
			`"response_format":{"type":"json_schema","json_schema":{"name":"out","schema":{"type":"object"},"strict":true}}}`)
		out, err := ConvertOpenAIToResponses(body, info)
		if err != nil {
			t.Fatalf("ConvertOpenAIToResponses: %v", err)
		}
		var m map[string]any
		if err := json.Unmarshal(out, &m); err != nil {
			t.Fatalf("bad json: %v\n%s", err, out)
		}
		text, _ := m["text"].(map[string]any)
		format, _ := text["format"].(map[string]any)
		if format["type"] != "json_schema" {
			t.Fatalf("format = %v", format)
		}
		if format["name"] != "out" {
			t.Errorf("format.name = %v, want out", format["name"])
		}
		if _, nested := format["json_schema"]; nested {
			t.Error("format should be flat（json_schema 嵌套必须解包）")
		}
		if s, _ := format["schema"].(map[string]any); s == nil {
			t.Errorf("format.schema = %v", format["schema"])
		}
	})
	t.Run("json_object", func(t *testing.T) {
		info := &common.RelayInfo{ChannelMeta: &common.ChannelMeta{}}
		body := []byte(`{"model":"gpt-4o","messages":[{"role":"user","content":"hi"}],"response_format":{"type":"json_object"}}`)
		out, err := ConvertOpenAIToResponses(body, info)
		if err != nil {
			t.Fatalf("ConvertOpenAIToResponses: %v", err)
		}
		var m map[string]any
		_ = json.Unmarshal(out, &m)
		text, _ := m["text"].(map[string]any)
		format, _ := text["format"].(map[string]any)
		if format["type"] != "json_object" {
			t.Errorf("format = %v", format)
		}
	})
}

// TestConvertOpenAIToResponses_StoreFalse 桥接方向显式 store:false：
// chat 客户端无法经 previous_response_id 引用响应，无需上游存储。
func TestConvertOpenAIToResponses_StoreFalse(t *testing.T) {
	info := &common.RelayInfo{ChannelMeta: &common.ChannelMeta{}}
	body := []byte(`{"model":"gpt-4o","messages":[{"role":"user","content":"hi"}]}`)
	out, err := ConvertOpenAIToResponses(body, info)
	if err != nil {
		t.Fatalf("ConvertOpenAIToResponses: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(out, &m); err != nil {
		t.Fatalf("bad json: %v\n%s", err, out)
	}
	if v, ok := m["store"]; !ok || v != false {
		t.Errorf("store = %v(%T), want explicit false", m["store"], m["store"])
	}
}

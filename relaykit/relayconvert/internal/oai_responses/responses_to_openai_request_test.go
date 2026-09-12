package oai_responses

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/qianfree/team-api/relaykit/dto"
	"github.com/qianfree/team-api/relaykit/relayconvert"
	"github.com/qianfree/team-api/relaykit/relayconvert/convmeta"
)

// convertResponsesBody 解析 Responses 请求 JSON、执行转换并把结果 chat 请求重新序列化为 map
// （对齐宿主 ConvertResponsesToOpenAI 测试的字节级断言口径）。
func convertResponsesBody(t *testing.T, info convmeta.Meta, body string) map[string]any {
	t.Helper()
	var req dto.OpenAIResponsesRequest
	if err := json.Unmarshal([]byte(body), &req); err != nil {
		t.Fatalf("parse responses request: %v", err)
	}
	conv := &ResponsesToOpenAIRequestConverter{}
	out, err := conv.ConvertRequest(context.Background(), info, &req)
	if err != nil {
		t.Fatalf("ConvertRequest: %v", err)
	}
	raw, err := json.Marshal(out)
	if err != nil {
		t.Fatalf("marshal chat request: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("bad json: %v\n%s", err, raw)
	}
	return m
}

func TestResponsesToOpenAIRequest_WrongType(t *testing.T) {
	conv := &ResponsesToOpenAIRequestConverter{}
	if _, err := conv.ConvertRequest(context.Background(), &convmeta.Values{}, "not a request"); err == nil {
		t.Fatal("wrong input type should be rejected")
	}
}

// TestResponsesToOpenAIRequest_PenaltyPassthrough Responses → chat 时 penalty 原样透传。
func TestResponsesToOpenAIRequest_PenaltyPassthrough(t *testing.T) {
	m := convertResponsesBody(t, &convmeta.Values{}, `{"model":"gpt-4o","input":"hi","frequency_penalty":0.5,"presence_penalty":0.2}`)
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

// TestResponsesToOpenAIRequest_PreviousResponseIDRejected 有状态请求（previous_response_id）
// 落在 chat-only 渠道时必须快速失败：降级转换会静默丢失全部会话上下文。
// 错误须携带哨兵以供宿主识别并驱动调度 FSM 换渠道。
func TestResponsesToOpenAIRequest_PreviousResponseIDRejected(t *testing.T) {
	var req dto.OpenAIResponsesRequest
	if err := json.Unmarshal([]byte(`{"model":"gpt-4o","input":"hi","previous_response_id":"resp_prev"}`), &req); err != nil {
		t.Fatalf("parse: %v", err)
	}
	conv := &ResponsesToOpenAIRequestConverter{}
	_, err := conv.ConvertRequest(context.Background(), &convmeta.Values{}, &req)
	if err == nil {
		t.Fatal("previous_response_id should be rejected on chat-only conversion")
	}
	if !errors.Is(err, relayconvert.ErrStatefulResponsesUnsupported) {
		t.Errorf("error should wrap ErrStatefulResponsesUnsupported, got: %v", err)
	}
}

// TestResponsesToOpenAIRequest_TextFormat Responses text.format（扁平结构）转 chat
// response_format（json_schema 嵌套结构）；text 类型不映射。
func TestResponsesToOpenAIRequest_TextFormat(t *testing.T) {
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
			m := convertResponsesBody(t, &convmeta.Values{}, `{"model":"gpt-4o","input":"hi","text":`+c.text+`}`)
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
		m := convertResponsesBody(t, &convmeta.Values{}, `{"model":"gpt-4o","input":"hi","text":{"format":{"type":"text"}}}`)
		if _, ok := m["response_format"]; ok {
			t.Error("text format should not map to response_format")
		}
	})
}

// TestResponsesToOpenAIRequest_StashesResponsesRequest 转换时经 ResponsesStash 能力接口
// stash 请求快照，供上游 chat 响应合成回 Responses 格式时 echo 请求参数。
func TestResponsesToOpenAIRequest_StashesResponsesRequest(t *testing.T) {
	info := newStashMeta("gpt-4o", "", false)
	// 先填充再转换，断言转换覆盖为最新请求体
	info.stashed = &dto.OpenAIResponsesRequest{Model: "stale"}
	var req dto.OpenAIResponsesRequest
	if err := json.Unmarshal([]byte(`{"model":"gpt-4o","input":"hi","temperature":0.7}`), &req); err != nil {
		t.Fatalf("parse: %v", err)
	}
	conv := &ResponsesToOpenAIRequestConverter{}
	if _, err := conv.ConvertRequest(context.Background(), info, &req); err != nil {
		t.Fatalf("ConvertRequest: %v", err)
	}
	if info.stashed == nil {
		t.Fatal("ResponsesRequest should be stashed")
	}
	if info.stashed.Model != "gpt-4o" {
		t.Errorf("stashed model = %q, want gpt-4o", info.stashed.Model)
	}
	if info.stashed.Temperature == nil || *info.stashed.Temperature != 0.7 {
		t.Errorf("stashed temperature = %v, want 0.7", info.stashed.Temperature)
	}
}

// TestResponsesToOpenAIRequest_NoStashCapability info 未实现 ResponsesStash 时静默跳过。
func TestResponsesToOpenAIRequest_NoStashCapability(t *testing.T) {
	var req dto.OpenAIResponsesRequest
	if err := json.Unmarshal([]byte(`{"model":"gpt-4o","input":"hi"}`), &req); err != nil {
		t.Fatalf("parse: %v", err)
	}
	conv := &ResponsesToOpenAIRequestConverter{}
	if _, err := conv.ConvertRequest(context.Background(), &convmeta.Values{}, &req); err != nil {
		t.Fatalf("ConvertRequest without stash capability: %v", err)
	}
}

// TestResponsesToOpenAIRequest_InputAudioAndFile 多模态输入透传：
// input_audio 与 Responses 同形透传；input_file 扁平结构转 chat 的 file 嵌套结构。
func TestResponsesToOpenAIRequest_InputAudioAndFile(t *testing.T) {
	m := convertResponsesBody(t, &convmeta.Values{}, `{"model":"gpt-4o","input":[{"type":"message","role":"user","content":[`+
		`{"type":"input_text","text":"listen"},`+
		`{"type":"input_audio","input_audio":{"data":"QUJD","format":"wav"}},`+
		`{"type":"input_file","file_data":"data:text/plain;base64,aGk=","filename":"a.txt"}`+
		`]}]}`)
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

// TestResponsesToOpenAIRequest_DeveloperRoleMappedToSystem codex 等客户端会在 input 中
// 发送 developer 角色消息（新式系统提示）；第三方 chat 上游（serde 严格校验）不识别该角色，
// 必须映射为 system，否则上游直接拒绝整个请求体。
func TestResponsesToOpenAIRequest_DeveloperRoleMappedToSystem(t *testing.T) {
	m := convertResponsesBody(t, &convmeta.Values{}, `{"model":"deepseek-v4-flash","instructions":"You are helpful.",`+
		`"input":[{"type":"message","role":"developer","content":[{"type":"input_text","text":"You are a coding agent."}]},`+
		`{"type":"message","role":"user","content":[{"type":"input_text","text":"hi"}]}],"stream":true}`)
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
	// stream:true 必须注入 stream_options.include_usage 保证流末 usage 可计费
	if m["stream"] != true {
		t.Errorf("stream = %v, want true", m["stream"])
	}
	so, _ := m["stream_options"].(map[string]any)
	if so == nil || so["include_usage"] != true {
		t.Errorf("stream_options = %v, want include_usage=true", m["stream_options"])
	}
}

// TestResponsesToOpenAIRequest_FunctionCallHistory codex 多轮会把历史 function_call /
// function_call_output / reasoning 项放进 input：function_call 需聚合为 assistant.tool_calls
// （其后的 tool 消息才有对应的 tool_call_id），reasoning 项直接跳过。
func TestResponsesToOpenAIRequest_FunctionCallHistory(t *testing.T) {
	m := convertResponsesBody(t, &convmeta.Values{}, `{"model":"deepseek-v4-flash","input":[`+
		`{"type":"message","role":"user","content":[{"type":"input_text","text":"list files"}]},`+
		`{"type":"reasoning","summary":[]},`+
		`{"type":"function_call","call_id":"call_a","name":"shell","arguments":"{\"cmd\":\"ls\"}"},`+
		`{"type":"function_call","call_id":"call_b","name":"read","arguments":"{\"path\":\"a.go\"}"},`+
		`{"type":"function_call_output","call_id":"call_a","output":"a.go b.go"},`+
		`{"type":"function_call_output","call_id":"call_b","output":"package main"}`+
		`]}`)
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

// TestResponsesToOpenAIRequest_ModelMapping 已附加渠道信息时使用映射后的上游模型名。
func TestResponsesToOpenAIRequest_ModelMapping(t *testing.T) {
	info := &convmeta.Values{OriginModelName: "gpt-4o", UpstreamModelName: "gpt-4o-upstream", ChannelMetaAttached: true}
	m := convertResponsesBody(t, info, `{"model":"gpt-4o","input":"hi"}`)
	if m["model"] != "gpt-4o-upstream" {
		t.Errorf("model = %v, want gpt-4o-upstream", m["model"])
	}
	// 无渠道信息：用客户端请求模型名
	m = convertResponsesBody(t, &convmeta.Values{}, `{"model":"gpt-4o","input":"hi"}`)
	if m["model"] != "gpt-4o" {
		t.Errorf("model = %v, want gpt-4o", m["model"])
	}
}

// TestResponsesToOpenAIRequest_ParamMapping 常规参数映射：温度/最大 token/logprobs/工具/工具选择。
func TestResponsesToOpenAIRequest_ParamMapping(t *testing.T) {
	m := convertResponsesBody(t, &convmeta.Values{}, `{"model":"gpt-4o","input":"hi",`+
		`"temperature":0.4,"top_p":0.8,"max_output_tokens":256,"logprobs":3,`+
		`"reasoning":{"effort":"high"},"service_tier":"flex","prompt_cache_key":"pk",`+
		`"tools":[{"type":"function","name":"get_weather","description":"d","parameters":{"type":"object"}},{"type":"web_search"}],`+
		`"tool_choice":{"type":"function","function":{"name":"get_weather"}},"metadata":{"k":"v"}}`)
	if m["temperature"] != 0.4 || m["top_p"] != 0.8 {
		t.Errorf("temperature/top_p = %v/%v", m["temperature"], m["top_p"])
	}
	if m["max_tokens"] != float64(256) {
		t.Errorf("max_tokens = %v, want 256", m["max_tokens"])
	}
	if m["logprobs"] != true || m["top_logprobs"] != float64(3) {
		t.Errorf("logprobs = %v, top_logprobs = %v", m["logprobs"], m["top_logprobs"])
	}
	if m["reasoning_effort"] != "high" {
		t.Errorf("reasoning_effort = %v, want high", m["reasoning_effort"])
	}
	if m["service_tier"] != "flex" || m["prompt_cache_key"] != "pk" {
		t.Errorf("service_tier/prompt_cache_key = %v/%v", m["service_tier"], m["prompt_cache_key"])
	}
	tools, _ := m["tools"].([]any)
	if len(tools) != 1 {
		t.Fatalf("tools = %v, want 1 (仅 function 类型保留)", tools)
	}
	fn, _ := tools[0].(map[string]any)["function"].(map[string]any)
	if fn["name"] != "get_weather" {
		t.Errorf("tools[0].function = %v", fn)
	}
	tc, _ := m["tool_choice"].(map[string]any)
	tcFn, _ := tc["function"].(map[string]any)
	if tc["type"] != "function" || tcFn["name"] != "get_weather" {
		t.Errorf("tool_choice = %v", tc)
	}
	md, _ := m["metadata"].(map[string]any)
	if md["k"] != "v" {
		t.Errorf("metadata = %v", m["metadata"])
	}
}

// TestResponsesToOpenAIRequest_ReasoningPassedBackAsReasoningContent 思考内容回传：
// DeepSeek 等推理上游的 thinking 模式要求多轮历史携带 reasoning_content，
// 丢弃 reasoning 项会导致上游 400（线上真实报错：
// "The `reasoning_content` in the thinking mode must be passed back to the API."）。
// 覆盖 summary（OpenAI 官方形态）与 content（部分聚合器直连形态）两种携带方式，
// 以及挂到文本消息 / 聚合到 tool_calls 消息两条路径。
func TestResponsesToOpenAIRequest_ReasoningPassedBackAsReasoningContent(t *testing.T) {
	t.Run("summary 形态挂到文本消息", func(t *testing.T) {
		m := convertResponsesBody(t, &convmeta.Values{}, `{"model":"deepseek-v4-pro","input":[`+
			`{"type":"message","role":"user","content":[{"type":"input_text","text":"hi"}]},`+
			`{"type":"reasoning","summary":[{"type":"summary_text","text":"思考A"}]},`+
			`{"type":"message","role":"assistant","content":[{"type":"output_text","text":"答案"}]},`+
			`{"type":"message","role":"user","content":[{"type":"input_text","text":"and?"}]}`+
			`]}`)
		msgs, _ := m["messages"].([]any)
		if len(msgs) != 3 {
			t.Fatalf("messages = %v, want 3", msgs)
		}
		assistant := msgs[1].(map[string]any)
		if assistant["reasoning_content"] != "思考A" {
			t.Errorf("assistant.reasoning_content = %v, want 思考A", assistant["reasoning_content"])
		}
		// user 消息不受影响
		if _, ok := msgs[0].(map[string]any)["reasoning_content"]; ok {
			t.Error("user 消息不应携带 reasoning_content")
		}
	})

	t.Run("content 形态挂到 tool_calls 聚合消息", func(t *testing.T) {
		m := convertResponsesBody(t, &convmeta.Values{}, `{"model":"deepseek-v4-pro","input":[`+
			`{"type":"message","role":"user","content":[{"type":"input_text","text":"ls"}]},`+
			`{"type":"reasoning","content":[{"type":"reasoning_text","text":"先用shell"}]},`+
			`{"type":"function_call","call_id":"call_a","name":"shell","arguments":"{\"cmd\":\"ls\"}"},`+
			`{"type":"function_call_output","call_id":"call_a","output":"a.go"}`+
			`]}`)
		msgs, _ := m["messages"].([]any)
		if len(msgs) != 3 {
			t.Fatalf("messages = %v, want 3 (user + assistant[tool_calls] + tool)", msgs)
		}
		assistant := msgs[1].(map[string]any)
		if assistant["reasoning_content"] != "先用shell" {
			t.Errorf("assistant.reasoning_content = %v, want 先用shell", assistant["reasoning_content"])
		}
		if _, ok := assistant["tool_calls"]; !ok {
			t.Error("reasoning 挂载不应破坏 tool_calls 聚合")
		}
	})

	t.Run("加密无明文不产出字段", func(t *testing.T) {
		// OpenAI 官方加密 reasoning（仅 encrypted_content）无可读文本，
		// 不应产出空 reasoning_content（部分严格 serde 上游会拒绝空值字段）
		m := convertResponsesBody(t, &convmeta.Values{}, `{"model":"gpt-5.6","input":[`+
			`{"type":"message","role":"user","content":[{"type":"input_text","text":"hi"}]},`+
			`{"type":"reasoning","encrypted_content":"gAAAAA"},`+
			`{"type":"message","role":"assistant","content":[{"type":"output_text","text":"ok"}]}`+
			`]}`)
		msgs, _ := m["messages"].([]any)
		assistant := msgs[1].(map[string]any)
		if _, ok := assistant["reasoning_content"]; ok {
			t.Errorf("无可读思考文本时不应携带 reasoning_content: %v", assistant)
		}
	})
}

// TestResponsesToOpenAIRequest_WebSearchOptionsAlwaysEmitted web_search_options 在本层
// 必须无条件产出：它同时是 Responses→OpenAI→Claude 链的**中间载体**（第二跳据此还原
// web_search 工具），按模型名裁剪会掐断 claude 原生渠道的搜索能力。
// 「chat 终端 + claude 系模型」的剥离在 openai 适配器出口做（聚合器场景），
// 见 relay/channel/openai/adaptor.go 的 stripWebSearchOptionsForClaudeModels。
func TestResponsesToOpenAIRequest_WebSearchOptionsAlwaysEmitted(t *testing.T) {
	for _, model := range []string{"claude-sonnet-5", "gpt-4o-search-preview"} {
		m := convertResponsesBody(t, &convmeta.Values{}, `{"model":"`+model+`","input":"hi","tools":[{"type":"web_search"}]}`)
		if _, ok := m["web_search_options"]; !ok {
			t.Errorf("%s: web_search_options 应作为链路中间载体无条件产出", model)
		}
	}
}

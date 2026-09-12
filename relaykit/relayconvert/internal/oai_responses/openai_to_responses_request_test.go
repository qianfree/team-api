package oai_responses

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/qianfree/team-api/relaykit/dto"
	"github.com/qianfree/team-api/relaykit/relayconvert/convmeta"
)

// convertChatBody 解析 chat 请求 JSON、执行转换并把结果 Responses 请求重新序列化为 map。
func convertChatBody(t *testing.T, info convmeta.Meta, body string) map[string]any {
	t.Helper()
	var req dto.GeneralOpenAIRequest
	if err := json.Unmarshal([]byte(body), &req); err != nil {
		t.Fatalf("parse chat request: %v", err)
	}
	conv := &OpenAIToResponsesRequestConverter{}
	out, err := conv.ConvertRequest(context.Background(), info, &req)
	if err != nil {
		t.Fatalf("ConvertRequest: %v", err)
	}
	raw, err := json.Marshal(out)
	if err != nil {
		t.Fatalf("marshal responses request: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("bad json: %v\n%s", err, raw)
	}
	return m
}

func TestOpenAIToResponsesRequest_WrongType(t *testing.T) {
	conv := &OpenAIToResponsesRequestConverter{}
	if _, err := conv.ConvertRequest(context.Background(), &convmeta.Values{}, 42); err == nil {
		t.Fatal("wrong input type should be rejected")
	}
}

// TestOpenAIToResponsesRequest_PenaltyDropped chat 入站转 Responses 出站时
// 丢弃官方不支持的 presence/frequency penalty（透传会被严格上游拒绝），
// 保留官方参数 prompt_cache_key。
func TestOpenAIToResponsesRequest_PenaltyDropped(t *testing.T) {
	m := convertChatBody(t, &convmeta.Values{}, `{"model":"gpt-4o","messages":[{"role":"user","content":"hi"}],"frequency_penalty":0.5,"presence_penalty":0.2,"prompt_cache_key":"abc"}`)
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

// TestOpenAIToResponsesRequest_TextFormatUnpack chat response_format 转 Responses
// text.format：json_schema 需解包为扁平结构（不能原样塞入嵌套形状）。
func TestOpenAIToResponsesRequest_TextFormatUnpack(t *testing.T) {
	t.Run("json_schema", func(t *testing.T) {
		m := convertChatBody(t, &convmeta.Values{}, `{"model":"gpt-4o","messages":[{"role":"user","content":"hi"}],`+
			`"response_format":{"type":"json_schema","json_schema":{"name":"out","schema":{"type":"object"},"strict":true}}}`)
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
		m := convertChatBody(t, &convmeta.Values{}, `{"model":"gpt-4o","messages":[{"role":"user","content":"hi"}],"response_format":{"type":"json_object"}}`)
		text, _ := m["text"].(map[string]any)
		format, _ := text["format"].(map[string]any)
		if format["type"] != "json_object" {
			t.Errorf("format = %v", format)
		}
	})
}

// TestOpenAIToResponsesRequest_StoreFalse 桥接方向显式 store:false：
// chat 客户端无法经 previous_response_id 引用响应，无需上游存储。
func TestOpenAIToResponsesRequest_StoreFalse(t *testing.T) {
	m := convertChatBody(t, &convmeta.Values{}, `{"model":"gpt-4o","messages":[{"role":"user","content":"hi"}]}`)
	if v, ok := m["store"]; !ok || v != false {
		t.Errorf("store = %v(%T), want explicit false", m["store"], m["store"])
	}
}

// TestOpenAIToResponsesRequest_InstructionsAndInput system/developer 消息合并为 instructions
// （\n\n 连接），其余消息转为 input 数组（user=input_text / assistant=output_text）。
func TestOpenAIToResponsesRequest_InstructionsAndInput(t *testing.T) {
	m := convertChatBody(t, &convmeta.Values{}, `{"model":"gpt-4o","messages":[`+
		`{"role":"system","content":"sys one"},`+
		`{"role":"developer","content":"dev two"},`+
		`{"role":"user","content":"hi"},`+
		`{"role":"assistant","content":"hello"}]}`)
	if m["instructions"] != "sys one\n\ndev two" {
		t.Errorf("instructions = %v", m["instructions"])
	}
	input, _ := m["input"].([]any)
	if len(input) != 2 {
		t.Fatalf("input = %v, want 2 items", input)
	}
	userItem, _ := input[0].(map[string]any)
	if userItem["type"] != "message" || userItem["role"] != "user" {
		t.Errorf("input[0] = %v", userItem)
	}
	userParts, _ := userItem["content"].([]any)
	up0, _ := userParts[0].(map[string]any)
	if up0["type"] != "input_text" || up0["text"] != "hi" {
		t.Errorf("input[0].content = %v", userParts)
	}
	asstItem, _ := input[1].(map[string]any)
	asstParts, _ := asstItem["content"].([]any)
	ap0, _ := asstParts[0].(map[string]any)
	if ap0["type"] != "output_text" || ap0["text"] != "hello" {
		t.Errorf("input[1].content = %v", asstParts)
	}
}

// TestOpenAIToResponsesRequest_ToolCallHistory assistant.tool_calls → function_call 项，
// tool 消息 → function_call_output 项。
func TestOpenAIToResponsesRequest_ToolCallHistory(t *testing.T) {
	m := convertChatBody(t, &convmeta.Values{}, `{"model":"gpt-4o","messages":[`+
		`{"role":"user","content":"list files"},`+
		`{"role":"assistant","content":null,"tool_calls":[{"id":"call_a","type":"function","function":{"name":"shell","arguments":"{\"cmd\":\"ls\"}"}}]},`+
		`{"role":"tool","tool_call_id":"call_a","content":"a.go b.go"}]}`)
	input, _ := m["input"].([]any)
	if len(input) != 3 {
		t.Fatalf("input = %v, want 3 items (user message + function_call + function_call_output)", input)
	}
	fc, _ := input[1].(map[string]any)
	if fc["type"] != "function_call" || fc["call_id"] != "call_a" || fc["name"] != "shell" || fc["arguments"] != `{"cmd":"ls"}` {
		t.Errorf("input[1] = %v", fc)
	}
	fco, _ := input[2].(map[string]any)
	if fco["type"] != "function_call_output" || fco["call_id"] != "call_a" || fco["output"] != "a.go b.go" {
		t.Errorf("input[2] = %v", fco)
	}
}

// TestOpenAIToResponsesRequest_ParamMapping 常规参数映射：
// max_tokens/max_completion_tokens 取较大者、reasoning_effort、tools、tool_choice 解包、
// user、parallel_tool_calls、stream。
func TestOpenAIToResponsesRequest_ParamMapping(t *testing.T) {
	m := convertChatBody(t, &convmeta.Values{}, `{"model":"gpt-4o","messages":[{"role":"user","content":"hi"}],`+
		`"stream":true,"temperature":0.4,"top_p":0.8,"max_tokens":100,"max_completion_tokens":300,`+
		`"reasoning_effort":"high","user":"u1","parallel_tool_calls":false,`+
		`"tools":[{"type":"function","function":{"name":"get_weather","description":"d","parameters":{"type":"object"}}}],`+
		`"tool_choice":{"type":"function","function":{"name":"get_weather"}}}`)
	if m["stream"] != true {
		t.Errorf("stream = %v", m["stream"])
	}
	if m["temperature"] != 0.4 || m["top_p"] != 0.8 {
		t.Errorf("temperature/top_p = %v/%v", m["temperature"], m["top_p"])
	}
	if m["max_output_tokens"] != float64(300) {
		t.Errorf("max_output_tokens = %v, want 300 (取 max_tokens 与 max_completion_tokens 较大者)", m["max_output_tokens"])
	}
	reasoning, _ := m["reasoning"].(map[string]any)
	if reasoning["effort"] != "high" || reasoning["summary"] != "detailed" {
		t.Errorf("reasoning = %v", reasoning)
	}
	if m["user"] != "u1" {
		t.Errorf("user = %v", m["user"])
	}
	if m["parallel_tool_calls"] != false {
		t.Errorf("parallel_tool_calls = %v, want false", m["parallel_tool_calls"])
	}
	tools, _ := m["tools"].([]any)
	if len(tools) != 1 {
		t.Fatalf("tools = %v", tools)
	}
	tool0, _ := tools[0].(map[string]any)
	if tool0["type"] != "function" || tool0["name"] != "get_weather" {
		t.Errorf("tools[0] = %v (Responses tools 为扁平结构)", tool0)
	}
	tc, _ := m["tool_choice"].(map[string]any)
	if tc["type"] != "function" || tc["name"] != "get_weather" {
		t.Errorf("tool_choice = %v (Responses tool_choice 为扁平结构)", tc)
	}
}

// TestOpenAIToResponsesRequest_ModelMapping 已附加渠道信息时使用映射后的上游模型名。
func TestOpenAIToResponsesRequest_ModelMapping(t *testing.T) {
	info := &convmeta.Values{OriginModelName: "gpt-4o", UpstreamModelName: "gpt-4o-upstream", ChannelMetaAttached: true}
	m := convertChatBody(t, info, `{"model":"gpt-4o","messages":[{"role":"user","content":"hi"}]}`)
	if m["model"] != "gpt-4o-upstream" {
		t.Errorf("model = %v, want gpt-4o-upstream", m["model"])
	}
}

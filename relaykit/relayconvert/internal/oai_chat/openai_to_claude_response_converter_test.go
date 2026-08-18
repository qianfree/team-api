package oai_chat

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/qianfree/team-api/relaykit/dto"
	"github.com/qianfree/team-api/relaykit/relayconvert/convmeta"
)

// mappedMeta 实现 ModelNameMapped/GetRequestID 能力接口的测试桩。
type mappedMeta struct {
	convmeta.Values
	mapped    bool
	requestID string
}

func (m *mappedMeta) ModelNameMapped() bool { return m.mapped }
func (m *mappedMeta) GetRequestID() string  { return m.requestID }

// 混合内容（thinking/text/tool_use）+ usage 扣减 + 映射渠道模型名。
func TestOpenAIToClaudeResponse_Mixed(t *testing.T) {
	resp := parseChatResp(t, `{
		"id":"c1","model":"gpt-4o-2024","choices":[{
			"index":0,
			"message":{"role":"assistant","content":"答案","reasoning_content":"思考",
				"tool_calls":[{"id":"t1","type":"function","function":{"name":"f","arguments":"{\"a\":1}"}}]},
			"finish_reason":"tool_calls"}],
		"usage":{"prompt_tokens":100,"completion_tokens":20,"total_tokens":120,
			"prompt_tokens_details":{"cached_tokens":30,"cache_write_tokens":0,"audio_tokens":0}}
	}`)
	info := &mappedMeta{Values: convmeta.Values{OriginModelName: "gpt-4o"}, mapped: true, requestID: "req-1"}

	result, _, err := (&OpenAIToClaudeResponseConverter{}).ConvertResponse(context.Background(), info, resp)
	if err != nil {
		t.Fatalf("convert: %v", err)
	}
	claude := result.(*dto.ClaudeResponse)

	if claude.ID != "msg_req-1" {
		t.Errorf("ID = %q, want msg_req-1", claude.ID)
	}
	if claude.Model != "gpt-4o" {
		t.Errorf("Model = %q, want gpt-4o（映射渠道用 OriginModelName）", claude.Model)
	}
	if claude.StopReason != "tool_use" {
		t.Errorf("StopReason = %q", claude.StopReason)
	}
	// 块序 thinking → text → tool_use
	if len(claude.Content) != 3 || claude.Content[0].Type != "thinking" ||
		claude.Content[1].Type != "text" || claude.Content[2].Type != "tool_use" {
		t.Errorf("content block order wrong: %+v", claude.Content)
	}
	if claude.Usage.InputTokens != 70 || claude.Usage.CacheReadInputTokens != 30 || claude.Usage.OutputTokens != 20 {
		t.Errorf("usage = %+v, want input=70(100-30) cache_read=30 output=20", claude.Usage)
	}
}

// 空 Choices 仍产完整骨架（空 text 块 + end_turn）；nil details 安全。
func TestOpenAIToClaudeResponse_EmptyChoices(t *testing.T) {
	resp := parseChatResp(t, `{"id":"c2","model":"m","choices":[],"usage":{"prompt_tokens":5,"completion_tokens":0,"total_tokens":5}}`)
	info := &mappedMeta{Values: convmeta.Values{OriginModelName: "o"}, requestID: "r2"}

	result, _, err := (&OpenAIToClaudeResponseConverter{}).ConvertResponse(context.Background(), info, resp)
	if err != nil {
		t.Fatalf("convert: %v", err)
	}
	claude := result.(*dto.ClaudeResponse)
	if claude.StopReason != "end_turn" {
		t.Errorf("StopReason = %q, want end_turn（空 choices 默认）", claude.StopReason)
	}
	if len(claude.Content) != 1 || claude.Content[0].Type != "text" {
		t.Errorf("应补空 text 块: %+v", claude.Content)
	}
}

// finish_reason legacy 语义：空串→end_turn；未知值原样透传（不 ToLower）。
func TestOpenAIToClaudeResponse_FinishReasonSemantics(t *testing.T) {
	cases := map[string]string{"": "end_turn", "STOP": "STOP", "function_call": "function_call", "content_filter": "refusal"}
	for in, want := range cases {
		resp := parseChatResp(t, `{"id":"c","model":"m","choices":[{"index":0,"message":{"role":"assistant","content":"x"},"finish_reason":"`+in+`"}],"usage":{}}`)
		result, _, _ := (&OpenAIToClaudeResponseConverter{}).ConvertResponse(context.Background(), nil, resp)
		if got := result.(*dto.ClaudeResponse).StopReason; got != want {
			t.Errorf("finish_reason %q → %q, want %q", in, got, want)
		}
	}
}

func parseChatResp(t *testing.T, raw string) *dto.ChatCompletionResponse {
	t.Helper()
	resp := &dto.ChatCompletionResponse{}
	if err := json.Unmarshal([]byte(raw), resp); err != nil {
		t.Fatalf("parse: %v", err)
	}
	return resp
}

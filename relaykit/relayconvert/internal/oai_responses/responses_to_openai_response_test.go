package oai_responses

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/qianfree/team-api/relaykit/dto"
	"github.com/qianfree/team-api/relaykit/relayconvert/convmeta"
)

func convertResponsesResponse(t *testing.T, info convmeta.Meta, body string) *dto.ChatCompletionResponse {
	t.Helper()
	var resp dto.OpenAIResponsesResponse
	if err := json.Unmarshal([]byte(body), &resp); err != nil {
		t.Fatalf("parse responses response: %v", err)
	}
	conv := &ResponsesToOpenAIResponseConverter{}
	out, err := conv.ConvertResponse(context.Background(), info, &resp)
	if err != nil {
		t.Fatalf("ConvertResponse: %v", err)
	}
	chatResp, ok := out.(*dto.ChatCompletionResponse)
	if !ok {
		t.Fatalf("expected *dto.ChatCompletionResponse, got %T", out)
	}
	return chatResp
}

func TestResponsesToOpenAIResponse_WrongType(t *testing.T) {
	conv := &ResponsesToOpenAIResponseConverter{}
	if _, err := conv.ConvertResponse(context.Background(), &convmeta.Values{}, "nope"); err == nil {
		t.Fatal("wrong input type should be rejected")
	}
}

// TestResponsesToOpenAIResponse_TextAndToolCalls output 中的 message 文本与 function_call
// 同时存在时：文本进 content，工具调用进 tool_calls，finish_reason 为 stop（有文本）。
func TestResponsesToOpenAIResponse_TextAndToolCalls(t *testing.T) {
	chatResp := convertResponsesResponse(t, &convmeta.Values{OriginModelName: "gpt-4o"}, `{
		"id":"resp_1","object":"response","created_at":100,"status":"completed","model":"gpt-4o-2024",
		"output":[
			{"type":"message","id":"msg_1","role":"assistant","content":[{"type":"output_text","text":"Hello "},{"type":"output_text","text":"world"}]},
			{"type":"function_call","id":"fc_1","call_id":"call_1","name":"get_weather","arguments":"{\"city\":\"sh\"}"}
		],
		"usage":{"input_tokens":10,"output_tokens":5,"total_tokens":15,
			"input_tokens_details":{"cached_tokens":2,"cache_write_tokens":1,"audio_tokens":0,"text_tokens":8},
			"output_tokens_details":{"reasoning_tokens":3,"accepted_prediction_tokens":0,"rejected_prediction_tokens":0}}
	}`)

	if chatResp.Object != "chat.completion" {
		t.Errorf("object = %q", chatResp.Object)
	}
	if len(chatResp.Choices) != 1 {
		t.Fatalf("choices = %v", chatResp.Choices)
	}
	choice := chatResp.Choices[0]
	if choice.FinishReason != "stop" {
		t.Errorf("finish_reason = %q, want stop（有文本时不是 tool_calls）", choice.FinishReason)
	}
	if choice.Message.Content != "Hello world" {
		t.Errorf("content = %v, want 拼接文本", choice.Message.Content)
	}
	if len(choice.Message.ToolCalls) != 1 {
		t.Fatalf("tool_calls = %v", choice.Message.ToolCalls)
	}
	tc := choice.Message.ToolCalls[0]
	if tc.ID != "call_1" || tc.Type != "function" || tc.Function.Name != "get_weather" || tc.Function.Arguments != `{"city":"sh"}` {
		t.Errorf("tool_call = %+v", tc)
	}
	// usage 与明细映射
	if chatResp.Usage.PromptTokens != 10 || chatResp.Usage.CompletionTokens != 5 || chatResp.Usage.TotalTokens != 15 {
		t.Errorf("usage = %+v", chatResp.Usage)
	}
	if d := chatResp.Usage.PromptTokensDetails; d == nil || d.CachedTokens != 2 || d.CacheWriteTokens != 1 || d.TextTokens != 8 {
		t.Errorf("prompt details = %+v", chatResp.Usage.PromptTokensDetails)
	}
	if d := chatResp.Usage.CompletionTokenDetails; d == nil || d.ReasoningTokens != 3 {
		t.Errorf("completion details = %+v", chatResp.Usage.CompletionTokenDetails)
	}
	// 未映射渠道：模型名取上游响应携带的模型名
	if chatResp.Model != "gpt-4o-2024" {
		t.Errorf("model = %q, want gpt-4o-2024", chatResp.Model)
	}
}

// TestResponsesToOpenAIResponse_ToolCallsOnly 仅工具调用无文本时 finish_reason 为 tool_calls，
// content 为空字符串。
func TestResponsesToOpenAIResponse_ToolCallsOnly(t *testing.T) {
	chatResp := convertResponsesResponse(t, &convmeta.Values{OriginModelName: "gpt-4o"}, `{
		"id":"resp_1","object":"response","created_at":100,"status":"completed","model":"gpt-4o",
		"output":[{"type":"function_call","id":"fc_1","call_id":"call_1","name":"f","arguments":"{}"}],
		"usage":{"input_tokens":4,"output_tokens":2,"total_tokens":0}
	}`)
	choice := chatResp.Choices[0]
	if choice.FinishReason != "tool_calls" {
		t.Errorf("finish_reason = %q, want tool_calls", choice.FinishReason)
	}
	if choice.Message.Content != "" {
		t.Errorf("content = %v, want empty", choice.Message.Content)
	}
	// total_tokens 缺失时补齐为 prompt+completion
	if chatResp.Usage.TotalTokens != 6 {
		t.Errorf("total_tokens = %d, want 6", chatResp.Usage.TotalTokens)
	}
}

// TestResponsesToOpenAIResponse_UsageEstimated 上游未返回 usage 时按文本长度估算（4 字符/token）。
func TestResponsesToOpenAIResponse_UsageEstimated(t *testing.T) {
	chatResp := convertResponsesResponse(t, &convmeta.Values{OriginModelName: "gpt-4o"}, `{
		"id":"resp_1","object":"response","created_at":100,"status":"completed","model":"gpt-4o",
		"output":[{"type":"message","id":"msg_1","role":"assistant","content":[{"type":"output_text","text":"12345678"}]}]
	}`)
	if chatResp.Usage.CompletionTokens != 2 {
		t.Errorf("completion_tokens = %d, want 2 (8 字符 / 4)", chatResp.Usage.CompletionTokens)
	}
	if chatResp.Usage.TotalTokens != 2 {
		t.Errorf("total_tokens = %d, want 2", chatResp.Usage.TotalTokens)
	}
}

// TestResponsesToOpenAIResponse_ModelMapped 模型映射渠道回写客户端请求的模型名。
func TestResponsesToOpenAIResponse_ModelMapped(t *testing.T) {
	info := &convmeta.Values{OriginModelName: "my-gpt", UpstreamModelName: "gpt-4o-upstream", ChannelMetaAttached: true}
	chatResp := convertResponsesResponse(t, info, `{
		"id":"resp_1","object":"response","created_at":100,"status":"completed","model":"gpt-4o-upstream",
		"output":[{"type":"message","id":"msg_1","role":"assistant","content":[{"type":"output_text","text":"hi"}]}]
	}`)
	if chatResp.Model != "my-gpt" {
		t.Errorf("model = %q, want my-gpt（映射渠道回写原始模型名）", chatResp.Model)
	}
}

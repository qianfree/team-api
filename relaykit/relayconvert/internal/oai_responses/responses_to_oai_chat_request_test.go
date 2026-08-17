package oai_responses

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/qianfree/team-api/relaykit/dto"
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

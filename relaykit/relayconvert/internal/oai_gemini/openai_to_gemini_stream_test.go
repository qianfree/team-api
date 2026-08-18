package oai_gemini

import (
	"context"
	"strings"
	"testing"

	"github.com/qianfree/team-api/relaykit/dto"
)

func runO2GStream(t *testing.T, sse string) ([]*dto.GeminiChatResponse, error) {
	t.Helper()
	var chunks []*dto.GeminiChatResponse
	err := (&OpenAIToGeminiStreamConverter{}).ConvertStreamResponse(
		context.Background(), nil, strings.NewReader(sse), func(chunk any) error {
			if g, ok := chunk.(*dto.GeminiChatResponse); ok {
				chunks = append(chunks, g)
			}
			return nil
		})
	return chunks, err
}

// 修复项验证：分片 arguments 聚合为完整 functionCall part（tail chunk 携带，非碎片垃圾）。
func TestO2GStream_ArgsAggregation(t *testing.T) {
	sse := "data: {\"id\":\"c\",\"choices\":[{\"index\":0,\"delta\":{\"role\":\"assistant\"}}]}\n\n" +
		"data: {\"id\":\"c\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"回答\"}}]}\n\n" +
		"data: " + `{"id":"c","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"id":"t1","type":"function","function":{"name":"f","arguments":"{\"ci"}}]}}]}` + "\n\n" +
		"data: " + `{"id":"c","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"arguments":"ty\":\"北京\"}"}}]}}]}` + "\n\n" +
		"data: {\"id\":\"c\",\"choices\":[{\"index\":0,\"delta\":{},\"finish_reason\":\"tool_calls\"}]}\n\n" +
		"data: {\"id\":\"c\",\"choices\":[],\"usage\":{\"prompt_tokens\":10,\"completion_tokens\":8,\"total_tokens\":18,\"completion_tokens_details\":{\"reasoning_tokens\":3}}}\n\n" +
		"data: [DONE]\n\n"
	chunks, err := runO2GStream(t, sse)
	if err != nil {
		t.Fatalf("stream: %v", err)
	}
	if len(chunks) < 2 {
		t.Fatalf("chunks = %d, want ≥2（文本 chunk + 尾 chunk）", len(chunks))
	}
	// 首个内容 chunk：文本 part
	first := chunks[0]
	if len(first.Candidates) != 1 || len(first.Candidates[0].Content.Parts) != 1 ||
		first.Candidates[0].Content.Parts[0].Text != "回答" {
		t.Errorf("first chunk parts = %+v, want text 回答", first.Candidates)
	}
	// 尾 chunk：聚合的完整 functionCall part（args={"city":"北京"}）+ finishReason + usage 扣减
	tail := chunks[len(chunks)-1]
	if tail.Candidates[0].FinishReason != "STOP" {
		t.Errorf("tail finishReason = %q, want STOP（tool_calls→STOP）", tail.Candidates[0].FinishReason)
	}
	parts := tail.Candidates[0].Content.Parts
	if len(parts) != 1 || parts[0].FunctionCall == nil || parts[0].FunctionCall.FunctionName != "f" {
		t.Fatalf("tail parts = %+v, want 单个完整 functionCall f", parts)
	}
	args, _ := parts[0].FunctionCall.Arguments.(map[string]any)
	if args["city"] != "北京" {
		t.Errorf("aggregated args = %+v, want city=北京（分片聚合修复项）", parts[0].FunctionCall.Arguments)
	}
	um := tail.UsageMetadata
	if um == nil || um.CandidatesTokenCount != 5 || um.ThoughtsTokenCount != 3 {
		t.Errorf("usageMetadata = %+v, want candidates=5(8-3) thoughts=3", um)
	}
}

// 空流：只发尾 chunk（无 parts、无 finish、无 usage——跳过条件不产 candidate）。
func TestO2GStream_EmptyStream(t *testing.T) {
	chunks, err := runO2GStream(t, "data: [DONE]\n\n")
	if err != nil {
		t.Fatalf("stream: %v", err)
	}
	if len(chunks) != 1 {
		t.Fatalf("chunks = %d, want 1（仅尾 chunk）", len(chunks))
	}
	tail := chunks[0]
	if len(tail.Candidates) != 1 || tail.UsageMetadata != nil {
		t.Errorf("空流尾 chunk = %+v, want 无 usage 的空尾", tail)
	}
}

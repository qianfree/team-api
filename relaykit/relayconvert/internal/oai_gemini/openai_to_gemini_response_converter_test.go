package oai_gemini

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/qianfree/team-api/relaykit/dto"
)

func parseChatRespGemini(t *testing.T, raw string) *dto.ChatCompletionResponse {
	t.Helper()
	resp := &dto.ChatCompletionResponse{}
	if err := jsonUnmarshalHelper([]byte(raw), resp); err != nil {
		t.Fatalf("parse: %v", err)
	}
	return resp
}

// thinking part + text + functionCall；usage 扣减 reasoning；tool_calls→STOP。
func TestOpenAIToGeminiResponse_Mixed(t *testing.T) {
	resp := parseChatRespGemini(t, `{
		"id":"c1","model":"gpt-4o","choices":[{
			"index":0,
			"message":{"role":"assistant","content":"答案","reasoning_content":"思考",
				"tool_calls":[{"id":"t1","type":"function","function":{"name":"f","arguments":"{\"a\":1}"}}]},
			"finish_reason":"tool_calls"}],
		"usage":{"prompt_tokens":50,"completion_tokens":30,"total_tokens":80,
			"prompt_tokens_details":{"cached_tokens":10},
			"completion_tokens_details":{"reasoning_tokens":12}}
	}`)
	result, _, err := (&OpenAIToGeminiResponseConverter{}).ConvertResponse(context.Background(), nil, resp)
	if err != nil {
		t.Fatalf("convert: %v", err)
	}
	gemini := result.(*dto.GeminiChatResponse)

	if len(gemini.Candidates) != 1 {
		t.Fatalf("candidates = %d", len(gemini.Candidates))
	}
	cand := gemini.Candidates[0]
	if cand.FinishReason != "STOP" {
		t.Errorf("finishReason = %q, want STOP（tool_calls→STOP legacy 怪癖）", cand.FinishReason)
	}
	// parts 序：thought → text → functionCall
	if len(cand.Content.Parts) != 3 {
		t.Fatalf("parts = %d, want 3", len(cand.Content.Parts))
	}
	if cand.Content.Parts[0].Thought == nil || !*cand.Content.Parts[0].Thought {
		t.Error("首 part 应为 thought:true")
	}
	if cand.Content.Parts[2].FunctionCall == nil || cand.Content.Parts[2].FunctionCall.ID != "t1" {
		t.Error("functionCall part 应带 ID t1")
	}
	// usage：candidates=30-12=18；cached=10；thoughts=12
	um := gemini.UsageMetadata
	if um.CandidatesTokenCount != 18 || um.CachedContentTokenCount != 10 || um.ThoughtsTokenCount != 12 {
		t.Errorf("usageMetadata = %+v, want candidates=18 cached=10 thoughts=12", um)
	}
	// ModelName 不填（legacy 口径）
	if gemini.ModelName != "" {
		t.Errorf("modelVersion = %q, want 空（legacy 不填）", gemini.ModelName)
	}
}

// 空 choices → 完全空对象。
func TestOpenAIToGeminiResponse_EmptyChoices(t *testing.T) {
	resp := parseChatRespGemini(t, `{"id":"c","model":"m","choices":[],"usage":{"prompt_tokens":1}}`)
	result, _, _ := (&OpenAIToGeminiResponseConverter{}).ConvertResponse(context.Background(), nil, resp)
	gemini := result.(*dto.GeminiChatResponse)
	if len(gemini.Candidates) != 0 || gemini.UsageMetadata != nil {
		t.Errorf("空 choices 应返回完全空对象: %+v", gemini)
	}
}

func jsonUnmarshalHelper(data []byte, v any) error {
	return json.Unmarshal(data, v)
}

package oai_gemini

import (
	"context"
	"strings"
	"testing"

	"github.com/qianfree/team-api/relaykit/dto"
	"github.com/qianfree/team-api/relaykit/relayconvert"
	"github.com/qianfree/team-api/relaykit/types"
)

func TestOpenAIToGeminiStreamConverter_Metadata(t *testing.T) {
	converter := &OpenAIToGeminiStreamConverter{}

	if converter.ID() != relayconvert.ResponseConverterOAIChatToGeminiChatStream {
		t.Errorf("ID() = %q, want %q", converter.ID(), relayconvert.ResponseConverterOAIChatToGeminiChatStream)
	}

	if converter.From() != types.RelayFormatOpenAI {
		t.Errorf("From() = %q, want %q", converter.From(), types.RelayFormatOpenAI)
	}

	if converter.To() != types.RelayFormatGemini {
		t.Errorf("To() = %q, want %q", converter.To(), types.RelayFormatGemini)
	}

	if converter.Quality() != relayconvert.ResponseConverterQualityGood {
		t.Errorf("Quality() = %q, want %q", converter.Quality(), relayconvert.ResponseConverterQualityGood)
	}
}

// collectGeminiStreamEvents 运行流式转换并收集全部 StreamEvent。
func collectGeminiStreamEvents(t *testing.T, streamData string) []*relayconvert.StreamEvent {
	t.Helper()
	converter := &OpenAIToGeminiStreamConverter{}

	var events []*relayconvert.StreamEvent
	chunkWriter := func(chunk any) error {
		event, ok := chunk.(*relayconvert.StreamEvent)
		if !ok {
			t.Fatalf("Expected *relayconvert.StreamEvent, got %T", chunk)
		}
		events = append(events, event)
		return nil
	}

	if err := converter.ConvertStreamResponse(context.Background(), nil, strings.NewReader(streamData), chunkWriter); err != nil {
		t.Fatalf("ConvertStreamResponse failed: %v", err)
	}
	return events
}

// geminiChunkOf 从 StreamEvent 中取出 Gemini chunk。
func geminiChunkOf(t *testing.T, event *relayconvert.StreamEvent) *dto.GeminiChatResponse {
	t.Helper()
	chunk, ok := event.Data.(*dto.GeminiChatResponse)
	if !ok {
		t.Fatalf("Expected *dto.GeminiChatResponse, got %T", event.Data)
	}
	return chunk
}

func TestOpenAIToGeminiStreamConverter_BasicStreaming(t *testing.T) {
	streamData := `data: {"id":"chatcmpl-1","object":"chat.completion.chunk","choices":[{"index":0,"delta":{"role":"assistant","content":"Hello"},"finish_reason":null}]}

data: {"id":"chatcmpl-1","object":"chat.completion.chunk","choices":[{"index":0,"delta":{"content":" world"},"finish_reason":null}]}

data: {"id":"chatcmpl-1","object":"chat.completion.chunk","choices":[{"index":0,"delta":{},"finish_reason":"stop"}],"usage":{"prompt_tokens":5,"completion_tokens":10,"total_tokens":15}}

data: [DONE]

`

	events := collectGeminiStreamEvents(t, streamData)

	// "Hello" + " world" + finish_reason 帧 + 最终帧
	if len(events) != 4 {
		t.Fatalf("Event count = %d, want 4", len(events))
	}

	// Gemini 客户端为纯 data 帧风格，Event 必须为空
	for i, ev := range events {
		if ev.Event != "" {
			t.Errorf("Event[%d].Event = %q, want empty", i, ev.Event)
		}
	}

	chunk0 := geminiChunkOf(t, events[0])
	if len(chunk0.Candidates) != 1 {
		t.Fatalf("First chunk candidates = %d, want 1", len(chunk0.Candidates))
	}
	if chunk0.Candidates[0].Content.Role != "model" {
		t.Errorf("First chunk role = %q, want model", chunk0.Candidates[0].Content.Role)
	}
	if chunk0.Candidates[0].Content.Parts[0].Text != "Hello" {
		t.Errorf("First chunk text = %q, want Hello", chunk0.Candidates[0].Content.Parts[0].Text)
	}

	chunk1 := geminiChunkOf(t, events[1])
	if chunk1.Candidates[0].Content.Parts[0].Text != " world" {
		t.Errorf("Second chunk text = %q, want ' world'", chunk1.Candidates[0].Content.Parts[0].Text)
	}

	// finish_reason 帧：parts 为空但 finish_reason 非 nil 也会输出候选（与旧实现一致）
	chunk2 := geminiChunkOf(t, events[2])
	if len(chunk2.Candidates) != 1 || len(chunk2.Candidates[0].Content.Parts) != 0 {
		t.Errorf("Third chunk = %+v", chunk2.Candidates)
	}

	// 最终帧：finishReason + usageMetadata
	finalChunk := geminiChunkOf(t, events[3])
	if len(finalChunk.Candidates) != 1 {
		t.Fatalf("Final chunk candidates = %d, want 1", len(finalChunk.Candidates))
	}
	if finalChunk.Candidates[0].FinishReason != "STOP" {
		t.Errorf("Final finishReason = %q, want STOP", finalChunk.Candidates[0].FinishReason)
	}
	if finalChunk.UsageMetadata == nil {
		t.Fatal("Final chunk missing usageMetadata")
	}
	if finalChunk.UsageMetadata.PromptTokenCount != 5 {
		t.Errorf("PromptTokenCount = %d, want 5", finalChunk.UsageMetadata.PromptTokenCount)
	}
	if finalChunk.UsageMetadata.CandidatesTokenCount != 10 {
		t.Errorf("CandidatesTokenCount = %d, want 10", finalChunk.UsageMetadata.CandidatesTokenCount)
	}
	if finalChunk.UsageMetadata.TotalTokenCount != 15 {
		t.Errorf("TotalTokenCount = %d, want 15", finalChunk.UsageMetadata.TotalTokenCount)
	}

	// 最终帧同时携带 StreamEvent.Usage 供宿主捕获
	if events[3].Usage == nil {
		t.Fatal("Final event missing Usage")
	}
	if events[3].Usage.PromptTokens != 5 || events[3].Usage.CompletionTokens != 10 || events[3].Usage.TotalTokens != 15 {
		t.Errorf("Final event usage = %+v", events[3].Usage)
	}
	// 中间帧不携带 Usage
	for i := 0; i < 3; i++ {
		if events[i].Usage != nil {
			t.Errorf("Event[%d] has unexpected Usage", i)
		}
	}
}

func TestOpenAIToGeminiStreamConverter_ReasoningStreaming(t *testing.T) {
	streamData := `data: {"choices":[{"index":0,"delta":{"reasoning_content":"thinking..."},"finish_reason":null}]}

data: {"choices":[{"index":0,"delta":{"content":"Answer"},"finish_reason":null}]}

data: {"choices":[{"index":0,"delta":{},"finish_reason":"stop"}],"usage":{"prompt_tokens":10,"completion_tokens":50,"total_tokens":60,"completion_tokens_details":{"reasoning_tokens":30}}}

data: [DONE]

`

	events := collectGeminiStreamEvents(t, streamData)
	if len(events) != 4 {
		t.Fatalf("Event count = %d, want 4", len(events))
	}

	// reasoning_content → thought part
	chunk0 := geminiChunkOf(t, events[0])
	part := chunk0.Candidates[0].Content.Parts[0]
	if part.Text != "thinking..." || part.Thought == nil || !*part.Thought {
		t.Errorf("Thought part = %+v", part)
	}

	chunk1 := geminiChunkOf(t, events[1])
	if chunk1.Candidates[0].Content.Parts[0].Text != "Answer" {
		t.Errorf("Text part = %+v", chunk1.Candidates[0].Content.Parts[0])
	}

	// 最终帧：candidatesTokenCount 扣减 reasoning（50 - 30 = 20），thoughtsTokenCount = 30
	finalChunk := geminiChunkOf(t, events[3])
	if finalChunk.UsageMetadata.CandidatesTokenCount != 20 {
		t.Errorf("CandidatesTokenCount = %d, want 20", finalChunk.UsageMetadata.CandidatesTokenCount)
	}
	if finalChunk.UsageMetadata.ThoughtsTokenCount != 30 {
		t.Errorf("ThoughtsTokenCount = %d, want 30", finalChunk.UsageMetadata.ThoughtsTokenCount)
	}
	// StreamEvent.Usage 保持 OpenAI 口径（completion 含 reasoning），并透传明细
	if events[3].Usage.CompletionTokens != 50 {
		t.Errorf("Usage CompletionTokens = %d, want 50", events[3].Usage.CompletionTokens)
	}
	if events[3].Usage.CompletionTokenDetails == nil || events[3].Usage.CompletionTokenDetails.ReasoningTokens != 30 {
		t.Errorf("Usage CompletionTokenDetails = %+v, want ReasoningTokens 30", events[3].Usage.CompletionTokenDetails)
	}
}

func TestOpenAIToGeminiStreamConverter_ToolCallStreaming(t *testing.T) {
	streamData := `data: {"choices":[{"index":0,"delta":{"tool_calls":[{"id":"call_1","type":"function","function":{"name":"get_weather","arguments":"{\"city\":\"北京\"}"}}]},"finish_reason":null}]}

data: {"choices":[{"index":0,"delta":{},"finish_reason":"tool_calls"}]}

data: [DONE]

`

	events := collectGeminiStreamEvents(t, streamData)
	if len(events) != 3 {
		t.Fatalf("Event count = %d, want 3", len(events))
	}

	chunk0 := geminiChunkOf(t, events[0])
	fc := chunk0.Candidates[0].Content.Parts[0].FunctionCall
	if fc == nil || fc.ID != "call_1" || fc.FunctionName != "get_weather" {
		t.Fatalf("FunctionCall = %+v", fc)
	}
	args, ok := fc.Arguments.(map[string]any)
	if !ok || args["city"] != "北京" {
		t.Errorf("Arguments = %v", fc.Arguments)
	}

	// tool_calls → STOP
	finalChunk := geminiChunkOf(t, events[2])
	if finalChunk.Candidates[0].FinishReason != "STOP" {
		t.Errorf("Final finishReason = %q, want STOP", finalChunk.Candidates[0].FinishReason)
	}
	// 无 usage 时最终帧不带 usageMetadata，但 StreamEvent.Usage 仍存在（零值）
	if finalChunk.UsageMetadata != nil {
		t.Errorf("UsageMetadata = %+v, want nil", finalChunk.UsageMetadata)
	}
	if events[2].Usage == nil || events[2].Usage.TotalTokens != 0 {
		t.Errorf("Final event usage = %+v, want zero-value usage", events[2].Usage)
	}
}

func TestOpenAIToGeminiStreamConverter_CachedTokensAndMalformedLines(t *testing.T) {
	// 含非法 JSON 行、非 data 行、空 data 行，均应被跳过
	streamData := `: ping

event: something

data: not-json

data:

data: {"choices":[{"index":0,"delta":{"content":"ok"},"finish_reason":null}]}

data: {"choices":[{"index":0,"delta":{},"finish_reason":"length"}],"usage":{"prompt_tokens":100,"completion_tokens":10,"total_tokens":110,"prompt_tokens_details":{"cached_tokens":80}}}

data: [DONE]

`

	events := collectGeminiStreamEvents(t, streamData)
	if len(events) != 3 {
		t.Fatalf("Event count = %d, want 3", len(events))
	}

	chunk0 := geminiChunkOf(t, events[0])
	if chunk0.Candidates[0].Content.Parts[0].Text != "ok" {
		t.Errorf("First chunk text = %q, want ok", chunk0.Candidates[0].Content.Parts[0].Text)
	}

	finalChunk := geminiChunkOf(t, events[2])
	if finalChunk.Candidates[0].FinishReason != "MAX_TOKENS" {
		t.Errorf("Final finishReason = %q, want MAX_TOKENS", finalChunk.Candidates[0].FinishReason)
	}
	if finalChunk.UsageMetadata.CachedContentTokenCount != 80 {
		t.Errorf("CachedContentTokenCount = %d, want 80", finalChunk.UsageMetadata.CachedContentTokenCount)
	}
	// 缓存明细透传进 StreamEvent.Usage（影响计费）
	if events[2].Usage == nil || events[2].Usage.PromptTokensDetails == nil || events[2].Usage.PromptTokensDetails.CachedTokens != 80 {
		t.Errorf("Final event usage details = %+v, want CachedTokens 80", events[2].Usage)
	}
}

func TestOpenAIToGeminiStreamConverter_NoDoneTrailer(t *testing.T) {
	// 上游流未发送 [DONE] 直接结束，也应输出最终帧
	streamData := `data: {"choices":[{"index":0,"delta":{"content":"partial"},"finish_reason":null}]}

`

	events := collectGeminiStreamEvents(t, streamData)
	if len(events) != 2 {
		t.Fatalf("Event count = %d, want 2", len(events))
	}

	// 无 finish_reason 时映射结果为空字符串（与旧实现一致，不默认 STOP）
	finalChunk := geminiChunkOf(t, events[1])
	if finalChunk.Candidates[0].FinishReason != "" {
		t.Errorf("Final finishReason = %q, want empty", finalChunk.Candidates[0].FinishReason)
	}
}

func TestOpenAIToGeminiStreamConverter_ContextCancelled(t *testing.T) {
	converter := &OpenAIToGeminiStreamConverter{}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	streamData := `data: {"choices":[{"index":0,"delta":{"content":"hi"},"finish_reason":null}]}

`

	err := converter.ConvertStreamResponse(ctx, nil, strings.NewReader(streamData), func(chunk any) error {
		t.Fatal("chunkWriter should not be called after cancellation")
		return nil
	})
	if err == nil {
		t.Fatal("Expected context error, got nil")
	}
	if err != context.Canceled {
		t.Errorf("err = %v, want context.Canceled", err)
	}
}

package dify_chat

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/qianfree/team-api/relaykit/dto"
	"github.com/qianfree/team-api/relaykit/relayconvert"
	"github.com/qianfree/team-api/relaykit/relayconvert/convmeta"
	"github.com/qianfree/team-api/relaykit/types"
)

// ---------- 请求转换 ----------

func TestOpenAIToDifyRequestConverter_Metadata(t *testing.T) {
	c := &OpenAIToDifyRequestConverter{}
	if c.ID() != relayconvert.ConverterOpenAIChatToDify {
		t.Errorf("ID() = %q", c.ID())
	}
	if c.From() != types.RelayFormatOpenAI || c.To() != types.RelayFormatDify {
		t.Errorf("From/To = %q/%q", c.From(), c.To())
	}
	if c.Quality() != relayconvert.RequestConverterQualityFair {
		t.Errorf("Quality() = %q", c.Quality())
	}
}

func TestOpenAIToDifyRequestConverter_BasicFlatten(t *testing.T) {
	c := &OpenAIToDifyRequestConverter{}
	req := &dto.GeneralOpenAIRequest{
		User: "u1",
		Messages: []dto.Message{
			{Role: "system", Content: "be helpful"},
			{Role: "user", Content: "hello"},
		},
	}
	res, err := c.ConvertRequest(context.Background(), nil, req)
	if err != nil {
		t.Fatalf("ConvertRequest: %v", err)
	}
	d, ok := res.(*dto.DifyRequest)
	if !ok {
		t.Fatalf("type %T", res)
	}
	if d.ResponseMode != "blocking" {
		t.Errorf("ResponseMode = %q, want blocking (nil info)", d.ResponseMode)
	}
	if !strings.Contains(d.Query, "System: be helpful") || !strings.Contains(d.Query, "User: hello") {
		t.Errorf("Query not flattened: %q", d.Query)
	}
	if d.User != "u1" {
		t.Errorf("User = %q", d.User)
	}
}

func TestOpenAIToDifyRequestConverter_StreamMode(t *testing.T) {
	c := &OpenAIToDifyRequestConverter{}
	req := &dto.GeneralOpenAIRequest{
		Messages: []dto.Message{{Role: "user", Content: "hi"}},
	}
	info := &convmeta.Values{IsStream: true}
	res, err := c.ConvertRequest(context.Background(), info, req)
	if err != nil {
		t.Fatalf("ConvertRequest: %v", err)
	}
	d := res.(*dto.DifyRequest)
	if d.ResponseMode != "streaming" {
		t.Errorf("ResponseMode = %q, want streaming", d.ResponseMode)
	}
}

func TestOpenAIToDifyRequestConverter_DefaultUser(t *testing.T) {
	c := &OpenAIToDifyRequestConverter{}
	req := &dto.GeneralOpenAIRequest{
		Messages: []dto.Message{{Role: "user", Content: "hi"}},
	}
	res, _ := c.ConvertRequest(context.Background(), nil, req)
	if d := res.(*dto.DifyRequest); d.User != "relay-user" {
		t.Errorf("User = %q, want relay-user", d.User)
	}
}

func TestOpenAIToDifyRequestConverter_NoMessages(t *testing.T) {
	c := &OpenAIToDifyRequestConverter{}
	_, err := c.ConvertRequest(context.Background(), nil, &dto.GeneralOpenAIRequest{})
	if err == nil {
		t.Fatal("want error for empty messages")
	}
}

func TestOpenAIToDifyRequestConverter_InvalidType(t *testing.T) {
	c := &OpenAIToDifyRequestConverter{}
	if _, err := c.ConvertRequest(context.Background(), nil, "bad"); err == nil {
		t.Fatal("want error for wrong type")
	}
}

func TestOpenAIToDifyRequestConverter_MultimodalText(t *testing.T) {
	c := &OpenAIToDifyRequestConverter{}
	req := &dto.GeneralOpenAIRequest{
		Messages: []dto.Message{{
			Role: "user",
			Content: []any{
				map[string]any{"type": "text", "text": "a"},
				map[string]any{"type": "text", "text": "b"},
				map[string]any{"type": "image_url", "image_url": map[string]any{"url": "x"}},
			},
		}},
	}
	res, err := c.ConvertRequest(context.Background(), nil, req)
	if err != nil {
		t.Fatalf("ConvertRequest: %v", err)
	}
	d := res.(*dto.DifyRequest)
	if !strings.Contains(d.Query, "a b") {
		t.Errorf("multimodal text not joined: %q", d.Query)
	}
}

// ---------- 非流式响应转换 ----------

func TestDifyToOpenAIResponseConverter_Basic(t *testing.T) {
	c := &DifyToOpenAIResponseConverter{}
	resp := &dto.DifyBlockingResponse{
		Answer: "Hello world",
		Metadata: dto.DifyMeta{Usage: dto.DifyUsage{
			PromptTokens: 5, CompletionTokens: 3, TotalTokens: 8,
		}},
	}
	res, err := c.ConvertResponse(context.Background(), nil, resp)
	if err != nil {
		t.Fatalf("ConvertResponse: %v", err)
	}
	o := res.(*dto.ChatCompletionResponse)
	if len(o.Choices) != 1 || o.Choices[0].Message.Content != "Hello world" {
		t.Errorf("unexpected response: %+v", o)
	}
	if o.Usage.TotalTokens != 8 {
		t.Errorf("Usage.TotalTokens = %d, want 8", o.Usage.TotalTokens)
	}
}

func TestDifyToOpenAIResponseConverter_EstimatedUsage(t *testing.T) {
	c := &DifyToOpenAIResponseConverter{}
	resp := &dto.DifyBlockingResponse{Answer: "12345678"} // 8 chars → 2 tokens
	res, _ := c.ConvertResponse(context.Background(), nil, resp)
	o := res.(*dto.ChatCompletionResponse)
	if o.Usage.CompletionTokens != 2 {
		t.Errorf("estimated CompletionTokens = %d, want 2", o.Usage.CompletionTokens)
	}
}

func TestDifyToOpenAIResponseConverter_InvalidType(t *testing.T) {
	c := &DifyToOpenAIResponseConverter{}
	if _, err := c.ConvertResponse(context.Background(), nil, "bad"); err == nil {
		t.Fatal("want error for wrong type")
	}
}

// ---------- 流式响应转换 ----------

func collectDifyStream(t *testing.T, sse string) []*dto.ChatCompletionStreamResponse {
	t.Helper()
	c := &DifyToOpenAIStreamConverter{}
	var chunks []*dto.ChatCompletionStreamResponse
	err := c.ConvertStreamResponse(context.Background(), nil, bytes.NewReader([]byte(sse)), func(chunk any) error {
		if sc, ok := chunk.(*dto.ChatCompletionStreamResponse); ok {
			chunks = append(chunks, sc)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("ConvertStreamResponse: %v", err)
	}
	return chunks
}

func TestDifyToOpenAIStreamConverter_Basic(t *testing.T) {
	sse := strings.Join([]string{
		`data: {"event":"message","answer":"Hello"}`,
		`data: {"event":"message","answer":" world"}`,
		`data: {"event":"message_end","metadata":{"usage":{"prompt_tokens":3,"completion_tokens":2,"total_tokens":5}}}`,
		"",
	}, "\n")
	chunks := collectDifyStream(t, sse)
	// 期望：role chunk + 2 个内容 chunk + finish chunk（带 usage）
	if len(chunks) != 4 {
		t.Fatalf("chunks count = %d, want 4", len(chunks))
	}
	// 第一个是 role chunk
	if chunks[0].Choices[0].Delta.Role != "assistant" {
		t.Errorf("first chunk role = %q", chunks[0].Choices[0].Delta.Role)
	}
	// 内容拼接
	if chunks[1].Choices[0].Delta.Content != "Hello" || chunks[2].Choices[0].Delta.Content != " world" {
		t.Errorf("content chunks: %q, %q", chunks[1].Choices[0].Delta.Content, chunks[2].Choices[0].Delta.Content)
	}
	// 最后一个 finish chunk 带 finish_reason + usage
	last := chunks[3]
	if last.Choices[0].FinishReason == nil || *last.Choices[0].FinishReason != "stop" {
		t.Errorf("finish reason: %v", last.Choices[0].FinishReason)
	}
	if last.Usage == nil || last.Usage.TotalTokens != 5 {
		t.Errorf("usage: %v", last.Usage)
	}
}

func TestDifyToOpenAIStreamConverter_Error(t *testing.T) {
	c := &DifyToOpenAIStreamConverter{}
	sse := `data: {"event":"error"}`
	err := c.ConvertStreamResponse(context.Background(), nil, bytes.NewReader([]byte(sse)), func(any) error { return nil })
	if err == nil {
		t.Fatal("want error for dify error event")
	}
}

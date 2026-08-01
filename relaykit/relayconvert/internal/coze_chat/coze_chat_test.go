package coze_chat

import (
	"bytes"
	"context"
	"testing"

	"github.com/qianfree/team-api/relaykit/dto"
	"github.com/qianfree/team-api/relaykit/relayconvert"
	"github.com/qianfree/team-api/relaykit/relayconvert/convmeta"
	"github.com/qianfree/team-api/relaykit/types"
)

// ---------- 请求转换 ----------

func TestOpenAIToCozeRequestConverter_Metadata(t *testing.T) {
	c := &OpenAIToCozeRequestConverter{}
	if c.ID() != relayconvert.ConverterOpenAIChatToCoze {
		t.Errorf("ID() = %q", c.ID())
	}
	if c.From() != types.RelayFormatOpenAI || c.To() != types.RelayFormatCoze {
		t.Errorf("From/To = %q/%q", c.From(), c.To())
	}
}

func TestOpenAIToCozeRequestConverter_Basic(t *testing.T) {
	c := &OpenAIToCozeRequestConverter{}
	req := &dto.GeneralOpenAIRequest{
		Model: "bot-origin",
		Messages: []dto.Message{
			{Role: "system", Content: "sys"},
			{Role: "user", Content: "first question"},
			{Role: "assistant", Content: "answer"},
			{Role: "user", Content: "latest question"},
		},
	}
	info := &convmeta.Values{UpstreamModelName: "bot-mapped"}
	res, err := c.ConvertRequest(context.Background(), info, req)
	if err != nil {
		t.Fatalf("ConvertRequest: %v", err)
	}
	r := res.(*dto.CozeCreateRequest)
	if r.Query != "latest question" {
		t.Errorf("Query = %q, want latest question (last user msg)", r.Query)
	}
	if r.BotID != "bot-mapped" {
		t.Errorf("BotID = %q, want bot-mapped (upstream)", r.BotID)
	}
	if !r.Stream {
		t.Error("Stream must be forced true")
	}
}

func TestOpenAIToCozeRequestConverter_BotIDFallbackToModel(t *testing.T) {
	c := &OpenAIToCozeRequestConverter{}
	req := &dto.GeneralOpenAIRequest{
		Model:    "bot-origin",
		Messages: []dto.Message{{Role: "user", Content: "q"}},
	}
	// 无 info（或 UpstreamModelName 为空）：bot_id 回退到客户端模型名
	res, err := c.ConvertRequest(context.Background(), nil, req)
	if err != nil {
		t.Fatalf("ConvertRequest: %v", err)
	}
	if r := res.(*dto.CozeCreateRequest); r.BotID != "bot-origin" {
		t.Errorf("BotID = %q, want bot-origin", r.BotID)
	}
}

func TestOpenAIToCozeRequestConverter_DefaultUser(t *testing.T) {
	c := &OpenAIToCozeRequestConverter{}
	req := &dto.GeneralOpenAIRequest{Model: "b", Messages: []dto.Message{{Role: "user", Content: "q"}}}
	res, _ := c.ConvertRequest(context.Background(), nil, req)
	if r := res.(*dto.CozeCreateRequest); r.User != "relay-user" {
		t.Errorf("User = %q", r.User)
	}
}

func TestOpenAIToCozeRequestConverter_NoUserMessage(t *testing.T) {
	c := &OpenAIToCozeRequestConverter{}
	req := &dto.GeneralOpenAIRequest{
		Messages: []dto.Message{{Role: "system", Content: "sys"}},
	}
	if _, err := c.ConvertRequest(context.Background(), nil, req); err == nil {
		t.Fatal("want error when no user message")
	}
}

func TestOpenAIToCozeRequestConverter_MultimodalLastUser(t *testing.T) {
	c := &OpenAIToCozeRequestConverter{}
	req := &dto.GeneralOpenAIRequest{
		Messages: []dto.Message{{
			Role: "user",
			Content: []any{
				map[string]any{"type": "image_url", "image_url": map[string]any{"url": "x"}},
				map[string]any{"type": "text", "text": "describe this"},
			},
		}},
	}
	res, err := c.ConvertRequest(context.Background(), nil, req)
	if err != nil {
		t.Fatalf("ConvertRequest: %v", err)
	}
	if r := res.(*dto.CozeCreateRequest); r.Query != "describe this" {
		t.Errorf("Query = %q", r.Query)
	}
}

func TestOpenAIToCozeRequestConverter_InvalidType(t *testing.T) {
	c := &OpenAIToCozeRequestConverter{}
	if _, err := c.ConvertRequest(context.Background(), nil, 42); err == nil {
		t.Fatal("want error for wrong type")
	}
}

// ---------- 非流式响应转换（解析缓冲 SSE） ----------

func TestCozeToOpenAIResponseConverter_Basic(t *testing.T) {
	c := &CozeToOpenAIResponseConverter{}
	sse := "event: conversation.message.delta\n" +
		`data: {"role":"assistant","type":"answer","content":"Hel"}` + "\n" +
		"event: conversation.message.completed\n" +
		`data: {"role":"assistant","type":"answer","content":"Hello"}` + "\n"
	res, err := c.ConvertResponse(context.Background(), nil, []byte(sse))
	if err != nil {
		t.Fatalf("ConvertResponse: %v", err)
	}
	o := res.(*dto.ChatCompletionResponse)
	// completed 事件优先，应取 "Hello"
	if o.Choices[0].Message.Content != "Hello" {
		t.Errorf("content = %q, want Hello", o.Choices[0].Message.Content)
	}
	if o.Usage.CompletionTokens != 1 { // "Hello" len 5 → 1
		t.Errorf("estimated CompletionTokens = %d, want 1", o.Usage.CompletionTokens)
	}
}

func TestCozeToOpenAIResponseConverter_OnlyDelta(t *testing.T) {
	c := &CozeToOpenAIResponseConverter{}
	sse := "event: conversation.message.delta\n" +
		`data: {"role":"assistant","type":"answer","content":"Hi"}` + "\n" +
		"event: conversation.message.delta\n" +
		`data: {"role":"assistant","type":"answer","content":" there"}` + "\n"
	res, _ := c.ConvertResponse(context.Background(), nil, []byte(sse))
	o := res.(*dto.ChatCompletionResponse)
	if o.Choices[0].Message.Content != "Hi there" {
		t.Errorf("content = %q, want 'Hi there'", o.Choices[0].Message.Content)
	}
}

func TestCozeToOpenAIResponseConverter_ErrorEvent(t *testing.T) {
	c := &CozeToOpenAIResponseConverter{}
	sse := "event: error\ndata: {\"code\":500}\n"
	if _, err := c.ConvertResponse(context.Background(), nil, []byte(sse)); err == nil {
		t.Fatal("want error for coze error event")
	}
}

func TestCozeToOpenAIResponseConverter_InvalidType(t *testing.T) {
	c := &CozeToOpenAIResponseConverter{}
	if _, err := c.ConvertResponse(context.Background(), nil, "not-bytes"); err == nil {
		t.Fatal("want error for wrong type")
	}
}

// ---------- 流式响应转换 ----------

func TestCozeToOpenAIStreamConverter_Basic(t *testing.T) {
	c := &CozeToOpenAIStreamConverter{}
	sse := "event: conversation.message.delta\n" +
		`data: {"role":"assistant","type":"answer","content":"Hello"}` + "\n" +
		"event: conversation.message.completed\n" +
		`data: {"role":"assistant","type":"answer","content":"Hello world"}` + "\n" +
		"event: done\ndata: {}\n"
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
	// role chunk + content delta + finish chunk（completed）
	if len(chunks) != 3 {
		t.Fatalf("chunks = %d, want 3", len(chunks))
	}
	if chunks[0].Choices[0].Delta.Role != "assistant" {
		t.Errorf("first chunk role = %q", chunks[0].Choices[0].Delta.Role)
	}
	if chunks[1].Choices[0].Delta.Content != "Hello" {
		t.Errorf("content delta = %q", chunks[1].Choices[0].Delta.Content)
	}
	if chunks[2].Choices[0].FinishReason == nil || *chunks[2].Choices[0].FinishReason != "stop" {
		t.Errorf("finish chunk missing/incorrect: %v", chunks[2].Choices[0].FinishReason)
	}
}

func TestCozeToOpenAIStreamConverter_NonAnswerIgnored(t *testing.T) {
	c := &CozeToOpenAIStreamConverter{}
	sse := "event: conversation.message.delta\n" +
		`data: {"role":"assistant","type":"follow_up","content":"ignored"}` + "\n" +
		"event: conversation.message.delta\n" +
		`data: {"role":"assistant","type":"answer","content":"real"}` + "\n" +
		"event: done\ndata: {}\n"
	var chunks []*dto.ChatCompletionStreamResponse
	_ = c.ConvertStreamResponse(context.Background(), nil, bytes.NewReader([]byte(sse)), func(chunk any) error {
		if sc, ok := chunk.(*dto.ChatCompletionStreamResponse); ok {
			chunks = append(chunks, sc)
		}
		return nil
	})
	// role + content("real") + finish(done 兜底) = 3；非 answer 不产出内容 chunk
	for _, ch := range chunks {
		if c := ch.Choices[0].Delta.Content; c == "ignored" {
			t.Errorf("non-answer content should be ignored, got %q", c)
		}
	}
}

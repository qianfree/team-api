package ollama_chat

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

func TestOpenAIToOllamaRequestConverter_Metadata(t *testing.T) {
	c := &OpenAIToOllamaRequestConverter{}
	if c.ID() != relayconvert.ConverterOpenAIChatToOllama {
		t.Errorf("ID() = %q", c.ID())
	}
	if c.From() != types.RelayFormatOpenAI || c.To() != types.RelayFormatOllama {
		t.Errorf("From/To = %q/%q", c.From(), c.To())
	}
}

func TestOpenAIToOllamaRequestConverter_BasicWithOptions(t *testing.T) {
	c := &OpenAIToOllamaRequestConverter{}
	temp, topP, maxTok, freq, pres := 0.5, 0.8, 100, 0.1, 0.2
	req := &dto.GeneralOpenAIRequest{
		Model:    "openai-model",
		Messages: []dto.Message{{Role: "user", Content: "hi"}},
		Temperature: &temp, TopP: &topP, MaxTokens: &maxTok,
		FrequencyPenalty: &freq, PresencePenalty: &pres, Stop: "STOP",
	}
	info := &convmeta.Values{UpstreamModelName: "llama3", IsStream: true}
	res, err := c.ConvertRequest(context.Background(), info, req)
	if err != nil {
		t.Fatalf("ConvertRequest: %v", err)
	}
	r := res.(*dto.OllamaChatRequest)
	if r.Model != "llama3" {
		t.Errorf("Model = %q, want llama3", r.Model)
	}
	if !r.Stream {
		t.Error("Stream should be true")
	}
	if len(r.Messages) != 1 || r.Messages[0].Content != "hi" {
		t.Errorf("Messages = %+v", r.Messages)
	}
	opts := r.Options
	if opts["temperature"] != 0.5 || opts["top_p"] != 0.8 {
		t.Errorf("temperature/top_p = %v/%v", opts["temperature"], opts["top_p"])
	}
	if opts["num_predict"] != 100 {
		t.Errorf("num_predict = %v, want 100", opts["num_predict"])
	}
	if opts["frequency_penalty"] != 0.1 || opts["presence_penalty"] != 0.2 {
		t.Errorf("penalties = %v/%v", opts["frequency_penalty"], opts["presence_penalty"])
	}
	if opts["stop"] != "STOP" {
		t.Errorf("stop = %v", opts["stop"])
	}
}

func TestOpenAIToOllamaRequestConverter_MaxCompletionTokens(t *testing.T) {
	c := &OpenAIToOllamaRequestConverter{}
	mct := 256
	req := &dto.GeneralOpenAIRequest{
		Messages:           []dto.Message{{Role: "user", Content: "hi"}},
		MaxCompletionTokens: &mct,
	}
	res, _ := c.ConvertRequest(context.Background(), nil, req)
	r := res.(*dto.OllamaChatRequest)
	if r.Options["num_predict"] != 256 {
		t.Errorf("num_predict = %v, want 256 (from max_completion_tokens)", r.Options["num_predict"])
	}
}

func TestOpenAIToOllamaRequestConverter_EmptyOptions(t *testing.T) {
	c := &OpenAIToOllamaRequestConverter{}
	req := &dto.GeneralOpenAIRequest{Messages: []dto.Message{{Role: "user", Content: "hi"}}}
	res, _ := c.ConvertRequest(context.Background(), nil, req)
	if r := res.(*dto.OllamaChatRequest); r.Options != nil {
		t.Errorf("Options should be nil when no params, got %v", r.Options)
	}
}

func TestOpenAIToOllamaRequestConverter_MultimodalText(t *testing.T) {
	c := &OpenAIToOllamaRequestConverter{}
	req := &dto.GeneralOpenAIRequest{
		Messages: []dto.Message{{
			Role: "user",
			Content: []any{
				map[string]any{"type": "text", "text": "a"},
				map[string]any{"type": "text", "text": "b"},
			},
		}},
	}
	res, _ := c.ConvertRequest(context.Background(), nil, req)
	r := res.(*dto.OllamaChatRequest)
	if r.Messages[0].Content != "ab" {
		t.Errorf("multimodal text = %q, want ab", r.Messages[0].Content)
	}
}

func TestOpenAIToOllamaRequestConverter_InvalidType(t *testing.T) {
	c := &OpenAIToOllamaRequestConverter{}
	if _, err := c.ConvertRequest(context.Background(), nil, 1.5); err == nil {
		t.Fatal("want error for wrong type")
	}
}

// ---------- 非流式响应转换 ----------

func TestOllamaToOpenAIResponseConverter_Basic(t *testing.T) {
	c := &OllamaToOpenAIResponseConverter{}
	resp := &dto.OllamaChatResponse{
		Message:         dto.OllamaMessage{Role: "assistant", Content: "answer"},
		PromptEvalCount: 10,
		EvalCount:       5,
	}
	res, err := c.ConvertResponse(context.Background(), nil, resp)
	if err != nil {
		t.Fatalf("ConvertResponse: %v", err)
	}
	o := res.(*dto.ChatCompletionResponse)
	if o.Choices[0].Message.Content != "answer" {
		t.Errorf("content = %q", o.Choices[0].Message.Content)
	}
	if o.Usage.PromptTokens != 10 || o.Usage.CompletionTokens != 5 || o.Usage.TotalTokens != 15 {
		t.Errorf("usage = %+v", o.Usage)
	}
}

func TestOllamaToOpenAIResponseConverter_InvalidType(t *testing.T) {
	c := &OllamaToOpenAIResponseConverter{}
	if _, err := c.ConvertResponse(context.Background(), nil, "bad"); err == nil {
		t.Fatal("want error for wrong type")
	}
}

// ---------- 流式响应转换（NDJSON） ----------

func TestOllamaToOpenAIStreamConverter_Basic(t *testing.T) {
	c := &OllamaToOpenAIStreamConverter{}
	ndjson := `{"model":"llama3","message":{"role":"assistant","content":"Hi"},"done":false}` + "\n" +
		`{"model":"llama3","message":{"role":"assistant","content":" there"},"done":false}` + "\n" +
		`{"model":"llama3","message":{"role":"assistant","content":""},"done":true,"prompt_eval_count":10,"eval_count":5}` + "\n"
	var chunks []*dto.ChatCompletionStreamResponse
	err := c.ConvertStreamResponse(context.Background(), nil, bytes.NewReader([]byte(ndjson)), func(chunk any) error {
		if sc, ok := chunk.(*dto.ChatCompletionStreamResponse); ok {
			chunks = append(chunks, sc)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("ConvertStreamResponse: %v", err)
	}
	// role chunk + content("Hi") + content(" there") + finish(usage)
	if len(chunks) != 4 {
		t.Fatalf("chunks = %d, want 4", len(chunks))
	}
	if chunks[0].Choices[0].Delta.Role != "assistant" {
		t.Errorf("first chunk role = %q", chunks[0].Choices[0].Delta.Role)
	}
	if chunks[1].Choices[0].Delta.Content != "Hi" || chunks[2].Choices[0].Delta.Content != " there" {
		t.Errorf("content = %q / %q", chunks[1].Choices[0].Delta.Content, chunks[2].Choices[0].Delta.Content)
	}
	last := chunks[3]
	if last.Choices[0].FinishReason == nil || *last.Choices[0].FinishReason != "stop" {
		t.Errorf("finish reason = %v", last.Choices[0].FinishReason)
	}
	if last.Usage == nil || last.Usage.TotalTokens != 15 {
		t.Errorf("usage = %v", last.Usage)
	}
}

func TestOllamaToOpenAIStreamConverter_NoDoneFallback(t *testing.T) {
	c := &OllamaToOpenAIStreamConverter{}
	// 流提前结束（无 done 行）：应补发终止 chunk
	ndjson := `{"model":"llama3","message":{"role":"assistant","content":"Hi"},"done":false}` + "\n"
	var chunks []*dto.ChatCompletionStreamResponse
	_ = c.ConvertStreamResponse(context.Background(), nil, bytes.NewReader([]byte(ndjson)), func(chunk any) error {
		if sc, ok := chunk.(*dto.ChatCompletionStreamResponse); ok {
			chunks = append(chunks, sc)
		}
		return nil
	})
	last := chunks[len(chunks)-1]
	if last.Choices[0].FinishReason == nil {
		t.Error("expected terminal finish chunk when no done line")
	}
}

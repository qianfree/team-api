package ollama_chat

import (
	"bytes"
	"context"
	"encoding/json"
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
		Model:       "openai-model",
		Messages:    []dto.Message{{Role: "user", Content: "hi"}},
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
		Messages:            []dto.Message{{Role: "user", Content: "hi"}},
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

// ---------- 请求转换：tools / format / think ----------

func TestOpenAIToOllamaRequestConverter_ToolsFormatThink(t *testing.T) {
	c := &OpenAIToOllamaRequestConverter{}
	req := &dto.GeneralOpenAIRequest{
		Messages: []dto.Message{{Role: "user", Content: "hi"}},
		Tools: []dto.Tool{{
			Type: "function",
			Function: dto.FunctionDef{
				Name:        "get_weather",
				Description: "get weather",
				Parameters:  map[string]any{"type": "object", "properties": map[string]any{"city": map[string]any{"type": "string"}}},
			},
		}},
		ResponseFormat:  &dto.ResponseFormat{Type: "json_schema", JSONSchema: map[string]any{"name": "weather", "schema": map[string]any{"type": "object", "properties": map[string]any{"city": map[string]any{"type": "string"}}}}},
		ReasoningEffort: "high",
	}
	res, err := c.ConvertRequest(context.Background(), nil, req)
	if err != nil {
		t.Fatalf("ConvertRequest: %v", err)
	}
	r := res.(*dto.OllamaChatRequest)
	if len(r.Tools) != 1 || r.Tools[0].Type != "function" || r.Tools[0].Function.Name != "get_weather" {
		t.Errorf("Tools = %+v", r.Tools)
	}
	// json_schema 应解包为裸 schema，不带外层包装字段
	format, ok := r.Format.(map[string]any)
	if !ok || format["type"] != "object" {
		t.Errorf("Format = %#v, want bare schema {type:object}", r.Format)
	}
	if _, hasName := format["name"]; hasName {
		t.Errorf("Format should not keep json_schema wrapper name, got %#v", r.Format)
	}
	if string(r.Think) != `"high"` {
		t.Errorf("Think = %s, want \"high\"", r.Think)
	}
}

func TestOpenAIToOllamaRequestConverter_JSONFormatAndThinkNone(t *testing.T) {
	c := &OpenAIToOllamaRequestConverter{}
	req := &dto.GeneralOpenAIRequest{
		Messages:        []dto.Message{{Role: "user", Content: "hi"}},
		ResponseFormat:  &dto.ResponseFormat{Type: "json_object"},
		ReasoningEffort: "none",
	}
	res, err := c.ConvertRequest(context.Background(), nil, req)
	if err != nil {
		t.Fatalf("ConvertRequest: %v", err)
	}
	r := res.(*dto.OllamaChatRequest)
	if r.Format != "json" {
		t.Errorf("Format = %#v, want json", r.Format)
	}
	if string(r.Think) != "false" {
		t.Errorf("Think = %s, want false", r.Think)
	}
}

func TestOpenAIToOllamaRequestConverter_ToolCallContext(t *testing.T) {
	c := &OpenAIToOllamaRequestConverter{}
	rc := "thinking text"
	req := &dto.GeneralOpenAIRequest{
		Messages: []dto.Message{
			{
				Role:             "assistant",
				Content:          "",
				ReasoningContent: &rc,
				ToolCalls: []dto.ToolCall{{
					ID:   "call_abc",
					Type: "function",
					Function: dto.FunctionCall{
						Name:      "get_weather",
						Arguments: `{"city":"Paris"}`,
					},
				}},
			},
			{
				Role:       "tool",
				Content:    "sunny",
				ToolCallID: "call_abc",
			},
		},
	}
	res, err := c.ConvertRequest(context.Background(), nil, req)
	if err != nil {
		t.Fatalf("ConvertRequest: %v", err)
	}
	r := res.(*dto.OllamaChatRequest)
	asst := r.Messages[0]
	if string(asst.Thinking) != `"thinking text"` {
		t.Errorf("assistant Thinking = %s, want \"thinking text\"", asst.Thinking)
	}
	if len(asst.ToolCalls) != 1 || asst.ToolCalls[0].ID != "call_abc" {
		t.Fatalf("assistant ToolCalls = %+v", asst.ToolCalls)
	}
	args, ok := asst.ToolCalls[0].Function.Arguments.(map[string]any)
	if !ok || args["city"] != "Paris" {
		t.Errorf("tool call arguments = %#v", asst.ToolCalls[0].Function.Arguments)
	}
	toolMsg := r.Messages[1]
	if toolMsg.ToolCallID != "call_abc" {
		t.Errorf("tool ToolCallID = %q, want call_abc", toolMsg.ToolCallID)
	}
	if toolMsg.ToolName != "get_weather" {
		t.Errorf("tool ToolName = %q, want get_weather (fallback from tool_calls)", toolMsg.ToolName)
	}
}

// ---------- 非流式响应转换：tool_calls / thinking ----------

func TestOllamaToOpenAIResponseConverter_ToolCallsAndThinking(t *testing.T) {
	c := &OllamaToOpenAIResponseConverter{}
	resp := &dto.OllamaChatResponse{
		Message: dto.OllamaMessage{
			Role:     "assistant",
			Content:  "",
			Thinking: json.RawMessage(`"step by step"`),
			ToolCalls: []dto.OllamaToolCall{{
				ID: "call_upstream",
			}},
		},
		PromptEvalCount: 5,
		EvalCount:       7,
	}
	resp.Message.ToolCalls[0].Function.Name = "get_weather"
	resp.Message.ToolCalls[0].Function.Arguments = map[string]any{"city": "Paris"}

	res, err := c.ConvertResponse(context.Background(), nil, resp)
	if err != nil {
		t.Fatalf("ConvertResponse: %v", err)
	}
	o := res.(*dto.ChatCompletionResponse)
	msg := o.Choices[0].Message
	if o.Choices[0].FinishReason != "tool_calls" {
		t.Errorf("finish reason = %q, want tool_calls", o.Choices[0].FinishReason)
	}
	if msg.ReasoningContent == nil || *msg.ReasoningContent != "step by step" {
		t.Errorf("reasoning content = %v", msg.ReasoningContent)
	}
	if len(msg.ToolCalls) != 1 {
		t.Fatalf("tool calls = %+v", msg.ToolCalls)
	}
	tc := msg.ToolCalls[0]
	if tc.ID != "call_upstream" || tc.Type != "function" {
		t.Errorf("tool call = %+v", tc)
	}
	if tc.Function.Name != "get_weather" || tc.Function.Arguments != `{"city":"Paris"}` {
		t.Errorf("tool call function = %+v", tc.Function)
	}
}

func TestOllamaToOpenAIResponseConverter_ThinkingOnly(t *testing.T) {
	c := &OllamaToOpenAIResponseConverter{}
	resp := &dto.OllamaChatResponse{
		Message: dto.OllamaMessage{Role: "assistant", Content: "answer", Thinking: json.RawMessage(`"draft"`)},
	}
	res, err := c.ConvertResponse(context.Background(), nil, resp)
	if err != nil {
		t.Fatalf("ConvertResponse: %v", err)
	}
	o := res.(*dto.ChatCompletionResponse)
	if o.Choices[0].FinishReason != "stop" {
		t.Errorf("finish reason = %q, want stop", o.Choices[0].FinishReason)
	}
	if o.Choices[0].Message.ReasoningContent == nil || *o.Choices[0].Message.ReasoningContent != "draft" {
		t.Errorf("reasoning content = %v", o.Choices[0].Message.ReasoningContent)
	}
	if len(o.Choices[0].Message.ToolCalls) != 0 {
		t.Errorf("tool calls should be empty, got %+v", o.Choices[0].Message.ToolCalls)
	}
}

func TestOllamaToOpenAIResponseConverter_ThinkingNullOrEmpty(t *testing.T) {
	c := &OllamaToOpenAIResponseConverter{}
	for _, raw := range []string{"null", `""`} {
		resp := &dto.OllamaChatResponse{
			Message: dto.OllamaMessage{Role: "assistant", Content: "answer", Thinking: json.RawMessage(raw)},
		}
		res, err := c.ConvertResponse(context.Background(), nil, resp)
		if err != nil {
			t.Fatalf("ConvertResponse(thinking=%s): %v", raw, err)
		}
		o := res.(*dto.ChatCompletionResponse)
		if o.Choices[0].Message.ReasoningContent != nil {
			t.Errorf("thinking=%s: reasoning content = %q, want nil", raw, *o.Choices[0].Message.ReasoningContent)
		}
	}
}

func TestOllamaToOpenAIResponseConverter_DoneReasonLength(t *testing.T) {
	c := &OllamaToOpenAIResponseConverter{}
	resp := &dto.OllamaChatResponse{
		Message:    dto.OllamaMessage{Role: "assistant", Content: "partial"},
		Done:       true,
		DoneReason: "length",
	}
	res, err := c.ConvertResponse(context.Background(), nil, resp)
	if err != nil {
		t.Fatalf("ConvertResponse: %v", err)
	}
	o := res.(*dto.ChatCompletionResponse)
	if o.Choices[0].FinishReason != "length" {
		t.Errorf("finish reason = %q, want length", o.Choices[0].FinishReason)
	}
}

// ---------- 流式响应转换：tool_calls ----------

func TestOllamaToOpenAIStreamConverter_ToolCalls(t *testing.T) {
	c := &OllamaToOpenAIStreamConverter{}
	ndjson := `{"model":"llama3","message":{"role":"assistant","content":"","tool_calls":[{"id":"call_1","function":{"name":"get_weather","arguments":{"city":"Paris"}}}]},"done":false}` + "\n" +
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
	// role 首包 + tool_calls 增量 + finish(usage) 共三段
	if len(chunks) != 3 {
		t.Fatalf("chunks = %d, want 3", len(chunks))
	}
	toolDelta := chunks[1].Choices[0].Delta
	if len(toolDelta.ToolCalls) != 1 {
		t.Fatalf("tool delta = %+v", toolDelta.ToolCalls)
	}
	tc := toolDelta.ToolCalls[0]
	if tc.Index != 0 || tc.ID != "call_1" || tc.Type != "function" {
		t.Errorf("tool call = %+v", tc)
	}
	if tc.Function.Name != "get_weather" || tc.Function.Arguments != `{"city":"Paris"}` {
		t.Errorf("tool call function = %+v", tc.Function)
	}
	last := chunks[2]
	if last.Choices[0].FinishReason == nil || *last.Choices[0].FinishReason != "tool_calls" {
		t.Errorf("finish reason = %v, want tool_calls", last.Choices[0].FinishReason)
	}
}

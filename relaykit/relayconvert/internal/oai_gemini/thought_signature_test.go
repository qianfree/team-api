package oai_gemini

import (
	"context"
	"strings"
	"testing"

	"github.com/qianfree/team-api/relaykit/dto"
	"github.com/qianfree/team-api/relaykit/relayconvert/convmeta"
)

// ==================== 响应侧：Gemini → OpenAI Chat 的签名捕获 ====================

// 流式：thought part 的签名挂消息级、functionCall part 的签名挂工具级、
// 签名孤儿 part 补发签名专用 chunk。
func TestG2OStream_ThoughtSignature(t *testing.T) {
	converter := &GeminiToOpenAIStreamConverter{}
	info := &g2oMeta{Values: convmeta.Values{OriginModelName: "gemini-3-pro"}, requestID: "req-sig"}

	stream := `data: {"candidates":[{"content":{"role":"model","parts":[{"text":"思考中","thought":true,"thoughtSignature":"sig-thought"}]},"index":0}]}

data: {"candidates":[{"content":{"role":"model","parts":[{"functionCall":{"name":"get_weather","args":{"city":"北京"}},"thoughtSignature":"sig-fc"}]},"index":0}]}

data: {"candidates":[{"content":{"role":"model","parts":[{"thoughtSignature":"sig-orphan"}]},"index":0,"finishReason":"STOP"}]}

`
	var chunks []*dto.ChatCompletionStreamResponse
	err := converter.ConvertStreamResponse(context.Background(), info, strings.NewReader(stream), func(chunk any) error {
		if c, ok := chunk.(*dto.ChatCompletionStreamResponse); ok {
			chunks = append(chunks, c)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("ConvertStreamResponse failed: %v", err)
	}

	var gotThought, gotFC, gotOrphan bool
	for _, c := range chunks {
		for _, choice := range c.Choices {
			if choice.Delta.ReasoningContent != nil && choice.Delta.ThoughtSignature == "sig-thought" {
				gotThought = true
			}
			for _, tc := range choice.Delta.ToolCalls {
				if tc.Function.Name == "get_weather" && tc.ThoughtSignature == "sig-fc" {
					gotFC = true
				}
			}
			if choice.Delta.ReasoningContent == nil && choice.Delta.Content == nil &&
				len(choice.Delta.ToolCalls) == 0 && choice.Delta.ThoughtSignature == "sig-orphan" {
				gotOrphan = true
			}
		}
	}
	if !gotThought {
		t.Error("thought part 的签名未挂到 reasoning chunk 的消息级 ThoughtSignature")
	}
	if !gotFC {
		t.Error("functionCall part 的签名未挂到工具级 ThoughtSignature")
	}
	if !gotOrphan {
		t.Error("签名孤儿 part 未补发签名专用 chunk")
	}
}

// 非流式：thought part 签名 → 消息级；functionCall part 签名 → 对应 tool call。
func TestG2OResponse_ThoughtSignature(t *testing.T) {
	converter := &GeminiToOpenAIResponseConverter{}
	info := &g2oMeta{Values: convmeta.Values{OriginModelName: "gemini-3-pro"}, requestID: "req-sig"}

	geminiResp := &dto.GeminiChatResponse{
		Candidates: []dto.GeminiCandidate{{
			Index:        0,
			FinishReason: "STOP",
			Content: &dto.GeminiContent{Role: "model", Parts: []dto.GeminiPart{
				{Text: "思考中", Thought: boolPtrSig(true), ThoughtSignature: "sig-thought"},
				{FunctionCall: &dto.GeminiFunctionCall{FunctionName: "get_weather", Arguments: map[string]any{"city": "北京"}}, ThoughtSignature: "sig-fc"},
			}},
		}},
	}

	result, err := converter.ConvertResponse(context.Background(), info, geminiResp)
	if err != nil {
		t.Fatalf("ConvertResponse failed: %v", err)
	}
	openaiResp := result.(*dto.ChatCompletionResponse)
	msg := openaiResp.Choices[0].Message
	if msg.ThoughtSignature != "sig-thought" {
		t.Errorf("消息级 ThoughtSignature = %q, want sig-thought", msg.ThoughtSignature)
	}
	if len(msg.ToolCalls) != 1 || msg.ToolCalls[0].ThoughtSignature != "sig-fc" {
		t.Errorf("工具级 ThoughtSignature = %+v, want sig-fc", msg.ToolCalls)
	}
}

// ==================== 请求侧：OpenAI Chat → Gemini 的签名回挂 ====================

// 工具级签名直挂对应 functionCall part；消息级签名优先回填首个无签名 functionCall part。
func TestO2GRequest_SignatureReattachToFunctionCall(t *testing.T) {
	converter := &OpenAIToGeminiRequestConverter{}
	rc := "思考"
	req := &dto.GeneralOpenAIRequest{
		Messages: []dto.Message{
			{
				Role:             "assistant",
				Content:          "",
				ReasoningContent: &rc,
				ThoughtSignature: "sig-msg",
				ToolCalls: []dto.ToolCall{
					{ID: "t1", Type: "function", Function: dto.FunctionCall{Name: "fa", Arguments: `{"x":1}`}},
					{ID: "t2", Type: "function", Function: dto.FunctionCall{Name: "fb", Arguments: `{}`}, ThoughtSignature: "sig-tc2"},
				},
			},
		},
	}

	result, err := converter.ConvertRequest(context.Background(), nil, req)
	if err != nil {
		t.Fatalf("ConvertRequest failed: %v", err)
	}
	geminiReq := result.(*dto.GeminiChatRequest)
	if len(geminiReq.Contents) != 1 {
		t.Fatalf("Contents len = %d, want 1", len(geminiReq.Contents))
	}
	var faSig, fbSig, thoughtSig string
	for _, p := range geminiReq.Contents[0].Parts {
		if p.FunctionCall != nil {
			switch p.FunctionCall.FunctionName {
			case "fa":
				faSig = p.ThoughtSignature
			case "fb":
				fbSig = p.ThoughtSignature
			}
		}
		if p.Thought != nil && *p.Thought {
			thoughtSig = p.ThoughtSignature
		}
	}
	// 消息级 sig-msg 回填到首个无签名 FC part（fa）；fb 保留自身工具级签名
	if faSig != "sig-msg" {
		t.Errorf("fa part 签名 = %q, want sig-msg（消息级回填）", faSig)
	}
	if fbSig != "sig-tc2" {
		t.Errorf("fb part 签名 = %q, want sig-tc2（工具级直挂）", fbSig)
	}
	if thoughtSig != "" {
		t.Errorf("thought part 签名 = %q, want 空（FC part 优先）", thoughtSig)
	}
}

// 无 functionCall 时消息级签名回填 thought part；纯文本时回填 text part。
func TestO2GRequest_SignatureFallbackToThoughtAndText(t *testing.T) {
	converter := &OpenAIToGeminiRequestConverter{}

	rc := "思考"
	reqThought := &dto.GeneralOpenAIRequest{
		Messages: []dto.Message{
			{Role: "assistant", Content: "回答", ReasoningContent: &rc, ThoughtSignature: "sig-a"},
		},
	}
	result, err := converter.ConvertRequest(context.Background(), nil, reqThought)
	if err != nil {
		t.Fatalf("ConvertRequest failed: %v", err)
	}
	parts := result.(*dto.GeminiChatRequest).Contents[0].Parts
	for _, p := range parts {
		isThought := p.Thought != nil && *p.Thought
		if isThought && p.ThoughtSignature != "sig-a" {
			t.Errorf("thought part 签名 = %q, want sig-a", p.ThoughtSignature)
		}
		if !isThought && p.ThoughtSignature != "" {
			t.Errorf("text part 签名 = %q, want 空（thought part 优先）", p.ThoughtSignature)
		}
	}

	reqText := &dto.GeneralOpenAIRequest{
		Messages: []dto.Message{
			{Role: "assistant", Content: "纯文本", ThoughtSignature: "sig-b"},
		},
	}
	result, err = converter.ConvertRequest(context.Background(), nil, reqText)
	if err != nil {
		t.Fatalf("ConvertRequest failed: %v", err)
	}
	parts = result.(*dto.GeminiChatRequest).Contents[0].Parts
	if len(parts) != 1 || parts[0].ThoughtSignature != "sig-b" {
		t.Errorf("text part 签名 = %+v, want sig-b", parts)
	}
}

func boolPtrSig(v bool) *bool { return &v }

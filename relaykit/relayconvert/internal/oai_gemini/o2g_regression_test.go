package oai_gemini

// 回归测试：openai→gemini 请求转换的 typed ContentPart 丢失、后缀 thinking 缺口，
// 以及 gemini→openai 流式的 tool_calls finish_reason（审查发现，修复后固化）。

import (
	"context"
	"strings"
	"testing"

	"github.com/qianfree/team-api/relaykit/dto"
	"github.com/qianfree/team-api/relaykit/relayconvert/convmeta"
)

// TestOpenAIToGeminiRequestConverter_TypedContentParts 回归：链式转换第一跳
// （claude→openai / responses→openai）产出的 typed []dto.ContentPart 此前命中不了
// convertUserParts 的任何分支，整条 user 消息（含文字）被静默丢弃。
func TestOpenAIToGeminiRequestConverter_TypedContentParts(t *testing.T) {
	req := &dto.GeneralOpenAIRequest{
		Messages: []dto.Message{
			{
				Role: "user",
				Content: []dto.ContentPart{
					{Type: "text", Text: "hello"},
					{Type: "image_url", ImageURL: &dto.ImageURL{URL: "data:image/png;base64,aGk="}},
				},
			},
		},
	}

	result, err := (&OpenAIToGeminiRequestConverter{}).ConvertRequest(context.Background(), nil, req)
	if err != nil {
		t.Fatalf("ConvertRequest: %v", err)
	}
	geminiReq := result.(*dto.GeminiChatRequest)

	if len(geminiReq.Contents) != 1 {
		t.Fatalf("contents len = %d, want 1（typed parts 整条丢失回归）: %#v", len(geminiReq.Contents), geminiReq.Contents)
	}
	parts := geminiReq.Contents[0].Parts
	if len(parts) != 2 {
		t.Fatalf("contents[0].parts = %#v, want 2", parts)
	}
	if parts[0].Text != "hello" {
		t.Errorf("parts[0].text = %q, want \"hello\"", parts[0].Text)
	}
	if parts[1].InlineData == nil || parts[1].InlineData.Data != "aGk=" || parts[1].InlineData.MimeType != "image/png" {
		t.Errorf("parts[1].inlineData = %#v, want data=aGk= mimeType=image/png", parts[1].InlineData)
	}
}

// TestOpenAIToGeminiRequestConverter_ThinkingSuffix 回归：模型名 -thinking 后缀此前
// 在桥接路径失效（gemini adaptor 的 injectGeminiThinking 被跳过且转换器无后缀处理）。
func TestOpenAIToGeminiRequestConverter_ThinkingSuffix(t *testing.T) {
	maxTokens := 8192
	req := &dto.GeneralOpenAIRequest{
		MaxTokens: &maxTokens,
		Messages:  []dto.Message{{Role: "user", Content: "hi"}},
	}
	info := &convmeta.Values{
		UpstreamModelName: "gemini-2.5-pro-thinking",
		Options: &convmeta.Options{
			Gemini: convmeta.GeminiOptions{
				ThinkingAdapterEnabled:                true,
				ThinkingAdapterBudgetTokensPercentage: 0.5,
			},
		},
	}

	result, err := (&OpenAIToGeminiRequestConverter{}).ConvertRequest(context.Background(), info, req)
	if err != nil {
		t.Fatalf("ConvertRequest: %v", err)
	}
	geminiReq := result.(*dto.GeminiChatRequest)

	tc := geminiReq.GenerationConfig.ThinkingConfig
	if tc == nil {
		t.Fatal("-thinking 后缀未注入 thinkingConfig（桥接路径后缀失效回归）")
	}
	if !tc.IncludeThoughts {
		t.Error("thinkingConfig.includeThoughts = false, want true")
	}
	if tc.ThinkingBudget == nil || *tc.ThinkingBudget != 4096 {
		t.Errorf("thinkingBudget = %v, want 4096（8192×0.5）", tc.ThinkingBudget)
	}
}

// TestGeminiToOpenAIStream_FunctionCallFinishReason 回归：发出过 functionCall 的流，
// 最终 finish_reason 须为 tool_calls（Gemini 的 STOP 直接映射成 stop 会让以
// finish_reason 为判据的 agent 客户端不执行工具）。对齐非流式的强制修正。
func TestGeminiToOpenAIStream_FunctionCallFinishReason(t *testing.T) {
	ss := strings.Join([]string{
		`data: {"candidates":[{"content":{"parts":[{"functionCall":{"functionName":"lookup","arguments":{"a":1}}}]}}]}`,
		`data: {"candidates":[{"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":5,"candidatesTokenCount":7,"totalTokenCount":12}}`,
		"",
	}, "\n")

	var finishReason *string
	err := (&GeminiToOpenAIStreamConverter{}).ConvertStreamResponse(
		context.Background(), nil, strings.NewReader(ss), func(chunk any) error {
			sc, ok := chunk.(*dto.ChatCompletionStreamResponse)
			if !ok || len(sc.Choices) == 0 {
				return nil
			}
			if fr := sc.Choices[0].FinishReason; fr != nil {
				finishReason = fr
			}
			return nil
		})
	if err != nil {
		t.Fatalf("ConvertStreamResponse: %v", err)
	}
	if finishReason == nil {
		t.Fatal("未收到带 finish_reason 的结束 chunk")
	}
	if *finishReason != "tool_calls" {
		t.Errorf("finish_reason = %q, want \"tool_calls\"", *finishReason)
	}
}

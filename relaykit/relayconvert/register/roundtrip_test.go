package register

import (
	"context"
	"testing"

	"github.com/qianfree/team-api/relaykit/dto"
	"github.com/qianfree/team-api/relaykit/relayconvert"
	"github.com/qianfree/team-api/relaykit/relayconvert/convmeta"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// sp 返回字符串指针（ClaudeContentBlock.Text 为 *string）。
func sp(s string) *string { return &s }

// TestRegisteredConverters_Roundtrip 经注册表实际调用各内置转换器的 Req.Convert / Resp.Convert，
// 验证 register 把正确的转换器结构体接进了正确的 spec（覆盖各 registerOpenAITo* 的响应侧 adapter 闭包，
// 这些闭包仅被 spec.Resp.Convert 调用时才执行，单纯断言 presence 无法覆盖）。
func TestRegisteredConverters_Roundtrip(t *testing.T) {
	ctx := context.Background()
	openaiReq := &dto.GeneralOpenAIRequest{
		Model:    "gpt-4",
		Messages: []dto.Message{{Role: "user", Content: "hi"}},
	}

	meta := func(upstream string) *convmeta.Values {
		return &convmeta.Values{
			ChannelMetaAttached: true,
			OriginModelName:     "gpt-4",
			UpstreamModelName:   upstream,
			Options: &convmeta.Options{
				Claude: convmeta.ClaudeOptions{DefaultMaxTokens: func(string) int { return 4096 }},
			},
		}
	}

	cases := []struct {
		name          string
		converterID   string
		upstream      string
		reqInput      any        // 请求侧输入（默认 openaiReq，Responses 入站方向不同）
		nativeResp    any        // 上游原生响应（响应侧输入）
		respResultPtr func() any // 响应侧期望类型断言（非 chat 方向）
	}{
		{
			name: "Claude", converterID: relayconvert.ConverterOpenAIChatToClaudeMessages, upstream: "claude-3-5-sonnet-20241022",
			nativeResp: &dto.ClaudeResponse{
				ID: "msg_1", Type: "message", Role: "assistant", Model: "claude-x", StopReason: "end_turn",
				Content: []dto.ClaudeContentBlock{{Type: "text", Text: sp("hi")}},
				Usage:   &dto.ClaudeUsage{InputTokens: 5, OutputTokens: 3},
			},
		},
		{
			name: "Gemini", converterID: relayconvert.ConverterOpenAIChatToGeminiContent, upstream: "gemini-2.0-flash",
			nativeResp: &dto.GeminiChatResponse{
				Candidates: []dto.GeminiCandidate{{
					Content:      &dto.GeminiContent{Role: "model", Parts: []dto.GeminiPart{{Text: "hi"}}},
					FinishReason: "STOP",
				}},
				UsageMetadata: &dto.GeminiUsageMetadata{PromptTokenCount: 5, CandidatesTokenCount: 3},
			},
		},
		{
			name: "Coze", converterID: relayconvert.ConverterOpenAIChatToCoze, upstream: "bot-123",
			// Coze 上游始终为 SSE，非流式客户端也由桥接缓冲整段后交给转换器解析。
			nativeResp: []byte("event: conversation.message.completed\n" +
				`data: {"role":"assistant","type":"answer","content":"hi"}` + "\n"),
		},
		{
			name: "Dify", converterID: relayconvert.ConverterOpenAIChatToDify, upstream: "dify-bot",
			nativeResp: &dto.DifyBlockingResponse{
				Answer:   "hi",
				Metadata: dto.DifyMeta{Usage: dto.DifyUsage{TotalTokens: 8, PromptTokens: 5, CompletionTokens: 3}},
			},
		},
		{
			name: "Ollama", converterID: relayconvert.ConverterOpenAIChatToOllama, upstream: "llama3",
			nativeResp: &dto.OllamaChatResponse{
				Model: "llama3", Message: dto.OllamaMessage{Role: "assistant", Content: "hi"},
				Done: true, PromptEvalCount: 5, EvalCount: 3,
			},
		},
		// P1-R：Responses 方向的 Resp 侧（输出类型为对应客户端格式，非 chat）
		{
			name: "ResponsesInbound", converterID: relayconvert.ConverterOpenAIResponsesToOpenAIChat, upstream: "gpt-4o",
			reqInput: &dto.OpenAIResponsesRequest{Model: "gpt-4o", Input: []byte(`"hi"`), MaxOutputTokens: new(uint)},
			nativeResp: &dto.ChatCompletionResponse{
				ID: "c1", Object: "chat.completion", Created: 1730000000, Model: "gpt-4o",
				Choices: []dto.Choice{{Index: 0, Message: dto.Message{Role: "assistant", Content: "hi"}, FinishReason: "stop"}},
				Usage: dto.UsageWithDetails{PromptTokens: 1, CompletionTokens: 1, TotalTokens: 2},
			},
			respResultPtr: func() any { return &dto.OpenAIResponsesResponse{} },
		},
		{
			name: "ChatViaResponses", converterID: relayconvert.ConverterOpenAIChatToOpenAIResponses, upstream: "gpt-4o",
			nativeResp: &dto.OpenAIResponsesResponse{
				ID: "resp_1", Object: "response", Model: "gpt-4o", Status: []byte(`"completed"`),
				Output: []dto.ResponsesOutput{{Type: "message", ID: "m", Role: "assistant",
					Content: []dto.ResponsesOutputContent{{Type: "output_text", Text: "hi"}}}},
			},
			respResultPtr: func() any { return &dto.ChatCompletionResponse{} },
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			spec, ok := relayconvert.LookupTextConverter(tc.converterID)
			require.True(t, ok, "converter %q not registered", tc.converterID)
			require.NotNil(t, spec.Req.Convert, "missing Req.Convert")
			require.NotNil(t, spec.Resp.Convert, "missing Resp.Convert")

			m := meta(tc.upstream)

			// 请求侧：客户端格式 → 上游原生（默认 chat 入参，Responses 入站方向自带 reqInput）
			reqInput := tc.reqInput
			if reqInput == nil {
				reqInput = openaiReq
			}
			reqResult, err := spec.Req.Convert(ctx, m, reqInput)
			require.NoError(t, err, "Req.Convert failed")
			assert.NotNil(t, reqResult, "Req.Convert returned nil")

			// 响应侧：原生 → 客户端格式
			respResult, usage, err := spec.Resp.Convert(ctx, m, tc.nativeResp)
			require.NoError(t, err, "Resp.Convert failed")
			require.NotNil(t, respResult, "Resp.Convert returned nil")
			assert.Nil(t, usage, "adapter closure must discard usage (return nil)")

			if tc.respResultPtr != nil {
				// Responses 方向：输出类型断言
				expected := tc.respResultPtr()
				assert.IsType(t, expected, respResult, "Resp.Convert result type = %T", respResult)
			} else {
				_, isChatResp := respResult.(*dto.ChatCompletionResponse)
				assert.True(t, isChatResp, "Resp.Convert result type = %T, want *dto.ChatCompletionResponse", respResult)
			}
		})
	}
}

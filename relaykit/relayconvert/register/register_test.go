package register

import (
	"testing"

	"github.com/qianfree/team-api/relaykit/relayconvert"
	"github.com/qianfree/team-api/relaykit/types"
)

// TestBuiltinConvertersRegistered 验证内置转换器已在 init() 中注册进运行时注册表。
func TestBuiltinConvertersRegistered(t *testing.T) {
	cases := []struct {
		name        string
		converterID string
		wantReq     bool
		wantResp    bool
	}{
		{
			name:        "OpenAI→Claude",
			converterID: relayconvert.ConverterOpenAIChatToClaudeMessages,
			wantReq:     true,
			wantResp:    true,
		},
		{
			name:        "OpenAI→Gemini",
			converterID: relayconvert.ConverterOpenAIChatToGeminiContent,
			wantReq:     true,
			wantResp:    true,
		},
		{
			name:        "OpenAI→Coze",
			converterID: relayconvert.ConverterOpenAIChatToCoze,
			wantReq:     true,
			wantResp:    true,
		},
		{
			name:        "OpenAI→Dify",
			converterID: relayconvert.ConverterOpenAIChatToDify,
			wantReq:     true,
			wantResp:    true,
		},
		{
			name:        "OpenAI→Ollama",
			converterID: relayconvert.ConverterOpenAIChatToOllama,
			wantReq:     true,
			wantResp:    true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			spec, ok := relayconvert.LookupTextConverter(tc.converterID)
			if !ok {
				t.Fatalf("converter %q not registered", tc.converterID)
			}
			if tc.wantReq && spec.Req.Convert == nil {
				t.Errorf("converter %q missing request converter", tc.converterID)
			}
			if tc.wantResp && spec.Resp.Convert == nil {
				t.Errorf("converter %q missing response converter", tc.converterID)
			}
		})
	}
}

// TestBuiltinStreamConvertersRegistered 验证流式转换器已在 init() 中
// 经 RegisterStreamConverter 登记进流式注册表——覆盖全部 14 个已注册方向
// （原生 5 个 + P1-R/P2/P3 新增 9 个，与迁移文档转换矩阵流式列一致）。
func TestBuiltinStreamConvertersRegistered(t *testing.T) {
	cases := []struct {
		name   string
		from   types.RelayFormat
		to     types.RelayFormat
		wantID string
	}{
		{
			name:   "Claude→OpenAI",
			from:   types.RelayFormatClaude,
			to:     types.RelayFormatOpenAI,
			wantID: relayconvert.ConverterClaudeMessagesToOpenAIChatStream,
		},
		{
			name:   "Gemini→OpenAI",
			from:   types.RelayFormatGemini,
			to:     types.RelayFormatOpenAI,
			wantID: relayconvert.ResponseConverterGeminiChatToOAIChatStream,
		},
		{
			name:   "Coze→OpenAI",
			from:   types.RelayFormatCoze,
			to:     types.RelayFormatOpenAI,
			wantID: relayconvert.ResponseConverterCozeChatToOAIChatStream,
		},
		{
			name:   "Dify→OpenAI",
			from:   types.RelayFormatDify,
			to:     types.RelayFormatOpenAI,
			wantID: relayconvert.ResponseConverterDifyChatToOAIChatStream,
		},
		{
			name:   "Ollama→OpenAI",
			from:   types.RelayFormatOllama,
			to:     types.RelayFormatOpenAI,
			wantID: relayconvert.ResponseConverterOllamaChatToOAIChatStream,
		},
		// P1-R：chat 上游 → responses 客户端（codex 主路径）
		{
			name:   "OpenAI→Responses",
			from:   types.RelayFormatOpenAI,
			to:     types.RelayFormatOpenAIResponses,
			wantID: relayconvert.ConverterOpenAIChatToOpenAIResponsesStream,
		},
		// P0：claude 上游 → responses 客户端
		{
			name:   "Claude→Responses",
			from:   types.RelayFormatClaude,
			to:     types.RelayFormatOpenAIResponses,
			wantID: relayconvert.ConverterClaudeMessagesToOpenAIResponsesStream,
		},
		// P2 D1：openai 上游 → claude/gemini 客户端
		{
			name:   "OpenAI→Claude",
			from:   types.RelayFormatOpenAI,
			to:     types.RelayFormatClaude,
			wantID: relayconvert.ConverterOpenAIChatToClaudeMessagesStream,
		},
		{
			name:   "OpenAI→Gemini",
			from:   types.RelayFormatOpenAI,
			to:     types.RelayFormatGemini,
			wantID: relayconvert.ConverterOpenAIChatToGeminiContentStream,
		},
		// P2 D2：跨原生流式组合（io.Pipe 串联，ID 为 register.go 内字面量）
		{
			name:   "Claude→Gemini",
			from:   types.RelayFormatClaude,
			to:     types.RelayFormatGemini,
			wantID: "anthropic_messages_to_gemini_generate_content_stream",
		},
		{
			name:   "Gemini→Claude",
			from:   types.RelayFormatGemini,
			to:     types.RelayFormatClaude,
			wantID: "gemini_generate_content_to_anthropic_messages_stream",
		},
		{
			name:   "Gemini→Responses",
			from:   types.RelayFormatGemini,
			to:     types.RelayFormatOpenAIResponses,
			wantID: "gemini_generate_content_to_oai_responses_stream",
		},
		// P3：responses 上游 → claude/gemini 客户端（B 流式 + P2 流式转换器组合）
		{
			name:   "Responses→Claude",
			from:   types.RelayFormatOpenAIResponses,
			to:     types.RelayFormatClaude,
			wantID: "oai_responses_to_anthropic_messages_stream",
		},
		{
			name:   "Responses→Gemini",
			from:   types.RelayFormatOpenAIResponses,
			to:     types.RelayFormatGemini,
			wantID: "oai_responses_to_gemini_generate_content_stream",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fn, id, ok := relayconvert.LookupStreamConverter(tc.from, tc.to)
			if !ok {
				t.Fatalf("stream converter %s→%s not registered", tc.from, tc.to)
			}
			if fn == nil {
				t.Fatalf("stream converter %s→%s fn is nil", tc.from, tc.to)
			}
			if id != tc.wantID {
				t.Errorf("stream converter %s→%s ID = %q, want %q", tc.from, tc.to, id, tc.wantID)
			}
		})
	}
}

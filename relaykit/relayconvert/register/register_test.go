package register

import (
	"testing"

	"github.com/qianfree/team-api/relaykit/relayconvert"
	"github.com/qianfree/team-api/relaykit/types"
)

// TestBuiltinConvertersRegistered 验证阶段 3 的内置转换器已在 init() 中注册进运行时注册表。
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

// TestBuiltinStreamConvertersRegistered 验证阶段 4 Task4 的流式转换器已在 init() 中
// 经 RegisterStreamConverter 登记进流式注册表（Claude→OpenAI、Gemini→OpenAI 两个方向）。
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

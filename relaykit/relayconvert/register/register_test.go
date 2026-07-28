package register

import (
	"testing"

	"github.com/qianfree/team-api/relaykit/relayconvert"
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

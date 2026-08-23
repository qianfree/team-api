package common

import (
	"context"
	"testing"
)

func TestParseGeminiSafetySetting(t *testing.T) {
	if fn := parseGeminiSafetySetting(context.Background(), ""); fn != nil {
		t.Error("empty config should return nil (no safetySettings)")
	}
	if fn := parseGeminiSafetySetting(context.Background(), "   "); fn != nil {
		t.Error("blank config should return nil")
	}
	if fn := parseGeminiSafetySetting(context.Background(), "not-json"); fn != nil {
		t.Error("malformed JSON should return nil (config error must not block conversion)")
	}
	if fn := parseGeminiSafetySetting(context.Background(), "{}"); fn != nil {
		t.Error("empty map should return nil")
	}

	fn := parseGeminiSafetySetting(context.Background(), `{"HARM_CATEGORY_HARASSMENT":"BLOCK_ONLY_HIGH","HARM_CATEGORY_DANGEROUS_CONTENT":"BLOCK_NONE"}`)
	if fn == nil {
		t.Fatal("valid map should return lookup func")
	}
	if got := fn("HARM_CATEGORY_HARASSMENT"); got != "BLOCK_ONLY_HIGH" {
		t.Errorf("HARM_CATEGORY_HARASSMENT = %q, want BLOCK_ONLY_HIGH", got)
	}
	if got := fn("HARM_CATEGORY_DANGEROUS_CONTENT"); got != "BLOCK_NONE" {
		t.Errorf("HARM_CATEGORY_DANGEROUS_CONTENT = %q, want BLOCK_NONE", got)
	}
	if got := fn("UNKNOWN_CATEGORY"); got != "" {
		t.Errorf("unknown category should return empty string, got %q", got)
	}
}

func TestParsePreserveThinkingSuffix(t *testing.T) {
	if fn := parsePreserveThinkingSuffix(""); fn != nil {
		t.Error("empty config should return nil (strip suffix for all models)")
	}
	if fn := parsePreserveThinkingSuffix("  "); fn != nil {
		t.Error("blank config should return nil")
	}
	if fn := parsePreserveThinkingSuffix(",, ,"); fn != nil {
		t.Error("config with only separators should return nil")
	}

	fn := parsePreserveThinkingSuffix("gemini-2.5-pro, gpt-4*,claude-3-5-sonnet")
	if fn == nil {
		t.Fatal("valid list should return match func")
	}
	cases := []struct {
		model string
		want  bool
	}{
		{"gemini-2.5-pro", true},          // 精确匹配
		{"gpt-4o", true},                  // 前缀匹配
		{"gpt-4.1-mini", true},            // 前缀匹配
		{"claude-3-5-sonnet", true},       // 精确匹配（含空格 trim）
		{"gemini-2.5-pro-thinking", false}, // 精确项不前缀放行
		{"claude-3-opus", false},           // 未列入
	}
	for _, c := range cases {
		if got := fn(c.model); got != c.want {
			t.Errorf("preserve(%q) = %v, want %v", c.model, got, c.want)
		}
	}
}

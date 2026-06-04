package openai

import (
	"encoding/json"
	"testing"

	"github.com/qianfree/team-api/relay/dto"
)

func TestExtractClaudeSystemText(t *testing.T) {
	t.Run("plain string", func(t *testing.T) {
		if got := extractClaudeSystemText("you are helpful"); got != "you are helpful" {
			t.Errorf("got %q", got)
		}
	})
	t.Run("array of text blocks joined", func(t *testing.T) {
		sys := []any{
			map[string]any{"type": "text", "text": "line1"},
			map[string]any{"type": "text", "text": "line2"},
		}
		if got := extractClaudeSystemText(sys); got != "line1\nline2" {
			t.Errorf("got %q, want \"line1\\nline2\"", got)
		}
	})
	t.Run("non-text blocks ignored", func(t *testing.T) {
		sys := []any{
			map[string]any{"type": "text", "text": "keep"},
			map[string]any{"type": "image", "text": "drop"},
		}
		if got := extractClaudeSystemText(sys); got != "keep" {
			t.Errorf("got %q, want \"keep\"", got)
		}
	})
	t.Run("unknown type returns empty", func(t *testing.T) {
		if got := extractClaudeSystemText(42); got != "" {
			t.Errorf("got %q, want empty", got)
		}
	})
}

func TestC2oJoinParts(t *testing.T) {
	tests := []struct {
		in   []string
		want string
	}{
		{nil, ""},
		{[]string{}, ""},
		{[]string{"only"}, "only"},
		{[]string{"a", "b", "c"}, "a\nb\nc"},
	}
	for _, tt := range tests {
		if got := c2oJoinParts(tt.in); got != tt.want {
			t.Errorf("c2oJoinParts(%v) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestC2oConvertThinkingToReasoningEffort(t *testing.T) {
	tests := []struct {
		name   string
		budget *int
		want   string
	}{
		{"nil budget defaults medium", nil, "medium"},
		{"low boundary", intPtr(2048), "low"},
		{"low", intPtr(1000), "low"},
		{"medium boundary", intPtr(16384), "medium"},
		{"medium", intPtr(8000), "medium"},
		{"high", intPtr(16385), "high"},
		{"high large", intPtr(50000), "high"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := c2oConvertThinkingToReasoningEffort(&dto.ClaudeThinking{BudgetTokens: tt.budget})
			if got != tt.want {
				t.Errorf("budget=%v => %q, want %q", tt.budget, got, tt.want)
			}
		})
	}
}

func TestG2oConvertThinkingConfig(t *testing.T) {
	// 与 Claude 同阈值
	tests := []struct {
		budget *int
		want   string
	}{
		{nil, "medium"},
		{intPtr(2048), "low"},
		{intPtr(16384), "medium"},
		{intPtr(20000), "high"},
	}
	for _, tt := range tests {
		got := g2oConvertThinkingConfig(&dto.GeminiThinkingConfig{ThoughtBudget: tt.budget})
		if got != tt.want {
			t.Errorf("budget=%v => %q, want %q", tt.budget, got, tt.want)
		}
	}
}

func TestC2oConvertToolChoice(t *testing.T) {
	t.Run("nil", func(t *testing.T) {
		if got := c2oConvertToolChoice(nil); got != nil {
			t.Errorf("got %v, want nil", got)
		}
	})
	t.Run("string passthrough", func(t *testing.T) {
		if got := c2oConvertToolChoice("auto"); got != "auto" {
			t.Errorf("got %v", got)
		}
	})
	t.Run("type mappings", func(t *testing.T) {
		cases := map[string]string{"auto": "auto", "any": "required", "none": "none"}
		for in, want := range cases {
			got := c2oConvertToolChoice(map[string]any{"type": in})
			if got != want {
				t.Errorf("type=%q => %v, want %q", in, got, want)
			}
		}
	})
	t.Run("specific tool maps to function", func(t *testing.T) {
		got := c2oConvertToolChoice(map[string]any{"type": "tool", "name": "get_weather"})
		m, ok := got.(map[string]any)
		if !ok {
			t.Fatalf("expected map, got %T", got)
		}
		if m["type"] != "function" {
			t.Errorf("type = %v, want function", m["type"])
		}
		fn, ok := m["function"].(map[string]any)
		if !ok || fn["name"] != "get_weather" {
			t.Errorf("function = %v, want name=get_weather", m["function"])
		}
	})
	t.Run("tool without name falls back to required", func(t *testing.T) {
		if got := c2oConvertToolChoice(map[string]any{"type": "tool"}); got != "required" {
			t.Errorf("got %v, want required", got)
		}
	})
}

func TestG2oMapRole(t *testing.T) {
	tests := map[string]string{
		"model": "assistant",
		"user":  "user",
		"tool":  "tool",
	}
	for in, want := range tests {
		if got := g2oMapRole(in); got != want {
			t.Errorf("g2oMapRole(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestC2rContentToString(t *testing.T) {
	if got := c2rContentToString(nil); got != "" {
		t.Errorf("nil => %q, want empty", got)
	}
	if got := c2rContentToString("hello"); got != "hello" {
		t.Errorf("string => %q", got)
	}
	// 非字符串 -> JSON 编码
	got := c2rContentToString([]any{map[string]any{"type": "text", "text": "x"}})
	var back []any
	if err := json.Unmarshal([]byte(got), &back); err != nil {
		t.Errorf("non-string content should be JSON-encoded, got %q (%v)", got, err)
	}
}

func TestC2rGetMaxTokens(t *testing.T) {
	tests := []struct {
		name    string
		maxTok  *int
		maxComp *int
		want    int
	}{
		{"both nil", nil, nil, 0},
		{"max_tokens only", intPtr(100), nil, 100},
		{"max_completion larger wins", intPtr(100), intPtr(200), 200},
		{"max_completion smaller keeps max_tokens", intPtr(300), intPtr(50), 300},
		{"zero max_tokens ignored", intPtr(0), nil, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := dto.GeneralOpenAIRequest{MaxTokens: tt.maxTok, MaxCompletionTokens: tt.maxComp}
			if got := c2rGetMaxTokens(req); got != tt.want {
				t.Errorf("got %d, want %d", got, tt.want)
			}
		})
	}
}

func TestConvertClaudeUserMessage_StringContent(t *testing.T) {
	msgs := convertClaudeUserMessage(dto.ClaudeMessage{Role: "user", Content: "hi there"})
	if len(msgs) != 1 {
		t.Fatalf("got %d messages, want 1", len(msgs))
	}
	if msgs[0].Role != "user" || msgs[0].Content != "hi there" {
		t.Errorf("got %+v", msgs[0])
	}
}

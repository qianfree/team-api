package helper

import "testing"

func TestParseThinkingSuffix(t *testing.T) {
	tests := []struct {
		input       string
		wantBase    string
		wantThink   bool
		wantEffort  string
		wantNoThink bool
	}{
		{"gpt-4o", "gpt-4o", false, "", false},
		{"claude-3-5-sonnet-thinking", "claude-3-5-sonnet", true, "", false},
		{"gpt-4o-nothinking", "gpt-4o", false, "", true},
		{"claude-3-opus-high", "claude-3-opus", false, "high", false},
		{"gpt-4o-low", "gpt-4o", false, "low", false},
		{"gemini-2.5-pro-medium", "gemini-2.5-pro", false, "medium", false},
		{"o3-xhigh", "o3", false, "xhigh", false},
		{"o3-max", "o3", false, "max", false},
		{"gemini-2.5-flash-minimal", "gemini-2.5-flash", false, "minimal", false},
		{"claude-opus-4-20250514-thinking", "claude-opus-4-20250514", true, "", false},
		{"gpt-4o-0613", "gpt-4o-0613", false, "", false}, // not a thinking suffix
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			info := ParseThinkingSuffix(tt.input)
			if info.BaseModel != tt.wantBase {
				t.Errorf("BaseModel = %q, want %q", info.BaseModel, tt.wantBase)
			}
			if info.IsThinking != tt.wantThink {
				t.Errorf("IsThinking = %v, want %v", info.IsThinking, tt.wantThink)
			}
			if info.EffortLevel != tt.wantEffort {
				t.Errorf("EffortLevel = %q, want %q", info.EffortLevel, tt.wantEffort)
			}
			if info.IsNoThinking != tt.wantNoThink {
				t.Errorf("IsNoThinking = %v, want %v", info.IsNoThinking, tt.wantNoThink)
			}
		})
	}
}

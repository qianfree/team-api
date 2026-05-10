package helper

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/qianfree/team-api/relay/common"
	"github.com/qianfree/team-api/relay/constant"
)

func TestInjectSystemPromptOpenAI(t *testing.T) {
	tests := []struct {
		name        string
		body        string
		prompt      string
		override    bool
		wantSystem  string // expected system message content
		wantSystems int    // expected number of system messages
	}{
		{
			name:        "prepend to empty messages",
			body:        `{"model":"gpt-4o","messages":[]}`,
			prompt:      "You are helpful.",
			override:    false,
			wantSystem:  "You are helpful.",
			wantSystems: 1,
		},
		{
			name:        "prepend to existing system message",
			body:        `{"model":"gpt-4o","messages":[{"role":"system","content":"Be safe."},{"role":"user","content":"Hi"}]}`,
			prompt:      "You are helpful.",
			override:    false,
			wantSystem:  "You are helpful.\nBe safe.",
			wantSystems: 1,
		},
		{
			name:        "override existing system message",
			body:        `{"model":"gpt-4o","messages":[{"role":"system","content":"Be safe."},{"role":"user","content":"Hi"}]}`,
			prompt:      "You are helpful.",
			override:    true,
			wantSystem:  "You are helpful.",
			wantSystems: 1,
		},
		{
			name:        "insert before non-system first message",
			body:        `{"model":"gpt-4o","messages":[{"role":"user","content":"Hi"}]}`,
			prompt:      "You are helpful.",
			override:    false,
			wantSystem:  "You are helpful.",
			wantSystems: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info := &common.RelayInfo{
				ChannelMeta: &common.ChannelMeta{
					ChannelType: int(constant.ProviderOpenAI),
					Settings: common.ChannelSettings{
						SystemPrompt:         tt.prompt,
						SystemPromptOverride: tt.override,
					},
				},
			}
			result := InjectSystemPrompt([]byte(tt.body), info)

			var req map[string]json.RawMessage
			if err := json.Unmarshal(result, &req); err != nil {
				t.Fatalf("invalid JSON: %v", err)
			}

			var messages []struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			}
			if err := json.Unmarshal(req["messages"], &messages); err != nil {
				t.Fatalf("invalid messages: %v", err)
			}

			systemCount := 0
			for _, m := range messages {
				if m.Role == "system" {
					systemCount++
					if m.Content != tt.wantSystem {
						t.Errorf("system content = %q, want %q", m.Content, tt.wantSystem)
					}
				}
			}
			if systemCount != tt.wantSystems {
				t.Errorf("system message count = %d, want %d", systemCount, tt.wantSystems)
			}
		})
	}
}

func TestInjectSystemPromptClaude(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		prompt     string
		override   bool
		wantSystem string
	}{
		{
			name:       "set system when nil",
			body:       `{"model":"claude-3","messages":[{"role":"user","content":"Hi"}],"max_tokens":1024}`,
			prompt:     "You are helpful.",
			override:   false,
			wantSystem: "You are helpful.",
		},
		{
			name:       "override existing system",
			body:       `{"model":"claude-3","system":"Be safe.","messages":[],"max_tokens":1024}`,
			prompt:     "You are helpful.",
			override:   true,
			wantSystem: "You are helpful.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info := &common.RelayInfo{
				ChannelMeta: &common.ChannelMeta{
					ChannelType: int(constant.ProviderClaude),
					Settings: common.ChannelSettings{
						SystemPrompt:         tt.prompt,
						SystemPromptOverride: tt.override,
					},
				},
			}
			result := InjectSystemPrompt([]byte(tt.body), info)

			var req map[string]json.RawMessage
			if err := json.Unmarshal(result, &req); err != nil {
				t.Fatalf("invalid JSON: %v", err)
			}

			var system string
			if err := json.Unmarshal(req["system"], &system); err != nil {
				t.Fatalf("invalid system: %v", err)
			}
			if system != tt.wantSystem {
				t.Errorf("system = %q, want %q", system, tt.wantSystem)
			}
		})
	}
}

func TestInjectSystemPromptGemini(t *testing.T) {
	tests := []struct {
		name        string
		body        string
		prompt      string
		override    bool
		wantInInstr bool
	}{
		{
			name:        "set systemInstruction when nil",
			body:        `{"contents":[{"role":"user","parts":[{"text":"Hi"}]}]}`,
			prompt:      "You are helpful.",
			override:    false,
			wantInInstr: true,
		},
		{
			name:        "override existing systemInstruction",
			body:        `{"systemInstruction":{"parts":[{"text":"Be safe."}]},"contents":[]}`,
			prompt:      "You are helpful.",
			override:    true,
			wantInInstr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info := &common.RelayInfo{
				ChannelMeta: &common.ChannelMeta{
					ChannelType: int(constant.ProviderGemini),
					Settings: common.ChannelSettings{
						SystemPrompt:         tt.prompt,
						SystemPromptOverride: tt.override,
					},
				},
			}
			result := InjectSystemPrompt([]byte(tt.body), info)

			if !strings.Contains(string(result), "systemInstruction") {
				t.Error("expected systemInstruction in result")
			}
			if !strings.Contains(string(result), tt.prompt) {
				t.Errorf("expected prompt %q in result", tt.prompt)
			}
		})
	}
}

func TestInjectSystemPromptEmpty(t *testing.T) {
	body := `{"model":"gpt-4o","messages":[]}`
	info := &common.RelayInfo{
		ChannelMeta: &common.ChannelMeta{
			ChannelType: int(constant.ProviderOpenAI),
			Settings:    common.ChannelSettings{},
		},
	}
	result := string(InjectSystemPrompt([]byte(body), info))
	if result != body {
		t.Errorf("expected unchanged body when SystemPrompt is empty, got: %s", result)
	}
}

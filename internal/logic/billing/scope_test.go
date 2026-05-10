package billing

import (
	"testing"
)

func TestCheckScope(t *testing.T) {
	tests := []struct {
		name      string
		scope     string
		relayMode string
		want      bool
	}{
		{"full allows everything", ScopeFull, "chat_completions", true},
		{"full allows embeddings", ScopeFull, "embeddings", true},
		{"empty scope allows everything", "", "chat_completions", true},
		{"chat_only allows chat", ScopeChatOnly, "chat_completions", true},
		{"chat_only allows completions", ScopeChatOnly, "completions", true},
		{"chat_only allows claude_messages", ScopeChatOnly, "claude_messages", true},
		{"chat_only denies embeddings", ScopeChatOnly, "embeddings", false},
		{"chat_only denies images", ScopeChatOnly, "images_generations", false},
		{"embeddings_only allows embeddings", ScopeEmbeddingsOnly, "embeddings", true},
		{"embeddings_only denies chat", ScopeEmbeddingsOnly, "chat_completions", false},
		{"images_only allows images", ScopeImagesOnly, "images_generations", true},
		{"images_only denies chat", ScopeImagesOnly, "chat_completions", false},
		{"read_only denies all with mode", ScopeReadOnly, "chat_completions", false},
		{"read_only allows empty mode (models GET)", ScopeReadOnly, "", true},
		{"audio_only allows audio", ScopeAudioOnly, "audio", true},
		{"audio_only denies chat", ScopeAudioOnly, "chat_completions", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CheckScope(tt.scope, tt.relayMode)
			if got != tt.want {
				t.Errorf("CheckScope(%q, %q) = %v, want %v", tt.scope, tt.relayMode, got, tt.want)
			}
		})
	}
}

func TestCheckScopeCustom(t *testing.T) {
	// custom scope: comma-separated list
	got := CheckScope("chat_completions,embeddings", "chat_completions")
	if !got {
		t.Error("expected custom scope to allow chat_completions")
	}

	got = CheckScope("chat_completions,embeddings", "embeddings")
	if !got {
		t.Error("expected custom scope to allow embeddings")
	}

	got = CheckScope("chat_completions,embeddings", "images_generations")
	if got {
		t.Error("expected custom scope to deny images_generations")
	}
}

func TestCheckIPWhitelist(t *testing.T) {
	tests := []struct {
		name      string
		whitelist string
		clientIP  string
		want      bool
	}{
		{"empty whitelist allows all", "", "192.168.1.1", true},
		{"exact match", "192.168.1.1,10.0.0.1", "192.168.1.1", true},
		{"second entry match", "192.168.1.1,10.0.0.1", "10.0.0.1", true},
		{"no match", "192.168.1.1,10.0.0.1", "172.16.0.1", false},
		{"with port", "192.168.1.1", "192.168.1.1:8080", true},
		{"whitespace tolerant", " 192.168.1.1 , 10.0.0.1 ", "192.168.1.1", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CheckIPWhitelist(tt.whitelist, tt.clientIP)
			if got != tt.want {
				t.Errorf("CheckIPWhitelist(%q, %q) = %v, want %v", tt.whitelist, tt.clientIP, got, tt.want)
			}
		})
	}
}

func TestIsReadOnlyScope(t *testing.T) {
	if !IsReadOnlyScope(ScopeReadOnly) {
		t.Error("expected read_only to be read only")
	}
	if IsReadOnlyScope(ScopeFull) {
		t.Error("expected full not to be read only")
	}
}

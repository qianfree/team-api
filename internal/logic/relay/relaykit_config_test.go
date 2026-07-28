package relay

import (
	"testing"
)

func TestIsChannelInProviders(t *testing.T) {
	tests := []struct {
		name        string
		providerKey string
		providers   map[string]struct{}
		want        bool
	}{
		{
			name:        "命中白名单",
			providerKey: "anthropic",
			providers:   map[string]struct{}{"anthropic": {}, "openai": {}},
			want:        true,
		},
		{
			name:        "不在白名单",
			providerKey: "gemini",
			providers:   map[string]struct{}{"anthropic": {}, "openai": {}},
			want:        false,
		},
		{
			name:        "空 providerKey",
			providerKey: "",
			providers:   map[string]struct{}{"anthropic": {}},
			want:        false,
		},
		{
			name:        "空白名单集合",
			providerKey: "anthropic",
			providers:   nil,
			want:        false,
		},
		{
			name:        "空白名单（enabled=true 但 providers 为空应禁用）",
			providerKey: "anthropic",
			providers:   map[string]struct{}{},
			want:        false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isChannelInProviders(tt.providerKey, tt.providers); got != tt.want {
				t.Errorf("isChannelInProviders(%q, %v) = %v, want %v", tt.providerKey, tt.providers, got, tt.want)
			}
		})
	}
}

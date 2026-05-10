package helper

import (
	"testing"

	"github.com/qianfree/team-api/relay/common"
)

func TestSanitizeFields(t *testing.T) {
	tests := []struct {
		name     string
		body     string
		settings common.ChannelSettings
		want     string
	}{
		{
			name:     "no fields to remove",
			body:     `{"model":"gpt-4o","messages":[]}`,
			settings: common.ChannelSettings{},
			want:     `{"model":"gpt-4o","messages":[]}`,
		},
		{
			name:     "remove service_tier by default",
			body:     `{"model":"gpt-4o","service_tier":"auto","messages":[]}`,
			settings: common.ChannelSettings{},
			want:     `{"messages":[],"model":"gpt-4o"}`,
		},
		{
			name:     "keep service_tier when allowed",
			body:     `{"model":"gpt-4o","service_tier":"auto","messages":[]}`,
			settings: common.ChannelSettings{AllowServiceTier: true},
			want:     `{"model":"gpt-4o","service_tier":"auto","messages":[]}`,
		},
		{
			name:     "remove inference_geo by default",
			body:     `{"model":"claude-3","inference_geo":"us","messages":[]}`,
			settings: common.ChannelSettings{},
			want:     `{"messages":[],"model":"claude-3"}`,
		},
		{
			name:     "remove speed by default",
			body:     `{"model":"claude-3","speed":"fast","messages":[]}`,
			settings: common.ChannelSettings{},
			want:     `{"messages":[],"model":"claude-3"}`,
		},
		{
			name:     "remove store when disabled",
			body:     `{"model":"gpt-4o","store":true,"messages":[]}`,
			settings: common.ChannelSettings{DisableStore: true},
			want:     `{"messages":[],"model":"gpt-4o"}`,
		},
		{
			name:     "keep store by default",
			body:     `{"model":"gpt-4o","store":true,"messages":[]}`,
			settings: common.ChannelSettings{},
			want:     `{"model":"gpt-4o","store":true,"messages":[]}`,
		},
		{
			name:     "remove safety_identifier by default",
			body:     `{"model":"gpt-4o","safety_identifier":"user123","messages":[]}`,
			settings: common.ChannelSettings{},
			want:     `{"messages":[],"model":"gpt-4o"}`,
		},
		{
			name:     "remove multiple fields",
			body:     `{"model":"gpt-4o","service_tier":"auto","store":true,"safety_identifier":"u1","messages":[]}`,
			settings: common.ChannelSettings{DisableStore: true},
			want:     `{"messages":[],"model":"gpt-4o"}`,
		},
		{
			name:     "skip when PassThroughBodyEnabled",
			body:     `{"model":"gpt-4o","service_tier":"auto","messages":[]}`,
			settings: common.ChannelSettings{PassThroughBodyEnabled: true},
			want:     `{"model":"gpt-4o","service_tier":"auto","messages":[]}`,
		},
		{
			name:     "invalid JSON returns as-is",
			body:     `{invalid}`,
			settings: common.ChannelSettings{},
			want:     `{invalid}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := string(SanitizeFields([]byte(tt.body), tt.settings))
			if got != tt.want {
				t.Errorf("SanitizeFields() = %s, want %s", got, tt.want)
			}
		})
	}
}

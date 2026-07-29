package helper

import (
	"testing"

	"github.com/qianfree/team-api/relay/constant"
)

// 阶段 7 安全清理：provider 映射的权威单测集中在本包。
// 迁移前 handler/passthrough_test.go 与 relaykit_bridge/response_test.go 各有一份重复表测，
// 现逻辑已合并到 helper.ProviderNativeFormat / ProviderKeyForChannelType，重复副本随之删除。

func TestProviderNativeFormat(t *testing.T) {
	tests := []struct {
		name         string
		providerType int
		want         constant.RelayFormat
	}{
		{"claude", int(constant.ProviderClaude), constant.RelayFormatClaude},
		{"gemini", int(constant.ProviderGemini), constant.RelayFormatGemini},
		{"coze", int(constant.ProviderCoze), constant.RelayFormatCoze},
		{"dify", int(constant.ProviderDify), constant.RelayFormatDify},
		{"ollama", int(constant.ProviderOllama), constant.RelayFormatOllama},
		{"openai", int(constant.ProviderOpenAI), constant.RelayFormatOpenAI},
		// OpenAI 兼容供应商无原生格式，默认走 OpenAI
		{"deepseek", int(constant.ProviderDeepSeek), constant.RelayFormatOpenAI},
		{"azure", int(constant.ProviderAzure), constant.RelayFormatOpenAI},
		{"aws", int(constant.ProviderAWS), constant.RelayFormatOpenAI},
		{"vertex", int(constant.ProviderVertex), constant.RelayFormatOpenAI},
		{"ali", int(constant.ProviderAli), constant.RelayFormatOpenAI},
		{"unknown_default", 999999, constant.RelayFormatOpenAI},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ProviderNativeFormat(tt.providerType); got != tt.want {
				t.Errorf("ProviderNativeFormat(%d) = %s, want %s", tt.providerType, got, tt.want)
			}
		})
	}
}

func TestProviderKeyForChannelType(t *testing.T) {
	tests := []struct {
		name         string
		providerType int
		want         string
	}{
		{"claude", int(constant.ProviderClaude), "claude"},
		{"gemini", int(constant.ProviderGemini), "gemini"},
		{"openai", int(constant.ProviderOpenAI), "openai"},
		{"coze", int(constant.ProviderCoze), "coze"},
		{"dify", int(constant.ProviderDify), "dify"},
		{"ollama", int(constant.ProviderOllama), "ollama"},
		// OpenAI 兼容供应商无 relaykit 转换器 → 空 key（等价于不启用）
		{"deepseek", int(constant.ProviderDeepSeek), ""},
		{"ali", int(constant.ProviderAli), ""},
		{"unknown_default", 999999, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ProviderKeyForChannelType(tt.providerType); got != tt.want {
				t.Errorf("ProviderKeyForChannelType(%d) = %q, want %q", tt.providerType, got, tt.want)
			}
		})
	}
}

package handler

import (
	"testing"

	"github.com/qianfree/team-api/relay/common"
	"github.com/qianfree/team-api/relay/constant"
)

func TestProviderNativeFormat(t *testing.T) {
	tests := []struct {
		providerType int
		want         constant.RelayFormat
	}{
		{int(constant.ProviderClaude), constant.RelayFormatClaude},
		{int(constant.ProviderGemini), constant.RelayFormatGemini},
		{int(constant.ProviderOpenAI), constant.RelayFormatOpenAI},
		{int(constant.ProviderDeepSeek), constant.RelayFormatOpenAI},
		{int(constant.ProviderAzure), constant.RelayFormatOpenAI},
		{int(constant.ProviderAWS), constant.RelayFormatOpenAI},
		{int(constant.ProviderVertex), constant.RelayFormatOpenAI},
		{int(constant.ProviderAli), constant.RelayFormatOpenAI},
	}
	for _, tt := range tests {
		got := providerNativeFormat(tt.providerType)
		if got != tt.want {
			t.Errorf("providerNativeFormat(%d) = %s, want %s", tt.providerType, got, tt.want)
		}
	}
}

func TestCanPassThrough_ExplicitEnabled(t *testing.T) {
	info := &common.RelayInfo{
		InboundFormat: constant.RelayFormatClaude,
		ChannelMeta: &common.ChannelMeta{
			ChannelType: int(constant.ProviderOpenAI),
			Settings: common.ChannelSettings{
				PassThroughBodyEnabled: true,
			},
		},
	}
	if !canPassThrough(info) {
		t.Error("should pass through when explicitly enabled")
	}
}

func TestCanPassThrough_ExplicitEnabledIgnoresFormatMismatch(t *testing.T) {
	info := &common.RelayInfo{
		InboundFormat: constant.RelayFormatOpenAI,
		ChannelMeta: &common.ChannelMeta{
			ChannelType:   int(constant.ProviderClaude),
			IsModelMapped: true,
			Settings: common.ChannelSettings{
				PassThroughBodyEnabled: true,
			},
		},
	}
	if !canPassThrough(info) {
		t.Error("explicit PassThroughBodyEnabled should bypass format/mapping checks")
	}
}

func TestCanPassThrough_AutoDetect_FormatMatch(t *testing.T) {
	info := &common.RelayInfo{
		InboundFormat: constant.RelayFormatOpenAI,
		ChannelMeta: &common.ChannelMeta{
			ChannelType:   int(constant.ProviderOpenAI),
			IsModelMapped: false,
			Settings:      common.ChannelSettings{},
		},
	}
	if !canPassThrough(info) {
		t.Error("should auto-detect pass through for OpenAI client → OpenAI upstream")
	}
}

func TestCanPassThrough_AutoDetect_ClaudeMatch(t *testing.T) {
	info := &common.RelayInfo{
		InboundFormat: constant.RelayFormatClaude,
		ChannelMeta: &common.ChannelMeta{
			ChannelType:   int(constant.ProviderClaude),
			IsModelMapped: false,
			Settings:      common.ChannelSettings{},
		},
	}
	if !canPassThrough(info) {
		t.Error("should auto-detect pass through for Claude client → Claude upstream")
	}
}

func TestCanPassThrough_AutoDetect_GeminiMatch(t *testing.T) {
	info := &common.RelayInfo{
		InboundFormat: constant.RelayFormatGemini,
		ChannelMeta: &common.ChannelMeta{
			ChannelType:   int(constant.ProviderGemini),
			IsModelMapped: false,
			Settings:      common.ChannelSettings{},
		},
	}
	if !canPassThrough(info) {
		t.Error("should auto-detect pass through for Gemini client → Gemini upstream")
	}
}

func TestCanPassThrough_AutoDetect_FormatMismatch(t *testing.T) {
	info := &common.RelayInfo{
		InboundFormat: constant.RelayFormatOpenAI,
		ChannelMeta: &common.ChannelMeta{
			ChannelType:   int(constant.ProviderClaude),
			IsModelMapped: false,
			Settings:      common.ChannelSettings{},
		},
	}
	if canPassThrough(info) {
		t.Error("should NOT pass through when OpenAI client → Claude upstream (format mismatch)")
	}
}

func TestCanPassThrough_AutoDetect_ModelMapped(t *testing.T) {
	info := &common.RelayInfo{
		InboundFormat: constant.RelayFormatOpenAI,
		ChannelMeta: &common.ChannelMeta{
			ChannelType:   int(constant.ProviderOpenAI),
			IsModelMapped: true,
			Settings:      common.ChannelSettings{},
		},
	}
	if canPassThrough(info) {
		t.Error("should NOT pass through when model mapping is needed")
	}
}

func TestCanPassThrough_AutoDetect_HasParamOverride(t *testing.T) {
	info := &common.RelayInfo{
		InboundFormat: constant.RelayFormatOpenAI,
		ChannelMeta: &common.ChannelMeta{
			ChannelType:   int(constant.ProviderOpenAI),
			IsModelMapped: false,
			Settings: common.ChannelSettings{
				ParamOverride: map[string]any{"temperature": 0.5},
			},
		},
	}
	if canPassThrough(info) {
		t.Error("should NOT pass through when ParamOverride is configured")
	}
}

func TestCanPassThrough_DeepSeekIsOpenAI(t *testing.T) {
	info := &common.RelayInfo{
		InboundFormat: constant.RelayFormatOpenAI,
		ChannelMeta: &common.ChannelMeta{
			ChannelType:   int(constant.ProviderDeepSeek),
			IsModelMapped: false,
			Settings:      common.ChannelSettings{},
		},
	}
	if !canPassThrough(info) {
		t.Error("DeepSeek is OpenAI-compatible, should pass through for OpenAI format requests")
	}
}

package handler

import (
	"testing"

	"github.com/qianfree/team-api/relay/common"
	"github.com/qianfree/team-api/relay/constant"
	"github.com/qianfree/team-api/relaykit/relayconvert"
)

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

// TestRelaykitRequestConverterID 验证请求侧 converter ID 解析，含阶段 5 新供应商
// 与 Ollama 仅 chat 路径启用的 RelayMode 守卫。
func TestRelaykitRequestConverterID(t *testing.T) {
	tests := []struct {
		name      string
		inbound   constant.RelayFormat
		upstream  constant.RelayFormat
		relayMode int
		want      string
	}{
		{"OpenAI→Claude", constant.RelayFormatOpenAI, constant.RelayFormatClaude, int(constant.RelayModeChatCompletions), relayconvert.ConverterOpenAIChatToClaudeMessages},
		{"OpenAI→Gemini", constant.RelayFormatOpenAI, constant.RelayFormatGemini, int(constant.RelayModeChatCompletions), relayconvert.ConverterOpenAIChatToGeminiContent},
		{"OpenAI→Coze", constant.RelayFormatOpenAI, constant.RelayFormatCoze, int(constant.RelayModeChatCompletions), relayconvert.ConverterOpenAIChatToCoze},
		{"OpenAI→Dify", constant.RelayFormatOpenAI, constant.RelayFormatDify, int(constant.RelayModeChatCompletions), relayconvert.ConverterOpenAIChatToDify},
		{"OpenAI→Ollama chat", constant.RelayFormatOpenAI, constant.RelayFormatOllama, int(constant.RelayModeChatCompletions), relayconvert.ConverterOpenAIChatToOllama},
		{"OpenAI→Ollama generate 不迁移", constant.RelayFormatOpenAI, constant.RelayFormatOllama, int(constant.RelayModeCompletions), ""},
		{"OpenAI→Ollama embedding 不迁移", constant.RelayFormatOpenAI, constant.RelayFormatOllama, int(constant.RelayModeEmbeddings), ""},
		{"同格式不转换", constant.RelayFormatOpenAI, constant.RelayFormatOpenAI, int(constant.RelayModeChatCompletions), ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := relaykitRequestConverterID(tt.inbound, tt.upstream, tt.relayMode)
			if got != tt.want {
				t.Errorf("relaykitRequestConverterID(%s,%s,%d) = %q, want %q", tt.inbound, tt.upstream, tt.relayMode, got, tt.want)
			}
		})
	}
}

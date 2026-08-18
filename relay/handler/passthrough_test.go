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

// TestCanPassThrough_MultiNative_AllFormats 多协议原生透传渠道（New API / Sub2API）
// 上游同时支持 OpenAI/Claude/Gemini，三种入站格式在无改写时都应原样直连转发。
func TestCanPassThrough_MultiNative_AllFormats(t *testing.T) {
	for _, format := range []constant.RelayFormat{
		constant.RelayFormatOpenAI,
		constant.RelayFormatClaude,
		constant.RelayFormatGemini,
	} {
		info := &common.RelayInfo{
			InboundFormat: format,
			ChannelMeta: &common.ChannelMeta{
				ChannelType:   int(constant.ProviderNewAPI),
				IsModelMapped: false,
				Settings:      common.ChannelSettings{},
			},
		}
		if !canPassThrough(info) {
			t.Errorf("New API channel should pass through %s inbound natively", format)
		}
	}
}

// TestCanPassThrough_MultiNative_ModelMappedForcesConversion 多协议原生透传渠道配置了
// 模型映射时仍需经过 ConvertRequest 替换模型名，不得原样透传。
func TestCanPassThrough_MultiNative_ModelMappedForcesConversion(t *testing.T) {
	info := &common.RelayInfo{
		InboundFormat: constant.RelayFormatClaude,
		ChannelMeta: &common.ChannelMeta{
			ChannelType:   int(constant.ProviderSub2API),
			IsModelMapped: true,
			Settings:      common.ChannelSettings{},
		},
	}
	if canPassThrough(info) {
		t.Error("Sub2API channel with model mapping should NOT pass through")
	}
}

// TestCanPassThrough_MultiNative_ResponsesNotNative Responses 入站不在原生透传格式集合内，
// 不得原样直连（需经 ConvertRequest 转为 OpenAI chat）。
func TestCanPassThrough_MultiNative_ResponsesNotNative(t *testing.T) {
	info := &common.RelayInfo{
		InboundFormat: constant.RelayFormatResponses,
		ChannelMeta: &common.ChannelMeta{
			ChannelType:   int(constant.ProviderNewAPI),
			IsModelMapped: false,
			Settings:      common.ChannelSettings{},
		},
	}
	if canPassThrough(info) {
		t.Error("New API channel should NOT pass through Responses inbound natively")
	}
}

// TestCanPassThrough_SupportsResponses_ResponsesInbound 模型能力声明支持 Responses 协议后，
// Responses 入站在无改写时应原样直连转发。
func TestCanPassThrough_SupportsResponses_ResponsesInbound(t *testing.T) {
	info := &common.RelayInfo{
		InboundFormat: constant.RelayFormatResponses,
		ChannelMeta: &common.ChannelMeta{
			ChannelType:       int(constant.ProviderOpenAI),
			IsModelMapped:     false,
			SupportsResponses: true,
		},
	}
	if !canPassThrough(info) {
		t.Error("channel declaring SupportsResponses should pass through Responses inbound natively")
	}
}

// TestCanPassThrough_SupportsResponses_NonResponsesInbound 渠道声明支持 Responses 协议时，
// 其它入站格式（如 chat）不属于原生匹配，不得原样直连。
func TestCanPassThrough_SupportsResponses_NonResponsesInbound(t *testing.T) {
	info := &common.RelayInfo{
		InboundFormat: constant.RelayFormatOpenAI,
		ChannelMeta: &common.ChannelMeta{
			ChannelType:       int(constant.ProviderOpenAI),
			IsModelMapped:     false,
			SupportsResponses: true,
		},
	}
	if canPassThrough(info) {
		t.Error("SupportsResponses channel should NOT pass through non-Responses inbound natively")
	}
}

// TestCanPassThrough_SupportsResponses_ModelMappedForcesConversion 上游为 Responses 协议
// 但配置了模型映射时，仍需经 ConvertRequest 替换模型名，不得原样透传。
func TestCanPassThrough_SupportsResponses_ModelMappedForcesConversion(t *testing.T) {
	info := &common.RelayInfo{
		InboundFormat: constant.RelayFormatResponses,
		ChannelMeta: &common.ChannelMeta{
			ChannelType:       int(constant.ProviderOpenAI),
			IsModelMapped:     true,
			SupportsResponses: true,
		},
	}
	if canPassThrough(info) {
		t.Error("SupportsResponses channel with model mapping should NOT pass through")
	}
}

// TestCanPassThrough_ChatViaResponses_ChatInboundBlocked responses-only 桥接渠道的
// chat 入站：即使显式开启直连（pass_through_body_enabled）也不得原样透传，
// chat 体必须经桥接转换后才能发 /v1/responses。
func TestCanPassThrough_ChatViaResponses_ChatInboundBlocked(t *testing.T) {
	info := &common.RelayInfo{
		InboundFormat: constant.RelayFormatOpenAI,
		RelayMode:     int(constant.RelayModeChatCompletions),
		ChannelMeta: &common.ChannelMeta{
			ChannelType:       int(constant.ProviderOpenAI),
			IsModelMapped:     false,
			ChatViaResponses:  true,
			SupportsResponses: true,
		},
	}
	info.ChannelMeta.Settings.PassThroughBodyEnabled = true
	if canPassThrough(info) {
		t.Error("chat inbound on chat_via_responses channel must NOT pass through, even with explicit passthrough")
	}
}

// TestCanPassThrough_ChatViaResponses_ResponsesInbound responses-only 桥接渠道的
// responses 入站视为原生匹配（上游本来就说 Responses 协议），可原样直连。
func TestCanPassThrough_ChatViaResponses_ResponsesInbound(t *testing.T) {
	info := &common.RelayInfo{
		InboundFormat: constant.RelayFormatResponses,
		ChannelMeta: &common.ChannelMeta{
			ChannelType:      int(constant.ProviderOpenAI),
			IsModelMapped:    false,
			ChatViaResponses: true,
		},
	}
	if !canPassThrough(info) {
		t.Error("responses inbound on chat_via_responses channel should pass through natively")
	}
}

// TestRelaykitRequestConverterID 验证请求侧 converter ID 解析，含新供应商
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

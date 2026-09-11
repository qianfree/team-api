package handler

import (
	"testing"

	"github.com/qianfree/team-api/relay/common"
	"github.com/qianfree/team-api/relay/constant"
	"github.com/qianfree/team-api/relay/relaykit_bridge"
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

// TestRequestConverterIDForRoute 验证请求侧 converter ID 解析（矩阵已统一收敛到
// relaykit_bridge.RequestConverterIDForRoute），含反向、跨原生方向与 Ollama 的 RelayMode 守卫。
func TestRequestConverterIDForRoute(t *testing.T) {
	tests := []struct {
		name      string
		inbound   constant.RelayFormat
		upstream  constant.RelayFormat
		relayMode int
		want      string
	}{
		{"OpenAI→Claude", constant.RelayFormatOpenAI, constant.RelayFormatClaude, int(constant.RelayModeChatCompletions), relayconvert.ConverterOpenAIChatToClaudeMessages},
		{"OpenAI→Gemini", constant.RelayFormatOpenAI, constant.RelayFormatGemini, int(constant.RelayModeChatCompletions), relayconvert.ConverterOpenAIChatToGeminiContent},
		{"OpenAI→Ollama chat", constant.RelayFormatOpenAI, constant.RelayFormatOllama, int(constant.RelayModeChatCompletions), relayconvert.ConverterOpenAIChatToOllama},
		{"OpenAI→Ollama generate 不迁移", constant.RelayFormatOpenAI, constant.RelayFormatOllama, int(constant.RelayModeCompletions), ""},
		{"OpenAI→Ollama embedding 不迁移", constant.RelayFormatOpenAI, constant.RelayFormatOllama, int(constant.RelayModeEmbeddings), ""},
		{"同格式不转换", constant.RelayFormatOpenAI, constant.RelayFormatOpenAI, int(constant.RelayModeChatCompletions), ""},
		{"Claude→OpenAI 反向", constant.RelayFormatClaude, constant.RelayFormatOpenAI, int(constant.RelayModeClaudeMessages), relayconvert.ConverterClaudeMessagesToOpenAIChat},
		{"Gemini→OpenAI 反向", constant.RelayFormatGemini, constant.RelayFormatOpenAI, int(constant.RelayModeGeminiChat), relayconvert.ConverterGeminiContentToOpenAIChat},
		{"Responses→OpenAI", constant.RelayFormatResponses, constant.RelayFormatOpenAI, int(constant.RelayModeResponses), relayconvert.ConverterOpenAIResponsesToOpenAIChat},
		{"OpenAI→Responses", constant.RelayFormatOpenAI, constant.RelayFormatResponses, int(constant.RelayModeChatCompletions), relayconvert.ConverterOpenAIChatToOpenAIResponses},
		{"Claude→Gemini 跨原生", constant.RelayFormatClaude, constant.RelayFormatGemini, int(constant.RelayModeClaudeMessages), relayconvert.ConverterClaudeMessagesToGeminiContent},
		{"Gemini→Claude 跨原生", constant.RelayFormatGemini, constant.RelayFormatClaude, int(constant.RelayModeGeminiChat), relayconvert.ConverterGeminiContentToClaudeMessages},
		{"Responses→Claude 跨原生", constant.RelayFormatResponses, constant.RelayFormatClaude, int(constant.RelayModeResponses), relayconvert.ConverterResponsesToClaudeMessages},
		{"Responses→Gemini 跨原生", constant.RelayFormatResponses, constant.RelayFormatGemini, int(constant.RelayModeResponses), relayconvert.ConverterOpenAIResponsesToGemini},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := relaykit_bridge.RequestConverterIDForRoute(tt.inbound, tt.upstream, tt.relayMode)
			if got != tt.want {
				t.Errorf("RequestConverterIDForRoute(%s,%s,%d) = %q, want %q", tt.inbound, tt.upstream, tt.relayMode, got, tt.want)
			}
		})
	}
}

// nativeClaudeInfo 构造 Claude 入站 + 指定供应商的 RelayInfo（无映射/改写，纯格式判定）
func nativeClaudeInfo(provider constant.ProviderType, inbound constant.RelayFormat) *common.RelayInfo {
	return &common.RelayInfo{
		InboundFormat: inbound,
		RelayMode:     int(constant.RelayModeClaudeMessages),
		ChannelMeta: &common.ChannelMeta{
			ChannelType: int(provider),
			Settings:    common.ChannelSettings{},
		},
	}
}

// TestEffectiveUpstreamFormat_NativeClaudeEndpoint 另挂 Anthropic 兼容端点的供应商，
// Claude 入站的有效上游格式必须是 Claude（同格式 → 不转换）。
//
// 判成 OpenAI 会两侧同时断链：请求体转成 chat 发到 /anthropic/v1/messages（上游 400），
// 响应侧又把 Claude 响应体当 OpenAI 解析。
func TestEffectiveUpstreamFormat_NativeClaudeEndpoint(t *testing.T) {
	// 另挂 Anthropic 端点（ClaudeMessages 走独立地址）
	nativeClaude := []constant.ProviderType{
		constant.ProviderAli, constant.ProviderZhipu, constant.ProviderDeepSeek,
		constant.ProviderMoonshot, constant.ProviderVolcengine, constant.ProviderMiniMax,
	}
	for _, p := range nativeClaude {
		info := nativeClaudeInfo(p, constant.RelayFormatClaude)
		if got := relaykit_bridge.EffectiveUpstreamFormat(info); got != constant.RelayFormatClaude {
			t.Errorf("provider %d: EffectiveUpstreamFormat = %q, want %q", p, got, constant.RelayFormatClaude)
		}
		// 同格式 → 矩阵不得返回转换器
		if id := relaykit_bridge.RequestConverterIDForRoute(constant.RelayFormatClaude,
			relaykit_bridge.EffectiveUpstreamFormat(info), info.RelayMode); id != "" {
			t.Errorf("provider %d: 不应命中转换器, got %q", p, id)
		}
	}

	// ClaudeMessages 与 chat 共用同一端点 → 上游只说 OpenAI，Claude 入站仍须转换
	openaiOnly := []constant.ProviderType{constant.ProviderXAI, constant.ProviderBaiduV2}
	for _, p := range openaiOnly {
		info := nativeClaudeInfo(p, constant.RelayFormatClaude)
		if got := relaykit_bridge.EffectiveUpstreamFormat(info); got != constant.RelayFormatOpenAI {
			t.Errorf("provider %d: EffectiveUpstreamFormat = %q, want %q", p, got, constant.RelayFormatOpenAI)
		}
	}

	// 覆盖只对 Claude 入站生效：同一供应商的 OpenAI 入站仍是 OpenAI
	info := nativeClaudeInfo(constant.ProviderDeepSeek, constant.RelayFormatOpenAI)
	info.RelayMode = int(constant.RelayModeChatCompletions)
	if got := relaykit_bridge.EffectiveUpstreamFormat(info); got != constant.RelayFormatOpenAI {
		t.Errorf("OpenAI 入站不应被覆盖: got %q, want %q", got, constant.RelayFormatOpenAI)
	}
}

// TestCanPassThrough_NativeClaudeEndpoint_ClaudeInbound 同一批供应商的 Claude 入站
// 在无映射/改写时应可原样直连（上游端点本就吃 Claude 体）。
func TestCanPassThrough_NativeClaudeEndpoint_ClaudeInbound(t *testing.T) {
	info := nativeClaudeInfo(constant.ProviderDeepSeek, constant.RelayFormatClaude)
	if !canPassThrough(info) {
		t.Error("Anthropic 兼容端点渠道的 Claude 入站应可直连")
	}

	// 有模型映射时仍须经转换（改写模型名），但方向是 Claude→Claude 而非 Claude→OpenAI
	info.ChannelMeta.IsModelMapped = true
	if canPassThrough(info) {
		t.Error("有模型映射时不应直连")
	}
	if got := relaykit_bridge.EffectiveUpstreamFormat(info); got != constant.RelayFormatClaude {
		t.Errorf("EffectiveUpstreamFormat = %q, want %q", got, constant.RelayFormatClaude)
	}
}

// vertexInfo 构造 Vertex 渠道的 RelayInfo（模型名驱动协议分流）。
func vertexInfo(model string, inbound constant.RelayFormat, mode constant.RelayMode) *common.RelayInfo {
	return &common.RelayInfo{
		InboundFormat:   inbound,
		ClientFormat:    inbound,
		RelayMode:       int(mode),
		OriginModelName: model,
		ChannelMeta: &common.ChannelMeta{
			ChannelType:       int(constant.ProviderVertex),
			UpstreamModelName: model,
			Settings:          common.ChannelSettings{},
		},
	}
}

// TestEffectiveUpstreamFormat_VertexIsModelDriven Vertex 是模型名驱动的双协议上游：
// 协议判定必须跟着模型走，否则矩阵按渠道类型判成 openai，把请求体转成 chat
// 发到 :generateContent / :rawPredict 原生端点，上游必然 400。
func TestEffectiveUpstreamFormat_VertexIsModelDriven(t *testing.T) {
	cases := []struct {
		model string
		want  constant.RelayFormat
	}{
		{"gemini-2.5-pro", constant.RelayFormatGemini},
		{"claude-sonnet-4-5", constant.RelayFormatClaude},
	}
	for _, tc := range cases {
		// 四种入站格式下有效上游格式都应等于模型对应的原生协议
		for _, inbound := range []constant.RelayFormat{
			constant.RelayFormatOpenAI, constant.RelayFormatClaude,
			constant.RelayFormatGemini, constant.RelayFormatResponses,
		} {
			info := vertexInfo(tc.model, inbound, constant.RelayModeChatCompletions)
			if got := relaykit_bridge.EffectiveUpstreamFormat(info); got != tc.want {
				t.Errorf("模型 %s / %s 入站: EffectiveUpstreamFormat = %q, want %q",
					tc.model, inbound, got, tc.want)
			}
		}
	}
}

// TestCanPassThrough_VertexNativeInboundMatches 与模型协议相同的入站可直连
// （Gemini 模型 + Gemini 入站：体与 :generateContent 端点天然匹配）。
func TestCanPassThrough_VertexNativeInboundMatches(t *testing.T) {
	info := vertexInfo("gemini-2.5-pro", constant.RelayFormatGemini, constant.RelayModeGeminiChat)
	if !canPassThrough(info) {
		t.Error("Vertex Gemini 模型的 Gemini 入站应可直连")
	}

	// OpenAI 入站与 Gemini 端点不匹配，必须经转换
	info = vertexInfo("gemini-2.5-pro", constant.RelayFormatOpenAI, constant.RelayModeChatCompletions)
	if canPassThrough(info) {
		t.Error("Vertex Gemini 模型的 OpenAI 入站不得直连（chat 体发不了 :generateContent）")
	}
}

// TestCanPassThrough_VertexClaudeNeverPassesThrough Vertex 的 Claude 模型即使入站
// 就是 Claude 格式也不得直连：rawPredict 端点要求体内带 anthropic_version、不接受
// model 字段，这段改写只在 ConvertRequest / PostProcessConvertedRequest 中执行，
// 直连会绕过两者导致上游 400。显式开启 pass_through_body_enabled 也不例外。
func TestCanPassThrough_VertexClaudeNeverPassesThrough(t *testing.T) {
	info := vertexInfo("claude-sonnet-4-5", constant.RelayFormatClaude, constant.RelayModeClaudeMessages)
	if canPassThrough(info) {
		t.Error("Vertex Claude 模型的 Claude 入站不得直连（会绕过 anthropic_version 注入）")
	}

	info.ChannelMeta.Settings.PassThroughBodyEnabled = true
	if canPassThrough(info) {
		t.Error("即使显式开启直连，Vertex Claude 模型仍不得原样透传")
	}
}

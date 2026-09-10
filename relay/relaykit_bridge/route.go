package relaykit_bridge

import (
	"github.com/qianfree/team-api/relay/common"
	"github.com/qianfree/team-api/relay/constant"
	"github.com/qianfree/team-api/relay/helper"
	"github.com/qianfree/team-api/relaykit/relayconvert"
	"github.com/qianfree/team-api/relaykit/types"

	// blank import 触发内置转换器注册（register.init() 调用 RegisterTextConverter
	// 与 RegisterStreamConverter）。放在桥接包而非某个调用方，保证任何使用桥接的包
	//（含各 channel 包及其单测）都能拿到已注册的转换器。
	_ "github.com/qianfree/team-api/relaykit/relayconvert/register"
)

// 本文件是宿主与 relaykit 之间「转换方向 → 转换器 ID」的唯一权威映射。
// 请求桥（relay/handler）与响应桥（本包）共用，避免矩阵多副本漂移。

// KitFormat 宿主 RelayFormat → relaykit types.RelayFormat。
// 除 Responses 外字符串值一致可直接强转；Responses 两侧命名不同
// （宿主 "responses" vs relaykit "openai_responses"），必须显式映射。
func KitFormat(f constant.RelayFormat) types.RelayFormat {
	if f == constant.RelayFormatResponses {
		return types.RelayFormatOpenAIResponses
	}
	return types.RelayFormat(f)
}

// EffectiveUpstreamFormat 计算本次请求的有效上游协议格式。
// 基础值取渠道供应商原生格式；OpenAI 主协议的渠道有三种覆盖：
//   - Claude 入站 + 供应商另挂 Anthropic 兼容端点：上游实际说 Claude（同格式 → 无转换）；
//   - UseResponsesAPI：chat 入站经 /v1/responses 桥接（渠道 ChatViaResponses 配置）；
//   - Responses 入站 + 上游原生支持 Responses：保持 Responses 直连（同格式 → 无转换）。
func EffectiveUpstreamFormat(info *common.RelayInfo) constant.RelayFormat {
	upstream := helper.ProviderNativeFormat(info.ChannelMeta.ChannelType)
	if upstream != constant.RelayFormatOpenAI {
		return upstream
	}
	// Anthropic 兼容端点覆盖：这类渠道 GetRequestURL 对 ClaudeMessages 返回独立的
	// /anthropic/v1/messages 类地址，请求与响应两侧都是 Claude 口径。若按主协议
	// 判成 OpenAI，会把 Claude 体转成 chat 发到 Anthropic 端点、再把 Claude 响应
	// 当 OpenAI 解析，两侧同时断链。
	if info.InboundFormat == constant.RelayFormatClaude &&
		constant.HasNativeClaudeEndpoint(info.ChannelMeta.ChannelType) {
		return constant.RelayFormatClaude
	}
	if info.UseResponsesAPI {
		return constant.RelayFormatResponses
	}
	if info.InboundFormat == constant.RelayFormatResponses && info.ChannelMeta.UpstreamSpeaksResponses() {
		return constant.RelayFormatResponses
	}
	return upstream
}

// RequestConverterIDForRoute 按 (入站格式, 有效上游格式, RelayMode) 返回配对转换器 ID。
// 返回空串表示该方向无 relaykit 转换器（同格式直连、或该模式未覆盖）。
// Ollama 仅注册了 chat 路径转换器，generate/embedding 模式返回空串。
func RequestConverterIDForRoute(inbound, upstream constant.RelayFormat, relayMode int) string {
	if inbound == upstream {
		return "" // 同格式无需转换（这类请求走 passthrough）
	}
	switch {
	// OpenAI 入站 → 原生上游
	case inbound == constant.RelayFormatOpenAI && upstream == constant.RelayFormatClaude:
		return relayconvert.ConverterOpenAIChatToClaudeMessages
	case inbound == constant.RelayFormatOpenAI && upstream == constant.RelayFormatGemini:
		return relayconvert.ConverterOpenAIChatToGeminiContent
	case inbound == constant.RelayFormatOpenAI && upstream == constant.RelayFormatOllama:
		if constant.RelayMode(relayMode) != constant.RelayModeChatCompletions {
			return ""
		}
		return relayconvert.ConverterOpenAIChatToOllama
	case inbound == constant.RelayFormatOpenAI && upstream == constant.RelayFormatResponses:
		return relayconvert.ConverterOpenAIChatToOpenAIResponses

	// 非 OpenAI 入站 → OpenAI 上游（反向方向）
	case inbound == constant.RelayFormatClaude && upstream == constant.RelayFormatOpenAI:
		return relayconvert.ConverterClaudeMessagesToOpenAIChat
	case inbound == constant.RelayFormatGemini && upstream == constant.RelayFormatOpenAI:
		return relayconvert.ConverterGeminiContentToOpenAIChat
	case inbound == constant.RelayFormatResponses && upstream == constant.RelayFormatOpenAI:
		return relayconvert.ConverterOpenAIResponsesToOpenAIChat

	// 跨原生方向（请求侧：Claude→Gemini 为直连，其余为经 OpenAI 中枢的步骤链）
	case inbound == constant.RelayFormatClaude && upstream == constant.RelayFormatGemini:
		return relayconvert.ConverterClaudeMessagesToGeminiContent
	case inbound == constant.RelayFormatGemini && upstream == constant.RelayFormatClaude:
		return relayconvert.ConverterGeminiContentToClaudeMessages
	case inbound == constant.RelayFormatResponses && upstream == constant.RelayFormatClaude:
		return relayconvert.ConverterResponsesToClaudeMessages
	case inbound == constant.RelayFormatResponses && upstream == constant.RelayFormatGemini:
		return relayconvert.ConverterOpenAIResponsesToGemini

	default:
		return ""
	}
}

// ResponseConverterIDForRoute 按 (上游格式, 客户端格式) 返回配对转换器 ID。
// 响应侧与请求侧共用同一配对 ID（配对的 From=客户端入站格式、To=上游格式），
// 因此直接以 (client, upstream) 委托请求侧矩阵；RelayMode 传 chat（响应侧无模式歧义）。
func ResponseConverterIDForRoute(upstream, client constant.RelayFormat) string {
	return RequestConverterIDForRoute(client, upstream, int(constant.RelayModeChatCompletions))
}

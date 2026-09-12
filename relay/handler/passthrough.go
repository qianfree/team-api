package handler

import (
	"github.com/qianfree/team-api/relay/common"
	"github.com/qianfree/team-api/relay/constant"
	"github.com/qianfree/team-api/relay/helper"
)

// canPassThrough 判断当前请求是否可以直连转发（跳过协议转换和参数改写）
func canPassThrough(info *common.RelayInfo) bool {
	settings := info.ChannelMeta.Settings

	// 阿里云同步 multimodal 图片（qwen-image-2.x）：入站 OpenAI Images 格式与上游 DashScope
	// multimodal-generation 的 messages 结构不兼容，必须经 ConvertRequest 转换。即使运营者
	// 显式开启直连、或未配置模型映射，也不能原样透传，否则上游因请求体格式错误报错。
	if constant.RelayMode(info.RelayMode) == constant.RelayModeImagesGenerations &&
		constant.IsAliSyncMultimodalImageModel(info.ChannelMeta.UpstreamModelName) {
		return false
	}

	// responses-only 桥接渠道的 chat 入站：chat 体与上游 /v1/responses 端点不兼容，
	// 必须经 relaykit chat→Responses 桥接转换，即使显式开启直连也不得原样透传。
	if info.ChannelMeta.ChatViaResponses &&
		constant.RelayMode(info.RelayMode) == constant.RelayModeChatCompletions {
		return false
	}

	// Vertex AI 的 Claude 模型：rawPredict 端点要求体内带 anthropic_version、
	// 且不接受 model 字段（模型由 URL 指定）。这段改写在 adaptor.ConvertRequest 与
	// PostProcessConvertedRequest 中完成，原样直连会绕过两者、上游直接 400，
	// 因此即使显式开启直连也不得透传。
	if constant.ProviderType(info.ChannelMeta.ChannelType) == constant.ProviderVertex &&
		helper.ProviderNativeFormatFor(info) == constant.RelayFormatClaude {
		return false
	}

	// 显式开启：运营者明确配置直连，不做额外检查
	if settings.PassThroughBodyEnabled {
		return true
	}

	// 自动检测：入站格式必须匹配渠道原生格式才可原样直连转发。
	// 多协议原生透传渠道（New API / Sub2API）的上游同时支持 OpenAI/Claude/Gemini，三者均匹配。
	if !inboundMatchesChannelNative(info) {
		return false
	}
	// 需要模型名映射 → 必须经过转换来替换模型名
	if info.ChannelMeta.IsModelMapped {
		return false
	}
	// 有参数改写规则 → 必须经过转换来应用改写
	if settings.ParamOverride != nil {
		return false
	}
	// 有系统提示词注入 → 必须经过转换
	if settings.SystemPrompt != "" {
		return false
	}
	return true
}

// inboundMatchesChannelNative 判断入站格式是否与渠道原生格式匹配（匹配则可原样直连转发）。
//   - 上游原生支持 Responses 协议（supports_responses / chat_via_responses）：Responses 入站视为原生匹配。
//   - 普通渠道：入站格式须等于该渠道的唯一原生格式（helper.ProviderNativeFormat）。
//   - 多协议原生透传渠道（New API / Sub2API）：OpenAI/Claude/Gemini 三种格式均视为匹配。
//
// 注意：chat_via_responses（responses-only 桥接渠道）的 chat 入站由 canPassThrough
// 前置硬排除——chat 体必须经 relaykit chat→Responses 转换后才能发 /v1/responses。
func inboundMatchesChannelNative(info *common.RelayInfo) bool {
	if info.ChannelMeta.UpstreamSpeaksResponses() {
		return info.InboundFormat == constant.RelayFormatResponses
	}
	if constant.IsMultiNativeProvider(info.ChannelMeta.ChannelType) {
		switch info.InboundFormat {
		case constant.RelayFormatOpenAI, constant.RelayFormatClaude, constant.RelayFormatGemini:
			return true
		}
		return false
	}
	// Anthropic 兼容端点渠道：主协议是 OpenAI，但 Claude 入站走独立的 Anthropic
	// 端点且两侧均为 Claude 口径，故 Claude 入站同样算「匹配原生」可直连。
	if info.InboundFormat == constant.RelayFormatClaude &&
		constant.HasNativeClaudeEndpoint(info.ChannelMeta.ChannelType) {
		return true
	}
	return helper.ProviderNativeFormatFor(info) == info.InboundFormat
}

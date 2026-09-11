package helper

import (
	"strings"

	"github.com/qianfree/team-api/relay/common"
	"github.com/qianfree/team-api/relay/constant"
)

// providerNativeFormat 在 relaykit 迁移期间曾被复制到多个包（handler/passthrough.go、
// handler/relaykit_bridge.go、relaykit_bridge/response.go）以规避循环引用。
// 统一本文件这一处权威实现，helper 是叶子工具包（不依赖 handler/relaykit_bridge），
// 各调用方改为 import 本包调用，消除「多副本需保持同步」的维护隐患。

// ProviderNativeFormat 根据 ProviderType 返回上游的原生请求格式。
//
// 注意：本函数只看渠道类型。**模型名驱动的双协议上游（Vertex AI）必须改用
// ProviderNativeFormatFor / ProviderNativeFormatForModel**，否则会判成 openai
// 而把请求体转成 chat 发到原生端点（见下方 VertexModelFormat 的说明）。
// 仅在确实没有模型信息、或已知供应商协议与模型无关时才直接调用本函数。
func ProviderNativeFormat(providerType int) constant.RelayFormat {
	switch constant.ProviderType(providerType) {
	case constant.ProviderClaude:
		return constant.RelayFormatClaude
	case constant.ProviderGemini:
		return constant.RelayFormatGemini
	case constant.ProviderOllama:
		return constant.RelayFormatOllama
	default:
		return constant.RelayFormatOpenAI
	}
}

// VertexModelFormat 判定 Vertex AI 渠道在给定模型下的上游原生协议。
//
// Vertex AI 是**模型名驱动的双协议上游**：同一渠道既可挂 Gemini 模型
// （publishers/google/{model}:generateContent，吃 Gemini 原生体），
// 也可挂 Claude 模型（publishers/anthropic/{model}:rawPredict，吃 Claude 原生体）。
// 这与「每渠道一种协议」的 ProviderNativeFormat 模型不匹配——按渠道类型判会落到
// default 分支得出 openai，于是矩阵把请求体转成 chat 发到原生端点、上游必然 400；
// 而 Vertex 的 GetRequestURL 不按 RelayMode 报错，矩阵守卫测试也发现不了
// （体与端点错配只有 E2E 层能捕获）。
//
// 判据与 vertex adaptor 选择委托适配器 / 构造 URL 的判据**必须是同一个**，
// 因此权威实现放在这里，adaptor 侧调用本函数，杜绝两处漂移。
func VertexModelFormat(model string) constant.RelayFormat {
	if strings.Contains(strings.ToLower(model), "claude") {
		return constant.RelayFormatClaude
	}
	return constant.RelayFormatGemini
}

// ProviderNativeFormatForModel 在 ProviderNativeFormat 基础上叠加模型名维度。
// 除 Vertex 外的供应商协议与模型无关，直接委托 ProviderNativeFormat。
func ProviderNativeFormatForModel(providerType int, upstreamModel string) constant.RelayFormat {
	if constant.ProviderType(providerType) == constant.ProviderVertex {
		return VertexModelFormat(upstreamModel)
	}
	return ProviderNativeFormat(providerType)
}

// ProviderNativeFormatFor 返回本次请求的上游原生协议格式（调用方首选入口）。
//
// 模型名取 UpstreamModelName（与 adaptor 构造 URL 所用的字段一致，保证
// 「判定协议」与「选端点」基于同一个模型名）；为空时回落客户端请求的模型名。
func ProviderNativeFormatFor(info *common.RelayInfo) constant.RelayFormat {
	if info == nil || info.ChannelMeta == nil {
		return constant.RelayFormatOpenAI
	}
	model := info.ChannelMeta.UpstreamModelName
	if model == "" {
		model = info.OriginModelName
	}
	return ProviderNativeFormatForModel(info.ChannelMeta.ChannelType, model)
}

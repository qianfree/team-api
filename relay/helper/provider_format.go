package helper

import "github.com/qianfree/team-api/relay/constant"

// providerNativeFormat 在 relaykit 迁移期间曾被复制到多个包（handler/passthrough.go、
// handler/relaykit_bridge.go、relaykit_bridge/response.go）以规避循环引用。
// 阶段 7 清理：统一为本文件这一处权威实现，helper 是叶子工具包（不依赖 handler/relaykit_bridge），
// 各调用方改为 import 本包调用，消除「多副本需保持同步」的维护隐患。

// ProviderNativeFormat 根据 ProviderType 返回上游的原生请求格式。
// 用于直连判定（canPassThrough）、系统提示词注入格式选择（InjectSystemPrompt）、
// relaykit 桥接的转换方向推断等所有场景。
func ProviderNativeFormat(providerType int) constant.RelayFormat {
	switch constant.ProviderType(providerType) {
	case constant.ProviderClaude:
		return constant.RelayFormatClaude
	case constant.ProviderGemini:
		return constant.RelayFormatGemini
	case constant.ProviderCoze:
		return constant.RelayFormatCoze
	case constant.ProviderDify:
		return constant.RelayFormatDify
	case constant.ProviderOllama:
		return constant.RelayFormatOllama
	default:
		return constant.RelayFormatOpenAI
	}
}

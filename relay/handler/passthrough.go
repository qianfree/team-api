package handler

import (
	"github.com/qianfree/team-api/relay/common"
	"github.com/qianfree/team-api/relay/constant"
)

// canPassThrough 判断当前请求是否可以直连转发（跳过协议转换和参数改写）
func canPassThrough(info *common.RelayInfo) bool {
	settings := info.ChannelMeta.Settings

	// 显式开启：运营者明确配置直连，不做额外检查
	if settings.PassThroughBodyEnabled {
		return true
	}

	// 自动检测：入站格式必须匹配上游原生格式
	if providerNativeFormat(info.ChannelMeta.ChannelType) != info.InboundFormat {
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
	// 有 thinking 后缀 → 必须经过转换来注入 thinking 参数
	if info.ThinkingEnabled || info.ThinkingDisabled || info.ReasoningEffort != "" {
		return false
	}
	return true
}

// providerNativeFormat 根据 ProviderType 返回上游的原生请求格式
func providerNativeFormat(providerType int) constant.RelayFormat {
	switch constant.ProviderType(providerType) {
	case constant.ProviderClaude:
		return constant.RelayFormatClaude
	case constant.ProviderGemini:
		return constant.RelayFormatGemini
	default:
		return constant.RelayFormatOpenAI
	}
}

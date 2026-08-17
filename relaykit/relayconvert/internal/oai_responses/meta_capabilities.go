package oai_responses

import (
	"github.com/qianfree/team-api/relaykit/relayconvert/convmeta"
)

// modelMappedProvider 宿主可选实现的 Meta 扩展接口：提供渠道模型映射标志。
// 宿主 relay/common.RelayInfo 实现本接口；未实现（如 convmeta.Values 测试桩）时
// 回退「未映射」语义，保证 golden 测试确定性。
type modelMappedProvider interface {
	ModelNameMapped() bool
}

// requestIDProvider 宿主可选实现的 Meta 扩展接口：提供请求 ID（合成响应 ID 用）。
type requestIDProvider interface {
	GetRequestID() string
}

// modelMappedOf 提取模型映射标志（接口未实现时回退 false）。
func modelMappedOf(info convmeta.Meta) bool {
	if provider, ok := info.(modelMappedProvider); ok && provider != nil {
		return provider.ModelNameMapped()
	}
	return false
}

// requestIDOf 提取请求 ID（接口未实现时回退空串）。
func requestIDOf(info convmeta.Meta) string {
	if provider, ok := info.(requestIDProvider); ok && provider != nil {
		return provider.GetRequestID()
	}
	return ""
}

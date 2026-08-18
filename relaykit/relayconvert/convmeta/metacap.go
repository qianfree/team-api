package convmeta

// 宿主可选能力接口与提取助手（P1-R/P1-B 模式：宿主 RelayInfo 实现，测试桩回退默认值）。
// 注意不能放 kitutil——kitutil→convmeta→types→kitutil 会成 import 环，本包是 Meta 的本家。

// modelMappedProvider 宿主可选实现的 Meta 扩展接口：提供渠道模型映射标志。
type modelMappedProvider interface {
	ModelNameMapped() bool
}

// requestIDProvider 宿主可选实现的 Meta 扩展接口：提供请求 ID（合成响应 ID 用）。
type requestIDProvider interface {
	GetRequestID() string
}

// ModelNameMappedOf 提取模型映射标志（接口未实现时回退 false，golden 确定性保证）。
func ModelNameMappedOf(info Meta) bool {
	if provider, ok := info.(modelMappedProvider); ok && provider != nil {
		return provider.ModelNameMapped()
	}
	return false
}

// RequestIDOf 提取请求 ID（接口未实现时回退空串）。
func RequestIDOf(info Meta) string {
	if provider, ok := info.(requestIDProvider); ok && provider != nil {
		return provider.GetRequestID()
	}
	return ""
}

package oai_responses

import (
	"github.com/qianfree/team-api/relaykit/relayconvert/convmeta"
)

// 能力接口助手已上移 convmeta/metacap.go（P1-B 起多个转换器包共用；kitutil 会造成
// import 环），此处保留薄代理维持本包内调用点的可读性。

// modelMappedOf 提取模型映射标志（接口未实现时回退 false）。
func modelMappedOf(info convmeta.Meta) bool {
	return convmeta.ModelNameMappedOf(info)
}

// requestIDOf 提取请求 ID（接口未实现时回退空串）。
func requestIDOf(info convmeta.Meta) string {
	return convmeta.RequestIDOf(info)
}

package convmeta

import (
	"github.com/qianfree/team-api/relaykit/dto"
)

// ResponsesStash 是宿主可选实现的能力接口：缓存 Responses 入站请求的解析快照，
// 供「上游 chat 响应合成回 Responses 格式」时 echo 请求参数
// （temperature / top_p / max_output_tokens / instructions）。
//
// 转换器对 Meta 做类型断言获取本接口；info 未实现（或未 stash）时静默跳过，
// echo 回退 OpenAI 默认值（temperature=1.0 / top_p=1.0，其余 nil）。
// 宿主端由 *relaycommon.RelayInfo 实现（对应旧字段 info.ResponsesRequest）。
type ResponsesStash interface {
	// StashResponsesRequest 存入最新一次解析后的 Responses 请求快照（覆盖旧值）。
	StashResponsesRequest(req *dto.OpenAIResponsesRequest)
	// StashedResponsesRequest 返回已 stash 的请求快照；未 stash 时返回 nil。
	StashedResponsesRequest() *dto.OpenAIResponsesRequest
}

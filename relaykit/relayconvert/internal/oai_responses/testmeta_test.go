package oai_responses

import (
	"github.com/qianfree/team-api/relaykit/dto"
	"github.com/qianfree/team-api/relaykit/relayconvert/convmeta"
)

// stashMeta 测试用 Meta 实现：嵌入 convmeta.Values 并实现 ResponsesStash 能力接口
// （对应宿主 RelayInfo 的 info.ResponsesRequest 字段）。
type stashMeta struct {
	*convmeta.Values
	stashed *dto.OpenAIResponsesRequest
}

func newStashMeta(origin, upstream string, hasChannel bool) *stashMeta {
	return &stashMeta{Values: &convmeta.Values{
		OriginModelName:     origin,
		UpstreamModelName:   upstream,
		ChannelMetaAttached: hasChannel,
	}}
}

func (m *stashMeta) StashResponsesRequest(req *dto.OpenAIResponsesRequest) {
	m.stashed = req
}

func (m *stashMeta) StashedResponsesRequest() *dto.OpenAIResponsesRequest {
	return m.stashed
}

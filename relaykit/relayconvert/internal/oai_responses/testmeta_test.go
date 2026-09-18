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

// carryMeta 测试用 Meta 实现：附带 ReasoningCarry 能力（对应宿主 RelayInfo 的进程内 TTL 缓存）。
type carryMeta struct {
	*convmeta.Values
	store map[string]string
}

func newCarryMeta() *carryMeta {
	return &carryMeta{Values: &convmeta.Values{OriginModelName: "deepseek-v4-flash"}, store: map[string]string{}}
}

func (m *carryMeta) StoreReasoningForCalls(callIDs []string, reasoning string) {
	for _, id := range callIDs {
		m.store[id] = reasoning
	}
}

func (m *carryMeta) LookupReasoningForCalls(callIDs []string) string {
	for _, id := range callIDs {
		if text, ok := m.store[id]; ok {
			return text
		}
	}
	return ""
}

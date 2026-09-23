package common

import (
	"sync"
	"time"

	"github.com/qianfree/team-api/relaykit/relayconvert/convmeta"
)

// 思考文本跨轮携带的进程内 TTL 存储（convmeta.ReasoningCarry 的宿主实现）。
//
// 为什么需要它：DeepSeek 等 thinking 上游要求多轮工具调用把 reasoning_content 传回，
// 而 ai-sdk 系 responses 客户端重建历史时会剥掉 reasoning 项（连 encrypted_content
// 一起丢）。网关在响应侧按 call_id 存本轮思考文本，下一轮请求按 function_call 的
// call_id 捞回。工具调用 call_id 全局唯一（上游生成），可安全作键。
//
// 局限：进程内存态。多实例部署时同一会话的两轮请求落在不同实例则携带失效——
// 客户端若正常回传 reasoning 项（encrypted_content 通道）不依赖本机制。
const (
	reasoningCarryTTL = 30 * time.Minute
	// reasoningCarrySweepAt 超过该条数时顺带清扫过期项（懒清扫，避免常驻 goroutine）
	reasoningCarrySweepAt = 4096
)

type reasoningCarryEntry struct {
	text     string
	expireAt time.Time
}

var (
	reasoningCarryMu       sync.Mutex
	reasoningCarryByCallID = make(map[string]reasoningCarryEntry)
)

// StoreReasoningForCalls 把本轮思考文本按工具调用 call_id 存入进程内缓存。
func (info *RelayInfo) StoreReasoningForCalls(callIDs []string, reasoning string) {
	if reasoning == "" || len(callIDs) == 0 {
		return
	}
	now := time.Now()
	reasoningCarryMu.Lock()
	defer reasoningCarryMu.Unlock()
	if len(reasoningCarryByCallID) >= reasoningCarrySweepAt {
		for k, v := range reasoningCarryByCallID {
			if now.After(v.expireAt) {
				delete(reasoningCarryByCallID, k)
			}
		}
	}
	entry := reasoningCarryEntry{text: reasoning, expireAt: now.Add(reasoningCarryTTL)}
	for _, id := range callIDs {
		if id != "" {
			reasoningCarryByCallID[id] = entry
		}
	}
}

// LookupReasoningForCalls 按 call_id 查思考文本，命中任一即返回。
// 不删除命中项：同一轮请求可能因重试被多次转换，读操作保持幂等，过期由 TTL 收敛。
func (info *RelayInfo) LookupReasoningForCalls(callIDs []string) string {
	now := time.Now()
	reasoningCarryMu.Lock()
	defer reasoningCarryMu.Unlock()
	for _, id := range callIDs {
		if id == "" {
			continue
		}
		if entry, ok := reasoningCarryByCallID[id]; ok {
			if now.After(entry.expireAt) {
				delete(reasoningCarryByCallID, id)
				continue
			}
			return entry.text
		}
	}
	return ""
}

var _ convmeta.ReasoningCarry = (*RelayInfo)(nil)

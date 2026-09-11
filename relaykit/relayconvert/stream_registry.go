// Package relayconvert — STREAM 响应转换器注册表。
//
// 流式转换器的真实签名为
//
//	ConvertStreamResponse(ctx, info convmeta.Meta, reader io.Reader, chunkWriter func(any) error) error
//
// 与 ResponseStreamConverterFunc（基于整条 response any 的回调）不兼容，因此不能复用
// response_registry.go 的字段。这里单独建表：按 (from,to) 路由 + converterID 双索引，
// 供宿主桥接层（relay/relaykit_bridge）查找并直接调用。
//
// internal 转换器（internal/oai_chat、internal/oai_gemini）由 register 子包在 init() 中
// 通过 RegisterStreamConverter 注册进来，主项目 blank-import register 即生效。
package relayconvert

import (
	"context"
	"fmt"
	"io"
	"sort"
	"strings"
	"sync"

	"github.com/qianfree/team-api/relaykit/relayconvert/convmeta"
	"github.com/qianfree/team-api/relaykit/types"
)

// StreamConverterFunc 流式响应转换函数签名。
// 读取上游 SSE reader，通过 chunkWriter 回调输出转换后的流式 chunk
// （如 *dto.ChatCompletionStreamResponse）。与 internal 转换器的 ConvertStreamResponse 一致。
type StreamConverterFunc func(ctx context.Context, info convmeta.Meta, reader io.Reader, chunkWriter func(chunk any) error) error

var (
	streamConverterMu     sync.RWMutex
	streamConverterRoutes = make(map[responseConverterRoute]string) // (from,to) → converterID
	streamConverterFuncs  = make(map[string]StreamConverterFunc)    // converterID → fn
)

// RegisterStreamConverter 注册一个流式响应转换器（按 from→to 路由 + converterID）。
// 由 register 子包在 init() 中调用。校验风格与 registerBuiltinResponseConverter 一致：
// 空 ID / from / to / nil fn 或重复路由 / 重复 ID 均 panic。
func RegisterStreamConverter(from, to types.RelayFormat, converterID string, fn StreamConverterFunc) {
	converterID = strings.TrimSpace(converterID)
	if converterID == "" {
		panic("stream converter ID is required")
	}
	if from == "" || to == "" {
		panic(fmt.Sprintf("stream converter %q must declare from and to formats", converterID))
	}
	if fn == nil {
		panic(fmt.Sprintf("stream converter %q must declare a convert function", converterID))
	}

	streamConverterMu.Lock()
	defer streamConverterMu.Unlock()

	route := responseConverterRoute{from: from, to: to}
	if existing, ok := streamConverterRoutes[route]; ok && existing != converterID {
		panic(fmt.Sprintf("stream converter route from %s to %s is already registered by %q", from, to, existing))
	}
	if _, exists := streamConverterFuncs[converterID]; exists {
		panic(fmt.Sprintf("stream converter %q is already registered", converterID))
	}

	streamConverterRoutes[route] = converterID
	streamConverterFuncs[converterID] = fn
}

// LookupStreamConverter 按 (from, to) 路由查找流式响应转换器。
// 返回 (转换函数, converterID, true)；未注册返回 (nil, "", false)。
func LookupStreamConverter(from, to types.RelayFormat) (StreamConverterFunc, string, bool) {
	streamConverterMu.RLock()
	defer streamConverterMu.RUnlock()

	converterID, ok := streamConverterRoutes[responseConverterRoute{from: from, to: to}]
	if !ok {
		return nil, "", false
	}
	fn, ok := streamConverterFuncs[converterID]
	if !ok {
		return nil, "", false
	}
	return fn, converterID, true
}

// StreamConverterSpec 一个已注册流式转换方向的元信息（枚举用）。
// From 是上游流格式，To 是客户端格式（流式方向与请求方向相反）。
type StreamConverterSpec struct {
	ID   string
	From types.RelayFormat
	To   types.RelayFormat
}

// ListStreamConverterSpecs 返回全部已注册流式转换方向（按 ID 升序，结果稳定）。
// 供注册表级测试套件（流式金样本 / 能力守恒）枚举：新注册方向零成本自动纳入。
func ListStreamConverterSpecs() []StreamConverterSpec {
	streamConverterMu.RLock()
	defer streamConverterMu.RUnlock()

	out := make([]StreamConverterSpec, 0, len(streamConverterRoutes))
	for route, id := range streamConverterRoutes {
		out = append(out, StreamConverterSpec{ID: id, From: route.from, To: route.to})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// LookupStreamConverterByID 按 converterID 查找流式转换函数。
// 与 ListStreamConverterSpecs 配套使用；未注册返回 (nil, false)。
func LookupStreamConverterByID(converterID string) (StreamConverterFunc, bool) {
	streamConverterMu.RLock()
	defer streamConverterMu.RUnlock()

	fn, ok := streamConverterFuncs[converterID]
	return fn, ok
}

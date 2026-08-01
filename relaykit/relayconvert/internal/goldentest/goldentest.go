// Package goldentest 提供转换器 Golden 测试的共享基础设施：
// 测试用例结构、fixture 读写、JSON 深度比较、-update-golden 标志。
//
// 各转换器包（internal/oai_chat、internal/oai_gemini 等）在其 golden/ 目录下放置
// 输入+期望输出的 JSON fixture，golden_test.go 经本包加载、转换、比较（或重生）。
// map→DTO 的转换推荐用 kitutil.Any2Type[T]，避免每个包重写 marshal 往返。
package goldentest

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"testing"

	"github.com/qianfree/team-api/relaykit/types"
)

// Update 控制 是否重新生成期望输出（首次编写或确认新输出时使用）。
// 用法：go test -run TestGolden -update-golden ./...
var Update = flag.Bool("update-golden", false, "Regenerate golden test expected outputs")

// TestCase 描述一条完整的转换 Golden 用例（输入 + 期望输出）。
type TestCase struct {
	Name      string            `json:"name"`
	From      types.RelayFormat `json:"from"`
	To        types.RelayFormat `json:"to"`
	Request   any               `json:"request,omitempty"`
	Response  any               `json:"response,omitempty"`
	StreamData string           `json:"stream_data,omitempty"`

	// 期望输出
	ExpectedRequest       any `json:"expected_request,omitempty"`
	ExpectedResponse      any `json:"expected_response,omitempty"`
	ExpectedStreamChunks  []any `json:"expected_stream_chunks,omitempty"`

	ConverterID string `json:"converter_id,omitempty"`
}

// Load 从 path 读取一条 Golden 用例。
func Load(t *testing.T, path string) TestCase {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("Failed to read golden file %s: %v", path, err)
	}
	var tc TestCase
	if err := json.Unmarshal(data, &tc); err != nil {
		t.Fatalf("Failed to parse golden file %s: %v", path, err)
	}
	return tc
}

// Save 将 Golden 用例写回 path（带缩进），用于 -update-golden 重生。
func Save(t *testing.T, path string, tc TestCase) {
	t.Helper()
	data, err := json.MarshalIndent(tc, "", "  ")
	if err != nil {
		t.Fatalf("Failed to marshal golden file %s: %v", path, err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("Failed to write golden file %s: %v", path, err)
	}
	t.Logf("Updated golden file: %s", path)
}

// Equal 对 a、b 做基于 JSON 的深度比较（map/slice/原始类型递归）。
func Equal(a, b any) bool {
	aMap := toMap(a)
	bMap := toMap(b)
	if aMap == nil || bMap == nil {
		// 非对象 JSON（数组/原始值）：退化为字节比较
		aJSON, _ := json.Marshal(a)
		bJSON, _ := json.Marshal(b)
		return string(aJSON) == string(bJSON)
	}
	return equalMaps(aMap, bMap)
}

// EqualExcluding 比较 a、b 但忽略指定的顶层键（用于 created 等非确定字段，使 golden 稳定）。
func EqualExcluding(a, b any, keys ...string) bool {
	aMap := toMap(a)
	bMap := toMap(b)
	if aMap == nil || bMap == nil {
		return Equal(a, b)
	}
	for _, k := range keys {
		delete(aMap, k)
		delete(bMap, k)
	}
	return equalMaps(aMap, bMap)
}

// toMap 将任意值 marshal 后解析为 map[string]any；非对象返回 nil。
func toMap(v any) map[string]any {
	data, err := json.Marshal(v)
	if err != nil {
		return nil
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		return nil
	}
	return m
}

// EqualChunksExcluding 逐元素比较两个流式 chunk 列表，忽略每个 chunk（顶层 map）的指定键。
// 用于流式 golden：chunk 的 id/created 常为实时时间戳（如 chatcmpl-<now>），需排除以保持稳定。
func EqualChunksExcluding(got, want []any, keys ...string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		gm := toMap(got[i])
		wm := toMap(want[i])
		if gm == nil || wm == nil {
			if !Equal(got[i], want[i]) {
				return false
			}
			continue
		}
		for _, k := range keys {
			delete(gm, k)
			delete(wm, k)
		}
		if !equalMaps(gm, wm) {
			return false
		}
	}
	return true
}

// equalMaps 递归比较两个 map（长度一致 + 每个键值深度相等）。
func equalMaps(a, b map[string]any) bool {
	if len(a) != len(b) {
		return false
	}
	for k, av := range a {
		bv, ok := b[k]
		if !ok {
			return false
		}
		if !equalValues(av, bv) {
			return false
		}
	}
	return true
}

// equalValues 比较两个值（含嵌套 map/slice 与原始类型）。
func equalValues(a, b any) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	if am, ok := a.(map[string]any); ok {
		if bm, ok := b.(map[string]any); ok {
			return equalMaps(am, bm)
		}
		return false
	}
	if as, ok := a.([]any); ok {
		if bs, ok := b.([]any); ok {
			if len(as) != len(bs) {
				return false
			}
			for i := range as {
				if !equalValues(as[i], bs[i]) {
					return false
				}
			}
			return true
		}
		return false
	}
	return fmt.Sprintf("%v", a) == fmt.Sprintf("%v", b)
}

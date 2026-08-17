package claude

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/qianfree/team-api/relay/common"
	"github.com/qianfree/team-api/relay/relaykit_bridge"
)

// parityRecorder httptest Recorder 的轻量包装。
type parityRecorder struct {
	rec *httptest.ResponseRecorder
}

func newParityRecorder(t *testing.T) *parityRecorder {
	t.Helper()
	return &parityRecorder{rec: httptest.NewRecorder()}
}

func (p *parityRecorder) bodyBytes() []byte {
	return p.rec.Body.Bytes()
}

// responsesSSEEvent 解析出的一条 Responses SSE 事件。
type responsesSSEEvent struct {
	eventType string
	payload   map[string]any
}

// parseResponsesSSE 将 `event: X\ndata: {...}` 流解析为事件列表。
func parseResponsesSSE(t *testing.T, body []byte) []responsesSSEEvent {
	t.Helper()
	var events []responsesSSEEvent
	for _, block := range strings.Split(string(body), "\n\n") {
		block = strings.TrimSpace(block)
		if block == "" {
			continue
		}
		var eventType, dataLine string
		for _, line := range strings.Split(block, "\n") {
			if strings.HasPrefix(line, "event: ") {
				eventType = strings.TrimPrefix(line, "event: ")
			} else if strings.HasPrefix(line, "data: ") {
				dataLine = strings.TrimPrefix(line, "data: ")
			}
		}
		if eventType == "" || dataLine == "" {
			continue
		}
		var payload map[string]any
		if err := json.Unmarshal([]byte(dataLine), &payload); err != nil {
			t.Fatalf("parse SSE data of %s: %v", eventType, err)
		}
		events = append(events, responsesSSEEvent{eventType: eventType, payload: payload})
	}
	return events
}

// normalizeEventTimestamps 递归归一化两侧已知形态差异，使事件 payload 深度可比：
//   - created_at/completed_at 时间戳：删除
//   - 时间戳生成的 resp_/msg_ ID：替换为占位
//   - prompt/conversation：relaykit 类型化 DTO 恒输出 null 键、legacy map 不含
//     （官方 Responses API 本就含这两个 null 字段，语义等价）：删除
//   - usage 细分的零值键：legacy 显式输出 0、relaykit omitempty 省略（数值一致）：删除零值键
func normalizeEventTimestamps(m map[string]any) {
	if _, hasUsage := m["input_tokens"]; hasUsage {
		for _, detailsKey := range []string{"input_tokens_details", "output_tokens_details"} {
			if details, ok := m[detailsKey].(map[string]any); ok {
				deleteZeroValues(details)
			}
		}
	}
	for k, v := range m {
		switch vv := v.(type) {
		case map[string]any:
			normalizeEventTimestamps(vv)
		case []any:
			if len(vv) == 0 {
				// legacy 恒输出 annotations:[]，relaykit 类型化 DTO omitempty 省略空数组（语义等价）
				delete(m, k)
			}
			for _, item := range vv {
				if child, ok := item.(map[string]any); ok {
					normalizeEventTimestamps(child)
				}
			}
		case string:
			if k == "id" || k == "item_id" {
				if strings.HasPrefix(vv, "resp_") || strings.HasPrefix(vv, "msg_") {
					trimmed := strings.TrimPrefix(strings.TrimPrefix(vv, "resp_"), "msg_")
					if isAllDigits(trimmed) {
						m[k] = "normalized"
					}
				}
			}
		}
	}
	for _, key := range []string{"created_at", "completed_at", "prompt", "conversation"} {
		delete(m, key)
	}
}

// deleteZeroValues 删除值为数值 0 的键（legacy 显式零值 vs relaykit omitempty 省略的形态差）。
func deleteZeroValues(m map[string]any) {
	for k, v := range m {
		if n, ok := v.(float64); ok && n == 0 {
			delete(m, k)
		}
	}
}

func isAllDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// 以下两个变量将对拍测试钉在 relaykit 桥接入口上（防止误删 import 后静默走不到新路径）。
var (
	_ = relaykit_bridge.TryConvertResponsesResponseViaRelaykit
	_ = relaykit_bridge.TryConvertResponsesStreamViaRelaykit
	_ = common.NewStreamStatus
)

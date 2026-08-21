package oai_responses

// 回归测试：Responses 扁平 tool_choice 的包装转换（审查发现，修复后固化）。

import (
	"encoding/json"
	"testing"
)

// TestR2CConvertToolChoice_FlatForm 回归：Responses API 的扁平 tool_choice
// {"type":"function","name":...} 此前原样透传，chat 上游要求嵌套
// {"type":"function","function":{"name":...}}——严格上游直接 400。
func TestR2CConvertToolChoice_FlatForm(t *testing.T) {
	out := r2cConvertToolChoice(json.RawMessage(`{"type":"function","name":"get_weather"}`))
	m, ok := out.(map[string]any)
	if !ok {
		t.Fatalf("tool_choice = %#v, want map", out)
	}
	if m["type"] != "function" {
		t.Errorf("type = %v, want function", m["type"])
	}
	fn, ok := m["function"].(map[string]any)
	if !ok {
		t.Fatalf("function = %#v, want nested map（扁平形态未被包装）", m["function"])
	}
	if fn["name"] != "get_weather" {
		t.Errorf("function.name = %v, want get_weather", fn["name"])
	}
}

package shared

import (
	"encoding/json"
	"strings"
	"testing"
)

// assertGeminiSchemaValid 递归校验输出满足 Gemini 的硬性约束：
//   - 每个节点都有 type，或是 anyOf 并集节点
//   - type=array 的节点必须带 items
//   - 不含白名单外的关键字（如 prefixItems / $schema / additionalProperties）
func assertGeminiSchemaValid(t *testing.T, node any, path string) {
	t.Helper()
	m, ok := node.(map[string]any)
	if !ok {
		t.Fatalf("%s: 应为对象节点，实际 %T", path, node)
	}
	_, hasType := m["type"]
	_, hasAnyOf := m["anyOf"]
	if !hasType && !hasAnyOf {
		t.Errorf("%s: 节点缺 type（Gemini 拒绝空/无类型 Schema）: %v", path, m)
	}
	if hasType {
		if typ, _ := m["type"].(string); strings.EqualFold(typ, "array") {
			if _, hasItems := m["items"]; !hasItems {
				t.Errorf("%s: 数组节点缺 items（Gemini 报 missing field）", path)
			}
		}
	}
	for k, v := range m {
		if _, allowed := geminiSchemaAllowedKeys[k]; !allowed {
			t.Errorf("%s: 含白名单外关键字 %q", path, k)
		}
		switch k {
		case "properties":
			for pk, pv := range v.(map[string]any) {
				assertGeminiSchemaValid(t, pv, path+".properties."+pk)
			}
		case "items":
			assertGeminiSchemaValid(t, v, path+".items")
		case "anyOf":
			for i, el := range v.([]any) {
				assertGeminiSchemaValid(t, el, path+".anyOf["+string(rune('0'+i))+"]")
			}
		}
	}
}

// TestCleanGeminiToolParams_PrefixItemsTuple 线上真实形状（2026-09 采集的 Claude Code
// Grep 工具 where 参数）：JSON Schema 2020-12 的 prefixItems 元组 + 空元素节点。
// 曾导致 Gemini 400：GenerateContentRequest...properties[where].items.items: missing field
// （Gemini 丢弃 prefixItems 后剩下无 items 的裸数组）。
func TestCleanGeminiToolParams_PrefixItemsTuple(t *testing.T) {
	// 取自语料 real_inputs 的真实 schema（工具名与枚举值已脱敏，结构原样）
	raw := `{
	  "type": "object",
	  "properties": {
	    "query": {
	      "type": "object",
	      "properties": {
	        "where": {
	          "type": "array",
	          "maxItems": 10,
	          "items": {
	            "type": "array",
	            "prefixItems": [
	              {"type": "string"},
	              {"enum": ["AND", "OR"], "type": "string"},
	              {}
	            ]
	          }
	        },
	        "path": {"type": "array", "items": {"type": "string"}},
	        "pattern": {"type": "string"}
	      }
	    }
	  }
}`
	var in map[string]any
	if err := json.Unmarshal([]byte(raw), &in); err != nil {
		t.Fatal(err)
	}

	out := CleanGeminiToolParams(in).(map[string]any)
	assertGeminiSchemaValid(t, out, "schema")

	if s := JSONString(t, out); strings.Contains(s, "prefixItems") {
		t.Errorf("prefixItems 应被折叠，不应出现在输出: %s", s)
	}
	// where.items 应变为 {type:array, items:{anyOf:[...]}}，三元组成员各带 type
	where := out["properties"].(map[string]any)["query"].(map[string]any)["properties"].(map[string]any)["where"].(map[string]any)
	items := where["items"].(map[string]any)
	folded, ok := items["items"].(map[string]any)
	if !ok {
		t.Fatalf("where.items.items 应为折叠后的 anyOf 容器: %s", JSONString(t, items))
	}
	anyOf, ok := folded["anyOf"].([]any)
	if !ok || len(anyOf) != 3 {
		t.Fatalf("where.items.items.anyOf 应含 3 个成员（含空节点兜底）: %s", JSONString(t, items))
	}
}

// TestCleanGeminiToolParams_NestedWhitelist 白名单过滤必须递归到嵌套层级——
// 早期版本只过滤顶层，嵌套的 $defs / additionalProperties 原样透传。
func TestCleanGeminiToolParams_NestedWhitelist(t *testing.T) {
	raw := `{
	  "type": "object",
	  "$schema": "https://json-schema.org/draft/2020-12/schema",
	  "properties": {
	    "loc": {
	      "type": "object",
	      "properties": {"city": {"type": "string", "$defs": {}, "additionalProperties": false}},
	      "additionalProperties": false
	    }
	  }
}`
	var in map[string]any
	if err := json.Unmarshal([]byte(raw), &in); err != nil {
		t.Fatal(err)
	}
	out := CleanGeminiToolParams(in).(map[string]any)
	assertGeminiSchemaValid(t, out, "schema")
	if s := JSONString(t, out); strings.Contains(s, "$schema") || strings.Contains(s, "additionalProperties") || strings.Contains(s, "$defs") {
		t.Errorf("嵌套层级的非白名单关键字未被过滤: %s", s)
	}
}

// TestCleanGeminiToolParams_BareArrayAndTypeFallback 裸数组补 items、
// 无 type 节点按内容推断兜底。
func TestCleanGeminiToolParams_BareArrayAndTypeFallback(t *testing.T) {
	raw := `{
	  "type": "object",
	  "properties": {
	    "tags": {"type": "array", "maxItems": 10},
	    "note": {"description": "无类型字段"},
	    "meta": {"properties": {"k": {"enum": ["a", "b"]}}},
	    "empty": {}
	  }
}`
	var in map[string]any
	if err := json.Unmarshal([]byte(raw), &in); err != nil {
		t.Fatal(err)
	}
	out := CleanGeminiToolParams(in).(map[string]any)
	assertGeminiSchemaValid(t, out, "schema")

	props := out["properties"].(map[string]any)
	if got := JSONString(t, props["tags"]); !strings.Contains(got, `"items"`) {
		t.Errorf("裸数组应补 items: %s", got)
	}
	if got := props["note"].(map[string]any)["type"]; got != "string" {
		t.Errorf("无类型节点应兜底 string，实际 %v", got)
	}
	if got := props["meta"].(map[string]any)["type"]; got != "object" {
		t.Errorf("含 properties 的节点应兜底 object，实际 %v", got)
	}
	if got := props["meta"].(map[string]any)["properties"].(map[string]any)["k"].(map[string]any)["type"]; got != "string" {
		t.Errorf("含 enum 的节点应兜底 string，实际 %v", got)
	}
	if got := props["empty"].(map[string]any)["type"]; got != "string" {
		t.Errorf("空节点应兜底 string，实际 %v", got)
	}
}

// TestCleanGeminiToolParams_NonMapPassthrough 非对象输入（nil / 字符串等）原样返回。
func TestCleanGeminiToolParams_NonMapPassthrough(t *testing.T) {
	if got := CleanGeminiToolParams(nil); got != nil {
		t.Errorf("nil 应原样返回，实际 %v", got)
	}
	s := "not-a-schema"
	if got := CleanGeminiToolParams(s); got != s {
		t.Errorf("字符串应原样返回，实际 %v", got)
	}
}

func JSONString(t *testing.T, v any) string {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

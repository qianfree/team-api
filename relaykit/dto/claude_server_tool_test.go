package dto

import (
	"encoding/json"
	"testing"
)

// TestClaudeServerToolUsageUnmarshal server_tool_use 动态解析：仅接受
// 「工具名_requests」且值为正整数的字段，其余字段/坏值逐个忽略不阻断
func TestClaudeServerToolUsageUnmarshal(t *testing.T) {
	raw := `{
		"web_search_requests": 3,
		"web_fetch_requests": 0,
		"code_execution_requests": 2,
		"browser_use_requests": 5,
		"total_tokens": 100,
		"bad_field": "not-a-number",
		"_requests": 7
	}`
	var u ClaudeServerToolUsage
	if err := json.Unmarshal([]byte(raw), &u); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	want := map[string]int{"web_search": 3, "code_execution": 2, "browser_use": 5}
	if len(u.Counts) != len(want) {
		t.Fatalf("Counts = %v, want %v", u.Counts, want)
	}
	for tool, n := range want {
		if u.Count(tool) != n {
			t.Errorf("Count(%s) = %d, want %d", tool, u.Counts[tool], n)
		}
	}

	// 零值 / 非数字 / 无工具名前缀 / 非后缀字段均被忽略：
	// web_fetch_requests=0、total_tokens、bad_field、_requests 不应出现
	for _, bad := range []string{"web_fetch", "total", "bad_field", ""} {
		if _, ok := u.Counts[bad]; ok {
			t.Errorf("Counts 不应包含 %q（%v）", bad, u.Counts)
		}
	}
}

// TestClaudeServerToolUsageMarshalRoundTrip 序列化对称还原：工具名加回
// _requests 后缀，round-trip 后计数不丢（协议转换桥重组响应保持 wire 形态）
func TestClaudeServerToolUsageMarshalRoundTrip(t *testing.T) {
	u := ClaudeServerToolUsage{Counts: map[string]int{"web_search": 3, "browser_use": 1}}
	data, err := json.Marshal(u)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var back ClaudeServerToolUsage
	if err := json.Unmarshal(data, &back); err != nil {
		t.Fatalf("Unmarshal round-trip: %v", err)
	}
	for tool, n := range u.Counts {
		if back.Count(tool) != n {
			t.Errorf("round-trip Count(%s) = %d, want %d", tool, back.Count(tool), n)
		}
	}
}

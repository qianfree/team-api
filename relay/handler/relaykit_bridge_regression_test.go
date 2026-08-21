package handler

// 回归测试：relaykit 请求桥接的方向覆盖缺口（审查发现的 bug 修复后固化）。

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/qianfree/team-api/relay/constant"
)

// TestConvertRequestViaRelaykit_ClaudeInboundToGemini 回归：claude 入站 → gemini 上游的
// 链式请求转换曾因 parseInboundRequest 缺 claude 分支而恒回退，落到 gemini adaptor 的
// 收割后硬错误（claude 客户端打 gemini 渠道 100% 失败）。
func TestConvertRequestViaRelaykit_ClaudeInboundToGemini(t *testing.T) {
	info := newRequestTestRelayInfo(constant.ProviderGemini, "gemini-2.0-flash", constant.RelayModeChatCompletions)
	info.InboundFormat = constant.RelayFormatClaude
	info.OriginModelName = "claude-3-5-sonnet-20241022"

	body := `{"model":"claude-3-5-sonnet-20241022","max_tokens":256,"messages":[{"role":"user","content":[{"type":"text","text":"hi"}]}]}`
	r, ok, err := tryConvertRequestViaRelaykit(context.Background(), info, []byte(body))
	if err != nil {
		t.Fatalf("tryConvertRequestViaRelaykit error: %v", err)
	}
	if !ok {
		t.Fatal("claude→gemini 请求链未被桥接接管（parseInboundRequest 缺 claude 分支回归）")
	}

	b := readAll(t, r)
	assertHasKeys(t, b, "contents")

	// 文本内容须保留（链式 claude→openai→gemini）
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("converted body is not JSON: %v", err)
	}
	contents, _ := m["contents"].([]any)
	if len(contents) == 0 {
		t.Fatalf("contents is empty: %s", b)
	}
	first, _ := contents[0].(map[string]any)
	parts, _ := first["parts"].([]any)
	if len(parts) == 0 {
		t.Fatalf("contents[0].parts is empty: %s", b)
	}
	part, _ := parts[0].(map[string]any)
	if part["text"] != "hi" {
		t.Errorf("contents[0].parts[0].text = %v, want \"hi\"", part["text"])
	}
}

package handler

import (
	"context"
	"encoding/json"
	"io"
	"testing"

	"github.com/qianfree/team-api/relay/common"
	"github.com/qianfree/team-api/relay/constant"
)

// newRequestTestRelayInfo 构造请求桥接测试用的 RelayInfo（OpenAI 入站，指定上游渠道）。
func newRequestTestRelayInfo(channel constant.ProviderType, upstreamModel string, relayMode constant.RelayMode) *common.RelayInfo {
	return &common.RelayInfo{
		RequestID:       "test-req",
		OriginModelName: "gpt-4",
		InboundFormat:   constant.RelayFormatOpenAI,
		RelayMode:       int(relayMode),
		ChannelMeta: &common.ChannelMeta{
			ChannelType:       int(channel),
			UpstreamModelName: upstreamModel,
		},
	}
}

// readAll 读取 io.Reader 全部内容。
func readAll(t *testing.T, r io.Reader) []byte {
	t.Helper()
	if r == nil {
		return nil
	}
	b, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("readAll: %v", err)
	}
	return b
}

// assertHasKeys 断言 JSON 对象包含指定顶层键。
func assertHasKeys(t *testing.T, body []byte, keys ...string) {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(body, &m); err != nil {
		t.Fatalf("body is not a JSON object: %v (body=%s)", err, body)
	}
	for _, k := range keys {
		if _, ok := m[k]; !ok {
			t.Errorf("expected key %q in converted body, got keys: %v", k, mapKeys(m))
		}
	}
}

func mapKeys(m map[string]any) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

// openAIRequestBody 构造一段最小合法 OpenAI Chat 请求体（含 max_tokens，避免 Claude 需要 hook）。
const openAIRequestBody = `{"model":"gpt-4","max_tokens":256,"messages":[{"role":"user","content":"hi"}]}`

func TestConvertRequestViaRelaykit_AllProviders(t *testing.T) {
	cases := []struct {
		name      string
		channel   constant.ProviderType
		upstream  string
		wantKeys  []string
	}{
		{"Claude", constant.ProviderClaude, "claude-3-5-sonnet-20241022", []string{"messages", "max_tokens"}},
		{"Gemini", constant.ProviderGemini, "gemini-2.0-flash", []string{"contents"}},
		{"Coze", constant.ProviderCoze, "bot-123", []string{"bot_id", "query"}},
		{"Dify", constant.ProviderDify, "dify-bot", []string{"query", "response_mode"}},
		{"Ollama", constant.ProviderOllama, "llama3", []string{"model", "messages"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			info := newRequestTestRelayInfo(c.channel, c.upstream, constant.RelayModeChatCompletions)
			r, ok := convertRequestViaRelaykit(context.Background(), info, []byte(openAIRequestBody))
			if !ok {
				t.Fatal("expected conversion to succeed")
			}
			body := readAll(t, r)
			if len(body) == 0 {
				t.Fatal("expected non-empty converted body")
			}
			assertHasKeys(t, body, c.wantKeys...)
		})
	}
}

func TestConvertRequestViaRelaykit_OllamaNonChatFallback(t *testing.T) {
	// Ollama 仅注册 chat 转换器；generate/embedding 模式应回退（ok=false）
	info := newRequestTestRelayInfo(constant.ProviderOllama, "llama3", constant.RelayModeEmbeddings)
	if _, ok := convertRequestViaRelaykit(context.Background(), info, []byte(openAIRequestBody)); ok {
		t.Fatal("expected ok=false for Ollama non-chat mode")
	}
}

func TestConvertRequestViaRelaykit_SameFormatFallback(t *testing.T) {
	// OpenAI 渠道 + OpenAI 入站：同格式 → 无转换器 → false
	info := newRequestTestRelayInfo(constant.ProviderOpenAI, "gpt-4", constant.RelayModeChatCompletions)
	if _, ok := convertRequestViaRelaykit(context.Background(), info, []byte(openAIRequestBody)); ok {
		t.Fatal("expected ok=false for same format")
	}
}

func TestConvertRequestViaRelaykit_MalformedBodyFallback(t *testing.T) {
	info := newRequestTestRelayInfo(constant.ProviderClaude, "claude-x", constant.RelayModeChatCompletions)
	if _, ok := convertRequestViaRelaykit(context.Background(), info, []byte(`not-json`)); ok {
		t.Fatal("expected ok=false for malformed body")
	}
}

func TestTryConvertRequestViaRelaykit_NilGuards(t *testing.T) {
	if _, ok := tryConvertRequestViaRelaykit(context.Background(), nil, []byte(openAIRequestBody)); ok {
		t.Fatal("expected ok=false for nil info")
	}
	info := newRequestTestRelayInfo(constant.ProviderClaude, "claude-x", constant.RelayModeChatCompletions)
	info.ChannelMeta = nil
	if _, ok := tryConvertRequestViaRelaykit(context.Background(), info, []byte(openAIRequestBody)); ok {
		t.Fatal("expected ok=false for nil ChannelMeta")
	}
}

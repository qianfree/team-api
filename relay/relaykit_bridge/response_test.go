package relaykit_bridge

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/qianfree/team-api/relay/constant"
	"github.com/qianfree/team-api/relay/dto"
	"github.com/qianfree/team-api/relaykit/relayconvert"
)

// --- 纯辅助函数表驱动 ---

func TestResponseConverterIDForRoute(t *testing.T) {
	cases := []struct {
		upstream, client constant.RelayFormat
		want             string
	}{
		{constant.RelayFormatClaude, constant.RelayFormatOpenAI, relayconvert.ConverterOpenAIChatToClaudeMessages},
		{constant.RelayFormatGemini, constant.RelayFormatOpenAI, relayconvert.ConverterOpenAIChatToGeminiContent},
		{constant.RelayFormatOllama, constant.RelayFormatOpenAI, relayconvert.ConverterOpenAIChatToOllama},
		{constant.RelayFormatOpenAI, constant.RelayFormatOpenAI, ""},                                                  // 同格式
		{constant.RelayFormatClaude, constant.RelayFormatGemini, relayconvert.ConverterGeminiContentToClaudeMessages}, // 跨原生：Gemini 客户端·Claude 上游
	}
	for _, c := range cases {
		if got := ResponseConverterIDForRoute(c.upstream, c.client); got != c.want {
			t.Errorf("upstream=%s client=%s: got %q, want %q", c.upstream, c.client, got, c.want)
		}
	}
}

// --- UsageFromConvertedChatResponse ---

func TestUsageFromConvertedChatResponse(t *testing.T) {
	body, _ := json.Marshal(dto.ChatCompletionResponse{
		Usage: dto.UsageWithDetails{
			PromptTokens: 5, CompletionTokens: 3, TotalTokens: 8,
			PromptTokensDetails:    &dto.TokenDetails{CachedTokens: 2, CachedCreationTokens: 1},
			CompletionTokenDetails: &dto.TokenDetails{ReasoningTokens: 4},
		},
	})
	usage, ok := UsageFromConvertedChatResponse(body)
	if !ok {
		t.Fatal("expected ok=true")
	}
	if usage == nil || usage.PromptTokens != 5 || usage.CompletionTokens != 3 ||
		usage.CacheCreationTokens != 1 || usage.PromptTokensDetails == nil ||
		usage.PromptTokensDetails.CachedTokens != 2 || usage.CompletionTokenDetails == nil ||
		usage.CompletionTokenDetails.ReasoningTokens != 4 {
		t.Errorf("unexpected usage: %+v", usage)
	}

	// 畸形 JSON
	if usage, ok := UsageFromConvertedChatResponse([]byte("not-json")); ok || usage != nil {
		t.Errorf("expected (nil,false) for malformed body, got (%+v,%v)", usage, ok)
	}
}

// --- convertResponseViaRelaykit 核心（config-free）---

func TestConvertResponseViaRelaykit_AllProviders(t *testing.T) {
	cases := []struct {
		name         string
		channel      constant.ProviderType
		upstreamBody string
	}{
		{
			name:    "Claude",
			channel: constant.ProviderClaude,
			upstreamBody: `{"id":"msg1","type":"message","role":"assistant","model":"claude-x",` +
				`"content":[{"type":"text","text":"hi"}],"stop_reason":"end_turn",` +
				`"usage":{"input_tokens":5,"output_tokens":3}}`,
		},
		{
			name:    "Gemini",
			channel: constant.ProviderGemini,
			upstreamBody: `{"candidates":[{"content":{"role":"model","parts":[{"text":"hi"}]},` +
				`"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":5,"candidatesTokenCount":3}}`,
		},
		{
			name:    "Ollama",
			channel: constant.ProviderOllama,
			upstreamBody: `{"model":"llama3","message":{"role":"assistant","content":"hi"},` +
				`"done":true,"prompt_eval_count":5,"eval_count":3}`,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			info := newStreamTestRelayInfo(c.channel, constant.RelayFormatOpenAI)
			body, usage, handled, err := convertResponseViaRelaykit(context.Background(), info, []byte(c.upstreamBody))
			if !handled || err != nil {
				t.Fatalf("expected conversion to succeed, handled=%v err=%v", handled, err)
			}
			if body == nil {
				t.Fatal("expected non-nil converted body")
			}
			if usage != nil {
				t.Errorf("adapter closure should discard usage (nil), got %+v", usage)
			}
			var resp dto.ChatCompletionResponse
			if err := json.Unmarshal(body, &resp); err != nil {
				t.Fatalf("converted body should be OpenAI response JSON: %v", err)
			}
			if len(resp.Choices) == 0 {
				t.Fatal("expected non-empty choices")
			}
			content, _ := resp.Choices[0].Message.Content.(string)
			if !strings.Contains(content, "hi") {
				t.Errorf("expected content to contain 'hi', got %q", content)
			}
		})
	}
}

func TestConvertResponseViaRelaykit_SameFormatFallback(t *testing.T) {
	// OpenAI 渠道 + OpenAI 客户端：upstream==client → 无转换器 → false
	info := newStreamTestRelayInfo(constant.ProviderOpenAI, constant.RelayFormatOpenAI)
	if _, _, handled, _ := convertResponseViaRelaykit(context.Background(), info, []byte(`{}`)); handled {
		t.Fatal("expected handled=false for same format")
	}
}

func TestConvertResponseViaRelaykit_ParseFailureFallback(t *testing.T) {
	// Claude 渠道但上游体非合法 Claude JSON → 回退
	info := newStreamTestRelayInfo(constant.ProviderClaude, constant.RelayFormatOpenAI)
	if _, _, handled, err := convertResponseViaRelaykit(context.Background(), info, []byte(`not-json`)); !handled || err == nil {
		t.Fatal("expected handled=true with error for malformed upstream body (hard-fail)")
	}
}

func TestConvertResponseViaRelaykit_ParseFailureAllStructuredProviders(t *testing.T) {
	// 结构化响应（需 json.Unmarshal）的供应商：畸形上游体都应回退（覆盖各自的 parse-fail 分支）。
	structured := []constant.ProviderType{
		constant.ProviderClaude, constant.ProviderGemini, constant.ProviderOllama,
	}
	for _, ch := range structured {
		info := newStreamTestRelayInfo(ch, constant.RelayFormatOpenAI)
		if _, _, handled, err := convertResponseViaRelaykit(context.Background(), info, []byte(`{not valid json`)); !handled || err == nil {
			t.Errorf("provider=%v: expected handled=true with error for malformed body", ch)
		}
	}
}

// --- 公开入口 nil 守卫 ---

func TestTryConvertResponseViaRelaykit_NilGuards(t *testing.T) {
	if _, _, handled, _ := TryConvertResponseViaRelaykit(context.Background(), nil, []byte(`{}`)); handled {
		t.Fatal("expected handled=false for nil info")
	}
	info := newStreamTestRelayInfo(constant.ProviderClaude, constant.RelayFormatOpenAI)
	info.ChannelMeta = nil
	if _, _, handled, _ := TryConvertResponseViaRelaykit(context.Background(), info, []byte(`{}`)); handled {
		t.Fatal("expected handled=false for nil ChannelMeta")
	}
}

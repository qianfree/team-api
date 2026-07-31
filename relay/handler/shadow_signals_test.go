package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/qianfree/team-api/relay/constant"
	"github.com/qianfree/team-api/relaykit/dispatch"
)

func TestExtractSessionSignals(t *testing.T) {
	body := []byte(`{
		"model": "claude-sonnet",
		"metadata": {"user_id": "user_abc_session_11111111-2222-3333-4444-555555555555"},
		"previous_response_id": "resp_123",
		"conversation_id": "conv_456"
	}`)
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(body, &raw); err != nil {
		t.Fatal(err)
	}

	sig := extractSessionSignals(raw)
	if sig.AnthropicUserID != "user_abc_session_11111111-2222-3333-4444-555555555555" {
		t.Errorf("AnthropicUserID = %q", sig.AnthropicUserID)
	}
	if sig.PreviousResponseID != "resp_123" {
		t.Errorf("PreviousResponseID = %q", sig.PreviousResponseID)
	}
	if sig.ConversationID != "conv_456" {
		t.Errorf("ConversationID = %q", sig.ConversationID)
	}

	// 无信号的请求体
	var empty map[string]json.RawMessage
	_ = json.Unmarshal([]byte(`{"model":"gpt-4o"}`), &empty)
	sig = extractSessionSignals(empty)
	if sig.AnthropicUserID != "" || sig.PreviousResponseID != "" || sig.ConversationID != "" {
		t.Errorf("空请求体不应提取出信号: %+v", sig)
	}

	// metadata 非对象（畸形）不 panic
	var malformed map[string]json.RawMessage
	_ = json.Unmarshal([]byte(`{"metadata": "not-an-object"}`), &malformed)
	_ = extractSessionSignals(malformed)
}

func TestReplayabilityOf(t *testing.T) {
	tests := []struct {
		mode constant.RelayMode
		want dispatch.Replayability
	}{
		{constant.RelayModeImagesGenerations, dispatch.ReplayUnsafe},
		{constant.RelayModeImagesEdits, dispatch.ReplayUnsafe},
		{constant.RelayModeVideoGenerations, dispatch.ReplayUnsafe},
		{constant.RelayModeEmbeddings, dispatch.ReplaySafe},
		{constant.RelayModeRerank, dispatch.ReplaySafe},
		{constant.RelayModeChatCompletions, dispatch.ReplayCostly},
		{constant.RelayModeClaudeMessages, dispatch.ReplayCostly},
	}
	for _, tt := range tests {
		if got := replayabilityOf(tt.mode); got != tt.want {
			t.Errorf("replayabilityOf(%v) = %v, want %v", tt.mode, got, tt.want)
		}
	}
}

func TestRetryAfterOf(t *testing.T) {
	// RelayError 携带 Retry-After → 提取
	err := constant.NewUpstreamError(429, "rate limited", nil).WithRetryAfter(1500 * time.Millisecond)
	if got := retryAfterOf(err); got != 1500*time.Millisecond {
		t.Errorf("retryAfterOf = %v, want 1.5s", got)
	}
	// 包装后仍可解包
	wrapped := fmt.Errorf("do response: %w", err)
	if got := retryAfterOf(wrapped); got != 1500*time.Millisecond {
		t.Errorf("包装后 retryAfterOf = %v, want 1.5s", got)
	}
	// 普通 error / nil → 0
	if got := retryAfterOf(errors.New("x")); got != 0 {
		t.Errorf("普通 error 应返回 0, got %v", got)
	}
	if got := retryAfterOf(nil); got != 0 {
		t.Errorf("nil 应返回 0, got %v", got)
	}
}

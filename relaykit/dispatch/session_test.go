package dispatch

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func profileWith(sig SessionSignals) RequestProfile {
	return RequestProfile{TenantID: 1, UserID: 2, APIKeyID: 3, Model: "gpt-4o", Signals: sig}
}

func defaultSessionPolicy() SessionPolicy {
	return DefaultRoutingPolicy().Session
}

func TestResolveSessionKey_ResolutionChainPriority(t *testing.T) {
	pol := defaultSessionPolicy()

	tests := []struct {
		name    string
		signals SessionSignals
		wantSrc SessionSource
	}{
		{"显式头优先", SessionSignals{HeaderSessionID: "sess-1", AnthropicUserID: "user_x_session_11111111-2222-3333-4444-555555555555", PreviousResponseID: "resp_1"}, SourceHeader},
		{"Anthropic 次之", SessionSignals{AnthropicUserID: "user_x_session_11111111-2222-3333-4444-555555555555", PreviousResponseID: "resp_1"}, SourceAnthropic},
		{"OpenAI previous_response_id", SessionSignals{PreviousResponseID: "resp_1"}, SourceOpenAI},
		{"OpenAI conversation_id", SessionSignals{ConversationID: "conv_1"}, SourceOpenAI},
		{"全空回退身份级", SessionSignals{}, SourceIdentity},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ResolveSessionKey(profileWith(tt.signals), pol)
			assert.Equal(t, tt.wantSrc, got.Source)
			assert.True(t, strings.HasPrefix(got.Key, "sk:"+string(tt.wantSrc)+":"))
		})
	}
}

func TestResolveSessionKey_AnthropicSessionSegmentExtraction(t *testing.T) {
	pol := defaultSessionPolicy()

	// 含 session UUID：提取段后哈希；两个不同 user 前缀但相同 session → 相同键
	a := ResolveSessionKey(profileWith(SessionSignals{
		AnthropicUserID: "user_aaa_account_9f1b0000-0000-0000-0000-000000000001_session_11111111-2222-3333-4444-555555555555",
	}), pol)
	b := ResolveSessionKey(profileWith(SessionSignals{
		AnthropicUserID: "user_bbb_account_9f1b0000-0000-0000-0000-000000000002_session_11111111-2222-3333-4444-555555555555",
	}), pol)
	assert.Equal(t, a.Key, b.Key, "相同 session 段应得到相同会话键")

	// 无 session 段：整个 user_id 作为会话键，仍为 anthropic 来源
	c := ResolveSessionKey(profileWith(SessionSignals{AnthropicUserID: "user_no_session_marker"}), pol)
	assert.Equal(t, SourceAnthropic, c.Source)
	assert.NotEqual(t, a.Key, c.Key)
}

func TestResolveSessionKey_PolicyToggles(t *testing.T) {
	pol := defaultSessionPolicy()
	pol.ParseAnthropicMetadata = false
	got := ResolveSessionKey(profileWith(SessionSignals{AnthropicUserID: "user_x_session_11111111-2222-3333-4444-555555555555"}), pol)
	assert.Equal(t, SourceIdentity, got.Source, "关闭 anthropic 解析后应回退身份级")

	pol = defaultSessionPolicy()
	pol.ParseOpenAIResponses = false
	got = ResolveSessionKey(profileWith(SessionSignals{PreviousResponseID: "resp_1"}), pol)
	assert.Equal(t, SourceIdentity, got.Source, "关闭 openai 解析后应回退身份级")
}

func TestResolveSessionKey_SkipsInvalidSignal(t *testing.T) {
	pol := defaultSessionPolicy()

	tests := []struct {
		name  string
		token string
	}{
		{"超长头", strings.Repeat("a", maxHeaderTokenLen+1)},
		{"控制字符", "abc\x00def"},
		{"换行注入", "abc\ndef"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ResolveSessionKey(profileWith(SessionSignals{HeaderSessionID: tt.token}), pol)
			assert.Equal(t, SourceIdentity, got.Source, "非法头应跳过并落到身份级")
		})
	}
}

func TestResolveSessionKey_NamespaceIsolation(t *testing.T) {
	pol := defaultSessionPolicy()
	sig := SessionSignals{HeaderSessionID: "same-session"}

	p1 := profileWith(sig)
	p2 := profileWith(sig)
	p2.TenantID = 99
	assert.NotEqual(t, ResolveSessionKey(p1, pol).Key, ResolveSessionKey(p2, pol).Key, "不同租户相同 session id 不应碰撞")

	p3 := profileWith(sig)
	p3.Model = "claude-sonnet"
	assert.NotEqual(t, ResolveSessionKey(p1, pol).Key, ResolveSessionKey(p3, pol).Key, "不同模型的绑定应互相独立")
}

func TestResolveSessionKey_Deterministic(t *testing.T) {
	pol := defaultSessionPolicy()
	p := profileWith(SessionSignals{HeaderSessionID: "stable"})
	first := ResolveSessionKey(p, pol)
	for range 100 {
		assert.Equal(t, first, ResolveSessionKey(p, pol))
	}
}

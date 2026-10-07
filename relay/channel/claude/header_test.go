package claude

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/qianfree/team-api/relay/common"
)

const testBetaHeader = "context-management-2025-06-27,interleaved-thinking-2025-05-14"

func newHeaderTestInfo(apiKey string) *common.RelayInfo {
	return &common.RelayInfo{
		ChannelMeta:    &common.ChannelMeta{ApiKey: apiKey},
		RequestHeaders: http.Header{"Anthropic-Beta": {testBetaHeader}, "X-Request-Id": {"req_test_123"}},
	}
}

// TestSetupRequestHeader_APIKeyForwardsBeta API Key 模式：客户端的 anthropic-beta
// 与 X-Request-Id 透传到上游（既有行为回归保护）。
func TestSetupRequestHeader_APIKeyForwardsBeta(t *testing.T) {
	info := newHeaderTestInfo("sk-ant-test")
	adaptor := &Adaptor{}
	adaptor.Init(info)

	header := http.Header{}
	if err := adaptor.SetupRequestHeader(header, info); err != nil {
		t.Fatalf("SetupRequestHeader 错误: %v", err)
	}

	if got := header.Get("x-api-key"); got != "sk-ant-test" {
		t.Errorf("x-api-key = %q, want %q", got, "sk-ant-test")
	}
	if got := header.Get("anthropic-beta"); got != testBetaHeader {
		t.Errorf("anthropic-beta = %q, want %q", got, testBetaHeader)
	}
	if got := header.Get("X-Request-Id"); got != "req_test_123" {
		t.Errorf("X-Request-Id = %q, want %q", got, "req_test_123")
	}
}

// TestSetupRequestHeader_OAuthForwardsBeta OAuth 模式：anthropic-beta 同样必须透传，
// 否则 OAuth 渠道的能力协商值被剥离，配对的 body 字段会被上游 400 拒绝。
func TestSetupRequestHeader_OAuthForwardsBeta(t *testing.T) {
	oauthKey, err := json.Marshal(map[string]any{
		"platform":     "claude",
		"access_token": "test-access-token",
	})
	if err != nil {
		t.Fatalf("构造 OAuth key 失败: %v", err)
	}
	info := newHeaderTestInfo(string(oauthKey))
	adaptor := &Adaptor{}
	adaptor.Init(info)

	header := http.Header{}
	if err := adaptor.SetupRequestHeader(header, info); err != nil {
		t.Fatalf("SetupRequestHeader 错误: %v", err)
	}

	if got := header.Get("Authorization"); got != "Bearer test-access-token" {
		t.Errorf("Authorization = %q, want Bearer token", got)
	}
	if got := header.Get("anthropic-beta"); got != testBetaHeader {
		t.Errorf("anthropic-beta = %q, want %q", got, testBetaHeader)
	}
	if got := header.Get("X-Request-Id"); got != "req_test_123" {
		t.Errorf("X-Request-Id = %q, want %q", got, "req_test_123")
	}
}

// TestSetupRequestHeader_NoBetaInRequest 客户端未带 anthropic-beta 时不凭空设置该头。
func TestSetupRequestHeader_NoBetaInRequest(t *testing.T) {
	info := &common.RelayInfo{
		ChannelMeta:    &common.ChannelMeta{ApiKey: "sk-ant-test"},
		RequestHeaders: http.Header{},
	}
	adaptor := &Adaptor{}
	adaptor.Init(info)

	header := http.Header{}
	if err := adaptor.SetupRequestHeader(header, info); err != nil {
		t.Fatalf("SetupRequestHeader 错误: %v", err)
	}

	if got := header.Get("anthropic-beta"); got != "" {
		t.Errorf("anthropic-beta = %q, want 空（客户端未携带）", got)
	}
}

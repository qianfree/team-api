package volcengine

import (
	"net/http"
	"testing"

	"github.com/qianfree/team-api/relay/common"
)

const testBetaHeader = "context-management-2025-06-27,interleaved-thinking-2025-05-14"

// TestClaudeAdaptorSetupRequestHeader 火山 Anthropic 兼容端点：统一鉴权头之外
// 还须携带 Anthropic 协议头，且客户端的 anthropic-beta 能力协商值必须透传。
func TestClaudeAdaptorSetupRequestHeader(t *testing.T) {
	info := &common.RelayInfo{
		ChannelMeta:    &common.ChannelMeta{ApiKey: "volc-test-key"},
		RequestHeaders: http.Header{"Anthropic-Beta": {testBetaHeader}},
	}
	adaptor := &claudeAdaptor{}

	header := http.Header{}
	if err := adaptor.SetupRequestHeader(header, info); err != nil {
		t.Fatalf("SetupRequestHeader 错误: %v", err)
	}

	if got := header.Get("Authorization"); got != "Bearer volc-test-key" {
		t.Errorf("Authorization = %q, want %q", got, "Bearer volc-test-key")
	}
	if got := header.Get("anthropic-version"); got != "2023-06-01" {
		t.Errorf("anthropic-version = %q, want 2023-06-01", got)
	}
	if got := header.Get("anthropic-beta"); got != testBetaHeader {
		t.Errorf("anthropic-beta = %q, want %q", got, testBetaHeader)
	}
}

// TestClaudeAdaptorSetupRequestHeader_NoBeta 客户端未带 anthropic-beta 时不凭空设置。
func TestClaudeAdaptorSetupRequestHeader_NoBeta(t *testing.T) {
	info := &common.RelayInfo{
		ChannelMeta:    &common.ChannelMeta{ApiKey: "volc-test-key"},
		RequestHeaders: http.Header{},
	}
	adaptor := &claudeAdaptor{}

	header := http.Header{}
	if err := adaptor.SetupRequestHeader(header, info); err != nil {
		t.Fatalf("SetupRequestHeader 错误: %v", err)
	}

	if got := header.Get("anthropic-beta"); got != "" {
		t.Errorf("anthropic-beta = %q, want 空（客户端未携带）", got)
	}
}

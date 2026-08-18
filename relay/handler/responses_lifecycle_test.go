package handler

import (
	"testing"

	"github.com/qianfree/team-api/relay/channel/openai"
	"github.com/qianfree/team-api/relay/common"
	"github.com/qianfree/team-api/relay/constant"
)

// TestBuildLifecycleUpstreamURL 生命周期端点上游 URL：复用 adaptor 的 Responses
// POST 基址追加 /{id}（cancel 再追加 /cancel），对 openai 形状的渠道天然正确。
func TestBuildLifecycleUpstreamURL(t *testing.T) {
	info := &common.RelayInfo{
		RelayMode: int(constant.RelayModeResponses),
		ChannelMeta: &common.ChannelMeta{
			BaseURL:           "https://up.example.com",
			SupportsResponses: true,
		},
	}
	a := &openai.Adaptor{}

	got, err := buildLifecycleUpstreamURL(a, info, "resp_1", false)
	if err != nil {
		t.Fatalf("buildLifecycleUpstreamURL: %v", err)
	}
	if want := "https://up.example.com/v1/responses/resp_1"; got != want {
		t.Errorf("retrieve URL = %q, want %q", got, want)
	}

	got, err = buildLifecycleUpstreamURL(a, info, "resp_1", true)
	if err != nil {
		t.Fatalf("buildLifecycleUpstreamURL(cancel): %v", err)
	}
	if want := "https://up.example.com/v1/responses/resp_1/cancel"; got != want {
		t.Errorf("cancel URL = %q, want %q", got, want)
	}
}

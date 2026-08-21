package openai

import (
	"testing"

	"github.com/qianfree/team-api/relay/common"
	"github.com/qianfree/team-api/relay/constant"
	// blank import 触发内置转换器注册（relaykit 桥接为唯一路径，测试二进制须自备注册）
	_ "github.com/qianfree/team-api/relaykit/relayconvert/register"
)

// TestGetRequestURL_GeminiChat P1-A 遗留 bug 修复验证：gemini 客户端打 openai 兼容渠道
// 此前 RelayModeGeminiChat 落 default 报 unsupported relay mode。
func TestGetRequestURL_GeminiChat(t *testing.T) {
	adaptor := &Adaptor{}
	info := &common.RelayInfo{
		RelayMode: int(constant.RelayModeGeminiChat),
		ChannelMeta: &common.ChannelMeta{
			BaseURL: "https://upstream.example.com",
		},
	}
	// 普通渠道 → chat 端点（g2o 已把 body 转为 chat 格式）
	url, err := adaptor.GetRequestURL(info)
	if err != nil {
		t.Fatalf("GeminiChat mode 应受支持（P1-A 遗留 bug 修复）: %v", err)
	}
	if url != "https://upstream.example.com/v1/chat/completions" {
		t.Errorf("url = %q, want chat completions", url)
	}
	// ChatViaResponses 渠道（P3：gemini 入站也置 UseResponsesAPI）→ responses 端点
	info.UseResponsesAPI = true
	url, err = adaptor.GetRequestURL(info)
	if err != nil {
		t.Fatalf("GeminiChat + UseResponsesAPI: %v", err)
	}
	if url != "https://upstream.example.com/v1/responses" {
		t.Errorf("url = %q, want responses", url)
	}
}

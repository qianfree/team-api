package mistral

import (
	"testing"

	"github.com/qianfree/team-api/relay/common"
	"github.com/qianfree/team-api/relay/constant"
)

// TestGetRequestURL_Rerank 守卫 rerank 端点分支（/v1/rerank）不被重构丢失。
func TestGetRequestURL_Rerank(t *testing.T) {
	info := &common.RelayInfo{
		RelayMode: int(constant.RelayModeRerank),
		ChannelMeta: &common.ChannelMeta{
			BaseURL: "https://api.mistral.ai",
		},
	}
	adaptor := &Adaptor{}
	adaptor.Init(info)

	url, err := adaptor.GetRequestURL(info)
	if err != nil {
		t.Fatalf("GetRequestURL 错误: %v", err)
	}
	if want := "https://api.mistral.ai/v1/rerank"; url != want {
		t.Errorf("URL = %q, want %q", url, want)
	}
}

// TestGetRequestURL_UnsupportedModeErrors 守卫 default 分支显式报错：
// 此前 default 静默回落 chat 端点，不支持的模式会带原体打错端点。
func TestGetRequestURL_UnsupportedModeErrors(t *testing.T) {
	info := &common.RelayInfo{
		RelayMode: int(constant.RelayModeAudioSpeech),
		ChannelMeta: &common.ChannelMeta{
			BaseURL: "https://api.mistral.ai",
		},
	}
	adaptor := &Adaptor{}
	adaptor.Init(info)

	if _, err := adaptor.GetRequestURL(info); err == nil {
		t.Errorf("不支持的 relay mode 应显式报错，实际返回了 URL")
	}
}

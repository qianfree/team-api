package siliconflow

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
			BaseURL: "https://api.siliconflow.cn",
		},
	}
	adaptor := &Adaptor{}
	adaptor.Init(info)

	url, err := adaptor.GetRequestURL(info)
	if err != nil {
		t.Fatalf("GetRequestURL 错误: %v", err)
	}
	if want := "https://api.siliconflow.cn/v1/rerank"; url != want {
		t.Errorf("URL = %q, want %q", url, want)
	}
}

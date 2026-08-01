package tencent

import (
	"testing"

	"github.com/qianfree/team-api/relay/channel/openai"
	"github.com/qianfree/team-api/relay/common"
)

func TestDispatchAdaptorInit(t *testing.T) {
	tests := []struct {
		name        string
		apiKey      string
		baseURL     string
		wantTC3     bool
		wantBaseURL string
	}{
		{
			name:        "secretId|secretKey 选择 TC3 适配器并保留混元地址",
			apiKey:      "AKIDxxxxxxxx|secretxxxxxxxx",
			baseURL:     "https://hunyuan.tencentcloudapi.com",
			wantTC3:     true,
			wantBaseURL: "https://hunyuan.tencentcloudapi.com",
		},
		{
			name:        "单段 TokenHub Key + 混元默认地址 改写为 TokenHub",
			apiKey:      "sk-xxxxxxxxxxxxxxxx",
			baseURL:     "https://hunyuan.tencentcloudapi.com",
			wantTC3:     false,
			wantBaseURL: tokenHubBaseURL,
		},
		{
			name:        "单段 TokenHub Key + 空地址 改写为 TokenHub",
			apiKey:      "sk-xxxxxxxxxxxxxxxx",
			baseURL:     "",
			wantTC3:     false,
			wantBaseURL: tokenHubBaseURL,
		},
		{
			name:        "单段 TokenHub Key + 自定义地址 保留自定义",
			apiKey:      "sk-xxxxxxxxxxxxxxxx",
			baseURL:     "https://proxy.example.com",
			wantTC3:     false,
			wantBaseURL: "https://proxy.example.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info := &common.RelayInfo{
				ChannelMeta: &common.ChannelMeta{
					ApiKey:  tt.apiKey,
					BaseURL: tt.baseURL,
				},
			}

			dispatch := &DispatchAdaptor{}
			dispatch.Init(info)

			if dispatch.Adaptor == nil {
				t.Fatalf("Init 后子适配器为空")
			}
			if tt.wantTC3 {
				if _, ok := dispatch.Adaptor.(*Adaptor); !ok {
					t.Errorf("期望子适配器类型 *tencent.Adaptor，实际 %T", dispatch.Adaptor)
				}
			} else {
				if _, ok := dispatch.Adaptor.(*openai.Adaptor); !ok {
					t.Errorf("期望子适配器类型 *openai.Adaptor，实际 %T", dispatch.Adaptor)
				}
			}
			if info.ChannelMeta.BaseURL != tt.wantBaseURL {
				t.Errorf("BaseURL 期望 %q，实际 %q", tt.wantBaseURL, info.ChannelMeta.BaseURL)
			}
		})
	}
}

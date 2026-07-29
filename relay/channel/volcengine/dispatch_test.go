package volcengine

import (
	"testing"

	"github.com/qianfree/team-api/relay/common"
	"github.com/qianfree/team-api/relay/constant"
)

func TestDispatchAdaptorInit(t *testing.T) {
	tests := []struct {
		name       string
		format     constant.RelayFormat
		wantClaude bool
	}{
		{"Claude 入站 → claudeAdaptor", constant.RelayFormatClaude, true},
		{"OpenAI 入站 → openaiAdaptor", constant.RelayFormatOpenAI, false},
		{"Gemini 入站 → openaiAdaptor", constant.RelayFormatGemini, false},
		{"空格式兜底 → openaiAdaptor", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info := &common.RelayInfo{ClientFormat: tt.format}
			dispatch := &DispatchAdaptor{}
			dispatch.Init(info)

			if dispatch.Adaptor == nil {
				t.Fatalf("Init 后子适配器为空")
			}
			if tt.wantClaude {
				if _, ok := dispatch.Adaptor.(*claudeAdaptor); !ok {
					t.Errorf("期望子适配器类型 *volcengine.claudeAdaptor，实际 %T", dispatch.Adaptor)
				}
			} else {
				if _, ok := dispatch.Adaptor.(*openaiAdaptor); !ok {
					t.Errorf("期望子适配器类型 *volcengine.openaiAdaptor，实际 %T", dispatch.Adaptor)
				}
			}
		})
	}
}

func TestSubAdaptorGetRequestURL(t *testing.T) {
	const baseURL = "https://ark.cn-beijing.volces.com"

	tests := []struct {
		name    string
		adaptor common.Adaptor
		mode    constant.RelayMode
		model   string
		wantURL string
	}{
		{"openai chat 普通模型", &openaiAdaptor{}, constant.RelayModeChatCompletions, "doubao-pro", baseURL + "/api/v3/chat/completions"},
		{"openai chat bot- 前缀", &openaiAdaptor{}, constant.RelayModeChatCompletions, "bot-xxx", baseURL + "/api/v3/bots/chat/completions"},
		{"openai embeddings", &openaiAdaptor{}, constant.RelayModeEmbeddings, "doubao-embedding", baseURL + "/api/v3/embeddings"},
		{"openai images", &openaiAdaptor{}, constant.RelayModeImagesGenerations, "doubao-IMG", baseURL + "/api/v3/images/generations"},
		{"claude messages", &claudeAdaptor{}, constant.RelayModeClaudeMessages, "doubao-pro", baseURL + "/api/coding/v1/messages"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info := &common.RelayInfo{
				RelayMode:       int(tt.mode),
				OriginModelName: tt.model,
				ChannelMeta: &common.ChannelMeta{
					BaseURL:       baseURL,
					IsModelMapped: false,
				},
			}
			tt.adaptor.Init(info)

			url, err := tt.adaptor.GetRequestURL(info)
			if err != nil {
				t.Fatalf("GetRequestURL 错误: %v", err)
			}
			if url != tt.wantURL {
				t.Errorf("URL 期望 %q，实际 %q", tt.wantURL, url)
			}
		})
	}
}

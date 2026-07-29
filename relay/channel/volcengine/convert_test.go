package volcengine

import (
	"context"
	"io"
	"strings"
	"testing"

	"github.com/qianfree/team-api/relay/common"
	"github.com/qianfree/team-api/relay/constant"
)

func TestOpenaiAdaptorStreamOptionsInjection(t *testing.T) {
	ctx := context.Background()
	body := []byte(`{"model":"doubao-pro","messages":[{"role":"user","content":"hi"}],"stream":true}`)

	tests := []struct {
		name      string
		mode      constant.RelayMode
		isStream  bool
		wantInj   bool
	}{
		{"流式 chat 注入 stream_options", constant.RelayModeChatCompletions, true, true},
		{"非流式 chat 不注入", constant.RelayModeChatCompletions, false, false},
		{"流式 images 不注入（原生参数）", constant.RelayModeImagesGenerations, true, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info := &common.RelayInfo{
				RelayMode:     int(tt.mode),
				IsStream:      tt.isStream,
				InboundFormat: constant.RelayFormatOpenAI,
				ChannelMeta:   &common.ChannelMeta{IsModelMapped: false},
			}
			a := &openaiAdaptor{}
			a.Init(info)

			r, err := a.ConvertRequest(ctx, info, body)
			if err != nil {
				t.Fatalf("ConvertRequest 错误: %v", err)
			}
			out, _ := io.ReadAll(r)

			contains := strings.Contains(string(out), `"stream_options":{"include_usage":true}`)
			if tt.wantInj && !contains {
				t.Errorf("期望注入 stream_options，实际: %s", string(out))
			}
			if !tt.wantInj && strings.Contains(string(out), "stream_options") {
				t.Errorf("不应注入 stream_options，实际: %s", string(out))
			}
		})
	}
}

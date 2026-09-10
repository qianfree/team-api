package xai

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/qianfree/team-api/relay/common"
	"github.com/qianfree/team-api/relay/constant"
)

// TestPostProcessConvertedRequest_StripsSuffixAndInjects 后处理钩子的核心职责：
// relaykit 转换后的 chat 体里 model 仍带 -search/-high 后缀（relaykit 无条件写
// GetUpstreamModelName，未配映射时它就等于带后缀的原始模型名），上游不认这个名字。
// 钩子必须剥离后缀并注入 xAI 私有开关。
func TestPostProcessConvertedRequest_StripsSuffixAndInjects(t *testing.T) {
	tests := []struct {
		name      string
		model     string
		wantModel string
		wantKey   string
		wantVal   string
	}{
		{"search 后缀", "grok-4-search", "grok-4", "search_parameters", `{"mode":"on"}`},
		{"high 后缀", "grok-4-high", "grok-4", "reasoning_effort", `"high"`},
		{"low 后缀", "grok-4-low", "grok-4", "reasoning_effort", `"low"`},
		{"无后缀", "grok-4", "grok-4", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info := &common.RelayInfo{
				InboundFormat:   constant.RelayFormatGemini,
				OriginModelName: tt.model,
				ChannelMeta: &common.ChannelMeta{
					ChannelType:       int(constant.ProviderXAI),
					UpstreamModelName: tt.model, // 未配映射时上游名 = 原始名（带后缀）
				},
			}
			// 模拟 relaykit Gemini→OpenAI chat 转换后的产物
			body := []byte(`{"model":"` + tt.model + `","messages":[{"role":"user","content":"hi"}]}`)

			out, err := (&Adaptor{}).PostProcessConvertedRequest(context.Background(), info, body)
			if err != nil {
				t.Fatalf("PostProcessConvertedRequest error: %v", err)
			}

			var got map[string]json.RawMessage
			if err := json.Unmarshal(out, &got); err != nil {
				t.Fatalf("产物不是合法 JSON: %v\nbody=%s", err, out)
			}
			if string(got["model"]) != `"`+tt.wantModel+`"` {
				t.Errorf("model = %s, want %q（后缀未剥离会被上游拒绝）", got["model"], tt.wantModel)
			}
			if tt.wantKey == "" {
				if _, ok := got["search_parameters"]; ok {
					t.Error("无后缀不应注入 search_parameters")
				}
				if _, ok := got["reasoning_effort"]; ok {
					t.Error("无后缀不应注入 reasoning_effort")
				}
				return
			}
			if string(got[tt.wantKey]) != tt.wantVal {
				t.Errorf("%s = %s, want %s", tt.wantKey, got[tt.wantKey], tt.wantVal)
			}
			// 消息体不得被后处理破坏
			if _, ok := got["messages"]; !ok {
				t.Error("messages 丢失")
			}
		})
	}
}

// TestPostProcessConvertedRequest_ModelMappedWins 配了模型映射时以映射名为准，
// 不做后缀剥离（运营者已显式指定上游模型名）。
func TestPostProcessConvertedRequest_ModelMappedWins(t *testing.T) {
	info := &common.RelayInfo{
		InboundFormat:   constant.RelayFormatClaude,
		OriginModelName: "grok-4-search",
		ChannelMeta: &common.ChannelMeta{
			ChannelType:       int(constant.ProviderXAI),
			UpstreamModelName: "grok-4-0709",
			IsModelMapped:     true,
		},
	}
	out, err := (&Adaptor{}).PostProcessConvertedRequest(context.Background(), info,
		[]byte(`{"model":"grok-4-0709","messages":[]}`))
	if err != nil {
		t.Fatalf("PostProcessConvertedRequest error: %v", err)
	}
	var got map[string]json.RawMessage
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatalf("产物不是合法 JSON: %v", err)
	}
	if string(got["model"]) != `"grok-4-0709"` {
		t.Errorf("model = %s, want \"grok-4-0709\"", got["model"])
	}
	// -search 后缀在原始名上，映射后仍应注入搜索开关（能力后缀与上游模型名解耦）
	if string(got["search_parameters"]) != `{"mode":"on"}` {
		t.Errorf("search_parameters = %s, want {\"mode\":\"on\"}", got["search_parameters"])
	}
}

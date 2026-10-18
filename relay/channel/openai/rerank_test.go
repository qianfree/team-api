package openai

import (
	"context"
	"encoding/json"
	"io"
	"testing"

	"github.com/qianfree/team-api/relay/common"
	"github.com/qianfree/team-api/relay/constant"
)

// TestConvertRequest_RerankModelMapping 守卫 rerank 请求侧模型映射：
// 响应侧 handleRerankResponse 在 IsModelMapped 时会把模型名回写为公共名，
// 请求侧必须对称地替换为上游名，否则映射渠道的 rerank 请求带公共名发上游。
func TestConvertRequest_RerankModelMapping(t *testing.T) {
	tests := []struct {
		name      string
		mapped    bool
		upstream  string
		wantModel string
	}{
		{"无映射透传", false, "", "rerank-v3"},
		{"映射渠道替换为上游名", true, "cohere/rerank-v3", "cohere/rerank-v3"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info := &common.RelayInfo{
				RelayMode:       int(constant.RelayModeRerank),
				InboundFormat:   constant.RelayFormatOpenAI,
				OriginModelName: "rerank-v3",
				ChannelMeta: &common.ChannelMeta{
					BaseURL:           "https://upstream.example.com",
					IsModelMapped:     tt.mapped,
					UpstreamModelName: tt.upstream,
				},
			}
			adaptor := &Adaptor{}
			adaptor.Init(info)

			body := `{"model":"rerank-v3","query":"q"}`
			result, err := adaptor.ConvertRequest(context.Background(), info, []byte(body))
			if err != nil {
				t.Fatalf("ConvertRequest 错误: %v", err)
			}
			out, _ := io.ReadAll(result)

			var probe struct {
				Model string `json:"model"`
				Query string `json:"query"`
			}
			if err := json.Unmarshal(out, &probe); err != nil {
				t.Fatalf("响应不是合法 JSON: %v", err)
			}
			if probe.Model != tt.wantModel {
				t.Errorf("model = %q, want %q（body=%s）", probe.Model, tt.wantModel, out)
			}
			if probe.Query != "q" {
				t.Errorf("query 载荷被意外改写: %s", out)
			}
		})
	}
}

package zhipu

// 回归测试：responses 入站 × zhipu chat-only 上游——接管点在 adaptor 层（ConvertToOpenAI
// 内部）后，GLM 定制后处理（applyGLMCompatibility / injectStreamOptions / injectThinkingParams）
// 对 responses 入站照常执行（#12 专项）。

import (
	"context"
	"encoding/json"
	"io"
	"testing"

	"github.com/qianfree/team-api/relay/common"
	"github.com/qianfree/team-api/relay/constant"

	// blank import 触发内置转换器注册（relaykit 桥接为唯一路径，测试二进制须自备注册）
	_ "github.com/qianfree/team-api/relaykit/relayconvert/register"
)

const zhipuResponsesBody = `{"model":"glm-4.7","input":[{"type":"message","role":"user","content":[{"type":"input_text","text":"hi"}]}],"top_p":1.0,"stream":true}`

func zhipuResponsesInfo() *common.RelayInfo {
	return &common.RelayInfo{
		RelayMode:       int(constant.RelayModeResponses),
		IsStream:        true,
		InboundFormat:   constant.RelayFormatResponses,
		ClientFormat:    constant.RelayFormatResponses,
		OriginModelName: "glm-4.7",
		StreamStatus:    common.NewStreamStatus(),
		ChannelMeta: &common.ChannelMeta{
			ChannelType:       int(constant.ProviderZhipu),
			BaseURL:           "https://open.bigmodel.example.com",
			UpstreamModelName: "glm-4.7-upstream",
			IsModelMapped:     true,
		},
	}
}

// TestZhipuResponsesInbound_GLMCompatibility responses 入站经 r2o 转换后，
// GLM 兼容后处理照常执行：top_p≥1.0 裁到 0.99、stream_options 注入、模型映射。
func TestZhipuResponsesInbound_GLMCompatibility(t *testing.T) {
	a := &Adaptor{}
	out, err := a.ConvertRequest(context.Background(), zhipuResponsesInfo(), []byte(zhipuResponsesBody))
	if err != nil {
		t.Fatalf("ConvertRequest error: %v", err)
	}
	raw, _ := io.ReadAll(out)
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("converted body is not json: %v, body: %s", err, raw)
	}
	if _, ok := m["messages"]; !ok {
		t.Errorf("converted body missing messages（r2o 转换未生效）: %s", raw)
	}
	if got := string(m["top_p"]); got != "0.99" {
		t.Errorf("top_p = %s, want 0.99（applyGLMCompatibility 裁剪未执行）", got)
	}
	if _, ok := m["stream_options"]; !ok {
		t.Errorf("stream_options should be injected: %s", raw)
	}
	if got := string(m["model"]); got != `"glm-4.7-upstream"` {
		t.Errorf("model = %s, want glm-4.7-upstream（模型映射未执行）", got)
	}
}

// TestZhipuResponsesInbound_ThinkingDisabled -nothinking 后缀：
// injectThinkingParams 显式关闭思考。
func TestZhipuResponsesInbound_ThinkingDisabled(t *testing.T) {
	a := &Adaptor{}
	info := zhipuResponsesInfo()
	info.ThinkingDisabled = true
	out, err := a.ConvertRequest(context.Background(), info, []byte(zhipuResponsesBody))
	if err != nil {
		t.Fatalf("ConvertRequest error: %v", err)
	}
	raw, _ := io.ReadAll(out)
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("converted body is not json: %v, body: %s", err, raw)
	}
	var thinking struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(m["thinking"], &thinking); err != nil || thinking.Type != "disabled" {
		t.Errorf("thinking = %s (err=%v), want type=disabled（injectThinkingParams 未执行）", m["thinking"], err)
	}
}
